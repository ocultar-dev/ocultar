package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	htmltmpl "html/template"
	texttmpl "text/template"
	"time"

	"github.com/google/uuid"
	"github.com/ocultar-dev/ocultar/pkg/audit"
)

// HandlePilotUpload accepts a pilot dataset upload, saving it under
// pilot_data/uploads. Open (no auditor-token gate, matches prior behavior).
func (h *Handler) HandlePilotUpload(w http.ResponseWriter, r *http.Request) {
	setLocalhostCORS(w, r)
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	err := r.ParseMultipartForm(10 << 20) //nolint:gosec // G120: body already bounded via http.MaxBytesReader above
	if err != nil {
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("dataset")
	if err != nil {
		http.Error(w, "Error retrieving file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Save to uploads — filepath.Base strips any "../" traversal from the
	// browser-supplied filename before joining it into the upload directory.
	safeBase := filepath.Base(handler.Filename)
	filename := fmt.Sprintf("%d_%s", time.Now().Unix(), safeBase)
	dstPath := filepath.Join("pilot_data/uploads", filename)
	dst, err := os.Create(dstPath)
	if err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}
	io.Copy(dst, file) //nolint:errcheck
	dst.Close()         //nolint:errcheck

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"filename": filename, "original_name": handler.Filename})
}

// HandlePilotRiskReport analyzes a previously uploaded pilot dataset and
// generates Markdown/HTML risk reports on disk. Open (no auditor-token gate,
// matches prior behavior).
func (h *Handler) HandlePilotRiskReport(w http.ResponseWriter, r *http.Request) {
	setLocalhostCORS(w, r)
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DatasetPath string `json:"dataset_path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if req.DatasetPath == "" {
		http.Error(w, "dataset_path is required", http.StatusBadRequest)
		return
	}

	// Robust file lookup for demo/pilot environments
	safePath := filepath.Base(req.DatasetPath)
	datasetFile := safePath
	if _, err := os.Stat(datasetFile); os.IsNotExist(err) {
		// Try root-relative if running from services/refinery
		altPath := filepath.Join("../../", safePath)
		if _, err := os.Stat(altPath); err == nil {
			datasetFile = altPath
		} else {
			// Try one level up just in case
			altPath = filepath.Join("../", safePath)
			if _, err := os.Stat(altPath); err == nil {
				datasetFile = altPath
			}
		}
	}

	// Read dataset from disk
	data, err := os.ReadFile(datasetFile)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read dataset: %v (Checked: %s and alternates)", err, req.DatasetPath), http.StatusInternalServerError)
		return
	}

	var dataset []map[string]interface{}
	if err := json.Unmarshal(data, &dataset); err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse JSON: %v", err), http.StatusInternalServerError)
		return
	}

	// Defaults for demo pilot
	qi := []string{"region", "dept"}
	sa := []string{"name", "iban", "email"}

	report := audit.AnalyzeDatasetRisk(dataset, qi, sa)

	// Map to full report for template generation
	reportID := strings.ToUpper(uuid.New().String()[:8])
	meta := reportMeta{
		ReportID:           reportID,
		GeneratedAt:        time.Now().UTC().Format("02 January 2006, 15:04 UTC"),
		DatasetScope:       datasetFile,
		MethodologyVersion: reportVersion,
		EngineVersion:      engineVersion,
		TotalRecords:       len(dataset),
	}
	before, after := buildScenarios(report)
	fullRpt := fullReport{Meta: meta, Risk: report, Before: before, After: after}

	// Ensure reports dir exists
	os.MkdirAll("pilot_data/reports", 0755) //nolint:errcheck

	// Generate on-disk Markdown
	mdTmpl := texttmpl.Must(texttmpl.New("md").Parse(mdTemplate))
	mdPath := filepath.Join("pilot_data/reports", "report_"+reportID+".md")
	mdFile, _ := os.Create(mdPath)
	if mdFile != nil {
		mdTmpl.Execute(mdFile, fullRpt) //nolint:errcheck
		mdFile.Close()
	}

	// Generate on-disk HTML
	funcMap := htmltmpl.FuncMap{"lower": strings.ToLower, "pct": func(score float64) int { return int(score * 10) }}
	htmlTmpl := htmltmpl.Must(htmltmpl.New("html").Funcs(funcMap).Parse(htmlTemplate))
	htmlPath := filepath.Join("pilot_data/reports", "report_"+reportID+".html")
	htmlFile, _ := os.Create(htmlPath)
	if htmlFile != nil {
		htmlTmpl.Execute(htmlFile, fullRpt) //nolint:errcheck
		htmlFile.Close()
	}

	// Update History Registry
	type historyItem struct {
		ID           string  `json:"id"`
		Timestamp    string  `json:"timestamp"`
		DatasetName  string  `json:"dataset_name"`
		OverallRisk  string  `json:"overall_risk"`
		RiskScore    float64 `json:"risk_score"`
		TotalRecords int     `json:"total_records"`
	}
	var history []historyItem
	histRaw, _ := os.ReadFile("pilot_data/history.json")
	json.Unmarshal(histRaw, &history) //nolint:errcheck

	history = append(history, historyItem{
		ID:           reportID,
		Timestamp:    meta.GeneratedAt,
		DatasetName:  filepath.Base(datasetFile),
		OverallRisk:  report.OverallRiskLevel,
		RiskScore:    report.OverallRiskScore,
		TotalRecords: len(dataset),
	})
	histUpdated, _ := json.MarshalIndent(history, "", "  ")
	os.WriteFile("pilot_data/history.json", histUpdated, 0600) //nolint:errcheck

	response := map[string]interface{}{
		"report":    report,
		"report_id": reportID,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandlePilotAssessment runs a stateless risk assessment for a lead-gated
// pilot submission, storing the lead and generating reports on disk. Open
// (no auditor-token gate, matches prior behavior).
func (h *Handler) HandlePilotAssessment(w http.ResponseWriter, r *http.Request) {
	setLocalhostCORS(w, r)
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var dataset []map[string]interface{}
	var email, company string

	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		r.Body = http.MaxBytesReader(w, r.Body, 100*1024)
		r.ParseMultipartForm(100 * 1024) //nolint:gosec,errcheck // G120: body already bounded via http.MaxBytesReader above; FormValue handles missing fields gracefully
		email = r.FormValue("email")
		company = r.FormValue("company")
		file, _, err := r.FormFile("dataset")
		if err == nil {
			defer file.Close() //nolint:errcheck
			data, _ := io.ReadAll(file)
			json.Unmarshal(data, &dataset) //nolint:errcheck
		}
	} else {
		var req struct {
			Email   string                   `json:"email"`
			Company string                   `json:"company"`
			Dataset []map[string]interface{} `json:"dataset"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			email = req.Email
			company = req.Company
			dataset = req.Dataset
		}
	}

	if email == "" {
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}

	// Stateless assessment
	qi := []string{"region", "dept", "age_group"}
	sa := []string{"name", "email", "iban", "salary"}
	report := audit.AnalyzeDatasetRisk(dataset, qi, sa)

	// Store Lead
	type lead struct {
		Email     string    `json:"email"`
		Company   string    `json:"company"`
		Timestamp time.Time `json:"timestamp"`
		RiskLevel string    `json:"risk_level"`
	}
	os.MkdirAll("pilot_data/reports", 0755) //nolint:errcheck
	var leads []lead
	leadRaw, _ := os.ReadFile("pilot_data/leads.json")
	json.Unmarshal(leadRaw, &leads) //nolint:errcheck
	leads = append(leads, lead{Email: email, Company: company, Timestamp: time.Now(), RiskLevel: report.OverallRiskLevel})
	leadUpdated, _ := json.MarshalIndent(leads, "", "  ")
	os.WriteFile("pilot_data/leads.json", leadUpdated, 0600) //nolint:errcheck

	// Map to full report for template generation
	reportID := strings.ToUpper(uuid.New().String()[:8])
	datasetScopeName := company + " Custom Upload"
	meta := reportMeta{
		ReportID:           reportID,
		GeneratedAt:        time.Now().UTC().Format("02 January 2006, 15:04 UTC"),
		DatasetScope:       datasetScopeName,
		MethodologyVersion: reportVersion,
		EngineVersion:      engineVersion,
		TotalRecords:       len(dataset),
	}
	before, after := buildScenarios(report)
	fullRpt := fullReport{Meta: meta, Risk: report, Before: before, After: after}

	// Generate on-disk Markdown
	mdTmpl := texttmpl.Must(texttmpl.New("md").Parse(mdTemplate))
	mdPath := filepath.Join("pilot_data/reports", "report_"+reportID+".md")
	if mdFile, _ := os.Create(mdPath); mdFile != nil {
		mdTmpl.Execute(mdFile, fullRpt) //nolint:errcheck
		mdFile.Close()
	}

	// Generate on-disk HTML
	funcMap := htmltmpl.FuncMap{"lower": strings.ToLower, "pct": func(score float64) int { return int(score * 10) }}
	htmlTmpl := htmltmpl.Must(htmltmpl.New("html").Funcs(funcMap).Parse(htmlTemplate))
	htmlPath := filepath.Join("pilot_data/reports", "report_"+reportID+".html")
	if htmlFile, _ := os.Create(htmlPath); htmlFile != nil {
		htmlTmpl.Execute(htmlFile, fullRpt) //nolint:errcheck
		htmlFile.Close()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "success",
		"report":      report,
		"report_id":   reportID,
		"full_report": fullRpt,
	})
}

