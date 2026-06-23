package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/ocultar-dev/ocultar/apps/slm-engine/pkg/inference"
)

var scanner inference.Tier2Engine

// initLogging configures slog's default logger as structured JSON, with the
// level controlled by OCU_LOG_LEVEL (debug|info|warn|error, default info).
func initLogging() {
	level := new(slog.LevelVar)
	level.Set(slog.LevelInfo)
	switch strings.ToLower(os.Getenv("OCU_LOG_LEVEL")) {
	case "debug":
		level.Set(slog.LevelDebug)
	case "warn":
		level.Set(slog.LevelWarn)
	case "error":
		level.Set(slog.LevelError)
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
}

// fatalf logs an error at Error level and exits — slog has no built-in
// fatal-and-exit, so this restores the log.Fatalf call sites it replaces.
func fatalf(msg string, args ...any) {
	slog.Error(msg, args...)
	os.Exit(1)
}

func handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	text := req["text"]
	if text == "" {
		json.NewEncoder(w).Encode(map[string][]string{})
		return
	}

	res, err := scanner.ScanForPII(text)
	if err != nil {
		http.Error(w, fmt.Sprintf("scan failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func main() {
	initLogging()
	var err error

	// SLM_ADAPTER selects the NER backend protocol (TIER2_ENGINE is a deprecated alias).
	adapter := os.Getenv("SLM_ADAPTER")
	if adapter == "" {
		if legacy := os.Getenv("TIER2_ENGINE"); legacy != "" {
			slog.Warn("[DEPRECATED] TIER2_ENGINE renamed to SLM_ADAPTER, please update your config")
			adapter = legacy
		}
	}
	switch adapter {
	case "privacy-filter", "":
		// default
		sidecarURL := os.Getenv("PYTHON_SIDECAR_URL")
		if old := os.Getenv("PRIVACY_FILTER_URL"); old != "" {
			slog.Warn("[DEPRECATED] PRIVACY_FILTER_URL renamed to PYTHON_SIDECAR_URL, please update your config")
			sidecarURL = old
		}
		if sidecarURL == "" {
			fatalf("PYTHON_SIDECAR_URL not set")
		}
		modelPath := os.Getenv("PRIVACY_FILTER_MODEL_PATH")
		if modelPath == "" {
			modelPath = "openai/privacy-filter"
		}
		scanner, err = inference.NewPrivacyFilterEngine(sidecarURL, modelPath)
		if err != nil {
			fatalf("failed to initialize privacy-filter scanner", "error", err)
		}
	default:
		fatalf("unsupported SLM_ADAPTER (slm-engine only supports \"privacy-filter\"; openai-chat/llama.cpp is handled by the refinery directly)", "adapter", adapter)
	}
	defer scanner.Close()

	http.HandleFunc("/scan", handleScan)
	http.HandleFunc("/health", handleHealth)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8085"
	}
	slog.Info("SLM sidecar running", "port", port)
	fatalf("server failed", "error", http.ListenAndServe(":"+port, nil))
}
