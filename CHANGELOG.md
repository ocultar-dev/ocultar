# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **MCP Entity Registry tools**: `register_entity`, `list_entities`, `seed_entities` added to the Claude and Mistral MCP extensions (auditor-only, matching `reveal_tokens`'s existing gate).
- **MCP `sombra_query` tool**: added to all three MCP extensions (Claude, Goose, Mistral) — sends a prompt through the Sombra gateway for redact-route-rehydrate in one call. Requires `OCULTAR_SOMBRA_TOKEN`.

### Changed
- **MCP Mistral extension default `OCULTAR_URL`**: changed from `http://localhost:8080` to `http://localhost:4141` to match the Claude and Goose extensions and this repo's standardized Refinery port. Existing 0.2.0 users relying on the old default will need to set `OCULTAR_URL` explicitly (or upgrade the Refinery's own `--serve` port to match) after upgrading.

### Fixed
- **Docs sidebar dead links**: removed the "Secret Management" entry (pointed to `content/SECRETS.md`, never created) and swapped the "Training Program" entry (pointed to `content/reference/TRAINING_PROGRAM.md`, also never created) for the existing but previously unlinked `reference/ONBOARDING_GUIDE.md`.
- **Documentation factual audit**: corrected wrong ports, stale 8-char token-format references, deprecated env var names (`TIER2_ENGINE`, `SLM_ENGINE`, `PRIVACY_FILTER_URL`), a fictional `configs/sombra.yaml`/separate-repo Sombra setup, a pre-`go.work` module layout in the dev/setup/release guides, wrong HTTP status codes, a missing Tier 2.5 pipeline row, and leftover enterprise/community-split language across `SOMBRA_GUIDE.md`, `CONNECTORS_GUIDE.md`, `DEVELOPER_GUIDE.md`, `SETUP_GUIDE.md`, `RELEASE_GUIDE.md`, `ENTERPRISE_SETUP_GUIDE.md`, `refinery_proxy_setup.md`, `VAULT_GUIDE.md`, `API_REFERENCE.md`, `PII_DETECTION.md`, `FAQ.md`, `PRODUCT_CONTEXT.md`, `GDPR_PRIVACY_BY_DESIGN.md`, `GDPR_FRENCH_FINANCE.md`, `SECURITY_MODEL.md`, `zero-egress-supply-chain.md`, `privacy_filter_eval.md`, and `skills_summary.md`.
- **`.gitignore` audit log coverage**: `audit.log*` only matched the proxy's default audit-log basename, missing Sombra's hardcoded `sombra_audit.log` and its rotated `*.archived` segments. Widened to `*audit.log*` so a careless `git add -A` can't commit a hash-chained compliance log to the repo.
- **MCP extension license metadata**: all three MCP packages (`ocultar-claude-mcp`, `ocultar-goose-mcp`, `ocultar-mistral-mcp`) were still declaring `Apache-2.0` after the repo-wide relicense to AGPLv3 on 2026-06-28 — corrected to `AGPL-3.0-only` across `pyproject.toml`, `manifest.json`/`extension.yaml`, and README files. `extensions/{claude,mistral}/manifest.json` also still pointed at the old `Edu963/ocultar` personal fork — corrected to `ocultar-dev/ocultar`.
- **MCP Goose extension error hint**: the connection-error remediation message referenced the personal Docker registry namespace `ghcr.io/edu963/ocultar` — corrected to `ghcr.io/ocultar-dev/ocultar`.

## [1.15.0] - 2026-06-26

### Added
- **HMAC-SHA256 Tokenization**: Upgraded deterministic tokens from 8 to 16 characters for enhanced security.
- **File Processing API**: New `/api/refine/file` endpoint for bulk data masking.
- **Vault Management**: New `/api/vault/delete` and `/api/vault/migrate` endpoints.
- **GDPR Data Retention Loop**: Implemented 90-day TTL for sensitive data.
- **Decompression Bomb Protection**: Added safeguards for OOXML/docx file processing.
- **Structured Logging**: Migrated to `log/slog` for unified structured logs.
- **Unified Gateway Logic**: Consolidated gateway logic across Proxy and Sombra.
- **Degraded NER Opt-out**: Introduced `OCU_SOMBRA_ALLOW_DEGRADED_NER` fail-closed configuration.
- **MCP Extensions**: Updated Claude, Goose, and Mistral MCP extensions to `v0.2.0` with 16-char token compatibility.

## [1.14.0] - 2026-06-09 — Initial public release

### Added
- **Tier 1 — Deterministic Refinery**: High-speed regex and heuristic detection pipeline covering 63 PII/PHI entity types across 12 categories.
- **Tier 2 — Contextual AI (SLM)**: Model-agnostic SLM adapter (Qwen / OpenAI-compatible) expanding coverage to 114/117 entity types with regulatory-grade accuracy.
- **Zero-Egress Proxy (Sombra)**: Transparent reverse proxy for OpenAI-compatible APIs with fail-closed enforcement — raw prompts are never forwarded if masking fails.
- **Sovereign Vault**: Encrypted local storage (DuckDB) with AES-256-GCM + HKDF-SHA256 key derivation for secure PII tokenization and audit replay.
- **Ed25519 Immutable Audit Log**: SHA-256 hash-chained, Ed25519-signed audit trail for verifiable compliance.
- **Base64 / JWT Evasion Shield**: Recursive decode-and-rescan loop to detect PII obfuscated via encoding.
- **Regulatory Framework Coverage**: Minimum detection thresholds met for HIPAA, GDPR, CCPA, PCI-DSS, SOX, FERPA, BIPA, and NYDFS.
- **Identity-Aware Auditing**: JWT header extraction for actor attribution in audit logs.
- **Claude & Goose MCP Extensions**: Native extensions for Claude Desktop and Goose AI IDE.
- **Shield Manager Dashboard**: React-based UI for live redaction testing and system monitoring.

---
[1.15.0]: https://github.com/ocultar-dev/ocultar/releases/tag/v1.15.0
[1.14.0]: https://github.com/ocultar-dev/ocultar/releases/tag/v1.14.0
