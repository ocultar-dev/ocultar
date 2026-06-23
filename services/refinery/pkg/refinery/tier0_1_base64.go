package refinery

import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"
)

var base64Regex = regexp.MustCompile(`([A-Za-z0-9+/=]{20,})`) // Lowered from 40: catches short PII (emails, names, phones) encoded in Base64

// tier01Base64Shield recursively decodes embedded Base64 spans and re-scans
// the decoded content for PII before re-encoding it back into place.
func tier01Base64Shield(e *Refinery, refined, actor string, preScanMap map[string][]string) string {
	base64Matches := base64Regex.FindAllStringIndex(refined, -1)
	if len(base64Matches) == 0 {
		return refined
	}

	var out strings.Builder
	lastPos := 0
	for _, match := range base64Matches {
		start, end := match[0], match[1]
		out.WriteString(refined[lastPos:start])

		b64Str := refined[start:end]
		if decodedBytes, err := decodeBase64(b64Str); err == nil && len(decodedBytes) > 0 {
			mod, procErr := e.processInterfaceRecursive(string(decodedBytes), actor, preScanMap)
			if procErr == nil {
				if modStr, ok := mod.(string); ok {
					out.WriteString(base64.StdEncoding.EncodeToString([]byte(modStr)))
				} else if modBytes, err := json.Marshal(mod); err == nil {
					if len(modBytes) >= 2 && modBytes[0] == '"' && modBytes[len(modBytes)-1] == '"' {
						out.WriteString(base64.StdEncoding.EncodeToString(modBytes[1 : len(modBytes)-1]))
					} else {
						out.WriteString(base64.StdEncoding.EncodeToString(modBytes))
					}
				} else {
					out.WriteString(b64Str)
				}
			} else {
				out.WriteString(b64Str)
			}
		} else {
			out.WriteString(b64Str)
		}
		lastPos = end
	}
	out.WriteString(refined[lastPos:])
	return out.String()
}
