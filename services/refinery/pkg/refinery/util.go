package refinery

import (
	"encoding/base64"
	"log"
	"regexp"
	"strings"

	"github.com/ocultar-dev/ocultar/pkg/config"
)

var tokenPattern = regexp.MustCompile(`\[[A-Z_]+_[0-9a-f]+\]`)

func containsAnyLower(s string, keywords ...string) bool {
	lower := strings.ToLower(s)
	for _, k := range keywords {
		if strings.Contains(lower, k) {
			return true
		}
	}
	return false
}

// isFullyTokenised checks if a string consists entirely of redacted tokens and formatting characters.
func (e *Refinery) isFullyTokenised(s string) bool {
	stripped := tokenPattern.ReplaceAllString(s, "")
	return regexp.MustCompile(`^[\s\p{P}\p{Z}>*_|\-=+#@~]+$`).MatchString(stripped)
}

// applyReplacement replaces exact target strings with vaulted tokens,
// skipping any match that falls entirely inside an already-tokenized span.
// This prevents SLM items (e.g. "Siret") from clobbering existing Tier-1 tokens
// that happen to contain the same substring (e.g. "[FRANCE_SIRET_NUMBER_…]").
func (e *Refinery) applyReplacement(line, target, piiType, source string, actor string) (string, error) {
	target = strings.TrimSpace(target)
	if len(target) < 3 {
		return line, nil
	}

	target = strings.ToValidUTF8(target, "")
	if len(target) < 3 {
		return line, nil
	}

	re, err := regexp.Compile("(?i)" + regexp.QuoteMeta(target))
	if err != nil {
		log.Printf("[WARN] applyReplacement: skipping invalid pattern for %q: %v", target, err)
		return line, nil
	}

	tokenRanges := tokenPattern.FindAllStringIndex(line, -1)
	matches := re.FindAllStringIndex(line, -1)
	if len(matches) == 0 {
		return line, nil
	}

	var out strings.Builder
	lastPos := 0
	for _, match := range matches {
		start, end := match[0], match[1]
		// Skip if the match is fully contained within an existing token
		inside := false
		for _, t := range tokenRanges {
			if start >= t[0] && end <= t[1] {
				inside = true
				break
			}
		}
		out.WriteString(line[lastPos:start])
		if inside {
			out.WriteString(line[start:end])
		} else {
			token, tokenErr := e.getOrSetSecureTokenLoc(line[start:end], piiType, source, start, end, actor)
			if tokenErr != nil {
				return "", tokenErr
			}
			out.WriteString(token)
		}
		lastPos = end
	}
	out.WriteString(line[lastPos:])
	return out.String(), nil
}

// applyReplacementProtected is like applyReplacement but also skips matches
// that overlap with the provided protectedSpans (e.g. pre-computed email/URL ranges).
// Used by Tier 0 to prevent dictionary terms from fragmenting structural PII before
// Tier 1 regex can claim the full match.
func (e *Refinery) applyReplacementProtected(line, target, piiType, source string, actor string, protectedSpans [][]int) (string, error) {
	target = strings.TrimSpace(target)
	if len(target) < 3 {
		return line, nil
	}

	target = strings.ToValidUTF8(target, "")
	if len(target) < 3 {
		return line, nil
	}

	re, err := regexp.Compile("(?i)" + regexp.QuoteMeta(target))
	if err != nil {
		log.Printf("[WARN] applyReplacementProtected: skipping invalid pattern for %q: %v", target, err)
		return line, nil
	}

	tokenRanges := tokenPattern.FindAllStringIndex(line, -1)
	matches := re.FindAllStringIndex(line, -1)
	if len(matches) == 0 {
		return line, nil
	}

	var out strings.Builder
	lastPos := 0
	for _, match := range matches {
		start, end := match[0], match[1]
		skip := false
		// Skip if inside an existing vault token
		for _, t := range tokenRanges {
			if start >= t[0] && end <= t[1] {
				skip = true
				break
			}
		}
		// Skip if inside a protected structural PII span (email, URL)
		if !skip {
			for _, p := range protectedSpans {
				if start >= p[0] && end <= p[1] {
					skip = true
					break
				}
			}
		}
		out.WriteString(line[lastPos:start])
		if skip {
			out.WriteString(line[start:end])
		} else {
			token, tokenErr := e.getOrSetSecureTokenLoc(line[start:end], piiType, source, start, end, actor)
			if tokenErr != nil {
				return "", tokenErr
			}
			out.WriteString(token)
		}
		lastPos = end
	}
	out.WriteString(line[lastPos:])
	return out.String(), nil
}

// replaceAllStringFuncErr applies a replacement function that can return an error
func replaceAllStringFuncErr(re *regexp.Regexp, input string, repl func(string) (string, error)) (string, error) {
	matches := re.FindAllStringIndex(input, -1)
	if len(matches) == 0 {
		return input, nil
	}

	var out strings.Builder
	lastPos := 0
	for _, match := range matches {
		start, end := match[0], match[1]
		out.WriteString(input[lastPos:start])

		r, err := repl(input[start:end])
		if err != nil {
			return "", err
		}
		out.WriteString(r)
		lastPos = end
	}
	out.WriteString(input[lastPos:])
	return out.String(), nil
}

// Helper types for migrating address/phone parsers to support errors
func parseAndReplaceWithErr(input string, extractor func(string) [][]int, repl func(match string, start, end int) (string, error)) (string, error) {
	matches := extractor(input)
	if len(matches) == 0 {
		return input, nil
	}

	tokens := tokenPattern.FindAllStringIndex(input, -1)

	var out strings.Builder
	lastPos := 0
	for _, match := range matches {
		start, end := match[0], match[1]

		// Ensure we aren't carving into already tokenized variables
		overlap := false
		for _, t := range tokens {
			if start < t[1] && end > t[0] {
				overlap = true
				break
			}
		}
		if overlap {
			continue
		}

		// If matches overlap due to nested tokens or bad indices, skip
		if start < lastPos {
			continue
		}

		out.WriteString(input[lastPos:start])

		r, err := repl(input[start:end], start, end)
		if err != nil {
			return "", err
		}
		out.WriteString(r)
		lastPos = end
	}
	out.WriteString(input[lastPos:])
	return out.String(), nil
}

// decodeBase64 attempts to decode standard base64 strings, and falls back to raw
// unpadded decoding to catch obfuscated PII.
func decodeBase64(s string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err == nil {
		return decoded, nil
	}
	return base64.RawStdEncoding.DecodeString(s)
}

func getComplianceMapping(piiType string) string {
	if config.Global.RegulatoryPolicy == nil {
		return "GENERAL_PII"
	}

	mappings, ok := config.Global.RegulatoryPolicy["mappings"].(map[string]interface{})
	if !ok {
		return "GENERAL_PII"
	}

	if m, ok := mappings[piiType].(map[string]interface{}); ok {
		if reg, ok := m["regulation"].(string); ok {
			return reg
		}
	}

	return "GENERAL_PII"
}
