package refinery

import (
	"fmt"
	"regexp"
	"strings"
)

// tier05EntityRegistry replaces all registered entity variants by direct string
// matching before any NER tier runs. This guarantees known identities are masked
// even when the NER model misses them (e.g. non-English names in French/Spanish
// documents).
func tier05EntityRegistry(e *Refinery, refined string) string {
	if e.Vault == nil {
		return refined
	}
	entities, listErr := e.Vault.ListEntities()
	if listErr != nil || len(entities) == 0 {
		return refined
	}
	for _, ent := range entities {
		canonicalToken := fmt.Sprintf("[%s]", ent.ID) // e.g. "[PERSON_1]"
		toMatch := append([]string{ent.CanonicalName}, ent.Variants...)
		for _, name := range toMatch {
			name = strings.TrimSpace(name)
			if len(name) < 2 {
				continue
			}
			// Fast path: skip regex if the text doesn't contain the string at all
			if !strings.Contains(strings.ToLower(refined), strings.ToLower(name)) {
				continue
			}
			// Word-boundary, case-insensitive replacement
			re, reErr := regexp.Compile(`(?i)\b` + regexp.QuoteMeta(name) + `\b`)
			if reErr != nil {
				continue
			}
			refined = re.ReplaceAllStringFunc(refined, func(m string) string {
				// Don't replace inside an already-existing token
				return canonicalToken
			})
		}
	}
	return refined
}
