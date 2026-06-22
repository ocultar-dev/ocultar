package handlers

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/ocultar-dev/ocultar/pkg/config"
	"github.com/ocultar-dev/ocultar/pkg/refinery"
	"github.com/ocultar-dev/ocultar/vault"
)

// HandleReveal decrypts vault tokens back to plaintext for authorized callers.
// Auditor-token gated and rate-limited (shared limiter with HandleVaultDelete).
func (h *Handler) HandleReveal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	auditorToken := os.Getenv("OCU_AUDITOR_TOKEN")
	if auditorToken == "" {
		http.Error(w, "Unauthorized: OCU_AUDITOR_TOKEN is not configured on this server.", http.StatusForbidden)
		return
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader != "Bearer "+auditorToken {
		h.Eng.AuditLogger.Log("UNKNOWN", "failed_reveal_auth", "N/A", "401 Unauthorized")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if !h.RevealLimiter.allow(authHeader) {
		h.Eng.AuditLogger.Log("auditor", "reveal_rate_limited", "N/A", "429 Too Many Requests")
		http.Error(w, "Too many reveal requests — slow down.", http.StatusTooManyRequests)
		return
	}

	var payload struct {
		Tokens []string `json:"tokens"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	results := make(map[string]string)
	for _, t := range payload.Tokens {
		decrypted, err := refinery.DecryptToken(h.Eng.Vault, h.Eng.MasterKey, t)
		if err == nil && decrypted != t {
			results[t] = decrypted
			h.Eng.AuditLogger.Log("auditor", "revealed", t, "N/A")
		} else {
			results[t] = "ERR_NOT_FOUND"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"results": results,
	})
}

// HandleVaultDelete supports on-demand data-subject erasure requests (GDPR
// Art. 17 "right to erasure"), scoped to single vault token rows — it
// deliberately does not cascade into the Entity Registry, matching
// HandleReveal's existing scope of operating on vault rows only.
// Auditor-token gated and rate-limited (shared limiter with HandleReveal).
func (h *Handler) HandleVaultDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	auditorToken := os.Getenv("OCU_AUDITOR_TOKEN")
	if auditorToken == "" {
		http.Error(w, "Unauthorized: OCU_AUDITOR_TOKEN is not configured on this server.", http.StatusForbidden)
		return
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader != "Bearer "+auditorToken {
		h.Eng.AuditLogger.Log("UNKNOWN", "failed_vault_delete_auth", "N/A", "401 Unauthorized")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if !h.RevealLimiter.allow(authHeader) {
		h.Eng.AuditLogger.Log("auditor", "vault_delete_rate_limited", "N/A", "429 Too Many Requests")
		http.Error(w, "Too many delete requests — slow down.", http.StatusTooManyRequests)
		return
	}

	var payload struct {
		Tokens []string `json:"tokens"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	results := make(map[string]string)
	for _, t := range payload.Tokens {
		deleted, err := h.Eng.Vault.DeleteToken(t)
		switch {
		case err != nil:
			results[t] = "ERR_FAILED"
		case deleted:
			results[t] = "DELETED"
			h.Eng.AuditLogger.Log("auditor", "erased", t, "N/A")
		default:
			results[t] = "NOT_FOUND"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"results": results,
	})
}

// HandleVaultMigrate migrates the vault backend from DuckDB to Postgres.
// Auditor-token gated.
func (h *Handler) HandleVaultMigrate(w http.ResponseWriter, r *http.Request) {
	if !requireAuditorToken(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		DSN string `json:"dsn"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}
	if err := vault.MigrateDuckDBtoPostgres(h.Eng.Vault, payload.DSN); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`)) //nolint:errcheck
}

// HandleVaultStats reports live vault token counts and backend type. Open
// (no auditor-token gate, matches prior behavior — this is dashboard
// telemetry, not a data-access endpoint).
func (h *Handler) HandleVaultStats(w http.ResponseWriter, r *http.Request) {
	setStandardCORS(w, r)
	w.Header().Set("Content-Type", "application/json")
	tokens := int64(0)
	if h.Eng.VaultCount != nil {
		tokens = h.Eng.VaultCount.Load()
	}
	backend := config.Global.VaultBackend
	if backend == "" {
		backend = "duckdb"
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_tokens":    tokens,
		"unique_entities": tokens, // In this architecture 1 token roughly maps to 1 entity
		"vault_size":      tokens * 256,
		"backend_type":    backend,
	})
}
