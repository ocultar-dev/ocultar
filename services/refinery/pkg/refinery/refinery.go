package refinery

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ocultar-dev/ocultar/internal/pii"
	"github.com/ocultar-dev/ocultar/pkg/config"
	"github.com/ocultar-dev/ocultar/vault"
	"golang.org/x/crypto/hkdf"
)

// AuditLogger defines the interface for the SIEM audit logger
type AuditLogger interface {
	Init(filePath string) error
	Log(user, action, result, mapping string)
	Close()
}

// AIScanner defines the interface for the Tier 2 AI NER scanner
type AIScanner interface {
	ScanForPII(text string) (map[string][]string, error)
	CheckHealth(host string)
	IsAvailable() bool
	SetDomain(domain string)
	// CircuitStateName returns "closed", "open", or "half-open" for observability.
	CircuitStateName() string
}

// NoopAuditLogger is a no-op implementation used when no audit logger is wired in.
type NoopAuditLogger struct{}

func (n NoopAuditLogger) Init(filePath string) error               { return nil }
func (n NoopAuditLogger) Log(user, action, result, mapping string) {}
func (n NoopAuditLogger) Close()                                   {}

// NoopAIScanner is a no-op implementation used when no Tier 2 scanner is configured.
type NoopAIScanner struct{}

func (n NoopAIScanner) ScanForPII(text string) (map[string][]string, error) { return nil, nil }
func (n NoopAIScanner) CheckHealth(host string)                             {}
func (n NoopAIScanner) IsAvailable() bool                                   { return false }
func (n NoopAIScanner) SetDomain(domain string)                             {}
func (n NoopAIScanner) CircuitStateName() string                            { return "closed" }

// slmLabelBlocklist contains document/legal keywords that the SLM sometimes
// misclassifies as person names or entity values. These are structural labels,
// not PII, and must survive redaction intact.
var slmLabelBlocklist = map[string]struct{}{
	"siret": {}, "siren": {}, "tva": {}, "vat": {}, "iban": {}, "bic": {},
	"facture": {}, "invoice": {}, "ref": {}, "date": {}, "total": {},
	"psychologue": {}, "psychologist": {}, "docteur": {}, "doctor": {},
	"monsieur": {}, "madame": {}, "mr": {}, "mme": {}, "ms": {},
}

// isBlockedSLMLabel returns true if item is a blocked label keyword or a
// BPE subword fragment of one (e.g. "iret" is a suffix fragment of "siret").
func isBlockedSLMLabel(item string) bool {
	lower := strings.ToLower(strings.TrimSpace(item))
	if _, ok := slmLabelBlocklist[lower]; ok {
		return true
	}
	for label := range slmLabelBlocklist {
		if len(label) > len(lower) && (strings.HasSuffix(label, lower) || strings.HasPrefix(label, lower)) {
			return true
		}
	}
	return false
}
var greetingRegex = regexp.MustCompile(`(?m)(?i)(?:Regards|Best|Cheers|Bonjour|Hello|Hi|Dear|Sincerely|Cordialement)[,.-]*\s+([A-ZÀ-Ÿ][a-zà-ÿ]+(?:\s+[A-ZÀ-Ÿ][a-zà-ÿ]+){0,2})\b`)

// Boundary artifact cleanup: absorb short (1-3 char) orphaned fragments adjacent to tokens
// left behind by SLM sub-word tokenization.
var trailingArtifact = regexp.MustCompile(`(\[[A-Za-z_]+_[0-9a-f]+\])([^\s\[\]"'{}\(\),.:;]{1,3})(?:[\s\[\]"'{}\(\),.:;]|$)`)
var leadingArtifact = regexp.MustCompile(`(?:[\s\[\]"'{}\(\),.:;]|^)([^\s\[\]"'{}\(\),.:;]{1,3})(\[[A-Za-z_]+_[0-9a-f]+\])`)

