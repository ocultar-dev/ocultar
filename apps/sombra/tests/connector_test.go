package connector_test

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/ocultar-dev/ocultar/apps/sombra/pkg/connector"
)

// buildDocx assembles a minimal in-memory .docx (a zip archive containing
// only word/document.xml) so tests don't depend on a binary fixture file.
func buildDocx(t *testing.T, documentXML string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create("word/document.xml")
	if err != nil {
		t.Fatalf("zip create: %v", err)
	}
	if _, err := f.Write([]byte(documentXML)); err != nil {
		t.Fatalf("zip write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func TestFileConnector_Fetch(t *testing.T) {
	policy := connector.DataPolicy{
		MaxBodyBytes: 1024,
	}
	fc := connector.NewFileConnector("file", policy)

	t.Run("In-memory raw body CSV", func(t *testing.T) {
		req := connector.FetchRequest{
			RawBody:     []byte("name,email\nJohn Doe,john@test.com"),
			ContentType: "text/csv",
		}
		resp, err := fc.Fetch(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.ContentType != "text/plain" {
			t.Errorf("expected plain text, got %q", resp.ContentType)
		}

		bodyStr := string(resp.Body)
		if !strings.Contains(bodyStr, "Record 1") || !strings.Contains(bodyStr, "john@test.com") {
			t.Errorf("failed to normalise CSV: %s", bodyStr)
		}
	})

	t.Run("Size limit exceeded", func(t *testing.T) {
		req := connector.FetchRequest{
			RawBody: make([]byte, 2048),
		}
		_, err := fc.Fetch(context.Background(), req)
		if err == nil || !strings.Contains(err.Error(), "exceeds policy limit") {
			t.Errorf("expected size limit error, got %v", err)
		}
	})
}

func TestFileConnector_Docx(t *testing.T) {
	policy := connector.DataPolicy{MaxBodyBytes: 10485760}
	fc := connector.NewFileConnector("file", policy)

	t.Run("Extracts plain text from document.xml", func(t *testing.T) {
		xml := `<w:document><w:body><w:p><w:r><w:t>Contact Jane Doe at jane@example.com</w:t></w:r></w:p></w:body></w:document>`
		req := connector.FetchRequest{
			RawBody:     buildDocx(t, xml),
			ContentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		}
		resp, err := fc.Fetch(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		body := string(resp.Body)
		if !strings.Contains(body, "jane@example.com") {
			t.Errorf("expected extracted text to contain email, got %q", body)
		}
		if strings.Contains(body, "<w:") {
			t.Errorf("expected XML tags to be stripped, got %q", body)
		}
	})

	t.Run("Auto-detects docx from zip magic bytes", func(t *testing.T) {
		xml := `<w:document><w:body><w:p><w:r><w:t>plain text body</w:t></w:r></w:p></w:body></w:document>`
		req := connector.FetchRequest{RawBody: buildDocx(t, xml)} // no ContentType given
		resp, err := fc.Fetch(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(string(resp.Body), "plain text body") {
			t.Errorf("expected detected docx content, got %q", resp.Body)
		}
	})

	t.Run("Rejects decompression bomb beyond the document.xml size cap", func(t *testing.T) {
		// 65 MiB of highly-compressible filler collapses to a tiny zip, well
		// under the connector's upload size limit, but must still be rejected
		// once decompressed past the document.xml cap.
		huge := strings.Repeat("A", 65<<20)
		req := connector.FetchRequest{
			RawBody:     buildDocx(t, huge),
			ContentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		}
		_, err := fc.Fetch(context.Background(), req)
		if err == nil || !strings.Contains(err.Error(), "zip bomb") {
			t.Errorf("expected zip bomb size-limit error, got %v", err)
		}
	})
}

func TestAPIConnector_Fetch(t *testing.T) {
	// Create a mock API server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-secret-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if r.URL.Path == "/v1/accounts/123" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"account_id": "123", "balance": 1000}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	cfg := connector.APIConnectorConfig{
		Name:     "mock-bank",
		Endpoint: ts.URL + "/v1/accounts",
		AuthType: "bearer",
		KeyEnv:   "MOCK_BANK_KEY",
		Policy:   connector.DataPolicy{},
	}
	ac := connector.NewAPIConnector(cfg)

	t.Run("Missing auth token", func(t *testing.T) {
		os.Unsetenv("MOCK_BANK_KEY")
		_, err := ac.Fetch(context.Background(), connector.FetchRequest{SourceID: "123"})
		if err == nil {
			t.Error("expected auth error")
		}
	})

	t.Run("Successful authenticated fetch", func(t *testing.T) {
		os.Setenv("MOCK_BANK_KEY", "test-secret-token")
		defer os.Unsetenv("MOCK_BANK_KEY")

		resp, err := ac.Fetch(context.Background(), connector.FetchRequest{SourceID: "123"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		bodyStr := string(resp.Body)
		if !strings.Contains(bodyStr, "1000") {
			t.Errorf("expected balance 1000 in response, got %s", bodyStr)
		}
	})
}
