package refinery

import (
	"log"
	"strings"
)

// tier11PhoneShield runs phone-number detection AFTER the Tier 1 PII registry
// scan. This ensures that digit sequences already claimed by national IDs/SSNs
// are not misidentified as phone numbers.
func tier11PhoneShield(e *Refinery, refined, actor string) (string, error) {
	if !strings.ContainsAny(refined, "0123456789") || e.isFullyTokenised(refined) {
		return refined, nil
	}
	var phoneErr error
	refined, phoneErr = parseAndReplaceWithErr(refined, ParseAndReplacePhonesRaw, func(match string, start, end int) (string, error) {
		log.Printf("[DEBUG] Tier 1.1 Phone hit: %s", match)
		return e.getOrSetSecureTokenLoc(match, "PHONE", "phone", start, end, actor)
	})
	if phoneErr != nil {
		return "", phoneErr
	}
	return refined, nil
}
