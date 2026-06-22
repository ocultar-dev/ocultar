package handlers

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/ocultar-dev/ocultar/pkg/audit"
	"github.com/ocultar-dev/ocultar/pkg/config"
)

// HandleAuditLogs returns the last 20 lines of audit.log, newest first. Open
// (no auditor-token gate, matches prior behavior).
func (h *Handler) HandleAuditLogs(w http.ResponseWriter, r *http.Request) {
	setStandardCORS(w, r)
	w.Header().Set("Content-Type", "application/json")

	lines, err := readLastLines("audit.log", 20)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"logs": []interface{}{}})
		return
	}

	var logEntries []map[string]interface{}
	for _, line := range lines {
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err == nil {
			logEntries = append(logEntries, entry)
		}
	}

	// Show newest first
	for i, j := 0, len(logEntries)-1; i < j; i, j = i+1, j-1 {
		logEntries[i], logEntries[j] = logEntries[j], logEntries[i]
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"logs": logEntries})
}

// HandleAuditRisk reports k-anonymity/l-diversity policy status (GET) or
// analyzes a submitted dataset for re-identification risk (POST). Open (no
// auditor-token gate, matches prior behavior).
func (h *Handler) HandleAuditRisk(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":                "active",
			"k_anonymity_threshold": 3,
			"l_diversity_threshold": 2,
			"description":           "Risk compliance radar monitoring dataset guarantees.",
			"regulatory_policy":     config.Global.RegulatoryPolicy,
		})
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Dataset             []map[string]interface{} `json:"dataset"`
		QuasiIdentifiers    []string                  `json:"quasi_identifiers"`
		SensitiveAttributes []string                  `json:"sensitive_attributes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	report := audit.AnalyzeDatasetRisk(req.Dataset, req.QuasiIdentifiers, req.SensitiveAttributes)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}

// HandleComplianceEvidence reports a snapshot of engine version, uptime,
// vault entry count, active policies/tiers, and a recent audit-log tail —
// the machine-readable evidence bundle for compliance reviewers. Open (no
// auditor-token gate, matches prior behavior).
func (h *Handler) HandleComplianceEvidence(w http.ResponseWriter, r *http.Request) {
	setLocalhostCORS(w, r)
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vaultEntries := int64(0)
	if h.Eng.VaultCount != nil {
		vaultEntries = h.Eng.VaultCount.Load()
	}

	auditTail, _ := readLastLines("audit.log", 10)
	var auditEntries []map[string]interface{}
	for _, line := range auditTail {
		var entry map[string]interface{}
		if json.Unmarshal([]byte(line), &entry) == nil {
			auditEntries = append(auditEntries, entry)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"schema_version":  "1",
		"generated_at":    time.Now().UTC().Format(time.RFC3339),
		"engine_version":  h.Version,
		"uptime":          time.Since(h.StartTime).String(),
		"vault_entries":   vaultEntries,
		"policies_active": len(config.Global.Policies),
		"policy_snapshot": config.Global.Policies,
		"tiers_active": map[string]bool{
			"tier0_dictionary": len(config.Global.Dictionaries) > 0,
			"tier1_regex":      len(config.Global.Regexes) > 0,
			"tier2_ai":         h.Eng.AIScanner != nil && h.Eng.AIScanner.IsAvailable(),
		},
		"audit_log_tail": auditEntries,
	})
}

// readLastLines reads the last count lines of the file at path.
func readLastLines(path string, count int) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if len(lines) > count {
		return lines[len(lines)-count:], nil
	}
	return lines, nil
}
