package refinery

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"regexp"

	"github.com/ocultar-dev/ocultar/internal/pii"
)

func (e *Refinery) getOrSetSecureToken(val, piiType, source string, actor string) (string, error) {
	res := pii.DetectionResult{
		Entity:     piiType,
		Value:      val,
		Confidence: 1.0,
		Method:     []string{source},
	}
	return e.getOrSetSecureResult(res, actor)
}

func (e *Refinery) getOrSetSecureTokenLoc(val, piiType, source string, start, end int, actor string) (string, error) {
	res := pii.DetectionResult{
		Entity:     piiType,
		Value:      val,
		Confidence: 1.0,
		Method:     []string{source},
	}
	res.Range.Start = start
	res.Range.End = end
	return e.getOrSetSecureResult(res, actor)
}

// entityRegistryTypes is the set of PII entity classes that participate in the
// entity registry (Path 3). For these types, the registry is checked before
// computing a hash-based token so all variants resolve to one canonical token.
var entityRegistryTypes = map[string]bool{
	"PERSON": true, "PERSON_VIP": true, "HEALTH_ENTITY": true,
	"PROTECTED_ENTITY": true, "ORGANIZATION": true,
}

// entityTokenRe matches the numeric-suffix canonical token format "[TYPE_N]"
// produced by the entity registry (e.g. "[PERSON_1]", "[ORGANIZATION_42]").
// It is distinct from hash-based tokens "[TYPE_hexhex8]".
var entityTokenRe = regexp.MustCompile(`^\[([A-Z_]+)_(\d+)\]$`)

// getOrSetSecureResult retrieves an existing token from the vault or generates, encrypts, and stores a new one.
func (e *Refinery) getOrSetSecureResult(res pii.DetectionResult, actor string) (string, error) {
	// [VULN-003] Enforce checksum validation for high-fidelity types
	if res.Entity == "CREDIT_CARD" && !isLuhnValid(res.Value) {
		// False positive avoidance: if it's not Luhn-valid, it's not a PII credit card
		return res.Value, nil
	}

	// ENTITY REGISTRY (Path 3): For PERSON-class types, check the registry
	// before hashing. This ensures that "John", "Doe", and "John Doe" all
	// resolve to the same canonical token (e.g. "[PERSON_1]") when they are
	// registered variants of a known identity.
	if entityRegistryTypes[res.Entity] {
		if canonicalToken, found := e.Vault.LookupVariant(res.Value); found {
			slog.Info("entity registry hit", "entity_type", res.Entity, "token", canonicalToken)
			return canonicalToken, nil
		}
	}

	hash := e.hashValue(res.Value)
	token := fmt.Sprintf("[%s_%s]", res.Entity, hash[:16])

	if e.DryRun || e.Report || e.Serve != "" {
		e.hitsMutex.Lock()
		res.ValueHash = hash
		if res.Location == "" && res.Range.End > 0 {
			res.Location = fmt.Sprintf("%d-%d", res.Range.Start, res.Range.End)
		}
		e.Hits = append(e.Hits, res)
		e.hitsMutex.Unlock()
	}

	// Check vault for an existing token
	if existing, found := e.Vault.GetToken(hash); found {
		if !e.DryRun {
			e.AuditLogger.Log(actor, "matched", existing, getComplianceMapping(res.Entity))
		}
		return existing, nil
	}

	if !e.DryRun {
		e.AuditLogger.Log(actor, "vaulted", token, getComplianceMapping(res.Entity))
	}

	encrypted, encErr := encrypt([]byte(res.Value), e.MasterKey)
	if encErr != nil {
		return "", fmt.Errorf("encryption failed: %w", encErr)
	}
	inserted, err := e.Vault.StoreToken(hash, token, encrypted)
	if err != nil {
		return "", fmt.Errorf("vault storage failed: %w", err)
	}
	if inserted {
		e.VaultCount.Add(1)
	}
	return token, nil
}

func (e *Refinery) hashValue(s string) string {
	mac := hmac.New(sha256.New, e.HmacKey)
	mac.Write([]byte(s))
	return hex.EncodeToString(mac.Sum(nil))
}

// isLuhnValid implements the Luhn algorithm (mod 10) for credit card checksum validation.
func isLuhnValid(s string) bool {
	// Strip non-digits
	digits := ""
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits += string(r)
		}
	}
	if len(digits) < 13 || len(digits) > 19 {
		return false
	}

	sum := 0
	shouldDouble := false
	for i := len(digits) - 1; i >= 0; i-- {
		n := int(digits[i] - '0')
		if shouldDouble {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		shouldDouble = !shouldDouble
	}
	return (sum % 10) == 0
}