// Generalized Multilingual Heuristics (Phase 1)
var conjunctionRegex = regexp.MustCompile(`(?i)\b(ET|AND|Y|UND|CON|WITH|&)\b`)
var profTitleRegex = regexp.MustCompile(`(?i)\b(DR|DOCTEUR|PROF|MME|MLLE|SR|SRA|HR|FR|MAÎTRE|AVOCAT)\b`)
var capitalizedWordRegex = regexp.MustCompile(`\b[A-ZÀ-Ÿ][A-ZÀ-Ÿa-zà-ÿ\-]{1,20}\b`)
var possessiveRegex = regexp.MustCompile(`(?i)\b[A-ZÀ-Ÿ][a-zà-ÿ\-]{1,20}['’]s\b`)
var semanticTriggerRegex = regexp.MustCompile(`(?i)\b(DIVORCE|MARIAGE|WEDDING|AVOCAT|LAWYER|HOSPITAL|CLINIQUE|TREATMENT|TRAITEMENT|CAMPAIGN|POLITICAL|CAMPAGNE|PEA)\b`)
var nameIntroRegex = regexp.MustCompile(`(?m)(?i)\b(?:my name is|i am|call me|this is)\s+([A-ZÀ-Ÿ][a-zà-ÿ]+(?:\s+[A-ZÀ-Ÿ][a-zà-ÿ]+){0,2})\b`)

// nameInSentenceRegex catches proper names (two+ capitalised words) referenced by
// interrogative or inquiry verbs: "where does John Galt live", "who is Jane Smith",
// "tell me about Bob Jones", "contact Sarah Lee". This extends Tier 1.5 name
// detection beyond self-disclosures to cover third-party name mentions in questions.
var nameInSentenceRegex = regexp.MustCompile(`(?i)\b(?:where(?:\s+does)?|who(?:\s+is)?|about|contact|find|email|call|meet)\s+([A-ZÀ-Ÿ][a-zà-ÿ\-]{1,20}(?:\s+[A-ZÀ-Ÿ][a-zà-ÿ\-]{1,20}){1,3})\b`)


// DryRunReport collects PII hit metadata when running in --dry-run or --report mode.
type DryRunReport struct {
	Mode       string                `json:"mode"`
	FilesIn    int                   `json:"files_scanned"`
	Hits       []pii.DetectionResult `json:"pii_hits"`
	TotalCount int                   `json:"total_pii_count"`
	Blocking   bool                  `json:"blocking"`
}

// Refinery is the OCULTAR core redaction refinery.
// The storage backend is fully abstracted behind vault.Provider; the refinery
// has no knowledge of DuckDB, PostgreSQL, or any other concrete implementation.
type Refinery struct {
	Vault        vault.Provider
	MasterKey    []byte
	HmacKey      []byte
	DryRun       bool
	Report       bool
	Serve        string
	// SkipDeepScan disables Tier 2 AI scanning for this instance without removing the scanner.
	// Useful for high-throughput batch jobs where speed takes priority over AI recall.
	SkipDeepScan bool
	// FailClosedOnSLMError makes SLM failures propagate as hard errors instead of
	// degrading gracefully to Tier 1. Set to true in proxy mode and, by default, in
	// Sombra (security requirement: an unreachable SLM must block the request rather
	// than risk PII — especially names/addresses, which only Tier 2 catches — reaching
	// a third-party model provider). Sombra exposes OCU_SOMBRA_ALLOW_DEGRADED_NER as an
	// explicit opt-out for deployments that prefer availability over completeness.
	// Defaults to false at the struct level so the /api/refine preview endpoint stays
	// responsive even when the SLM sidecar is not running.
	FailClosedOnSLMError bool
	VaultCount   *atomic.Int64
	AuditLogger  AuditLogger
	AIScanner    AIScanner

	// DomainScanners holds optional per-domain scanners registered via SetDomainScanner.
	// The active scanner is selected at call time using config.Global.DomainSnapshot.
	DomainScanners map[string]AIScanner

	Hits      []pii.DetectionResult
	hitsMutex sync.Mutex

	// SessionCache provides a fast-path for identical strings during a single batch/recursion run.
	sessionCache atomic.Pointer[sync.Map]
}

