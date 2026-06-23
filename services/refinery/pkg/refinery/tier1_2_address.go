package refinery

import "strings"

// tier12AddressShield runs heuristic address detection.
func tier12AddressShield(e *Refinery, refined, actor string) (string, error) {
	if !(len(refined) > 10 && (strings.ContainsAny(refined, "0123456789") || containsAnyLower(refined, "rue", "calle", "street", "ave", "road", "str."))) {
		return refined, nil
	}
	var err error
	refined, err = parseAndReplaceWithErr(refined, ParseAndReplaceAddressesRaw, func(match string, start, end int) (string, error) {
		return e.getOrSetSecureTokenLoc(match, "ADDRESS", "address", start, end, actor)
	})
	if err != nil {
		return "", err
	}
	return refined, nil
}