// HandlePilotHistory returns the pilot report history registry. Open (no
// auditor-token gate, matches prior behavior).
func (h *Handler) HandlePilotHistory(w http.ResponseWriter, r *http.Request) {
	setLocalhostCORS(w, r)
	w.Header().Set("Content-Type", "application/json")
	content, _ := os.ReadFile("pilot_data/history.json")
	if len(content) == 0 {
		w.Write([]byte(`[]`))
		return
	}
	w.Write(content)
}

// HandlePilotReport serves a previously generated pilot HTML report by ID.
// Open (no auditor-token gate, matches prior behavior). The id query param is
// validated against [A-F0-9] only before being joined into a file path.
func (h *Handler) HandlePilotReport(w http.ResponseWriter, r *http.Request) {
	setLocalhostCORS(w, r)
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Report ID missing", http.StatusBadRequest)
		return
	}
	// IDs are generated as 8 uppercase hex chars (uuid[:8] + ToUpper).
	// Reject anything else so a crafted id can't escape the reports directory.
	for _, c := range id {
		if (c < 'A' || c > 'F') && (c < '0' || c > '9') {
			http.Error(w, "Invalid report ID", http.StatusBadRequest)
			return
		}
	}

	path := filepath.Join("pilot_data/reports", "report_"+id+".html")
	content, err := os.ReadFile(path) //nolint:gosec // G703: id is validated above to [A-F0-9] only — no path traversal possible
	if err != nil {
		http.Error(w, "Report not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write(content) //nolint:gosec // G705: content is from a file generated by our own template; path validated to [A-F0-9]{8} above
}
