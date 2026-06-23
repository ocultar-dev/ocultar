package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/ocultar-dev/ocultar/apps/sombra/pkg/connector"
	"github.com/ocultar-dev/ocultar/apps/sombra/pkg/handler"
	"github.com/ocultar-dev/ocultar/apps/sombra/pkg/router"
	"github.com/ocultar-dev/ocultar/pkg/audit"
	"github.com/ocultar-dev/ocultar/pkg/config"
	"github.com/ocultar-dev/ocultar/pkg/inference"
	"github.com/ocultar-dev/ocultar/pkg/refinery"
	"github.com/ocultar-dev/ocultar/vault"
	"golang.org/x/crypto/hkdf"
)

const defaultSalt = "ocultar-v112-kdf-salt-fixed-16"

func getSalt() string {
	if s := os.Getenv("OCU_SALT"); s != "" {
		return s
	}
	return defaultSalt
}

func getMasterKey() []byte {
	keyMaterial := os.Getenv("OCU_MASTER_KEY")
	if keyMaterial == "" {
		log.Printf("[WARN] OCU_MASTER_KEY is not set — using insecure dev key.")
		keyMaterial = "default-dev-key-32-chars-long-!!!"
	}

	salt := []byte(getSalt())
	info := []byte("ocultar-aes-key")

	r := hkdf.New(sha256.New, []byte(keyMaterial), salt, info)
	derived := make([]byte, 32)
	if _, err := io.ReadFull(r, derived); err != nil {
		log.Fatalf("[FATAL] HKDF key derivation failed: %v", err)
	}
	return derived
}

func main() {
	fmt.Println("Booting Ocultar Sombra Gateway...")

	config.InitDefaults()
	config.Load()

	if config.Global.JWTSecret == "" {
		log.Printf("[WARN] OCU_JWT_SECRET is not set — Sombra is running in insecure dev mode. Any Bearer value is accepted as actor identity. Set OCU_JWT_SECRET in production.")
	}

	masterKey := getMasterKey()

	vaultPath := os.Getenv("OCU_VAULT_PATH")
	if vaultPath == "" {
		vaultPath = "sombra_vault.db"
	}

	v, err := vault.New(config.Settings{VaultBackend: "duckdb"}, vaultPath)
	if err != nil {
		log.Fatalf("Failed to initialize vault: %v", err)
	}
	defer v.Close()

	eng, err := refinery.NewRefinery(v, masterKey)
	if err != nil {
		log.Fatalf("Failed to initialize refinery: %v", err)
	}

	// Fail-closed by default: if the SLM sidecar is unreachable, block the request
	// rather than silently degrading to Tier 1-only detection (which cannot catch
	// names/addresses). Operators who knowingly want availability over completeness
	// can opt out explicitly.
	if os.Getenv("OCU_SOMBRA_ALLOW_DEGRADED_NER") == "true" {
		log.Printf("[WARN] OCU_SOMBRA_ALLOW_DEGRADED_NER is set — Sombra will degrade to Tier 1-only detection if the SLM sidecar is unavailable, instead of failing closed. Names/addresses may not be redacted during SLM outages.")
	} else {
		eng.FailClosedOnSLMError = true
	}

	// Initialize Tier 2 SLM Scanner if sidecar URL is configured
	sidecarURL := os.Getenv("SLM_SIDECAR_URL")
	if sidecarURL == "" {
		sidecarURL = "http://localhost:8085"
	}
	scanner, err := inference.NewRemoteScanner(sidecarURL)
	if err != nil {
		log.Fatalf("[FATAL] %v", err)
	}
	eng.SetAIScanner(scanner)
	log.Printf("[INFO] Tier 2 AI active via SLM sidecar: %s", sidecarURL)

	// Setup Multi-Model Router
	allowedDomains := []string{"generativelanguage.googleapis.com", "api.openai.com", "api.mistral.ai", "api.anthropic.com", "127.0.0.1"}
	r := router.New("gemini-flash-latest", allowedDomains)

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
		log.Printf("[INFO] Demo mode: mock-ai registered at %s", mockURL)
	}

	// Initialize Immutable Audit Logger
	auditor, err := audit.NewImmutableLogger("sombra_audit.log")
	if err != nil {
		log.Printf("[WARN] Failed to initialize Immutable Logger: %v", err)
	} else {
		log.Printf("[INFO] Immutable Audit Log active. PubKey: %s", auditor.PublicKeyHex())
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
					log.Printf("[WARN] retention sweep error: %v", err)
					return
				}
				if deleted > 0 {
					log.Printf("[INFO] retention sweep purged %d expired vault token(s)", deleted)
				}
			},
		)
	}

	g := handler.NewGateway(eng, v, masterKey, r, auditor)

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

	log.Printf("[INFO] Sombra Gateway running on http://localhost:%s", port)

	srv := &http.Server{
		Addr:              ":" + port,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
