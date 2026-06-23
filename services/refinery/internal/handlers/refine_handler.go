package handlers

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ocultar-dev/ocultar/pkg/config"
	"github.com/ocultar-dev/ocultar/pkg/policy"
)

// HandleRefine runs the full Refinery pipeline over a JSON or newline-delimited
// text body and returns the redacted output plus a per-request PII hit report.
// Open (no auditor-token gate, matches prior behavior — this is the core
// product surface, not an admin endpoint).
func (h *Handler) HandleRefine(w http.ResponseWriter, r *http.Request) {
	setLocalhostCORS(w, r)
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.Eng.ResetHits()

	inputData, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	var refinedOutput string
	var jsonRaw interface{}
	actor := r.RemoteAddr

	if err := json.Unmarshal(inputData, &jsonRaw); err == nil {
		refinedData, err := h.Eng.ProcessInterface(jsonRaw, actor)
		if err != nil {
			slog.Error("refinery error", "error", err)
			http.Error(w, "Ocultar Refinery: internal refinery error", http.StatusInternalServerError)
			return
		}
		outBytes, _ := json.MarshalIndent(refinedData, "", "    ")
		refinedOutput = string(outBytes)
	} else {
		var refinedLines []string
		for _, line := range strings.Split(string(inputData), "\n") {
			if strings.TrimSpace(line) == "" {
				refinedLines = append(refinedLines, line)
				continue
			}
			refined, err := h.Eng.RefineString(line, actor, nil)
			if err != nil {
				slog.Error("refinery error", "error", err)
				http.Error(w, "Ocultar Refinery: internal refinery error", http.StatusInternalServerError)
				return
			}
			refinedLines = append(refinedLines, refined)
		}
		refinedOutput = strings.Join(refinedLines, "\n")
	}

	rpt := h.Eng.GenerateReport(1)

	if len(config.Global.Policies) > 0 {
		if d := policy.Evaluate(config.Global.Policies, rpt.Hits); d.Blocked {
			h.Eng.AuditLogger.Log(actor, "POLICY_BLOCK", d.PolicyName, d.BlockedEntity)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{
				"error":          "policy_violation",
				"message":        "Request blocked by policy '" + d.PolicyName + "'.",
				"policy":         d.PolicyName,
				"blocked_entity": d.BlockedEntity,
			})
			return
		}
	}

	response := map[string]interface{}{
		"refined": refinedOutput,
		"report":  rpt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleRefineFile runs the Refinery pipeline over an uploaded JSON, CSV, or
// line-delimited text file and streams the redacted result back. Open (no
// auditor-token gate, matches prior behavior).
func (h *Handler) HandleRefineFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Invalid file upload", http.StatusBadRequest)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=cleaned_%s", handler.Filename))

	h.Eng.ResetHits()
	actor := r.RemoteAddr
	if strings.HasSuffix(strings.ToLower(handler.Filename), ".json") {
		w.Header().Set("Content-Type", "application/json")
		var data interface{}
		if err := json.NewDecoder(file).Decode(&data); err != nil {
			http.Error(w, "Invalid JSON file", http.StatusBadRequest)
			return
		}
		refinedData, err := h.Eng.ProcessInterface(data, actor)
		if err != nil {
			slog.Error("refinery error (json)", "error", err)
			http.Error(w, "Ocultar Refinery: internal refinery error", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(refinedData)
		return
	}

	if strings.HasSuffix(strings.ToLower(handler.Filename), ".csv") {
		w.Header().Set("Content-Type", "text/csv")
		reader := csv.NewReader(file)
		reader.FieldsPerRecord = -1
		writer := csv.NewWriter(w)
		defer writer.Flush()

		for {
			record, err := reader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				slog.Error("error reading CSV record", "error", err)
				continue
			}

			var refinedRecord []string
			for _, field := range record {
				if strings.TrimSpace(field) == "" {
					refinedRecord = append(refinedRecord, field)
				} else {
					refined, err := h.Eng.RefineString(field, actor, nil)
					if err != nil {
						slog.Error("refinery error (csv)", "error", err)
						http.Error(w, "Ocultar Refinery: internal refinery error", http.StatusInternalServerError)
						return
					}
					refinedRecord = append(refinedRecord, refined)
				}
			}
			writer.Write(refinedRecord) //nolint:errcheck
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			fmt.Fprintln(w, line)
			continue
		}
		refined, err := h.Eng.RefineString(line, actor, nil)
		if err != nil {
			slog.Error("refinery error (jsonl)", "error", err)
			http.Error(w, "Ocultar Refinery: internal refinery error", http.StatusInternalServerError)
			return
		}
		fmt.Fprintln(w, refined) //nolint:gosec // G705: content-type is application/octet-stream, PII already masked

		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Error("error scanning file", "error", err)
	}
}
