package refinery

import (
	"regexp"
	"strings"
)

var greetingRegex = regexp.MustCompile(`(?m)(?i)(?:Regards|Best|Cheers|Bonjour|Hello|Hi|Dear|Sincerely|Cordialement)[,.-]*\s+([A-ZÀ-Ÿ][a-zà-ÿ]+(?:\s+[A-ZÀ-Ÿ][a-zà-ÿ]+){0,2})\b`)

var nameIntroRegex = regexp.MustCompile(`(?m)(?i)\b(?:my name is|i am|call me|this is)\s+([A-ZÀ-Ÿ][a-zà-ÿ]+(?:\s+[A-ZÀ-Ÿ][a-zà-ÿ]+){0,2})\b`)

// nameInSentenceRegex catches proper names (two+ capitalised words) referenced by
// interrogative or inquiry verbs: "where does John Galt live", "who is Jane Smith",
// "tell me about Bob Jones", "contact Sarah Lee". This extends Tier 1.5 name
// detection beyond self-disclosures to cover third-party name mentions in questions.
var nameInSentenceRegex = regexp.MustCompile(`(?i)\b(?:where(?:\s+does)?|who(?:\s+is)?|about|contact|find|email|call|meet)\s+([A-ZÀ-Ÿ][a-zà-ÿ\-]{1,20}(?:\s+[A-ZÀ-Ÿ][a-zà-ÿ\-]{1,20}){1,3})\b`)

// tier15GreetingShield catches names disclosed via salutations ("Regards, John"),
// self-introductions ("My name is Jane"), and third-party name mentions in
// interrogative sentences ("where does John Galt live"). Runs after phone/address
// parsing to avoid false-positive collisions with numeric fields.
func tier15GreetingShield(e *Refinery, refined, actor string) (string, error) {
	greetingMatches := greetingRegex.FindAllStringSubmatchIndex(refined, -1)
	nameIntroMatches := nameIntroRegex.FindAllStringSubmatchIndex(refined, -1)
	nameSentenceMatches := nameInSentenceRegex.FindAllStringSubmatchIndex(refined, -1)
	allNameMatches := append(append(greetingMatches, nameIntroMatches...), nameSentenceMatches...)

	var err error
	for _, match := range allNameMatches {
		if len(match) > 2 {
			start, end := match[2], match[3]
			nameStr := refined[start:end]
			if !strings.HasPrefix(nameStr, "[") {
				refined, err = e.applyReplacement(refined, nameStr, "PERSON", "greeting", actor)
				if err != nil {
					return "", err
				}
			}
		}
	}
	return refined, nil
}
