package connector_test

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ocultar-dev/ocultar/apps/sombra/pkg/connector"
	"github.com/ocultar-dev/ocultar/apps/sombra/pkg/handler"
	"github.com/ocultar-dev/ocultar/apps/sombra/pkg/router"
	"github.com/ocultar-dev/ocultar/pkg/config"
	"github.com/ocultar-dev/ocultar/pkg/refinery"
	"github.com/ocultar-dev/ocultar/vault"
)

// erroringAIScanner simulates an unreachable SLM sidecar: every call fails.
type erroringAIScanner struct{}

func (erroringAIScanner) ScanForPII(text string) (map[string][]string, error) {
	return nil, fmt.Errorf("simulated SLM sidecar unavailable")
}
func (erroringAIScanner) CheckHealth(host string) {}

// IsAvailable reports true even though every ScanForPII call fails: this
// mock simulates a configured-but-unreachable sidecar (e.g. connection
// refused), which is the scenario that actually reaches the
// FailClosedOnSLMError branch. A scanner that reports IsAvailable() ==
// false is skipped before it's ever called (see refinery.go's activeScanner
// gating), so it would never exercise this code path.
func (erroringAIScanner) IsAvailable() bool        { return true }
func (erroringAIScanner) SetDomain(domain string)  {}
func (erroringAIScanner) CircuitStateName() string { return "open" }

func newCircuitBreakerTestGateway(t *testing.T, failClosed bool) (*handler.Gateway, *mockModelAdapter) {
	t.Helper()
	v, err := vault.New(config.Settings{VaultBackend: "duckdb"}, "")
	if err != nil {
		t.Fatalf("create vault: %v", err)
	}
	t.Cleanup(func() { v.Close() })

	masterKey := make([]byte, 32)
	config.InitDefaults()
	eng := refinery.NewRefinery(v, masterKey)
	eng.SetAIScanner(erroringAIScanner{})
	eng.FailClosedOnSLMError = failClosed

	upstream := &mockModelAdapter{name: "mock-model"}
	r := router.New("mock-model", []string{"mock-internal"})
	r.Register(upstream)

	gw := handler.NewGateway(eng, v, masterKey, r, nil)
	gw.RegisterConnector(connector.NewFileConnector("file", connector.DataPolicy{
		AllowedModels: []string{"mock-model"},
	}))
	return gw, upstream
}

func doCircuitBreakerRequest(t *testing.T, gw *handler.Gateway) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.WriteField("connector", "file")
	mw.WriteField("model", "mock-model")
	mw.WriteField("prompt", "Summarise this document.")
	part, _ := mw.CreateFormFile("file", "doc.txt")
	part.Write([]byte("Contact john.doe@example.com or call 555-123-4567."))
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/query", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer test-actor")

	rr := httptest.NewRecorder()
	gw.HandleQuery(rr, req)
	return rr
}

// TestFailClosed_SLMUnavailable_BlocksByDefault verifies Sombra's default posture:
// when the Tier 2 SLM sidecar errors, the request must be blocked (5xx) and the
// upstream model must never be called. This is the default because Tier 1 alone
// cannot catch names/addresses, and Sombra forwards content to third-party model
// providers — letting that traffic through ungated on a Tier 2 failure would be a
// real PII leak to an external party, not just a degraded preview.
func TestFailClosed_SLMUnavailable_BlocksByDefault(t *testing.T) {
	gw, upstream := newCircuitBreakerTestGateway(t, true)
	rr := doCircuitBreakerRequest(t, gw)

	if rr.Code == http.StatusOK {
		t.Errorf("fail-closed violation: got 200 OK despite SLM failure; body: %s", rr.Body.String())
	}
	if upstream.called {
		t.Error("fail-closed violation: upstream model was called despite SLM failure")
	}
}

// TestCircuitBreaker_SLMUnavailable_DegradedNEROptIn verifies the explicit opt-out:
// when OCU_SOMBRA_ALLOW_DEGRADED_NER is set, operators have knowingly chosen
// availability over completeness, and the gateway falls back to Tier 1-only
// detection — returning 200 OK without hanging or hard-failing.
func TestCircuitBreaker_SLMUnavailable_DegradedNEROptIn(t *testing.T) {
	gw, _ := newCircuitBreakerTestGateway(t, false)
	rr := doCircuitBreakerRequest(t, gw)

	if rr.Code != http.StatusOK {
		t.Errorf("degraded-NER fallback failed: expected 200 OK, got %d\nbody: %s",
			rr.Code, rr.Body.String())
	}
}
