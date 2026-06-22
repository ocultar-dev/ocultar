package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/ocultar-dev/ocultar/vault"
)

// HandleEntities lists (GET) or registers (POST) canonical Entity Registry
// entries. Auditor-token gated.
func (h *Handler) HandleEntities(w http.ResponseWriter, r *http.Request) {
	if !requireAuditorToken(w, r) {
		return
	}
	if h.Eng.Vault == nil {
		http.Error(w, "entity registry unavailable", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodGet:
		records, err := h.Eng.Vault.ListEntities()
		if err != nil {
			http.Error(w, fmt.Sprintf("list failed: %v", err), http.StatusInternalServerError)
			return
		}
		if records == nil {
			records = []vault.EntityRecord{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(records)

	case http.MethodPost:
		var req struct {
			EntityType    string   `json:"entity_type"`
			CanonicalName string   `json:"canonical_name"`
			Variants      []string `json:"variants"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.EntityType == "" || req.CanonicalName == "" {
			http.Error(w, "entity_type and canonical_name are required", http.StatusBadRequest)
			return
		}
		token, err := h.Eng.Vault.RegisterEntity(req.EntityType, req.CanonicalName, req.Variants)
		if err != nil {
			http.Error(w, fmt.Sprintf("registration failed: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"canonical_token": token})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleEntitiesSeed bulk-registers canonical Entity Registry entries from
// either a bare JSON array or a {"entities": [...]} wrapper. Auditor-token
// gated.
func (h *Handler) HandleEntitiesSeed(w http.ResponseWriter, r *http.Request) {
	if !requireAuditorToken(w, r) {
		return
	}
	if h.Eng.Vault == nil {
		http.Error(w, "entity registry unavailable", http.StatusServiceUnavailable)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	type seedItem struct {
		EntityType    string   `json:"entity_type"`
		CanonicalName string   `json:"canonical_name"`
		Variants      []string `json:"variants"`
	}
	var seeds []seedItem
	if json.Unmarshal(body, &seeds) != nil {
		var wrapper struct {
			Entities []seedItem `json:"entities"`
		}
		if err := json.Unmarshal(body, &wrapper); err != nil || len(wrapper.Entities) == 0 {
			http.Error(w, `body must be a JSON array or {"entities":[...]}`, http.StatusBadRequest)
			return
		}
		seeds = wrapper.Entities
	}
	var tokens []string
	for _, s := range seeds {
		if s.EntityType == "" || s.CanonicalName == "" {
			continue
		}
		tok, err := h.Eng.Vault.RegisterEntity(s.EntityType, s.CanonicalName, s.Variants)
		if err != nil {
			http.Error(w, fmt.Sprintf("seed failed for %q: %v", s.CanonicalName, err), http.StatusInternalServerError)
			return
		}
		tokens = append(tokens, tok)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"seeded": len(tokens), "tokens": tokens})
}
