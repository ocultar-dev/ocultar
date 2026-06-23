package refinery

import (
	"log/slog"

	"github.com/ocultar-dev/ocultar/internal/pii"
	"github.com/ocultar-dev/ocultar/pkg/config"
)

// tier1RuleEngine runs the centralized deterministic regex pipeline (SSN,
// credit cards, IBAN, etc.) and vaults every detection that doesn't overlap
// a span already claimed by an earlier tier's token.
func tier1RuleEngine(e *Refinery, refined, actor string) (string, error) {
	eng := pii.NewRefinery()
	if config.Global.AliasMapping != nil {
		eng.SetMapping(config.Global.AliasMapping)
	}

	// Scan first to identify structured PII (SSN, Credit Cards, etc.)
	detections := eng.Scan(refined)
	slog.Debug("tier 1 scan complete", "detections", len(detections))

	tokens := tokenPattern.FindAllStringIndex(refined, -1)
	refined, err := eng.Redact(refined, func(d pii.DetectionResult) (string, error) {
		overlap := false
		for _, t := range tokens {
			if d.Range.Start < t[1] && d.Range.End > t[0] {
				overlap = true
				break
			}
		}
		if overlap {
			return d.Value, nil
		}
		return e.getOrSetSecureResult(d, actor)
	})
	if err != nil {
		return "", err
	}
	return refined, nil
}
