package refinery

import (
	"regexp"

	"github.com/ocultar-dev/ocultar/pkg/config"
)

// tier0DictionaryShield applies the Dynamic Exclusion Dictionaries (Tier 0).
//
// It first pre-computes structural PII spans (emails, URLs) to protect them from
// dictionary-term fragmentation. Without this guard, a dictionary term like "trejos"
// replaces the name fragment inside "e.trejos@gmail.com" before the email regex runs,
// breaking the address into "e.[PERSON_VIP_...]@gmail.com" and causing a partial PII leak.
func tier0DictionaryShield(e *Refinery, refined, actor string) (string, error) {
	structuralPIIRe := regexp.MustCompile(`(?i)\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b|https?://[^\s"<>\{\}\[\]\\]+`)
	structuralSpans := structuralPIIRe.FindAllStringIndex(refined, -1)

	var err error
	for _, dictRule := range config.Global.Dictionaries {
		for _, term := range dictRule.Terms {
			refined, err = e.applyReplacementProtected(refined, term, dictRule.Type, "dictionary", actor, structuralSpans)
			if err != nil {
				return "", err
			}
		}
	}
	return refined, nil
}
