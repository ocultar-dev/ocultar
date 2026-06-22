package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// HandleIndex serves static assets and falls back to index.html for SPA
// routing. Open (no auditor-token gate, matches prior behavior).
func (h *Handler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	// Specific file routes for legacy or multi-page support
	if r.URL.Path == "/index_v2.html" {
		http.ServeFile(w, r, filepath.Join(h.StaticDir, "index_v2.html"))
		return
	}

	// API routes are handled elsewhere by DefaultServeMux (longest prefix match)
	// but we ensure we don't serve index.html for them if they fall through
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}

	// Fail-safe for missing assets: don't serve index.html for missing /assets/ files
	if strings.HasPrefix(r.URL.Path, "/assets/") {
		h.AssetsFS.ServeHTTP(w, r)
		return
	}

	// Fallback to index.html for SPA routing
	http.ServeFile(w, r, filepath.Join(h.StaticDir, "index.html"))
}

// HandleContent returns sample templates for the playground. Open (no
// auditor-token gate, matches prior behavior).
func (h *Handler) HandleContent(w http.ResponseWriter, r *http.Request) {
	setStandardCORS(w, r)
	w.Header().Set("Content-Type", "application/json")

	// Sample templates for the playground
	templates := []map[string]string{
		{
			"id":      "support_ticket",
			"name":    "Customer Support Ticket",
			"content": "Hi, this is John Doe (john.doe@example.com). I need help with my account ending in 1234. My phone is +34 612 345 678.",
		},
		{
			"id":      "database_row",
			"name":    "Database Record (JSON)",
			"content": `{"user_id": 45, "email": "admin@company.net", "last_login_ip": "1.2.3.4"}`,
		},
		{
			"id":      "medical_note",
			"name":    "Medical Consultation",
			"content": "Patient: Jane Smith. Treatment started at New York City Hospital for diabetes. Follow-up with Dr. House.",
		},
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"templates": templates,
	})
}

// HandleDocs returns the contents of README.md alongside engine version and
// last-modified metadata. Open (no auditor-token gate, matches prior
// behavior).
func (h *Handler) HandleDocs(w http.ResponseWriter, r *http.Request) {
	setStandardCORS(w, r)
	w.Header().Set("Content-Type", "application/json")
	readmeBytes, err := os.ReadFile("README.md")
	var readme string
	if err != nil {
		readme = "# Documentation\nError loading README.md"
	} else {
		readme = string(readmeBytes)
	}
	stat, err := os.Stat("README.md")
	lastUpdated := ""
	if err == nil {
		lastUpdated = stat.ModTime().Format(time.RFC3339)
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"documentation": readme,
		"version":       h.Version,
		"last_updated":  lastUpdated,
	})
}