// NewRefinery constructs an Refinery using a vault.Provider as its storage backend.
func NewRefinery(v vault.Provider, key []byte) *Refinery {
	count := int64(0)
	if v != nil {
		count = v.CountAll()
	}

	// Derive HMAC key from MasterKey via HKDF for token generation
	r := hkdf.New(sha256.New, key, nil, []byte("ocultar-token-hmac"))
	hmacKey := make([]byte, 32)
	if _, err := io.ReadFull(r, hmacKey); err != nil {
		panic(fmt.Sprintf("failed to derive HMAC key via HKDF: %v", err))
	}

	e := &Refinery{
		Vault:       v,
		MasterKey:   key,
		HmacKey:     hmacKey,
		VaultCount:  &atomic.Int64{},
		Hits:        []pii.DetectionResult{},
		AuditLogger: NoopAuditLogger{},
		AIScanner:   NoopAIScanner{},
	}
	e.sessionCache.Store(&sync.Map{})
	e.VaultCount.Store(count)
	return e
}

// SetAuditLogger injects a functional Enterprise SIEM logger.
func (e *Refinery) SetAuditLogger(l AuditLogger) {
	if l != nil {
		e.AuditLogger = l
	}
}

// SetAIScanner injects a functional Enterprise Deep Scan NER.
func (e *Refinery) SetAIScanner(s AIScanner) {
	if s != nil {
		e.AIScanner = s
	}
}

// RegisterEntity registers a canonical entity and its name variants with the
// vault's entity registry. After registration, any of the variant strings
// encountered during refinement will be replaced with the canonical token
// (e.g. "[PERSON_1]") instead of a SHA-256-based hash token.
// Returns the canonical token string.
func (e *Refinery) RegisterEntity(entityType, canonicalName string, variants []string) (string, error) {
	return e.Vault.RegisterEntity(entityType, canonicalName, variants)
}

// SetDomainScanner registers a domain-specific scanner.
// When DomainSnapshot in config matches the given domain, this scanner is used instead of AIScanner.
func (e *Refinery) SetDomainScanner(domain string, s AIScanner) {
	if domain == "" || s == nil {
		return
	}
	if e.DomainScanners == nil {
		e.DomainScanners = make(map[string]AIScanner)
	}
	e.DomainScanners[domain] = s
	log.Printf("[INFO] Domain Tier 2 scanner registered: '%s'", domain)
}

// activeScanner returns the scanner for the currently configured domain,
// falling back to the default AIScanner if no domain-specific one is registered.
func (e *Refinery) activeScanner() AIScanner {
	domain := config.Global.DomainSnapshot
	if domain != "" && domain != "standard" && e.DomainScanners != nil {
		if s, ok := e.DomainScanners[domain]; ok {
			return s
		}
	}
	return e.AIScanner
}

// GenerateReport aggregates the current session's PII hits into a DryRunReport.
func (e *Refinery) GenerateReport(filesScanned int) DryRunReport {
	e.hitsMutex.Lock()
	defer e.hitsMutex.Unlock()

	blocking := len(e.Hits) > 0
	mode := "report"
	if e.DryRun {
		mode = "dry-run"
	}
	if e.Serve != "" {
		mode = "serve"
	}
	total := len(e.Hits)

	// Copy hits to avoid race conditions with JSON marshaling
	hitsCopy := append([]pii.DetectionResult{}, e.Hits...)

	return DryRunReport{
		Mode:       mode,
		FilesIn:    filesScanned,
		Hits:       hitsCopy,
		TotalCount: total,
		Blocking:   blocking,
	}
}

// ResetHits clears the in-memory record of detected PII and the session cache.
func (e *Refinery) ResetHits() {
	e.hitsMutex.Lock()
	defer e.hitsMutex.Unlock()
	e.Hits = []pii.DetectionResult{}
	e.sessionCache.Store(&sync.Map{})
}

// RefineBatch processes a slice of generic objects in parallel using a bounded worker pool.
// This enables High-Density Batch Tokenization for gigabyte-scale data ingestion.
func (e *Refinery) RefineBatch(items []interface{}, actor string) ([]interface{}, error) {
	if len(items) == 0 {
		return items, nil
	}

	results := make([]interface{}, len(items))
	errs := make([]error, len(items))
	var wg sync.WaitGroup

	// Bounded worker pool to prevent memory/goroutine exhaustion
	concurrency := 100
	sem := make(chan struct{}, concurrency)

	for i, item := range items {
		wg.Add(1)
		go func(idx int, val interface{}) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire token
			defer func() { <-sem }() // Release token

			res, err := e.ProcessInterface(val, actor)
			results[idx] = res
			errs[idx] = err
		}(i, item)
	}

	wg.Wait()

	// Fail-Closed: If any item fails in batch, the entire batch fails securely.
	for _, err := range errs {
		if err != nil {
			return nil, fmt.Errorf("RefineBatch failed securely during processing: %w", err)
		}
	}

	return results, nil
}

