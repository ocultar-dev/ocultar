# Roadmap

This document describes what we are working on and what is coming next. It is updated as priorities change.

For bugs and feature requests, open an issue. For security vulnerabilities, see [CONTRIBUTING.md](CONTRIBUTING.md).

---

## Released — v1.14.0 (June 2026)

Initial public release.

- Tier 1 deterministic refinery: 63 PII/PHI entity types across 12 categories
- Tier 2 contextual AI (SLM): 114/117 entity types with a local NER sidecar
- Base64 and JWT evasion shield (Tier 0.1)
- Sovereign vault: AES-256-GCM + HKDF-SHA256, DuckDB backend
- Ed25519-signed immutable audit log
- Fail-closed design: vault failure blocks requests, never forwards raw text
- Regulatory framework coverage: HIPAA, GDPR, CCPA, PCI-DSS, SOX, FERPA, BIPA, NYDFS
- Persistent entity registry with pre-seeding support
- Zero-egress proxy (Sombra gateway)
- Tier 2 circuit breaker: automatic Tier 1 fallback when SLM sidecar is unavailable
- Claude MCP extension (`extensions/claude/`)
- Goose MCP extension (`extensions/goose/`)
- Docker image published to GHCR (`ghcr.io/ocultar-dev/ocultar`)
- Cosign-signed Docker images (Sigstore keyless)
- SBOM published with each release (CycloneDX JSON)

---

## Near-term (v1.15)

### PII coverage

- [ ] Raise Tier 1 (deterministic) coverage from 63 → 80+ entity types
- [ ] Fix 5 partially-detected types: PASSPORT, VEHICLE_ID, BIOMETRIC_ID, GENETIC_ID, DEVICE_ID
- [ ] Add test fixtures for FERPA, BIPA, and NYDFS regulatory frameworks

### Testing

- [ ] Expand adversarial test suite: Base64-encoded PII, JSON-nested PII, mixed-language (FR+EN), Unicode obfuscation
- [ ] Integration test: Sombra zero-egress — verify no raw PII reaches the upstream AI
- [ ] Circuit breaker test: SLM down → Tier 1 only, no hang, no error

### Developer experience

- [ ] Cursor / Windsurf MCP connector
- [ ] Structured error codes in API responses (machine-readable failure reasons)
- [x] `golangci-lint` pass across all modules
- [ ] Tighten "Protect Main" ruleset (github.com/ocultar-dev/ocultar/settings/rules/17754757): change org-admin bypass from "Always" to "For pull requests only", and add `lint apps/web` to required status checks alongside `golangci-lint`/`govulncheck` — currently direct pushes to `main` silently bypass PR review and all CI checks

### Marketing & outreach (high priority — starting 2026-08-29)

- [ ] Publish launch/marketing content about OCULTAR across the web (blog posts, dev community launches, socials)
- [x] Add a founder/creator bio section to ocultar.dev — portfolio-worthy, discoverable, points back to the maintainer

---

## Medium-term (v1.16 – v2.0)

### Observability

- [ ] Prometheus metrics endpoint (`/metrics`): request count, refine latency, vault size, tier hit rates
- [ ] OpenTelemetry trace export (optional, off by default)

### Entity coverage

- [ ] 117/117 Tier 1 coverage (close the remaining SLM-only gap)
- [ ] Custom entity type API: define new PII types at runtime via the entity registry
- [ ] French-finance-tuned Tier 2 NER model — partner-driven engagement, needs 5K+ real-world labeled examples (see FAQ.md); an internal fine-tune attempt on 200 synthetic examples failed to converge (eval F1 ≈ 0.06) and was scrapped
- [ ] Phone Tier-B's `nonPhoneContextRe` context gate runs *before* Tier A's strict `libphonenumber` check, so a provably-valid phone number near a guard keyword ("compte", "référence", etc.) gets skipped even though Tier A already confirmed it's genuine. Reorder so Tier A's match short-circuits the context check, or scope the gate to Tier B only (found 2026-09-17 fixing a WISC-V false-positive; not fixed)
- [ ] `semanticTriggerRegex`'s "TRAITEMENT" trigger can't disambiguate French "traitement" (processing) from "traitement médical" (medical treatment) — e.g. "Vitesse de traitement" (a WISC-V subtest name, Processing Speed) gets masked as `SENSITIVE_EVENT`. Needs real disambiguation, not a keyword tweak (found 2026-09-17; not fixed)

### Resilience

- [ ] Request-level timeout configuration (`OCU_REFINE_TIMEOUT`)
- [ ] Vault compaction — prune tokens older than a configurable TTL
- [ ] Document the expected host-supervision contract: ocultar has no self-restart capability — if the process dies (crash, OOM, killed), an embedding host (e.g. Ki!'s Tauri sidecar) must detect it and respawn a fresh instance. Found 2026-09-17 via Ki!: no such watchdog exists there today, so a dead sidecar requires a full app restart. This item is about documenting/exposing what a host needs (e.g. is `/api/health` sufficient, or is a graceful-shutdown signal needed too) — the actual watchdog logic belongs in each host, not here.

### Deployment

- [ ] Helm chart for Kubernetes
- [ ] ARM64 Docker image (Apple Silicon / AWS Graviton)
- [ ] `apt`, `brew`, and `rpm` native packages

---

## Long-term (v2.x+)

- [ ] Policy engine: per-connector rules defining which PII types to mask and which to pass through
- [ ] Streaming refinery: mask text in real-time as it streams from an AI model response
- [ ] FIPS 140-2 cryptographic module (`GOEXPERIMENT=boringcrypto`)
- [ ] SOC 2 Type II audit
- [ ] GDPR Article 25 DPA template reviewed by EU privacy counsel
- [ ] Third-party penetration test — publish summary (redacted)
- [ ] ANSSI CSPN evaluation (French regulated market)

---

## Not planned

- **Cloud-hosted version** — Ocultar is designed to be sovereign and local-only.
- **Built-in AI model** — the SLM sidecar is intentionally user-supplied to avoid bundling large model weights.
- **Automatic upstream AI credential management** — Ocultar is a sidecar; it does not manage AI provider API keys or routing.

---

*AGPLv3 — Self-hosting is free and always will be. Commercial licensing available, see [COMMERCIAL_LICENSE.md](COMMERCIAL_LICENSE.md).*
