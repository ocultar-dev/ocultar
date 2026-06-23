package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/ocultar-dev/ocultar/apps/sombra/pkg/metrics"
	"github.com/ocultar-dev/ocultar/apps/sombra/pkg/router"
	"github.com/ocultar-dev/ocultar/pkg/config"
	"github.com/ocultar-dev/ocultar/pkg/proxy"
)

// OpenAIChatCompletionRequest is the standard OpenAI /v1/chat/completions request shape.
type OpenAIChatCompletionRequest struct {
	Model       string           `json:"model"`
	Messages    []router.Message `json:"messages"`
	Stream      bool             `json:"stream"`
	Temperature float64          `json:"temperature,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
}

func newCompletionID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "chatcmpl-" + hex.EncodeToString(b)
}

// HandleV1ChatCompletions is a drop-in replacement for the OpenAI
// /v1/chat/completions endpoint. Point any OpenAI-compatible SDK at Sombra
// (OPENAI_BASE_URL=http://sombra-host/v1) and every message is scrubbed
// before it leaves the building.
//
// Flow: scrub each message → route to requested model → rehydrate response
// → return standard OpenAI JSON (or real SSE token stream if stream:true).
func (g *Gateway) HandleV1ChatCompletions(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	w = rec
	defer func() {
		metrics.RequestsTotal.WithLabelValues("chat_completions", strconv.Itoa(rec.status)).Inc()
		metrics.RequestLatency.WithLabelValues("chat_completions").Observe(time.Since(start).Seconds())
	}()

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	actor := g.extractActor(r)
	if actor == "" {
		http.Error(w, "unauthorized: invalid or missing token", http.StatusUnauthorized)
		return
	}

	var req OpenAIChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	if req.Model == "" {
		http.Error(w, "model is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// 1. PII redaction — every message content scrubbed independently.
	for i, msg := range req.Messages {
		if msg.Content == "" {
			continue
		}
		prescrubbed, err := g.scrubber.Prescrub(msg.Content)
		if err != nil {
			http.Error(w, fmt.Sprintf("pre-scrub failed on message %d: %v", i, err), http.StatusInternalServerError)
			return
		}
		redacted, err := g.eng.RefineString(prescrubbed, actor, nil)
		if err != nil {
			metrics.FailClosedTotal.WithLabelValues("redaction_chat_completions").Inc()
			http.Error(w, fmt.Sprintf("redaction failed on message %d: %v", i, err), http.StatusInternalServerError)
			return
		}
		req.Messages[i].Content = redacted
	}

	opts := router.ModelOpts{
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}
	completionID := newCompletionID()
	created := time.Now().Unix()

	// 2a. Streaming path.
	if req.Stream {
		g.handleStreamingResponse(w, r, actor, ctx, req, opts, completionID, created)
		return
	}

	// 2b. Buffered path.
	aiResp, err := g.router.Send(ctx, req.Model, req.Messages, opts)
	if err != nil {
		http.Error(w, fmt.Sprintf("model routing failed: %v", err), http.StatusBadGateway)
		return
	}

	rehydrated, err := proxy.RehydrateString(g.vault, g.masterKey, aiResp)
	if err != nil {
		metrics.RehydrationFailuresTotal.WithLabelValues("chat_completions").Inc()
		if g.auditor != nil {
			g.auditor.Log(actor, "PROXY_CHAT_COMPLETION", req.Model, "FAILED", "rehydration error")
		}
		if !config.Global.RehydrateFallbackEnabled {
			http.Error(w, fmt.Sprintf("rehydration failed: %v", err), http.StatusInternalServerError)
			return
		}
		log.Printf("[WARN] Re-hydration failed, falling back to tokenized response (Safety: ON)")
		rehydrated = aiResp // Return tokens instead of leaking data or failing
	}

	if g.auditor != nil {
		g.auditor.Log(actor, "PROXY_CHAT_COMPLETION", req.Model, "SUCCESS",
			fmt.Sprintf("messages=%d stream=false", len(req.Messages)))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":      completionID,
		"object":  "chat.completion",
		"created": created,
		"model":   req.Model,
		"choices": []map[string]interface{}{
			{
				"index":         0,
				"message":       map[string]interface{}{"role": "assistant", "content": rehydrated},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0,
		},
	})
}

// handleStreamingResponse streams the model response back to the client as
// OpenAI-compatible SSE. Each upstream text chunk is passed through the
// streamRehydrator before being forwarded — vault tokens that span chunk
// boundaries are held in the rehydrator buffer until they are complete.
func (g *Gateway) handleStreamingResponse(
	w http.ResponseWriter,
	r *http.Request,
	actor string,
	ctx interface{ Done() <-chan struct{} },
	req OpenAIChatCompletionRequest,
	opts router.ModelOpts,
	id string,
	created int64,
) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, canFlush := w.(http.Flusher)

	emit := func(content string) {
		if content == "" {
			return
		}
		b, _ := json.Marshal(map[string]interface{}{
			"id":      id,
			"object":  "chat.completion.chunk",
			"created": created,
			"model":   req.Model,
			"choices": []map[string]interface{}{
				{"index": 0, "delta": map[string]interface{}{"content": content}, "finish_reason": nil},
			},
		})
		fmt.Fprintf(w, "data: %s\n\n", b)
		if canFlush {
			flusher.Flush()
		}
	}

	rehy := newStreamRehydrator(g.vault, g.masterKey)

	// Use context from the original HTTP request.
	httpCtx := r.Context()

	streamErr := g.router.SendStream(httpCtx, req.Model, req.Messages, opts, func(delta string) error {
		safe, err := rehy.Push(delta)
		if err != nil {
			metrics.RehydrationFailuresTotal.WithLabelValues("chat_completions_stream").Inc()
			if !config.Global.RehydrateFallbackEnabled {
				return err
			}
			// Safety: ON — emit the best-effort output (failed tokens left
			// in raw [TYPE_...] form) instead of aborting the stream.
		}
		emit(safe)
		return nil
	})

	// Flush any token held at the end of the stream.
	remaining, flushErr := rehy.Flush()
	if flushErr != nil {
		metrics.RehydrationFailuresTotal.WithLabelValues("chat_completions_stream").Inc()
	}
	if flushErr == nil || config.Global.RehydrateFallbackEnabled {
		emit(remaining)
	}
	if flushErr != nil && !config.Global.RehydrateFallbackEnabled && streamErr == nil {
		streamErr = flushErr
	}

	if streamErr != nil {
		if g.auditor != nil {
			// Log "stream error" without the raw upstream message — provider errors
			// may echo PII token IDs, which must not appear in the immutable audit chain.
			g.auditor.Log(actor, "PROXY_CHAT_COMPLETION", req.Model, "FAILED", "stream error")
		}
		// Headers already sent; signal error in-band as a final chunk.
		b, _ := json.Marshal(map[string]interface{}{
			"id": id, "object": "chat.completion.chunk",
			"choices": []map[string]interface{}{
				{"index": 0, "delta": map[string]interface{}{"content": "[stream error]"}, "finish_reason": "stop"},
			},
		})
		fmt.Fprintf(w, "data: %s\n\n", b)
	} else {
		// Stop chunk.
		b, _ := json.Marshal(map[string]interface{}{
			"id":      id,
			"object":  "chat.completion.chunk",
			"created": created,
			"model":   req.Model,
			"choices": []map[string]interface{}{
				{"index": 0, "delta": map[string]interface{}{}, "finish_reason": "stop"},
			},
		})
		fmt.Fprintf(w, "data: %s\n\n", b)

		if g.auditor != nil {
			g.auditor.Log(actor, "PROXY_CHAT_COMPLETION", req.Model, "SUCCESS",
				fmt.Sprintf("messages=%d stream=true", len(req.Messages)))
		}
	}

	fmt.Fprintf(w, "data: [DONE]\n\n")
	if canFlush {
		flusher.Flush()
	}
}