// ProcessInterface recursively processes dynamic JSON data to identify and redact PII.
func (e *Refinery) ProcessInterface(data interface{}, actor string) (interface{}, error) {
	// 1. If it's a large complex object, extract all text and run SLM ONCE per record
	var preScanMap map[string][]string
	scanner := e.activeScanner()
	if scanner.IsAvailable() && !e.SkipDeepScan {
		// Marshal the record to a flat string to scan it contextually in one go
		textBytes, err := json.Marshal(data)
		if err == nil {
			textStr := string(textBytes)
			if len(textStr) > 50 && !e.isFullyTokenised(textStr) {
				var slmErr error
				preScanMap, slmErr = scanner.ScanForPII(textStr)
				if slmErr != nil {
					if e.FailClosedOnSLMError {
						return nil, fmt.Errorf("SLM inference failed: %w", slmErr)
					}
					log.Printf("[WARN] Tier 2 SLM unavailable, degrading to Tier 1: %v", slmErr)
					preScanMap = nil
				}
				if len(preScanMap) > 0 {
					log.Printf("[INFO] SLM Batch Scan: %d entity type(s) detected in record", len(preScanMap))
				}
			}
		}
	}

	return e.processInterfaceRecursive(data, actor, preScanMap)
}

// Refine is a convenience method that delegates to ProcessInterface.
func (e *Refinery) Refine(data interface{}) (interface{}, error) {
	return e.ProcessInterface(data, "system-refinery")
}

// processInterfaceRecursive is the internal recursive helper for traversing JSON structures.
func (e *Refinery) processInterfaceRecursive(data interface{}, actor string, preScanMap map[string][]string) (interface{}, error) {
	switch val := data.(type) {
	case string:
		// Attempt Base64 decoding — skip short strings (≤8 chars) which are
		// almost always false positives (e.g. "No" decodes to "6", "Various" etc.).
		if len(val) > 8 {
		if decodedBytes, err := decodeBase64(val); err == nil && len(decodedBytes) > 0 {
			// Try to treat decoded Base64 as JSON or string
			var unmarshaled interface{}
			if err := json.Unmarshal(decodedBytes, &unmarshaled); err == nil {
				mod, err := e.processInterfaceRecursive(unmarshaled, actor, preScanMap)
				if err != nil {
					return nil, err
				}
				if remarshed, err := json.Marshal(mod); err == nil {
					return base64.StdEncoding.EncodeToString(remarshed), nil
				}
			}
			// Fallback: treat decoded Base64 as pure string
			refinedStr, err := e.RefineString(string(decodedBytes), actor, preScanMap)
			if err != nil {
				return nil, err
			}
			return base64.StdEncoding.EncodeToString([]byte(refinedStr)), nil
		}
		}

		// Attempt URL decoding
		if strings.Contains(val, "%") {
			if unescaped, err := url.QueryUnescape(val); err == nil && unescaped != val {
				mod, err := e.processInterfaceRecursive(unescaped, actor, preScanMap)
				if err != nil {
					return nil, err
				}
				if modStr, ok := mod.(string); ok {
					return url.QueryEscape(modStr), nil
				} else if remarshed, err := json.Marshal(mod); err == nil {
					return url.QueryEscape(string(remarshed)), nil
				}
			}
		}

		// Attempt nested JSON decoding
		var unmarshaled interface{}
		if err := json.Unmarshal([]byte(val), &unmarshaled); err == nil {
			switch unmarshaled.(type) {
			case map[string]interface{}, []interface{}:
				mod, err := e.processInterfaceRecursive(unmarshaled, actor, preScanMap)
				if err != nil {
					return nil, err
				}
				if remarshed, err := json.Marshal(mod); err == nil {
					return string(remarshed), nil
				}
			}
		}

		return e.RefineString(val, actor, preScanMap)
	case map[string]interface{}:
		for k, v := range val {
			mod, err := e.processInterfaceRecursive(v, actor, preScanMap)
			if err != nil {
				return nil, err
			}
			val[k] = mod
		}
		return val, nil
	case []interface{}:
		if len(val) < 5 {
			// Sequential for small arrays to avoid goroutine overhead
			for i, v := range val {
				mod, err := e.processInterfaceRecursive(v, actor, preScanMap)
				if err != nil {
					return nil, err
				}
				val[i] = mod
			}
			return val, nil
		}

		// Parallel for larger arrays
		results := make([]interface{}, len(val))
		errs := make([]error, len(val))
		var wg sync.WaitGroup

		// Use a bounded worker pool (shared with RefineBatch logic)
		concurrency := 50 // Conservative default for recursion
		sem := make(chan struct{}, concurrency)

		for i, v := range val {
			wg.Add(1)
			go func(idx int, item interface{}) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				mod, err := e.processInterfaceRecursive(item, actor, preScanMap)
				results[idx] = mod
				errs[idx] = err
			}(i, v)
		}
		wg.Wait()

		for _, err := range errs {
			if err != nil {
				return nil, err
			}
		}
		return results, nil
	default:
		return val, nil
	}
}

