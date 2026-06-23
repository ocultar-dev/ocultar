package refinery

import "regexp"

// Boundary artifact cleanup: absorb short (1-3 char) orphaned fragments adjacent to tokens
// left behind by SLM sub-word tokenization.
var trailingArtifact = regexp.MustCompile(`(\[[A-Za-z_]+_[0-9a-f]+\])([^\s\[\]"'{}\(\),.:;]{1,3})(?:[\s\[\]"'{}\(\),.:;]|$)`)
var leadingArtifact = regexp.MustCompile(`(?:[\s\[\]"'{}\(\),.:;]|^)([^\s\[\]"'{}\(\),.:;]{1,3})(\[[A-Za-z_]+_[0-9a-f]+\])`)

// boundaryCleanup absorbs orphaned short fragments (1-3 chars) that are
// immediately adjacent to tokens. These are artifacts of SLM sub-word
// tokenization where the model's BPE boundaries don't align with PII
// value boundaries (e.g. "XXX-XX-556" is tokenized but trailing "7" leaks).
func boundaryCleanup(s string) string {
	// Pass 1: trailing artifacts — e.g. "[organization_abc12345]7 " → "[organization_abc12345] "
	s = trailingArtifact.ReplaceAllStringFunc(s, func(match string) string {
		// Extract the token and the trailing fragment
		subs := trailingArtifact.FindStringSubmatch(match)
		if len(subs) < 3 {
			return match
		}
		token := subs[1]
		// fragment := subs[2]  // the orphaned chars — dropped
		// Preserve the delimiter that ended the match (space, EOF, or '[')
		suffix := match[len(token)+len(subs[2]):]
		return token + suffix
	})

	// Pass 2: leading artifacts — e.g. " H[organization_abc12345]" → " [organization_abc12345]"
	s = leadingArtifact.ReplaceAllStringFunc(s, func(match string) string {
		subs := leadingArtifact.FindStringSubmatch(match)
		if len(subs) < 3 {
			return match
		}
		fragment := subs[1]
		token := subs[2]
		// Preserve the delimiter that started the match (space, BOL, or ']')
		prefix := match[:len(match)-len(fragment)-len(token)]
		return prefix + token
	})

	return s
}
