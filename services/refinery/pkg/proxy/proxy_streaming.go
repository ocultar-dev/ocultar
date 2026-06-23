package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/ocultar-dev/ocultar/pkg/refinery"
	"github.com/ocultar-dev/ocultar/vault"
)

func (h *Handler) redactBody(body []byte, actor string) ([]byte, bool, error) {
	if len(body) == 0 {
		return body, false, nil
	}

	bodyStr := string(body)
	if strings.Contains(bodyStr, "%7B") && strings.Contains(bodyStr, "%22") {
		return nil, false, fmt.Errorf("obfuscated payload detected: url-encoded JSON")
	}
	if (strings.HasPrefix(strings.TrimSpace(bodyStr), "ey") || strings.HasPrefix(strings.TrimSpace(bodyStr), "eyJ")) &&
		!strings.Contains(bodyStr, " ") && len(bodyStr) > 50 {
		return nil, false, fmt.Errorf("obfuscated payload detected: base64/JWT")
	}

	h.eng.ResetHits()

	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var outBuf bytes.Buffer

	if err := streamRefineJSON(dec, h.eng, actor, &outBuf); err == nil {
		if _, err := dec.Token(); err == io.EOF {
			report := h.eng.GenerateReport(1)
			return outBuf.Bytes(), report.TotalCount > 0, nil
		}
	}

	lines := strings.Split(string(body), "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			refined, refErr := h.gateway.RedactString(line, actor)
			if refErr != nil {
				return nil, false, refErr
			}
			lines[i] = refined
		}
	}
	report := h.eng.GenerateReport(1)
	return []byte(strings.Join(lines, "\n")), report.TotalCount > 0, nil
}

// rehydrateBody resolves vault tokens in body back to plaintext. degraded is
// true whenever the underlying rehydration encountered an error — even if
// config.Global.RehydrateFallbackEnabled let the request continue with the
// still-tokenized body instead of failing — so ServeHTTP can still audit-log
// the failure distinctly from a clean success.
func (h *Handler) rehydrateBody(body []byte, contentType string) ([]byte, bool, error) {
	if len(body) == 0 || !refinery.ContainsTokensInBody(body) {
		return body, false, nil
	}

	isJSON := strings.Contains(contentType, "application/json") ||
		strings.Contains(contentType, "text/json")

	if isJSON {
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.UseNumber()
		var outBuf bytes.Buffer
		if err := streamRehydrateJSON(dec, h.vault, h.masterKey, &outBuf); err == nil {
			if _, err := dec.Token(); err == io.EOF {
				return outBuf.Bytes(), false, nil
			}
		}
	}

	res, degraded, err := h.gateway.RehydrateString(string(body))
	if err != nil {
		return nil, degraded, err
	}
	return []byte(res), degraded, nil
}

func streamRefineJSON(dec *json.Decoder, eng *refinery.Refinery, actor string, out *bytes.Buffer) error {
	t, err := dec.Token()
	if err != nil {
		return err
	}

	switch v := t.(type) {
	case json.Delim:
		out.WriteString(v.String())
		switch v {
		case '{':
			first := true
			for dec.More() {
				if !first {
					out.WriteString(",")
				}
				first = false
				kt, err := dec.Token()
				if err != nil {
					return err
				}
				b, _ := json.Marshal(kt)
				out.Write(b)
				out.WriteString(":")
				if err := streamRefineJSON(dec, eng, actor, out); err != nil {
					return err
				}
			}
			et, err := dec.Token()
			if err != nil {
				return err
			}
			out.WriteString(et.(json.Delim).String())
		case '[':
			first := true
			for dec.More() {
				if !first {
					out.WriteString(",")
				}
				first = false
				if err := streamRefineJSON(dec, eng, actor, out); err != nil {
					return err
				}
			}
			et, err := dec.Token()
			if err != nil {
				return err
			}
			out.WriteString(et.(json.Delim).String())
		}
	case string:
		sanitised, err := eng.ProcessInterface(v, actor)
		if err != nil {
			return err
		}
		b, _ := json.Marshal(sanitised)
		out.Write(b)
	default:
		b, _ := json.Marshal(v)
		out.Write(b)
	}
	return nil
}

func streamRehydrateJSON(dec *json.Decoder, v vault.Provider, masterKey []byte, out *bytes.Buffer) error {
	t, err := dec.Token()
	if err != nil {
		return err
	}

	switch val := t.(type) {
	case json.Delim:
		out.WriteString(val.String())
		switch val {
		case '{':
			first := true
			for dec.More() {
				if !first {
					out.WriteString(",")
				}
				first = false
				kt, err := dec.Token()
				if err != nil {
					return err
				}
				b, _ := json.Marshal(kt)
				out.Write(b)
				out.WriteString(":")
				if err := streamRehydrateJSON(dec, v, masterKey, out); err != nil {
					return err
				}
			}
			et, err := dec.Token()
			if err != nil {
				return err
			}
			out.WriteString(et.(json.Delim).String())
		case '[':
			first := true
			for dec.More() {
				if !first {
					out.WriteString(",")
				}
				first = false
				if err := streamRehydrateJSON(dec, v, masterKey, out); err != nil {
					return err
				}
			}
			et, err := dec.Token()
			if err != nil {
				return err
			}
			out.WriteString(et.(json.Delim).String())
		}
	case string:
		hydrated, err := refinery.RehydrateString(v, masterKey, val)
		if err != nil {
			return err
		}
		b, _ := json.Marshal(hydrated)
		out.Write(b)
	default:
		b, _ := json.Marshal(val)
		out.Write(b)
	}
	return nil
}
