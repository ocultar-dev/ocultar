package refinery

import (
	"fmt"
	"log/slog"
	"strings"
)

// slmLabelBlocklist contains document/legal keywords that the SLM sometimes
// misclassifies as person names or entity values. These are structural labels,
// not PII, and must survive redaction intact.
var slmLabelBlocklist = map[string]struct{}{
	"siret": {}, "siren": {}, "tva": {}, "vat": {}, "iban": {}, "bic": {},
	"facture": {}, "invoice": {}, "ref": {}, "date": {}, "total": {},
	"psychologue": {}, "psychologist": {}, "docteur": {}, "doctor": {},
	"monsieur": {}, "madame": {}, "mr": {}, "mme": {}, "ms": {},
}

// isBlockedSLMLabel returns true if item is a blocked label keyword or a
// BPE subword fragment of one (e.g. "iret" is a suffix fragment of "siret").
func isBlockedSLMLabel(item string) bool {
	lower := strings.ToLower(strings.TrimSpace(item))
	if _, ok := slmLabelBlocklist[lower]; ok {
		return true
	}
	for label := range slmLabelBlocklist {
		if len(label) > len(lower) && (strings.HasSuffix(label, lower) || strings.HasPrefix(label, lower)) {
			return true
		}
	}
	return false
}

// tier2AINer runs the SLM NER scan (Mandatory Phase). When preScanMap is
// non-nil (the request body was already pre-scanned upstream), it replays
// those pre-computed hits instead of calling the live scanner. Otherwise it
// invokes the active scanner directly and, on failure, either fails closed
// (FailClosedOnSLMError) or degrades to Tier 1-only coverage.
func tier2AINer(e *Refinery, refined, actor string, preScanMap map[string][]string) (string, error) {
	var err error
	if preScanMap != nil {
		for piiType, items := range preScanMap {
			canonType := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(piiType), " ", "_"))
			if canonType == "" {
				continue
			}
			for _, item := range items {
				trimmed := strings.TrimSpace(item)
				if len(trimmed) < 3 || !strings.Contains(refined, trimmed) {
					continue
				}
				if isBlockedSLMLabel(trimmed) {
					continue
				}
				refined, err = e.applyReplacement(refined, trimmed, canonType, "ai-ner", actor)
				if err != nil {
					return "", err
				}
			}
		}
	} else if e.activeScanner().IsAvailable() && !e.SkipDeepScan && len(refined) > 15 && !e.isFullyTokenised(refined) {
		// Strip existing Tier-1 tokens before sending to SLM.
		// Without this, the SLM sees token content like "HEALTH_ENTITY_f62c" and
		// misclassifies the hex hashes as account numbers or person names, producing
		// double-bracket artifacts such as [[private_person_...]3b20].
		textForSLM := tokenPattern.ReplaceAllString(refined, " ")
		piiMap, slmErr := e.activeScanner().ScanForPII(textForSLM)
		if slmErr != nil {
			if e.FailClosedOnSLMError {
				return "", fmt.Errorf("SLM inference failed: %w", slmErr)
			}
			slog.Warn("Tier 2 SLM unavailable, degrading to Tier 1", "error", slmErr)
			piiMap = nil
		}
		for piiType, items := range piiMap {
			// Normalize SLM entity type to UPPERCASE so tokens are consistent with
			// Tier-1 output (e.g. "private_person" → "PRIVATE_PERSON").
			// This ensures ki!'s build_replacement_map and extract_tokens recognize
			// SLM tokens, and that tokenPattern protects them from re-processing.
			canonType := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(piiType), " ", "_"))
			if canonType == "" {
				continue
			}
			for _, item := range items {
				trimmed := strings.TrimSpace(item)
				if len(trimmed) < 3 {
					continue
				}
				if isBlockedSLMLabel(trimmed) {
					slog.Debug("Tier 2 SLM: skipping blocked label", "length", len(trimmed))
					continue
				}
				slog.Debug("Tier 2 SLM hit", "entity_type", canonType, "length", len(trimmed))
				refined, err = e.applyReplacement(refined, trimmed, canonType, "ai-ner", actor)
				if err != nil {
					return "", err
				}
			}
		}
	}
	return refined, nil
}
