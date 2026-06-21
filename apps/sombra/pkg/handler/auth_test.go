package handler

// auth_test.go — unit tests for the two auth surfaces in Sombra: JWT Bearer
// validation in extractActor (handler.go) and Slack request-signature
// verification in verifySlackSignature (slack_app.go).

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/ocultar-dev/ocultar/pkg/config"
)

func withJWTSecret(t *testing.T, secret string) {
	t.Helper()
	original := config.Global.JWTSecret
	config.Global.JWTSecret = secret
	t.Cleanup(func() { config.Global.JWTSecret = original })
}

func signHS256(t *testing.T, secret string, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign test token: %v", err)
	}
	return signed
}

func TestExtractActor_ValidToken(t *testing.T) {
	withJWTSecret(t, "test-secret")

	tok := signHS256(t, "test-secret", jwt.MapClaims{
		"sub": "alice@example.com",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+tok)

	g := &Gateway{}
	if actor := g.extractActor(r); actor != "alice@example.com" {
		t.Errorf("want actor %q, got %q", "alice@example.com", actor)
	}
}

func TestExtractActor_FallsBackToEmailClaim(t *testing.T) {
	withJWTSecret(t, "test-secret")

	tok := signHS256(t, "test-secret", jwt.MapClaims{
		"email": "bob@example.com",
		"exp":   time.Now().Add(time.Hour).Unix(),
	})

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+tok)

	g := &Gateway{}
	if actor := g.extractActor(r); actor != "bob@example.com" {
		t.Errorf("want actor %q, got %q", "bob@example.com", actor)
	}
}

func TestExtractActor_ExpiredTokenRejected(t *testing.T) {
	withJWTSecret(t, "test-secret")

	tok := signHS256(t, "test-secret", jwt.MapClaims{
		"sub": "alice@example.com",
		"exp": time.Now().Add(-time.Hour).Unix(),
	})

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+tok)

	g := &Gateway{}
	if actor := g.extractActor(r); actor != "" {
		t.Errorf("expired token should yield empty actor, got %q", actor)
	}
}

func TestExtractActor_MissingExpClaimRejected(t *testing.T) {
	withJWTSecret(t, "test-secret")

	// No "exp" claim at all — jwt.WithExpirationRequired() must reject this.
	tok := signHS256(t, "test-secret", jwt.MapClaims{
		"sub": "alice@example.com",
	})

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+tok)

	g := &Gateway{}
	if actor := g.extractActor(r); actor != "" {
		t.Errorf("token without exp claim should yield empty actor, got %q", actor)
	}
}

func TestExtractActor_WrongSigningMethodRejected(t *testing.T) {
	withJWTSecret(t, "test-secret")

	// HS256-validated extractActor must reject a token claiming "none" / any
	// non-HMAC algorithm, even if the claims would otherwise be valid.
	token := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"sub": "attacker@example.com",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	signed, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("failed to sign none-alg token: %v", err)
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+signed)

	g := &Gateway{}
	if actor := g.extractActor(r); actor != "" {
		t.Errorf("none-algorithm token should be rejected, got actor %q", actor)
	}
}

func TestExtractActor_WrongSecretRejected(t *testing.T) {
	withJWTSecret(t, "test-secret")

	tok := signHS256(t, "wrong-secret", jwt.MapClaims{
		"sub": "alice@example.com",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+tok)

	g := &Gateway{}
	if actor := g.extractActor(r); actor != "" {
		t.Errorf("token signed with wrong secret should be rejected, got actor %q", actor)
	}
}

func TestExtractActor_MissingAuthorizationHeader(t *testing.T) {
	withJWTSecret(t, "test-secret")

	r := httptest.NewRequest(http.MethodGet, "/", nil)

	g := &Gateway{}
	if actor := g.extractActor(r); actor != "" {
		t.Errorf("missing Authorization header should yield empty actor, got %q", actor)
	}
}

func TestExtractActor_MalformedAuthorizationHeader(t *testing.T) {
	withJWTSecret(t, "test-secret")

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "not-a-bearer-token-at-all")

	g := &Gateway{}
	if actor := g.extractActor(r); actor != "" {
		t.Errorf("malformed Authorization header should yield empty actor, got %q", actor)
	}
}

