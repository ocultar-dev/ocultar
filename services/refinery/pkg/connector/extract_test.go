package connector

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

// buildZipEntry assembles a minimal in-memory OOXML-style zip containing a
// single named entry, so tests don't depend on binary fixture files.
func buildZipEntry(t *testing.T, name, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create(name)
	if err != nil {
		t.Fatalf("zip create: %v", err)
	}
	if _, err := f.Write([]byte(content)); err != nil {
		t.Fatalf("zip write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func TestExtractText_Docx(t *testing.T) {
	xml := `<w:document><w:body><w:p><w:r><w:t>Contact Jane Doe at jane@example.com</w:t></w:r></w:p></w:body></w:document>`
	data := buildZipEntry(t, "word/document.xml", xml)

	text, err := extractText("report.docx", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(text, "jane@example.com") {
		t.Errorf("expected extracted text to contain email, got %q", text)
	}
}

func TestExtractText_Pptx(t *testing.T) {
	xml := `<a:txBody><a:p><a:r><a:t>Quarterly results for Acme Corp</a:t></a:r></a:p></a:txBody>`
	data := buildZipEntry(t, "ppt/slides/slide1.xml", xml)

	text, err := extractText("deck.pptx", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(text, "Acme Corp") {
		t.Errorf("expected extracted text to contain slide text, got %q", text)
	}
}

func TestExtractText_Xlsx(t *testing.T) {
	xml := `<sst><si><t>Account Number</t></si><si><t>123456789</t></si></sst>`
	data := buildZipEntry(t, "xl/sharedStrings.xml", xml)

	text, err := extractText("ledger.xlsx", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(text, "123456789") {
		t.Errorf("expected extracted text to contain shared string, got %q", text)
	}
}

func TestExtractZipXML_RejectsDecompressionBomb(t *testing.T) {
	// 65 MiB of highly-compressible filler collapses to a tiny zip entry but
	// declares its true decompressed size in the zip directory — the entry
	// must be skipped before any decompression happens.
	huge := strings.Repeat("A", 65<<20)
	xml := `<w:document><w:body><w:p><w:r><w:t>` + huge + `</w:t></w:r></w:p></w:body></w:document>`
	data := buildZipEntry(t, "word/document.xml", xml)

	text, err := extractText("bomb.docx", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "" {
		t.Errorf("expected oversized entry to be skipped entirely, got %d bytes of text", len(text))
	}
}
