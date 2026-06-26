# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
