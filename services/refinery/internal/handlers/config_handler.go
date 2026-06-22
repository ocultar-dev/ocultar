package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/ocultar-dev/ocultar/pkg/config"
)

// HandleConfig returns the global config with secrets redacted. Auditor-token gated.
func (h *Handler) HandleConfig(w http.ResponseWriter, r *http.Request) {
	if !requireAuditorToken(w, r) {
		return
	}
	safeConfig := config.Global
	safeConfig.JWTSecret = ""
	safeConfig.CRMApiKey = ""
	safeConfig.PostgresDSN = ""
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(safeConfig)
}

// HandleConfigRegex manages Tier 1 regex rules (GET list, POST add, DELETE remove).
// Auditor-token gated.
func (h *Handler) HandleConfigRegex(w http.ResponseWriter, r *http.Request) {
	if !requireAuditorToken(w, r) {
		return
	}
	setStandardCORS(w, r)
	switch r.Method {
	case http.MethodPost:
		var rule config.RegexRule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := config.ValidateRegex(rule.Pattern); err != nil {
			http.Error(w, "Invalid regex: "+err.Error(), http.StatusBadRequest)
			return
		}
		if err := config.AddRegexRule(rule); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := config.Save(); err != nil {
			log.Printf("[CONFIG] failed to persist regex rule: %v", err)
		}
		h.Eng.AuditLogger.Log("admin", "ADD_REGEX", "SUCCESS", rule.Type)
		w.WriteHeader(http.StatusCreated)
	case http.MethodDelete:
		var payload struct {
			Type string `json:"type"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err == nil {
			config.RemoveRegexRule(payload.Type)
			if err := config.Save(); err != nil {
				log.Printf("[CONFIG] failed to persist regex deletion: %v", err)
			}
			h.Eng.AuditLogger.Log("admin", "DEL_REGEX", "SUCCESS", payload.Type)
		}
		w.WriteHeader(http.StatusOK)
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		type RuleWithMapping struct {
			config.RegexRule
			CanonicalMapping string `json:"canonical_mapping,omitempty"`
		}
		var results []RuleWithMapping
		for _, rule := range config.Global.Regexes {
			results = append(results, RuleWithMapping{
				RegexRule:        rule,
				CanonicalMapping: config.Global.AliasMapping[rule.Type],
			})
		}
		json.NewEncoder(w).Encode(results)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleConfigDictionary manages Tier 0 dictionary terms (GET list, POST add).
// Auditor-token gated.
func (h *Handler) HandleConfigDictionary(w http.ResponseWriter, r *http.Request) {
	if !requireAuditorToken(w, r) {
		return
	}
	setStandardCORS(w, r)
	switch r.Method {
	case http.MethodPost:
		var payload struct {
			Type string `json:"type"`
			Term string `json:"term"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err == nil {
			config.AddDictionaryTerm(payload.Type, payload.Term)
			if err := config.Save(); err != nil {
				log.Printf("[CONFIG] failed to persist dictionary term: %v", err)
			}
			h.Eng.AuditLogger.Log("admin", "ADD_DICT", "SUCCESS", payload.Type)
			w.WriteHeader(http.StatusCreated)
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		type DictWithMapping struct {
			config.DictRule
			CanonicalMapping string `json:"canonical_mapping,omitempty"`
		}
		var results []DictWithMapping
		for _, d := range config.Global.Dictionaries {
			results = append(results, DictWithMapping{
				DictRule:         d,
				CanonicalMapping: config.Global.AliasMapping[d.Type],
			})
		}
		json.NewEncoder(w).Encode(results)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleConfigSystem manages system concurrency/queue limits (GET/POST). Open (no auditor-token gate, matches prior behavior).
func (h *Handler) HandleConfigSystem(w http.ResponseWriter, r *http.Request) {
	setStandardCORS(w, r)
	switch r.Method {
	case http.MethodPost:
		var payload struct {
			MaxConcurrency int `json:"max_concurrency"`
			QueueSize      int `json:"queue_size"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err == nil {
			config.UpdateSystemLimits(payload.MaxConcurrency, payload.QueueSize)
			if err := config.Save(); err != nil {
				log.Printf("[CONFIG] failed to persist system limits: %v", err)
			}
			h.Eng.AuditLogger.Log("admin", "UPDATE_SYSTEM_LIMITS", "SUCCESS", "Configured Limits")
			w.WriteHeader(http.StatusOK)
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"max_concurrency": config.Global.MaxConcurrency,
			"queue_size":      config.Global.QueueSize,
		})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleConfigMapping returns the canonical PII-type alias mapping. Open (no auditor-token gate, matches prior behavior).
func (h *Handler) HandleConfigMapping(w http.ResponseWriter, r *http.Request) {
	setLocalhostCORS(w, r)
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(config.Global.AliasMapping)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
