// Package gateway holds the redact/rehydrate logic shared by apps/proxy and
// apps/sombra. Both previously reimplemented this independently, including
// the config.Global.RehydrateFallbackEnabled branch — this package is the
// single place that decision now lives.
package gateway

import (
	"github.com/ocultar-dev/ocultar/pkg/config"
	"github.com/ocultar-dev/ocultar/pkg/proxy"
	"github.com/ocultar-dev/ocultar/pkg/refinery"
	"github.com/ocultar-dev/ocultar/vault"
)

// Service wraps the refinery + vault pair every redact/rehydrate caller needs.
type Service struct {
	Eng       *refinery.Refinery
	Vault     vault.Provider
	MasterKey []byte
}

// New constructs a Service. eng, v, and masterKey must already be the
// caller's fully-initialized instances — Service does not own their lifecycle.
func New(eng *refinery.Refinery, v vault.Provider, masterKey []byte) *Service {
	return &Service{Eng: eng, Vault: v, MasterKey: masterKey}
}

// RedactString runs Tier 0-3 PII redaction over a plain string.
func (s *Service) RedactString(text, actor string) (string, error) {
	return s.Eng.RefineString(text, actor, nil)
}

// RedactInterface runs Tier 0-3 PII redaction over a decoded JSON structure.
func (s *Service) RedactInterface(data interface{}, actor string) (interface{}, error) {
	return s.Eng.ProcessInterface(data, actor)
}

// RehydrateString resolves vault tokens in text back to plaintext.
//
// If the underlying rehydration fails, behavior depends on
// config.Global.RehydrateFallbackEnabled: when true, it returns the
// original (still-tokenized) text with degraded=true and a nil error,
// so the caller can complete the request without leaking partial state;
// when false (the default), it returns the error and degraded=true, and
// the caller must fail the request (strict data-loss protection).
// degraded is true whenever the underlying rehydration errored, regardless
// of which branch was taken — callers use it to log/instrument the failure
// even when the request itself was allowed to proceed.
//
// NOTE: as of this writing, refinery.DecryptToken is deliberately fail-safe
// and never returns a non-nil error — on lookup-miss or decrypt failure it
// logs a warning and returns the token unchanged. That makes this method's
// error/degraded path currently unreachable via the real code path; it
// exists so proxy and sombra react identically the moment that changes,
// rather than silently diverging again.
func (s *Service) RehydrateString(text string) (out string, degraded bool, err error) {
	out, rehydrateErr := proxy.RehydrateString(s.Vault, s.MasterKey, text)
	if rehydrateErr == nil {
		return out, false, nil
	}
	if config.Global.RehydrateFallbackEnabled {
		return text, true, nil
	}
	return "", true, rehydrateErr
}

// RehydrateInterface resolves vault tokens in a decoded JSON structure back
// to plaintext. See RehydrateString for the fallback semantics.
func (s *Service) RehydrateInterface(data interface{}) (out interface{}, degraded bool, err error) {
	out, rehydrateErr := proxy.RehydrateInterface(s.Vault, s.MasterKey, data)
	if rehydrateErr == nil {
		return out, false, nil
	}
	if config.Global.RehydrateFallbackEnabled {
		return data, true, nil
	}
	return nil, true, rehydrateErr
}
