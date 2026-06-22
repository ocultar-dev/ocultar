// Package handlers contains the named, independently-testable HTTP handlers
// for the OCULTAR Refinery's local dashboard/API server. Each handler is a
// method on Handler rather than an inline closure inside cmd/main.go's
// startServer(), so auth-gated routes can be unit-tested with
// httptest.NewRecorder/httptest.NewRequest the same way Sombra's Gateway
// handlers are (apps/sombra/pkg/handler/auth_test.go).
package handlers

import (
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ocultar-dev/ocultar/pkg/refinery"
)

// Handler aggregates the dependencies shared by every route registered by
// RegisterRoutes. It deliberately stays thin — eng already exposes Vault,
// MasterKey, AuditLogger, and VaultCount.
type Handler struct {
	Eng           *refinery.Refinery
	RevealLimiter *revealRateLimiter
	StaticDir     string
	AssetsFS      http.Handler
	Version       string
	StartTime     time.Time
}

// New constructs a Handler. version and startTime are passed in rather than
// read from package-level globals because cmd/main.go owns those values
// (VERSION const, startTime var) and this package has no reason to duplicate
// them.
func New(eng *refinery.Refinery, version string, startTime time.Time) *Handler {
	// 30 reveal calls per minute per auditor token — generous for legitimate
	// dashboard use, tight enough to block bulk vault extraction via a leaked token.
	revealLimiter := newRevealRateLimiter(30, time.Minute)

	// Serve static files from the "dashboard" directory if it exists, otherwise root.
	staticDir := "dashboard"
	if _, err := os.Stat(staticDir); os.IsNotExist(err) {
		staticDir = "."
	}

	return &Handler{
		Eng:           eng,
		RevealLimiter: revealLimiter,
		StaticDir:     staticDir,
		AssetsFS:      http.FileServer(http.Dir(staticDir)),
		Version:       version,
		StartTime:     startTime,
	}
}

// RegisterRoutes wires every handler method onto mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("/assets/", h.AssetsFS)
	mux.HandleFunc("/", h.HandleIndex)
	mux.HandleFunc("/api/content", h.HandleContent)
	mux.HandleFunc("/api/docs", h.HandleDocs)

	mux.HandleFunc("/api/config", h.HandleConfig)
	mux.HandleFunc("/api/config/regex", h.HandleConfigRegex)
	mux.HandleFunc("/api/config/dictionary", h.HandleConfigDictionary)
	mux.HandleFunc("/api/config/system", h.HandleConfigSystem)
	mux.HandleFunc("/api/config/mapping", h.HandleConfigMapping)

	mux.HandleFunc("/api/system/status", h.HandleSystemStatus)
	mux.HandleFunc("/api/system/metrics", h.HandleSystemMetrics)
	mux.HandleFunc("/api/health", h.HandleHealth)

	mux.HandleFunc("/api/reveal", h.HandleReveal)
	mux.HandleFunc("/api/vault/delete", h.HandleVaultDelete)
	mux.HandleFunc("/api/vault/migrate", h.HandleVaultMigrate)
	mux.HandleFunc("/api/vault/stats", h.HandleVaultStats)

	mux.HandleFunc("/api/entities", h.HandleEntities)
	mux.HandleFunc("/api/entities/seed", h.HandleEntitiesSeed)

	mux.HandleFunc("/api/refine", h.HandleRefine)
	mux.HandleFunc("/api/refine/file", h.HandleRefineFile)

	mux.HandleFunc("/api/audit/logs", h.HandleAuditLogs)
	mux.HandleFunc("/api/audit/risk", h.HandleAuditRisk)
	mux.HandleFunc("/api/compliance/evidence", h.HandleComplianceEvidence)

	mux.HandleFunc("/api/pilot/upload", h.HandlePilotUpload)
	mux.HandleFunc("/api/pilot/riskreport", h.HandlePilotRiskReport)
	mux.HandleFunc("/api/pilot-assessment", h.HandlePilotAssessment)
	mux.HandleFunc("/api/pilot/history", h.HandlePilotHistory)
	mux.HandleFunc("/api/pilot/report", h.HandlePilotReport)
}

// requireAuditorToken enforces the standard OCU_AUDITOR_TOKEN Bearer auth
// shared by every admin/sensitive endpoint. Returns false (and has already
// written the error response) if the request should not proceed.
func requireAuditorToken(w http.ResponseWriter, r *http.Request) bool {
	auditorToken := os.Getenv("OCU_AUDITOR_TOKEN")
	if auditorToken == "" {
		http.Error(w, "Unauthorized: OCU_AUDITOR_TOKEN is not configured on this server.", http.StatusForbidden)
		return false
	}
	if r.Header.Get("Authorization") != "Bearer "+auditorToken {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

// setLocalhostCORS sets Access-Control-Allow-Origin only for trusted localhost
// origins. Requests from any other origin (e.g. a malicious webpage) receive no
// ACAO header, so the browser blocks the cross-origin response automatically.
// Server-to-server callers (Tauri Rust backend) send no Origin header and are
// always allowed.
func setLocalhostCORS(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	switch {
	case origin == "":
		// No Origin — server-to-server (Tauri Rust backend). No ACAO needed.
	case strings.HasPrefix(origin, "http://localhost") ||
		strings.HasPrefix(origin, "http://127.0.0.1") ||
		strings.HasPrefix(origin, "tauri://localhost"):
		w.Header().Set("Access-Control-Allow-Origin", origin)
	// Any other origin: no ACAO header set → browser blocks the response.
	}
}

// corsHandler is available for callers (e.g. cmd/main.go) that want to wrap
// the whole mux the way startServer() previously wrapped http.DefaultServeMux.
func corsHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setLocalhostCORS(w, r)
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// CORSHandler exposes corsHandler outside the package (cmd/main.go wraps the
// mux returned by RegisterRoutes with this, exactly as startServer() did).
func CORSHandler(h http.Handler) http.Handler {
	return corsHandler(h)
}

// setStandardCORS applies the common (non-OPTIONS-short-circuiting) header
// set used by most GET/POST/DELETE handlers below.
func setStandardCORS(w http.ResponseWriter, r *http.Request) {
	setLocalhostCORS(w, r)
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

// revealRateLimiter is a simple in-memory sliding-window limiter for
// /api/reveal and /api/vault/delete: it bounds how many calls a given
// Authorization header value can make per window, so a leaked or misused
// OCU_AUDITOR_TOKEN can't be used to mass-extract or mass-erase the vault in
// a single burst.
type revealRateLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	requests map[string][]time.Time
}

func newRevealRateLimiter(limit int, window time.Duration) *revealRateLimiter {
	return &revealRateLimiter{
		limit:    limit,
		window:   window,
		requests: make(map[string][]time.Time),
	}
}

func (rl *revealRateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)
	kept := rl.requests[key][:0]
	for _, t := range rl.requests[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= rl.limit {
		rl.requests[key] = kept
		return false
	}
	rl.requests[key] = append(kept, now)
	return true
}
