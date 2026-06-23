package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ocultar-dev/ocultar/internal/handlers"
	"github.com/ocultar-dev/ocultar/pkg/config"
	"github.com/ocultar-dev/ocultar/pkg/connector"
	_ "github.com/ocultar-dev/ocultar/pkg/connector/slack"
	"github.com/ocultar-dev/ocultar/pkg/identities"
	"github.com/ocultar-dev/ocultar/pkg/inference"
	"github.com/ocultar-dev/ocultar/pkg/recon"
	"github.com/ocultar-dev/ocultar/pkg/refinery"
	"github.com/ocultar-dev/ocultar/pkg/reporter"
	"github.com/ocultar-dev/ocultar/vault"
	"golang.org/x/crypto/hkdf"
)

const VERSION = "1.14"

const defaultSalt = "ocultar-v112-kdf-salt-fixed-16"
var startTime = time.Now()

// initLogging configures slog's default logger as structured JSON, with the
// level controlled by OCU_LOG_LEVEL (debug|info|warn|error, default info).
// This also redirects the standard log package's output (used throughout
// this file and its dependencies) through the same JSON handler — see
// https://pkg.go.dev/log/slog#SetDefault.
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


// getSalt retrieves the cryptographic salt from the environment or falls back to a default value.
func getSalt() string {
	if s := os.Getenv("OCU_SALT"); s != "" {
		return s
	}
	log.Printf("[WARN] OCU_SALT is not set — using built-in default salt. Set OCU_SALT in production.")
	return defaultSalt
}

// getMasterKey derives the 32-byte AES master key using HKDF based on the environment-provided key material.
func getMasterKey() []byte {
	keyMaterial := os.Getenv("OCU_MASTER_KEY")
	if keyMaterial == "" {
		log.Fatalf("[FATAL] OCU_MASTER_KEY is required but not set.")
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

// main is the entry point for the OCULTAR Refinery Refinery CLI and HTTP server.
type BasicFileLogger struct {
	mu            sync.Mutex
	path          string
	maxSizeBytes  int64
	archiveMaxAge time.Duration
}

func (l *BasicFileLogger) Init(path string) error {
	l.path = path
	return nil
}

// SetRotation configures size-based rotation for this plain JSON audit log.
// Unlike ImmutableLogger's entries, these aren't hash-chained, so rotation is
// a plain rename-and-purge with no checkpoint event needed. maxSizeBytes <= 0
// (the default) disables rotation, preserving today's unbounded-growth
// behavior for operators who opt out via RetentionEnabled=false.
func (l *BasicFileLogger) SetRotation(maxSizeBytes int64, archiveMaxAge time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.maxSizeBytes = maxSizeBytes
	l.archiveMaxAge = archiveMaxAge
}

func (l *BasicFileLogger) Log(user, action, result, mapping string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.rotateIfNeeded()

	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600) //nolint:gosec // G703: path is operator-configured at startup, not derived from user input
	if err != nil {
		return
	}
	defer f.Close() //nolint:errcheck

	entry := map[string]string{
		"timestamp":          time.Now().UTC().Format(time.RFC3339),
		"user":               user,
		"action":             action,
		"result":             result,
		"compliance_mapping": mapping,
	}
	bytes, _ := json.Marshal(entry)
	if _, err := f.Write(append(bytes, '\n')); err != nil {
		log.Printf("[AUDIT] failed to write audit entry: %v", err)
	}
}

// rotateIfNeeded archives the current log file once it exceeds
// l.maxSizeBytes, then purges archives older than l.archiveMaxAge. Callers
// must hold l.mu.
func (l *BasicFileLogger) rotateIfNeeded() {
	if l.maxSizeBytes <= 0 {
		return
	}

	info, err := os.Stat(l.path)
	if err != nil || info.Size() < l.maxSizeBytes {
		return
	}

	archivePath := fmt.Sprintf("%s.%s.archived", l.path, time.Now().UTC().Format("20060102T150405.000000000"))
	if err := os.Rename(l.path, archivePath); err != nil {
		log.Printf("[AUDIT] log rotation rename failed: %v", err)
		return
	}

	l.purgeOldArchives()
}

// purgeOldArchives deletes rotated archive files older than l.archiveMaxAge.
// Callers must hold l.mu.
func (l *BasicFileLogger) purgeOldArchives() {
	if l.archiveMaxAge <= 0 {
		return
	}

	matches, err := filepath.Glob(l.path + ".*.archived")
	if err != nil {
		return
	}

	cutoff := time.Now().Add(-l.archiveMaxAge)
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(m) //nolint:errcheck
		}
	}
}

func (l *BasicFileLogger) Close() {}

