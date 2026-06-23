package identities

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/ocultar-dev/ocultar/pkg/config"
)

// StartSyncWorker boots a background goroutine that polls CRM/LDAP APIs
// and synchronizes fetched identities into the config's Tier 0 Dictionary.
func StartSyncWorker() {
	interval, err := time.ParseDuration(config.Global.SyncInterval)
	if err != nil {
		slog.Error("CRM sync: invalid sync_interval, defaulting to 5m", "sync_interval", config.Global.SyncInterval)
		interval = 5 * time.Minute
	}

	go func() {
		slog.Info("CRM identity sync worker started", "endpoint", config.Global.CRMEndpoint, "interval", interval)

		// Initial sync
		performSync()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			performSync()
		}
	}()
}

func performSync() {
	endpoint := config.Global.CRMEndpoint
	if endpoint == "" {
		slog.Debug("CRM sync: skipping poll, crm_endpoint not configured")
		return
	}

	apiKey := config.Global.CRMApiKey
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		slog.Error("CRM sync: failed to create request", "error", err)
		return
	}
	if apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("CRM sync: polling failed", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Error("CRM sync: unexpected status code", "status_code", resp.StatusCode)
		return
	}

	var identities []string
	if err := json.NewDecoder(resp.Body).Decode(&identities); err != nil {
		slog.Error("CRM sync: failed to decode response", "error", err)
		return
	}

	added := 0
	for _, id := range identities {
		config.AddDictionaryTerm("PROTECTED_ENTITY", id)
		added++
	}

	if added > 0 {
		if err := config.Save(); err != nil {
			slog.Error("CRM sync: failed to save synced identities", "error", err)
		} else {
			slog.Info("CRM sync: automatically ingested protected identities", "count", added)
		}
	}
}