// RefineString is the core logic that orchestrates PII detection tiers (Regex, Dictionaries, SLM) on a single string.
// Security is mandatory: Tier 2 (AI) is always prioritized if available.
func (e *Refinery) RefineString(input string, actor string, preScanMap map[string][]string) (string, error) {
	if len(input) < 3 {
		return input, nil
	}

	cache := e.sessionCache.Load()
	if cached, ok := cache.Load(input); ok {
		return cached.(string), nil
	}

	trimmed := strings.TrimSpace(input)
	if len(trimmed) < 3 {
		return input, nil
	}

	refined := input
	var err error

	// TIER 0.1: Embedded Base64 Evasion Shield
	refined = tier01Base64Shield(e, refined, actor, preScanMap)

	// TIER 0: Dynamic Exclusion Dictionaries
	refined, err = tier0DictionaryShield(e, refined, actor)
	if err != nil {
		return "", err
	}

	// TIER 0.5: Entity Registry Pre-Pass
	// Replace all registered entity variants by direct string matching before any
	// NER tier runs. This guarantees known identities are masked even when the NER
	// model misses them (e.g. non-English names in French/Spanish documents).
	if e.Vault != nil {
		if entities, listErr := e.Vault.ListEntities(); listErr == nil && len(entities) > 0 {
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
		}
	}

	// TIER 1: Centralized Deterministic Pipeline
	eng := pii.NewRefinery()
	if config.Global.AliasMapping != nil {
		eng.SetMapping(config.Global.AliasMapping)
	}

	// Scan first to identify structured PII (SSN, Credit Cards, etc.)
	detections := eng.Scan(refined)
	log.Printf("[DEBUG] Tier 1 Scan found %d detections", len(detections))

	tokens := tokenPattern.FindAllStringIndex(refined, -1)
	refined, err = eng.Redact(refined, func(d pii.DetectionResult) (string, error) {
		overlap := false
		for _, t := range tokens {
			if d.Range.Start < t[1] && d.Range.End > t[0] {
				overlap = true
				break
			}
		}
		if overlap {
			return d.Value, nil
		}
		return e.getOrSetSecureResult(d, actor)
	})
	if err != nil {
		return "", err
	}

	// TIER 1.1 (FALLBACK): Phone detection runs AFTER the PII registry scan.
	// This ensures that digit sequences already claimed by national IDs/SSNs
	// are not misidentified as phone numbers.
	if strings.ContainsAny(refined, "0123456789") && !e.isFullyTokenised(refined) {
		var phoneErr error
		refined, phoneErr = parseAndReplaceWithErr(refined, ParseAndReplacePhonesRaw, func(match string, start, end int) (string, error) {
			log.Printf("[DEBUG] Tier 1.1 Phone hit: %s", match)
			return e.getOrSetSecureTokenLoc(match, "PHONE", "phone", start, end, actor)
		})
		if phoneErr != nil {
			return "", phoneErr
		}
	}


	if len(refined) > 10 && (strings.ContainsAny(refined, "0123456789") || containsAnyLower(refined, "rue", "calle", "street", "ave", "road", "str.")) {
		refined, err = parseAndReplaceWithErr(refined, ParseAndReplaceAddressesRaw, func(match string, start, end int) (string, error) {
			return e.getOrSetSecureTokenLoc(match, "ADDRESS", "address", start, end, actor)
		})
		if err != nil {
			return "", err
		}
	}

	// TIER 1.5: Greeting & Signature Shield
	// Catches names disclosed via salutations ("Regards, John") and self-introductions ("My name is Jane").
	// Runs after phone/address parsing to avoid false-positive collisions with numeric fields.
	greetingMatches := greetingRegex.FindAllStringSubmatchIndex(refined, -1)
	nameIntroMatches := nameIntroRegex.FindAllStringSubmatchIndex(refined, -1)
	nameSentenceMatches := nameInSentenceRegex.FindAllStringSubmatchIndex(refined, -1)
	allNameMatches := append(append(greetingMatches, nameIntroMatches...), nameSentenceMatches...)

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

	// TIER 2: SLM NER Scan (Mandatory Phase)
	if preScanMap != nil {
		for piiType, items := range preScanMap {
			canonType := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(piiType), " ", "_"))
			if canonType == "" {
				continue
			}
			for _, item := range items {
				trimmed := strings.TrimSpace(item)
				if len(trimmed) < 3 || !strings.Contains(refined, trimmed) {
					continue
				}
				if isBlockedSLMLabel(trimmed) {
					continue
				}
				refined, err = e.applyReplacement(refined, trimmed, canonType, "ai-ner", actor)
				if err != nil {
					return "", err
				}
			}
		}
	} else if e.activeScanner().IsAvailable() && !e.SkipDeepScan && len(refined) > 15 && !e.isFullyTokenised(refined) {
		// Strip existing Tier-1 tokens before sending to SLM.
		// Without this, the SLM sees token content like "HEALTH_ENTITY_f62c" and
		// misclassifies the hex hashes as account numbers or person names, producing
		// double-bracket artifacts such as [[private_person_...]3b20].
		textForSLM := tokenPattern.ReplaceAllString(refined, " ")
		piiMap, slmErr := e.activeScanner().ScanForPII(textForSLM)
		if slmErr != nil {
			if e.FailClosedOnSLMError {
				return "", fmt.Errorf("SLM inference failed: %w", slmErr)
			}
			log.Printf("[WARN] Tier 2 SLM unavailable, degrading to Tier 1: %v", slmErr)
			piiMap = nil
		}
		for piiType, items := range piiMap {
			// Normalize SLM entity type to UPPERCASE so tokens are consistent with
			// Tier-1 output (e.g. "private_person" → "PRIVATE_PERSON").
			// This ensures ki!'s build_replacement_map and extract_tokens recognize
			// SLM tokens, and that tokenPattern protects them from re-processing.
			canonType := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(piiType), " ", "_"))
			if canonType == "" {
				continue
			}
			for _, item := range items {
				trimmed := strings.TrimSpace(item)
				if len(trimmed) < 3 {
					continue
				}
				if isBlockedSLMLabel(trimmed) {
					log.Printf("[DEBUG] Tier 2 SLM: skipping blocked label %q", trimmed)
					continue
				}
				log.Printf("[DEBUG] Tier 2 SLM hit: %s (%s)", trimmed, canonType)
				refined, err = e.applyReplacement(refined, trimmed, canonType, "ai-ner", actor)
				if err != nil {
					return "", err
				}
			}
		}
	}

	// TIER 2.5: Boundary Artifact Cleanup
	// SLM sub-word tokenization can leave orphaned 1-3 char residues adjacent
	// to tokens (e.g. "[organization_...]7" or "H[organization_...]").
	// Absorb these fragments to prevent partial PII leakage.
	refined = boundaryCleanup(refined)

	// TIER 3: Structural Heuristics
	refined, err = e.applyStructuralHeuristics(refined, actor)
	if err != nil {
		return "", err
	}

	cache.Store(input, refined)

	return refined, nil
}

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

