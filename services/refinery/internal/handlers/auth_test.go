package handlers

// auth_test.go — unit tests for the OCU_AUDITOR_TOKEN Bearer auth gate shared
// by the 8 admin/sensitive endpoints. Each test checks the same three cases
// the gate is responsible for: token unset (403), wrong token (401), and a
// valid token reaching the handler's own logic (anything but 401/403).

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ocultar-dev/ocultar/pkg/config"
	"github.com/ocultar-dev/ocultar/pkg/refinery"
	"github.com/ocultar-dev/ocultar/vault"
)

// newTestHandler builds a Handler backed by an in-memory DuckDB vault, ready
// to exercise any handler method directly via httptest.
func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	v, err := vault.New(config.Settings{VaultBackend: "duckdb"}, "")
	if err != nil {
		t.Fatalf("failed to init vault: %v", err)
	}
	t.Cleanup(func() { v.Close() }) //nolint:errcheck
	eng := refinery.NewRefinery(v, []byte("01234567890123456789012345678901"))
	return New(eng, "test-version", time.Now())
}

// checkAuthGate exercises the three auth states against fn (a handler method
// bound to a fresh Handler each call) and hands the valid-token response back
// to assertSuccess for handler-specific checks.
func checkAuthGate(t *testing.T, method, path string, body []byte, fn func(*Handler) http.HandlerFunc, assertSuccess func(t *testing.T, rec *httptest.ResponseRecorder)) {
	t.Run("missing token", func(t *testing.T) {
		t.Setenv("OCU_AUDITOR_TOKEN", "")
		h := newTestHandler(t)
		r := httptest.NewRequest(method, path, bytes.NewReader(body))
		w := httptest.NewRecorder()
		fn(h)(w, r)
		if w.Code != http.StatusForbidden {
			t.Errorf("want 403 when OCU_AUDITOR_TOKEN is unset, got %d", w.Code)
		}
	})

	t.Run("wrong token", func(t *testing.T) {
		t.Setenv("OCU_AUDITOR_TOKEN", "correct-token")
		h := newTestHandler(t)
		r := httptest.NewRequest(method, path, bytes.NewReader(body))
		r.Header.Set("Authorization", "Bearer wrong-token")
		w := httptest.NewRecorder()
		fn(h)(w, r)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("want 401 for a wrong token, got %d", w.Code)
		}
	})

	t.Run("valid token", func(t *testing.T) {
		t.Setenv("OCU_AUDITOR_TOKEN", "correct-token")
		h := newTestHandler(t)
		r := httptest.NewRequest(method, path, bytes.NewReader(body))
		r.Header.Set("Authorization", "Bearer correct-token")
		w := httptest.NewRecorder()
		fn(h)(w, r)
		if w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden {
			t.Fatalf("valid token must pass the auth gate, got %d", w.Code)
		}
		assertSuccess(t, w)
	})
}

func TestHandleConfig_Auth(t *testing.T) {
	checkAuthGate(t, http.MethodGet, "/api/config", nil,
		func(h *Handler) http.HandlerFunc { return h.HandleConfig },
		func(t *testing.T, w *httptest.ResponseRecorder) {
			if w.Code != http.StatusOK {
				t.Errorf("want 200, got %d", w.Code)
			}
			var cfg config.Settings
			if err := json.NewDecoder(w.Body).Decode(&cfg); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if cfg.JWTSecret != "" || cfg.CRMApiKey != "" || cfg.PostgresDSN != "" {
				t.Error("secrets must be redacted from the returned config")
			}
		})
}

func TestHandleConfigRegex_Auth(t *testing.T) {
	checkAuthGate(t, http.MethodGet, "/api/config/regex", nil,
		func(h *Handler) http.HandlerFunc { return h.HandleConfigRegex },
		func(t *testing.T, w *httptest.ResponseRecorder) {
			if w.Code != http.StatusOK {
				t.Errorf("want 200, got %d", w.Code)
			}
		})
}

