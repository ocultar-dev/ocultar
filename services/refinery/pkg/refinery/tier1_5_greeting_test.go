package refinery

import (
	"strings"
	"testing"
)

// TestTier15GreetingShield_NoFalsePositiveOnLowercaseWords guards against a
// regression where a blanket (?i) on the trigger-word alternation also folded
// the [A-ZÀ-Ÿ] capitalised-first-letter check in the capture group, letting
// lowercase words after "call"/"about"/"contact"/etc. match as fake PERSON
// names (e.g. "call me at ..." tokenizing "me at" as a person).
func TestTier15GreetingShield_NoFalsePositiveOnLowercaseWords(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"pronoun me", "call me at 415-555-2671 tomorrow"},
		{"pronoun her", "call her at 415-555-2671 tomorrow"},
		{"pronoun him", "call him at 415-555-2671 tomorrow"},
		{"lowercase noun phrase", "call the doctor at 415-555-2671 tomorrow"},
		{"greeting lowercase", "Hi there, quick question."},
		{"name intro lowercase", "this is fine, thanks."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			eng := newTestRefinery(&SpyScanner{})
			out, err := tier15GreetingShield(eng, c.input, "test")
			if err != nil {
				t.Fatalf("tier15GreetingShield: %v", err)
			}
			if strings.Contains(out, "[PERSON_") {
				t.Errorf("input %q: unexpected PERSON token in output %q", c.input, out)
			}
		})
	}
}

// TestTier15GreetingShield_StillCatchesRealNames ensures the fix doesn't
// regress genuine capitalised-name detection.
func TestTier15GreetingShield_StillCatchesRealNames(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"call + full name", "call John Galt at 415-555-2671 tomorrow"},
		{"greeting", "Regards, Jane Smith"},
		{"name intro", "my name is Jane Doe"},
		{"interrogative", "where does John Galt live"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			eng := newTestRefinery(&SpyScanner{})
			out, err := tier15GreetingShield(eng, c.input, "test")
			if err != nil {
				t.Fatalf("tier15GreetingShield: %v", err)
			}
			if !strings.Contains(out, "[PERSON_") {
				t.Errorf("input %q: expected a PERSON token in output %q", c.input, out)
			}
		})
	}
}
