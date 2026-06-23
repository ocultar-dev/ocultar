package scrubber

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/ocultar-dev/ocultar/internal/pii"
	"github.com/ocultar-dev/ocultar/pkg/refinery"
	"github.com/ocultar-dev/ocultar/vault"
)

// art9Tag is prepended to lines containing health-related data.
const art9Tag = "[ART9_HEALTH_RECORD]"

// Scrubber holds a reference to the vault and the encryption key.
type Scrubber struct {
	vault     vault.Provider
	masterKey []byte
	hmacKey   []byte
	engine    *pii.Refinery
}

// New creates a Scrubber backed by the same vault as the gateway. The HMAC
// key is derived from masterKey the same way refinery.NewRefinery does, so
// scrubber-minted tokens carry the same per-deployment guarantee as tokens
// minted by the main detection pipeline sharing this vault.
func New(v vault.Provider, masterKey []byte) (*Scrubber, error) {
	hmacKey, err := refinery.DeriveHMACKey(masterKey)
	if err != nil {
		return nil, fmt.Errorf("scrubber: derive HMAC key: %w", err)
	}
	return &Scrubber{
		vault:     v,
		masterKey: masterKey,
		hmacKey:   hmacKey,
		engine:    pii.NewRefinery(),
	}, nil
}

// Prescrub runs the rigorous deterministic EU+UK detection engine over the text.
func (s *Scrubber) Prescrub(text string) (string, error) {
	// Find all detections
	detections := s.engine.Scan(text)
	if len(detections) == 0 {
		return text, nil
	}

	// Identify lines containing health-related data for Article 9 tagging
	healthLines := make(map[int]bool)
	var lineOffsets []int
	lineOffsets = append(lineOffsets, 0)
	for i, ch := range text {
		if ch == '\n' {
			lineOffsets = append(lineOffsets, i+1)
		}
	}

	for _, d := range detections {
		if d.Entity == "HEALTH_ENTITY" {
			lineIdx := 0
			for i, off := range lineOffsets {
				if d.Range.Start >= off {
					lineIdx = i
				} else {
					break
				}
			}
			healthLines[lineIdx] = true
		}
	}

	// Redact using the centralized engine
	redactedText, err := s.engine.Redact(text, func(d pii.DetectionResult) (string, error) {
		return s.storeToken(d.Value, d.Entity)
	})
	if err != nil {
		return "", fmt.Errorf("engine redact: %w", err)
	}

	// Prepend `art9Tag` to lines that had a HEALTH_ENTITY
	if len(healthLines) > 0 {
		lines := strings.Split(redactedText, "\n")
		for i := range lines {
			if healthLines[i] {
				if !strings.HasPrefix(lines[i], art9Tag) {
					lines[i] = art9Tag + " " + lines[i]
				}
			}
		}
		redactedText = strings.Join(lines, "\n")
	}

	return redactedText, nil
}

func (s *Scrubber) storeToken(value string, piiType string) (string, error) {
	mac := hmac.New(sha256.New, s.hmacKey)
	mac.Write([]byte(value))
	h := fmt.Sprintf("%x", mac.Sum(nil))
	token := fmt.Sprintf("[%s_%s]", piiType, h[:16])
	encrypted, err := encrypt([]byte(value), s.masterKey)
	if err != nil {
		return "", err
	}
	if _, err := s.vault.StoreToken(h, token, encrypted); err != nil {
		return "", err
	}
	return token, nil
}
