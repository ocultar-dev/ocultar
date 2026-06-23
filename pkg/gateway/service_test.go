package gateway_test

import (
	"strings"
	"testing"

	"github.com/ocultar-dev/ocultar/pkg/config"
	"github.com/ocultar-dev/ocultar/pkg/gateway"
	"github.com/ocultar-dev/ocultar/pkg/refinery"
	"github.com/ocultar-dev/ocultar/vault"
)

func newTestService(t *testing.T) *gateway.Service {
	t.Helper()
	config.InitDefaults()
	v, err := vault.New(config.Settings{VaultBackend: "duckdb"}, "")
	if err != nil {
		t.Fatalf("vault init: %v", err)
	}
	t.Cleanup(func() { v.Close() })

	masterKey := []byte("01234567890123456789012345678901") // 32 bytes
	eng, err := refinery.NewRefinery(v, masterKey)
	if err != nil {
		t.Fatalf("NewRefinery: %v", err)
	}
	return gateway.New(eng, v, masterKey)
}

func TestService_RedactAndRehydrateString_RoundTrip(t *testing.T) {
	svc := newTestService(t)

	redacted, err := svc.RedactString("contact me at alice@example.com", "test-actor")
	if err != nil {
		t.Fatalf("RedactString: %v", err)
	}
	if strings.Contains(redacted, "alice@example.com") {
		t.Fatalf("expected email to be redacted, got %q", redacted)
	}

	rehydrated, degraded, err := svc.RehydrateString(redacted)
	if err != nil {
		t.Fatalf("RehydrateString: %v", err)
	}
	if degraded {
		t.Error("expected degraded=false on a successful rehydration")
	}
	if !strings.Contains(rehydrated, "alice@example.com") {
		t.Errorf("expected original email back, got %q", rehydrated)
	}
}

// TestService_RehydrateString_UnknownToken documents current behavior:
// refinery.DecryptToken is deliberately fail-safe and never returns an
// error — an unvaulted/unknown token comes back unchanged with a nil error,
// regardless of RehydrateFallbackEnabled. See the NOTE on RehydrateString.
// This means degraded is always false via the real code path today; the
// degraded=true branches only activate if DecryptToken's contract changes.
func TestService_RehydrateString_UnknownToken(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		config.Global.RehydrateFallbackEnabled = fallback
		fakeToken := "[EMAIL_0000000000000000]"
		svc := newTestService(t)

		out, degraded, err := svc.RehydrateString(fakeToken)
		if err != nil {
			t.Errorf("fallback=%v: expected no error (DecryptToken is fail-safe), got: %v", fallback, err)
		}
		if degraded {
			t.Errorf("fallback=%v: expected degraded=false (DecryptToken never errors today)", fallback)
		}
		if out != fakeToken {
			t.Errorf("fallback=%v: expected unknown token returned unchanged, got %q", fallback, out)
		}
	}
	config.Global.RehydrateFallbackEnabled = false
}

func TestService_RedactAndRehydrateInterface_RoundTrip(t *testing.T) {
	svc := newTestService(t)

	data := map[string]interface{}{"note": "contact me at bob@example.com"}
	redacted, err := svc.RedactInterface(data, "test-actor")
	if err != nil {
		t.Fatalf("RedactInterface: %v", err)
	}
	redactedMap, ok := redacted.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", redacted)
	}
	if strings.Contains(redactedMap["note"].(string), "bob@example.com") {
		t.Fatalf("expected email to be redacted, got %v", redacted)
	}

	rehydrated, degraded, err := svc.RehydrateInterface(redacted)
	if err != nil {
		t.Fatalf("RehydrateInterface: %v", err)
	}
	if degraded {
		t.Error("expected degraded=false on a successful rehydration")
	}
	rehydratedMap, ok := rehydrated.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", rehydrated)
	}
	if !strings.Contains(rehydratedMap["note"].(string), "bob@example.com") {
		t.Errorf("expected original email back, got %v", rehydrated)
	}
}