func main() {
	initLogging()
	mime.AddExtensionType(".js", "application/javascript") //nolint:errcheck
	showVersion := flag.Bool("version", false, "Print the OCULTAR refinery version and exit")
	showVersionShort := flag.Bool("v", false, "Print the OCULTAR refinery version and exit (alias)")
	dryRun := flag.Bool("dry-run", false, "Scan for PII without writing to vault; output JSON report")
	report := flag.Bool("report", false, "Produce redacted output AND append a JSON PII report to stderr")
	serve := flag.String("serve", "", "Start local HTTP dashboard on given PORT (e.g. '8080')")
	complianceReport := flag.String("compliance-report", "", "Generate HTML compliance report from the given audit log file")
	complianceOutput := flag.String("report-out", "report.html", "Output path for the HTML compliance report")
	reconPath := flag.String("recon", "", "Run the Data Recon Crawler on a local directory")
	flag.Parse()

	if *showVersion || *showVersionShort {
		fmt.Printf("OCULTAR Refinery Refinery v%s\n", VERSION)
		os.Exit(0)
	}

	if *complianceReport != "" {
		rep := reporter.New()
		err := rep.GenerateHTMLReport(*complianceReport, *complianceOutput)
		if err != nil {
			log.Fatalf("Failed to generate compliance report: %v", err)
		}
		fmt.Printf("Successfully generated compliance report at: %s\n", *complianceOutput)
		os.Exit(0)
	}

	masterKey := getMasterKey()

	// Load config before opening the vault so that VaultBackend / PostgresDSN are available.
	config.Load()

	// Determine the vault path (DuckDB only; ignored for postgres backend).
	vaultPath := os.Getenv("OCU_VAULT_PATH")
	if vaultPath == "" {
		vaultPath = "vault.db"
	}
	if *dryRun {
		vaultPath = ":memory:"
	}

	// Open the vault using the provider selected by configuration.
	vaultProvider, err := vault.New(config.Global, vaultPath)
	if err != nil {
		log.Fatal("Failed to open vault: ", err)
	}
	defer vaultProvider.Close() //nolint:errcheck

	eng, err := refinery.NewRefinery(vaultProvider, masterKey)
	if err != nil {
		log.Fatal("Failed to initialize refinery: ", err)
	}
	eng.DryRun = *dryRun
	eng.Report = *report

	// Enable basic logging for dashboard visibility
	basicLogger := &BasicFileLogger{path: "audit.log"}
	if config.Global.RetentionEnabled {
		basicLogger.SetRotation(
			int64(config.Global.AuditLogMaxSizeMB)*1024*1024,
			time.Duration(config.Global.AuditLogArchiveRetentionDays)*24*time.Hour,
		)
	}
	eng.AuditLogger = basicLogger

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
					basicLogger.Log("system", "retention_sweep", "ERROR", err.Error())
					return
				}
				if deleted > 0 {
					basicLogger.Log("system", "retention_sweep", "OK", fmt.Sprintf("purged %d expired vault token(s)", deleted))
				}
			},
		)
	}

	// SLM_ADAPTER selects the Tier 2 NER backend protocol:
	//   "openai-chat"    → QwenScanner  (llama.cpp /v1/chat/completions)
	//   "privacy-filter" → RemoteScanner (privacy-filter / piiranha sidecar) [default]
	// TIER2_ENGINE is a deprecated alias; SLM_ADAPTER takes precedence when set.
	var scanner refinery.AIScanner
	adapter := os.Getenv("SLM_ADAPTER")
	if adapter == "" {
		if legacy := os.Getenv("TIER2_ENGINE"); legacy != "" {
			log.Printf("[DEPRECATED] TIER2_ENGINE renamed to SLM_ADAPTER. Please update your config.")
			switch legacy {
			case "llama-cpp", "qwen":
				adapter = "openai-chat"
			default:
				adapter = "privacy-filter"
			}
		}
	}
	sidecarURL := os.Getenv("SLM_SIDECAR_URL")
	if sidecarURL == "" {
		sidecarURL = "http://localhost:8085"
	}
	switch adapter {
	case "openai-chat":
		scanner = inference.NewQwenScanner(sidecarURL)
		log.Printf("[INFO] Tier 2 AI active via openai-chat (Qwen/llama.cpp): %s", sidecarURL) //nolint:gosec // G706: sidecarURL is an operator-configured value, not user input
		eng.SetAIScanner(scanner)
	case "none", "disabled":
		log.Printf("[INFO] Tier 2 AI deactivated (NoopAIScanner active)")
	default:
		remoteScanner, err := inference.NewRemoteScanner(sidecarURL)
		if err != nil {
			log.Fatalf("[FATAL] %v", err)
		}
		scanner = remoteScanner
		log.Printf("[INFO] Tier 2 AI active via privacy-filter sidecar: %s", sidecarURL) //nolint:gosec // G706: sidecarURL is an operator-configured value, not user input
		eng.SetAIScanner(scanner)
	}

	// Register per-domain sidecars from config (e.g. fr-finance → http://localhost:8087).
	for domain, sidecarURL := range config.Global.Tier2DomainSidecars {
		ds, err := inference.NewRemoteScanner(sidecarURL)
		if err != nil {
			log.Fatalf("[FATAL] %v", err)
		}
		eng.SetDomainScanner(domain, ds)
	}
	if len(config.Global.Tier2DomainSidecars) > 0 {
		log.Printf("[INFO] Active domain: '%s'", config.Global.DomainSnapshot)
	}
	eng.AIScanner.SetDomain(config.Global.DomainSnapshot)

	if *reconPath != "" {
		c := recon.NewCrawler(eng)
		heatmap, err := c.CrawlLocalDirectory(*reconPath)
		if err != nil {
			log.Fatalf("[FATAL] Recon Crawler failed: %v", err)
		}
		fmt.Println(heatmap.ToJSON())
		os.Exit(0)
	}

	// Initialize and start connectors
	cm := connector.NewManager(eng)
	if os.Getenv("SLACK_TOKEN") != "" {
		slackCfg := map[string]interface{}{
			"id":           "slack-default",
			"token":        os.Getenv("SLACK_TOKEN"),
			"workspace_id": os.Getenv("SLACK_WORKSPACE_ID"),
		}
		if err := cm.LoadAndStart("slack-default", "slack", slackCfg); err != nil {
			log.Printf("[ERROR] Failed to start Slack connector: %v", err)
		}
	}

	if os.Getenv("MS_CLIENT_ID") != "" {
		spCfg := map[string]interface{}{
			"id":            "sharepoint-default",
			"tenant_id":     os.Getenv("MS_TENANT_ID"),
			"client_id":     os.Getenv("MS_CLIENT_ID"),
			"client_secret": os.Getenv("MS_CLIENT_SECRET"),
			"site_id":       os.Getenv("MS_SHAREPOINT_SITE_ID"),
		}
		if err := cm.LoadAndStart("sharepoint-default", "sharepoint-graph", spCfg); err != nil {
			log.Printf("[ERROR] Failed to start SharePoint connector: %v", err)
		}
	}

	// Phase B: Start Live CRM/LDAP sync daemon
	identities.StartSyncWorker()

	if *serve != "" {
		eng.Serve = *serve
		startServer(eng, *serve)
		return
	}

	inputData, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal("Failed to read input: ", err)
	}

	if len(inputData) == 0 {
		if *dryRun {
			printReport(eng, 0)
		}
		return
	}

	actor := "cli-user"
	var jsonRaw interface{}
	if err := json.Unmarshal(inputData, &jsonRaw); err == nil {
		refinedData, err := eng.ProcessInterface(jsonRaw, actor)
		if err != nil {
			log.Fatalf("Refinery failure: %v", err)
		}
		if !*dryRun {
			output, _ := json.MarshalIndent(refinedData, "", "    ")
			fmt.Println(string(output))
		}
	} else {
		lines := strings.Split(string(inputData), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			refined, err := eng.RefineString(line, actor, nil)
			if err != nil {
				log.Fatalf("Refinery failure: %v", err)
			}
			if !*dryRun {
				fmt.Println(refined)
			}
		}
	}

	if *dryRun || *report {
		printReport(eng, 1)
	}
}

// printReport outputs the PII redaction metadata to standard error.
func printReport(eng *refinery.Refinery, filesScanned int) {
	rpt := eng.GenerateReport(filesScanned)
	out, _ := json.MarshalIndent(rpt, "", "  ")
	fmt.Fprintln(os.Stderr, string(out))
}


// startServer initializes and starts the local HTTP dashboard and API
// endpoints, delegating route handling to the internal/handlers package.
func startServer(eng *refinery.Refinery, servePort string) {
	h := handlers.New(eng, VERSION, startTime)
	h.RegisterRoutes(http.DefaultServeMux)

	port := strings.TrimPrefix(servePort, ":")
	addr := "0.0.0.0:" + port
	fmt.Printf("OCULTAR REST API running on http://%s\n", addr)
	srv := &http.Server{
		Addr:              addr,
		Handler:           handlers.CORSHandler(http.DefaultServeMux),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		// WriteTimeout is intentionally unset: LLM streaming responses are long-lived.
	}
	log.Fatal(srv.ListenAndServe())
}
