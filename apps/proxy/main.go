// Package main is the entry-point for the OCULTAR transparent HTTP proxy.
package main

import (
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ocultar-dev/ocultar/pkg/audit"
	"github.com/ocultar-dev/ocultar/pkg/config"
	"github.com/ocultar-dev/ocultar/pkg/proxy"
	"github.com/ocultar-dev/ocultar/pkg/refinery"
	"github.com/ocultar-dev/ocultar/pkg/inference"
	"github.com/ocultar-dev/ocultar/vault"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/crypto/hkdf"
)

const VERSION = "1.1.0"

const defaultSalt = "ocultar-v112-kdf-salt-fixed-16"

var devMode bool

func init() {
	flag.BoolVar(&devMode, "dev", false, "Enable development mode (allows insecure defaults)")
}

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
	slog.Error(msg, args...) //nolint:gosec // intentional wrapper
	os.Exit(1)
}

// auditAdapter bridges audit.ImmutableLogger to the refinery.AuditLogger interface.
type auditAdapter struct {
	logger *audit.ImmutableLogger
}

func (a *auditAdapter) Init(_ string) error { return nil }
func (a *auditAdapter) Close()              { a.logger.Close() }
func (a *auditAdapter) Log(actor, action, resource, mapping string) {
	if err := a.logger.Log(actor, action, resource, "ALLOW", mapping); err != nil {
		slog.Warn("audit write failed", "error", err)
	}
}

func getSalt() string {
	s := os.Getenv("OCU_SALT")
	if s == "" {
		if !devMode {
			fatalf("OCU_SALT is missing; production environments must define a unique OCU_SALT")
		}
		slog.Warn("OCU_SALT not set, using built-in default salt (allowed only in --dev mode)")
		return defaultSalt
	}
	return s
}

func getMasterKey() []byte {
	keyMaterial := os.Getenv("OCU_MASTER_KEY")
	if keyMaterial == "" {
		if !devMode {
			fatalf("OCU_MASTER_KEY is missing; production environments must define a high-entropy master key")
		}
		slog.Warn("OCU_MASTER_KEY not set, using insecure dev key (allowed only in --dev mode)")
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
	flag.Parse()
	initLogging()
	slog.Info("OCULTAR Privacy Proxy starting", "version", VERSION, "dev_mode", devMode)

	cfg := proxy.LoadConfig()
	masterKey := getMasterKey()
	config.Load()

	vaultProvider, err := vault.New(config.Global, cfg.VaultPath)
	if err != nil {
		fatalf("failed to open vault", "error", err)
	}
	defer vaultProvider.Close()

	eng, err := refinery.NewRefinery(vaultProvider, masterKey)
	if err != nil {
		fatalf("failed to initialize refinery", "error", err)
	}
	eng.Serve = "proxy"

	// Immutable audit log — active when OCU_AUDIT_PRIVATE_KEY is set.
	auditActive := false
	var immutableLog *audit.ImmutableLogger
	if keyHex := os.Getenv("OCU_AUDIT_PRIVATE_KEY"); keyHex != "" {
		privKey, err := audit.LoadPrivateKeyFromHex(keyHex)
		if err != nil {
			fatalf(err.Error())
		}
		logPath := os.Getenv("OCU_AUDIT_LOG_PATH")
		if logPath == "" {
			logPath = filepath.Join(filepath.Dir(cfg.VaultPath), "audit.log")
		}
		immutableLog, err = audit.NewImmutableLoggerWithKey(logPath, privKey)
		if err != nil {
			fatalf("failed to open audit log", "path", logPath, "error", err)
		}
		defer immutableLog.Close()
		if config.Global.RetentionEnabled {
			immutableLog.SetRotation(
				int64(config.Global.AuditLogMaxSizeMB)*1024*1024,
				time.Duration(config.Global.AuditLogArchiveRetentionDays)*24*time.Hour,
			)
		}
		eng.SetAuditLogger(&auditAdapter{logger: immutableLog})
		auditActive = true
		slog.Info("immutable audit log active", "path", logPath, "public_key", immutableLog.PublicKeyHex()) //nolint:gosec // intentional
	} else {
		eng.SetAuditLogger(&refinery.NoopAuditLogger{})
	}

	// GDPR Art. 5(1)(e) storage limitation: periodically purge vault rows
	// older than VaultRetentionDays. Never touches the Entity Registry.
	if config.Global.RetentionEnabled {
		retentionCtx, cancelRetention := context.WithCancel(context.Background())
		defer cancelRetention()
		go vault.RunRetentionLoop(
			retentionCtx,
			vaultProvider,
			time.Duration(config.Global.RetentionSweepMinutes)*time.Minute,
			time.Duration(config.Global.VaultRetentionDays)*24*time.Hour,
			func(deleted int64, err error) {
				if err != nil {
					eng.AuditLogger.Log("system", "retention_sweep", "N/A", err.Error())
					return
				}
				if deleted > 0 {
					eng.AuditLogger.Log("system", "retention_sweep", "N/A", fmt.Sprintf("purged %d expired vault token(s)", deleted))
				}
			},
		)
	}

	// Tier 2 AI NER — endpoint configured via configs/config.yaml (slm_sidecar_url),
	// defaulting to SLM_SIDECAR_URL/localhost:8085.
	sidecarURL := config.Global.SLMSidecarURL
	scanner, err := inference.NewRemoteScanner(sidecarURL)
	if err != nil {
		fatalf(err.Error())
	}
	eng.SetAIScanner(scanner)
	slog.Info("Tier 2 AI (remote sidecar) configured", "sidecar_url", sidecarURL)

	handler, err := proxy.NewHandler(eng, vaultProvider, masterKey, cfg.TargetURL)
	if err != nil {
		fatalf(err.Error())
	}
	if immutableLog != nil {
		handler.SetAuditLogger(immutableLog)
	}

	addr := ":" + cfg.Port
	if cfg.Port != "" && cfg.Port[0] == ':' {
		addr = cfg.Port
	}

	fmt.Printf("┌──────────────────────────────────────────────────────┐\n")
	fmt.Printf("│  OCULTAR Privacy Proxy v%-27s │\n", VERSION)
	fmt.Printf("│  Listening : http://localhost%-24s │\n", addr)
	fmt.Printf("│  Target    : %-38s │\n", cfg.TargetURL)
	fmt.Printf("│  Vault     : %-38s │\n", cfg.VaultPath)
	fmt.Printf("│  Metrics   : http://localhost%-23s │\n", addr+"/metrics")
	fmt.Printf("│  Audit log : %-38v │\n", auditActive)
	fmt.Printf("│  DevMode   : %-38v │\n", devMode)
	fmt.Printf("└──────────────────────────────────────────────────────┘\n")

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok", "version":"` + VERSION + `"}`))
	})

	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/", handler)

	if err := http.ListenAndServe(addr, mux); err != nil {
		fatalf("server exited", "error", err)
	}
}
