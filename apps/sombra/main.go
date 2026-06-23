package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ocultar-dev/ocultar/apps/sombra/pkg/connector"
	"github.com/ocultar-dev/ocultar/apps/sombra/pkg/handler"
	"github.com/ocultar-dev/ocultar/apps/sombra/pkg/router"
	"github.com/ocultar-dev/ocultar/pkg/audit"
	"github.com/ocultar-dev/ocultar/pkg/config"
	"github.com/ocultar-dev/ocultar/pkg/inference"
	"github.com/ocultar-dev/ocultar/pkg/refinery"
	"github.com/ocultar-dev/ocultar/vault"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/crypto/hkdf"
)

const defaultSalt = "ocultar-v112-kdf-salt-fixed-16"

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

func getSalt() string {
	if s := os.Getenv("OCU_SALT"); s != "" {
		return s
	}
	return defaultSalt
}

func getMasterKey() []byte {
	keyMaterial := os.Getenv("OCU_MASTER_KEY")
	if keyMaterial == "" {
		slog.Warn("OCU_MASTER_KEY not set, using insecure dev key")
		keyMaterial = "default-dev-key-32-chars-long-!!!"
	}

	salt := []byte(getSalt())
	info := []byte("ocultar-aes-key")

	r := hkdf.New(sha256.New, []byte(keyMaterial), salt, info)
	derived := make([]byte, 32)
	if _, err := io.ReadFull(r, derived); err != nil {
		fatalf("HKDF key derivation failed", "error", err)
	}
	return derived
}

func main() {
	initLogging()
	fmt.Println("Booting Ocultar Sombra Gateway...")

	config.InitDefaults()
	config.Load()

	if config.Global.JWTSecret == "" {
		slog.Warn("OCU_JWT_SECRET not set — Sombra running in insecure dev mode (any Bearer value accepted as actor identity)")
	}

	masterKey := getMasterKey()

	vaultPath := os.Getenv("OCU_VAULT_PATH")
	if vaultPath == "" {
		vaultPath = "sombra_vault.db"
	}

	v, err := vault.New(config.Settings{VaultBackend: "duckdb"}, vaultPath)
	if err != nil {
		fatalf("failed to initialize vault", "error", err)
	}
	defer v.Close()

	eng, err := refinery.NewRefinery(v, masterKey)
	if err != nil {
		fatalf("failed to initialize refinery", "error", err)
	}

	// Fail-closed by default: if the SLM sidecar is unreachable, block the request
	// rather than silently degrading to Tier 1-only detection (which cannot catch
	// names/addresses). Operators who knowingly want availability over completeness
	// can opt out explicitly.
	if os.Getenv("OCU_SOMBRA_ALLOW_DEGRADED_NER") == "true" {
		slog.Warn("OCU_SOMBRA_ALLOW_DEGRADED_NER is set — degrading to Tier 1-only detection instead of failing closed if the SLM sidecar is unavailable")
	} else {
		eng.FailClosedOnSLMError = true
	}

	// Initialize Tier 2 SLM Scanner — endpoint configured via configs/config.yaml
	// (slm_sidecar_url), defaulting to SLM_SIDECAR_URL/localhost:8085.
	sidecarURL := config.Global.SLMSidecarURL
	scanner, err := inference.NewRemoteScanner(sidecarURL)
	if err != nil {
		fatalf(err.Error())
	}
	eng.SetAIScanner(scanner)
	slog.Info("Tier 2 AI active via SLM sidecar", "sidecar_url", sidecarURL)

	// Setup Multi-Model Router — allowlist configured via configs/config.yaml
	// (sombra_allowed_domains); adding a new provider doesn't require a rebuild.
	r := router.New("gemini-flash-latest", config.Global.SombraAllowedDomains)

	gemini := router.NewGemini("gemini-flash-latest", "", "GEMINI_API_KEY", "gemini-2.0-flash")
	r.Register(gemini)

	openai := router.NewOpenAI("gpt-4o", "", "OPENAI_API_KEY")
	r.Register(openai)
	openaiMini := router.NewOpenAI("gpt-4o-mini", "", "OPENAI_API_KEY")
	r.Register(openaiMini)

	mistral := router.NewOpenAI("mistral-large-latest", "https://api.mistral.ai/v1", "MISTRAL_API_KEY")
	r.Register(mistral)

	claude := router.NewClaude("claude-sonnet-4-6", "", "ANTHROPIC_API_KEY")
	r.Register(claude)

	if mockURL := os.Getenv("SOMBRA_MOCK_AI_URL"); mockURL != "" {
		r.Register(router.NewLocal("mock-ai", mockURL))
		slog.Info("demo mode: mock-ai registered", "mock_url", mockURL)
	}

	// Initialize Immutable Audit Logger
	auditor, err := audit.NewImmutableLogger("sombra_audit.log")
	if err != nil {
		slog.Warn("failed to initialize immutable logger", "error", err)
	} else {
		slog.Info("immutable audit log active", "public_key", auditor.PublicKeyHex())
		defer auditor.Close()
		if config.Global.RetentionEnabled {
			auditor.SetRotation(
				int64(config.Global.AuditLogMaxSizeMB)*1024*1024,
				time.Duration(config.Global.AuditLogArchiveRetentionDays)*24*time.Hour,
			)
		}
	}

	// GDPR Art. 5(1)(e) storage limitation: periodically purge vault rows
	// older than VaultRetentionDays. Never touches the Entity Registry.
	if config.Global.RetentionEnabled {
		retentionCtx, cancelRetention := context.WithCancel(context.Background())
		defer cancelRetention()
		go vault.RunRetentionLoop(
			retentionCtx,
			v,
			time.Duration(config.Global.RetentionSweepMinutes)*time.Minute,
			time.Duration(config.Global.VaultRetentionDays)*24*time.Hour,
			func(deleted int64, err error) {
				if err != nil {
					slog.Warn("retention sweep error", "error", err)
					return
				}
				if deleted > 0 {
					slog.Info("retention sweep purged expired vault tokens", "count", deleted)
				}
			},
		)
	}

	g, err := handler.NewGateway(eng, v, masterKey, r, auditor)
	if err != nil {
		fatalf("failed to initialize gateway", "error", err)
	}

	filePolicy := connector.DataPolicy{
		AllowedModels: []string{"gemini-flash-latest", "local-slm", "gpt-4o", "gpt-4o-mini", "mistral-large-latest", "claude-sonnet-4-6"},
		MaxBodyBytes:  10485760,
	}
	g.RegisterConnector(connector.NewFileConnector("file", filePolicy))

	port := os.Getenv("SOMBRA_PORT")
	if port == "" {
		port = "8086"
	}

	http.HandleFunc("/query", g.HandleQuery)
	http.HandleFunc("/v1/chat/completions", g.HandleV1ChatCompletions)
	http.HandleFunc("/v1/slack/events", g.HandleSlackEvent)

	http.HandleFunc("/v1/entities", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			g.HandleEntityRegister(w, r)
		case http.MethodGet:
			g.HandleEntityList(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	http.HandleFunc("/v1/entities/seed", g.HandleEntitySeed)

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status": "ok"}`)
	})

	http.Handle("/metrics", promhttp.Handler())

	slog.Info("Sombra Gateway running", "addr", "http://localhost:"+port)

	srv := &http.Server{
		Addr:              ":" + port,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		fatalf("failed to start server", "error", err)
	}
}