func TestHandleConfigDictionary_Auth(t *testing.T) {
	checkAuthGate(t, http.MethodGet, "/api/config/dictionary", nil,
		func(h *Handler) http.HandlerFunc { return h.HandleConfigDictionary },
		func(t *testing.T, w *httptest.ResponseRecorder) {
			if w.Code != http.StatusOK {
				t.Errorf("want 200, got %d", w.Code)
			}
		})
}

func TestHandleVaultMigrate_Auth(t *testing.T) {
	checkAuthGate(t, http.MethodPost, "/api/vault/migrate", []byte(`{"dsn":""}`),
		func(h *Handler) http.HandlerFunc { return h.HandleVaultMigrate },
		func(t *testing.T, w *httptest.ResponseRecorder) {
			// An empty DSN fails the migration itself (500) — what matters
			// here is that the auth gate let the request through at all.
			if w.Code != http.StatusInternalServerError {
				t.Errorf("want 500 from the migration failing on an empty DSN, got %d", w.Code)
			}
		})
}

func TestHandleReveal_Auth(t *testing.T) {
	checkAuthGate(t, http.MethodPost, "/api/reveal", []byte(`{"tokens":["[EMAIL_doesnotexist]"]}`),
		func(h *Handler) http.HandlerFunc { return h.HandleReveal },
		func(t *testing.T, w *httptest.ResponseRecorder) {
			if w.Code != http.StatusOK {
				t.Errorf("want 200, got %d", w.Code)
			}
			var resp struct {
				Results map[string]string `json:"results"`
			}
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if resp.Results["[EMAIL_doesnotexist]"] != "ERR_NOT_FOUND" {
				t.Errorf("want ERR_NOT_FOUND for an unknown token, got %q", resp.Results["[EMAIL_doesnotexist]"])
			}
		})
}

func TestHandleVaultDelete_Auth(t *testing.T) {
	checkAuthGate(t, http.MethodPost, "/api/vault/delete", []byte(`{"tokens":["[EMAIL_doesnotexist]"]}`),
		func(h *Handler) http.HandlerFunc { return h.HandleVaultDelete },
		func(t *testing.T, w *httptest.ResponseRecorder) {
			if w.Code != http.StatusOK {
				t.Errorf("want 200, got %d", w.Code)
			}
			var resp struct {
				Results map[string]string `json:"results"`
			}
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if resp.Results["[EMAIL_doesnotexist]"] != "NOT_FOUND" {
				t.Errorf("want NOT_FOUND for an unknown token, got %q", resp.Results["[EMAIL_doesnotexist]"])
			}
		})
}

func TestHandleEntities_Auth(t *testing.T) {
	checkAuthGate(t, http.MethodGet, "/api/entities", nil,
		func(h *Handler) http.HandlerFunc { return h.HandleEntities },
		func(t *testing.T, w *httptest.ResponseRecorder) {
			if w.Code != http.StatusOK {
				t.Errorf("want 200, got %d", w.Code)
			}
			var records []vault.EntityRecord
			if err := json.NewDecoder(w.Body).Decode(&records); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if len(records) != 0 {
				t.Errorf("want an empty registry, got %d records", len(records))
			}
		})
}

func TestHandleEntitiesSeed_Auth(t *testing.T) {
	body := []byte(`{"entities":[{"entity_type":"PERSON","canonical_name":"Auth Test Person","variants":["Auth Test"]}]}`)
	checkAuthGate(t, http.MethodPost, "/api/entities/seed", body,
		func(h *Handler) http.HandlerFunc { return h.HandleEntitiesSeed },
		func(t *testing.T, w *httptest.ResponseRecorder) {
			if w.Code != http.StatusOK {
				t.Errorf("want 200, got %d", w.Code)
			}
			var resp struct {
				Seeded int      `json:"seeded"`
				Tokens []string `json:"tokens"`
			}
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if resp.Seeded != 1 || len(resp.Tokens) != 1 || !strings.HasPrefix(resp.Tokens[0], "[PERSON_") {
				t.Errorf("want 1 seeded entity with a [PERSON_*] token, got %+v", resp)
			}
		})
}