func TestExtractActor_DevModeWhenSecretUnset(t *testing.T) {
	withJWTSecret(t, "")

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer anything-the-caller-supplies")

	g := &Gateway{}
	// Must return a fixed dev actor, never echo the caller-supplied token
	// string back into the audit log (that would let a caller spoof identity).
	if actor := g.extractActor(r); actor != "dev-anonymous" {
		t.Errorf("want dev-anonymous in insecure dev mode, got %q", actor)
	}
}

func TestVerifySlackSignature_Valid(t *testing.T) {
	secret := "slack-test-secret"
	body := []byte(`{"type":"event_callback"}`)
	ts := fmt.Sprintf("%d", time.Now().Unix())

	sig := computeSlackSignature(secret, ts, body)

	if !verifySlackSignature(secret, ts, sig, body) {
		t.Error("expected valid signature to be accepted")
	}
}

func TestVerifySlackSignature_WrongSecretRejected(t *testing.T) {
	body := []byte(`{"type":"event_callback"}`)
	ts := fmt.Sprintf("%d", time.Now().Unix())

	sig := computeSlackSignature("correct-secret", ts, body)

	if verifySlackSignature("wrong-secret", ts, sig, body) {
		t.Error("signature computed with a different secret must be rejected")
	}
}

func TestVerifySlackSignature_TamperedBodyRejected(t *testing.T) {
	secret := "slack-test-secret"
	ts := fmt.Sprintf("%d", time.Now().Unix())

	sig := computeSlackSignature(secret, ts, []byte(`{"type":"event_callback"}`))

	tamperedBody := []byte(`{"type":"event_callback","injected":"true"}`)
	if verifySlackSignature(secret, ts, sig, tamperedBody) {
		t.Error("signature must not validate against a tampered body")
	}
}

func TestVerifySlackSignature_StaleTimestampRejected(t *testing.T) {
	secret := "slack-test-secret"
	body := []byte(`{"type":"event_callback"}`)
	staleTs := fmt.Sprintf("%d", time.Now().Add(-10*time.Minute).Unix())

	sig := computeSlackSignature(secret, staleTs, body)

	if verifySlackSignature(secret, staleTs, sig, body) {
		t.Error("a timestamp older than 5 minutes must be rejected (replay protection)")
	}
}

func TestVerifySlackSignature_FutureTimestampRejected(t *testing.T) {
	secret := "slack-test-secret"
	body := []byte(`{"type":"event_callback"}`)
	futureTs := fmt.Sprintf("%d", time.Now().Add(10*time.Minute).Unix())

	sig := computeSlackSignature(secret, futureTs, body)

	if verifySlackSignature(secret, futureTs, sig, body) {
		t.Error("a timestamp more than 5 minutes in the future must be rejected")
	}
}

func TestVerifySlackSignature_MalformedTimestampRejected(t *testing.T) {
	secret := "slack-test-secret"
	body := []byte(`{"type":"event_callback"}`)

	if verifySlackSignature(secret, "not-a-unix-timestamp", "v0=anything", body) {
		t.Error("a non-numeric timestamp must be rejected")
	}
}

// computeSlackSignature mirrors Slack's own signing algorithm so tests can
// produce a valid v0 signature without depending on verifySlackSignature's
// internals (avoids the test being a tautology against the code under test).
func computeSlackSignature(secret, timestamp string, body []byte) string {
	sigBase := fmt.Sprintf("v0:%s:%s", timestamp, string(body))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(sigBase))
	return "v0=" + hex.EncodeToString(mac.Sum(nil))
}
