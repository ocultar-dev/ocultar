package connector_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ocultar-dev/ocultar/pkg/config"
	"github.com/ocultar-dev/ocultar/pkg/refinery"
	"github.com/ocultar-dev/ocultar/vault"
	"github.com/ocultar-dev/ocultar/apps/sombra/pkg/connector"
	"github.com/ocultar-dev/ocultar/apps/sombra/pkg/handler"
	"github.com/ocultar-dev/ocultar/apps/sombra/pkg/metrics"
	"github.com/ocultar-dev/ocultar/apps/sombra/pkg/router"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// mockModelAdapter simulates an AI backend that just echoes the prompt
type mockModelAdapter struct {
	name   string
	called bool
}

func (m *mockModelAdapter) Name() string { return m.name }
func (m *mockModelAdapter) Send(ctx context.Context, messages []router.Message, opts router.ModelOpts) (string, error) {
	m.called = true
	prompt := ""
	if len(messages) > 0 {
		prompt = messages[len(messages)-1].Content
	}
	return "AI Response: I see " + prompt, nil
}
func (m *mockModelAdapter) Endpoint() string { return "http://mock-internal" }
func (m *mockModelAdapter) HealthCheck(ctx context.Context) error { return nil }

// setupTestEnv initializes an in-memory vault, refinery, router, and gateway.
func setupTestEnv(t *testing.T) *handler.Gateway {
	v, err := vault.New(config.Settings{VaultBackend: "duckdb"}, "")
	if err != nil {
		t.Fatalf("create vault: %v", err)
	}
	t.Cleanup(func() { v.Close() })

	masterKey := make([]byte, 32)
	config.InitDefaults()
	eng, err := refinery.NewRefinery(v, masterKey)
	if err != nil {
		t.Fatalf("NewRefinery: %v", err)
	}

	r := router.New("mock-model", []string{"mock-internal"})
	r.Register(&mockModelAdapter{name: "mock-model"})

	gw, err := handler.NewGateway(eng, v, masterKey, r, nil)
	if err != nil {
		t.Fatalf("NewGateway: %v", err)
	}

	fc := connector.NewFileConnector("file", connector.DataPolicy{
		AllowedModels: []string{"mock-model"},
	})
	gw.RegisterConnector(fc)

	return gw
}

func TestGateway_HandleQuery(t *testing.T) {
	gw := setupTestEnv(t)

	// We create a multipart form request simulating a file upload.
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	w.WriteField("connector", "file")
	w.WriteField("model", "mock-model")
	w.WriteField("prompt", "Summarize this file")

	part, _ := w.CreateFormFile("file", "statement.txt")
	part.Write([]byte("My email is alice@company.org"))
	w.Close()

	req := httptest.NewRequest("POST", "/query", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer test-actor")

	rr := httptest.NewRecorder()
	gw.HandleQuery(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Response string `json:"response"`
		Metadata struct {
			Redacted bool `json:"pii_was_redacted"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// The proxy layer inside the handler should have rehydrated the vault token.
	if !resp.Metadata.Redacted {
		t.Error("expected metadata.redacted to be true")
	}

	if !bytes.Contains([]byte(resp.Response), []byte("alice@company.org")) {
		t.Errorf("expected original email in response, got: %s", resp.Response)
	}
}

// TestGateway_HandleQuery_RecordsMetrics verifies HandleQuery actually
// increments Sombra's Prometheus counters — apps/proxy has had request
// metrics since its first version; Sombra previously had none.
func TestGateway_HandleQuery_RecordsMetrics(t *testing.T) {
	gw := setupTestEnv(t)

	before := testutil.ToFloat64(metrics.RequestsTotal.WithLabelValues("query", "200"))

	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	w.WriteField("connector", "file")
	w.WriteField("model", "mock-model")
	w.WriteField("prompt", "Summarize this file")
	part, _ := w.CreateFormFile("file", "statement.txt")
	part.Write([]byte("My email is bob@company.org"))
	w.Close()

	req := httptest.NewRequest("POST", "/query", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer test-actor")

	rr := httptest.NewRecorder()
	gw.HandleQuery(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}

	after := testutil.ToFloat64(metrics.RequestsTotal.WithLabelValues("query", "200"))
	if after-before != 1 {
		t.Errorf("want RequestsTotal{query,200} delta=1, got %.0f", after-before)
	}
}
