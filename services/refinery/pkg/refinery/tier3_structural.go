package refinery

import (
	"regexp"
	"strings"
)

// Generalized Multilingual Heuristics (Phase 1)
var conjunctionRegex = regexp.MustCompile(`(?i)\b(ET|AND|Y|UND|CON|WITH|&)\b`)
var profTitleRegex = regexp.MustCompile(`(?i)\b(DR|DOCTEUR|PROF|MME|MLLE|SR|SRA|HR|FR|MAÎTRE|AVOCAT)\b`)
var capitalizedWordRegex = regexp.MustCompile(`\b[A-ZÀ-Ÿ][A-ZÀ-Ÿa-zà-ÿ\-]{1,20}\b`)
var possessiveRegex = regexp.MustCompile(`(?i)\b[A-ZÀ-Ÿ][a-zà-ÿ\-]{1,20}['’]s\b`)
var semanticTriggerRegex = regexp.MustCompile(`(?i)\b(DIVORCE|MARIAGE|WEDDING|AVOCAT|LAWYER|HOSPITAL|CLINIQUE|TREATMENT|TRAITEMENT|CAMPAIGN|POLITICAL|CAMPAGNE|PEA)\b`)

// applyStructuralHeuristics executes generalized rules for entity expansion and linkages.
func (e *Refinery) applyStructuralHeuristics(input string, actor string) (string, error) {
	refined := input

	// 1. Semantic Scrubbing: [TRIGGER] [SUBJECT]
	// Done first to ensure it runs even if no tokens are present.
	refined, _ = replaceAllStringFuncErr(semanticTriggerRegex, refined, func(match string) (string, error) {
		// Redact the trigger itself to hide the sensitive category
		return e.getOrSetSecureToken(match, "SENSITIVE_EVENT", "structural", actor)
	})

	// 2. Professional Shield: [TITLE] [CAPITALIZED_NAME]
	refined, _ = replaceAllStringFuncErr(profTitleRegex, refined, func(match string) (string, error) {
		// Lookahead for capitalized words
		remaining := refined[strings.Index(refined, match)+len(match):]
		words := strings.Fields(remaining)
		if len(words) > 0 && capitalizedWordRegex.MatchString(words[0]) {
			// Redact the title and the following word(s)
			expanded := match + " " + words[0]
			// Greedy expansion for multi-part names after title
			for j := 1; j < len(words); j++ {
				if capitalizedWordRegex.MatchString(words[j]) {
					expanded += " " + words[j]
				} else {
					break
				}
			}
			return e.getOrSetSecureToken(expanded, "HEALTH_ENTITY", "structural", actor)
		}
		return match, nil // No expansion
	})

	// 3. Possessive Catch: [CAPITALIZED_WORD]'s
	refined, _ = replaceAllStringFuncErr(possessiveRegex, refined, func(match string) (string, error) {
		return e.getOrSetSecureToken(match, "PERSON", "structural", actor)
	})

	// 4. Greedy Neighborhood & Conjunctions: [TOKEN] [CONJUNCTION] [CAPITALIZED_NAME]
	tokens := tokenPattern.FindAllStringIndex(refined, -1)
	if len(tokens) == 0 {
		return refined, nil
	}

	var out strings.Builder
	lastPos := 0
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		start, end := t[0], t[1]
		if start < lastPos {
			continue // Already processed in an expanded token
		}

		out.WriteString(refined[lastPos:start])

		currentToken := refined[start:end]
		lookaheadEnd := end

		// Iterative Greedy Expansion
		for {
			remaining := refined[lookaheadEnd:]
			words := strings.Fields(remaining)
			if len(words) == 0 {
				break
			}

			firstWord := words[0]
			expandedThisTurn := false

			// Case A: Conjunction linkage (e.g. [TOKEN] ET MULLER)
			if conjunctionRegex.MatchString(firstWord) && len(words) > 1 && capitalizedWordRegex.MatchString(words[1]) {
				lookaheadEnd += strings.Index(remaining, words[1]) + len(words[1])
				expandedThisTurn = true
			} else if capitalizedWordRegex.MatchString(firstWord) || possessiveRegex.MatchString(firstWord) {
				// Case B: Direct surname proximity or possessive
				lookaheadEnd += strings.Index(remaining, firstWord) + len(firstWord)
				expandedThisTurn = true
			}

			if !expandedThisTurn {
				break
			}
		}

		if lookaheadEnd > end {
			// Expansion occurred
			expandedPII := refined[start:lookaheadEnd]
			piiType := strings.Split(strings.Trim(currentToken, "[]"), "_")[0]
			newToken, err := e.getOrSetSecureToken(expandedPII, piiType, "structural", actor)
			if err != nil {
				return "", err
			}
			out.WriteString(newToken)
			lastPos = lookaheadEnd
		} else {
			out.WriteString(currentToken)
			lastPos = end
		}
	}
	out.WriteString(refined[lastPos:])
	return out.String(), nil
}
