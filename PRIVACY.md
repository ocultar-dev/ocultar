# Privacy Policy

**Effective date:** 28 April 2026
**Product:** OCULTAR PII Refinery (including `ocultar-claude-mcp`, `ocultar-goose-mcp`, `ocultar-mistral-mcp`, and all related extensions)
**Contact:** [edu@ocultar.dev](mailto:edu@ocultar.dev)

---

## 1. What OCULTAR Does

OCULTAR is a local, zero-egress PII detection and redaction engine. It runs entirely within your own infrastructure. No data you submit to OCULTAR is transmitted to any external server, cloud service, or third party by OCULTAR itself.

---

## 2. Data Processing

### What is processed
Text you submit through the `refine_text` tool or the `/api/refine` endpoint is analyzed locally to detect and redact personally identifiable information (PII). The redacted output and an encrypted form of the original values are stored in a local vault on your own machine or server.

### Where processing happens
All detection, tokenization, and vault storage occur on the machine running the OCULTAR Refinery. No text, tokens, or vault contents are transmitted off that machine by OCULTAR.

### What is stored
- A local encrypted vault (AES-256-GCM) mapping deterministic token IDs to encrypted PII ciphertext. This file remains on your infrastructure.
- An optional audit log (Ed25519 hash-chained) recording operation metadata (actor, action type, token ID, timestamp). No plaintext PII is written to the audit log.

---

## 3. No Telemetry

OCULTAR collects no usage analytics, crash reports, or telemetry of any kind. No data is sent to the OCULTAR project, its author, or any analytics platform.

---

## 4. MCP Extensions (`ocultar-claude-mcp`, `ocultar-goose-mcp`, `ocultar-mistral-mcp`)

The MCP extensions communicate exclusively with the locally running OCULTAR Refinery over localhost. They make no outbound network calls to any external service. If the local Refinery is unreachable, all extensions fail closed — they return an error and refuse to forward your text elsewhere.

---

## 5. Your Role as Data Controller

Because all data stays within your infrastructure, you — the operator deploying OCULTAR — are the data controller under GDPR and similar regulations. OCULTAR acts as a local data processor running entirely under your control. You are responsible for configuring access controls, key management, and audit log retention in accordance with your applicable data protection obligations.

---

## 6. Third-Party Services

OCULTAR does not integrate with any third-party services by default. If you configure an upstream API target (e.g. `OCU_PROXY_TARGET`), OCULTAR forwards only the **redacted** output — never raw PII — to that target. The privacy practices of that upstream service are governed by its own policy.

---

## 7. Data Retention and Deletion

Vault contents and audit logs are stored on your infrastructure. OCULTAR provides no mechanism to transmit this data externally and retains no copy of it.

By default, OCULTAR enforces an automatic retention policy on your behalf, in support of GDPR Art. 5(1)(e) storage limitation:

- **Vault tokens**: each tokenized PII record (the encrypted value behind a `[TYPE_token]`) is automatically purged 90 days after creation. This is configurable via `vault_retention_days` in `configs/config.yaml`, or disabled entirely by setting `retention_enabled: false`.
- **Audit logs**: both the plain JSON audit log (`audit.log`) and the cryptographically signed `ImmutableLogger` chain (used by the Proxy and Sombra Gateway) rotate once they exceed 50MB (`audit_log_max_size_mb`) to a timestamped archive file, and rotated archives are deleted after 365 days (`audit_log_archive_retention_days`). Rotation of the signed log preserves tamper-evidence: a signed checkpoint event marks the rotation boundary, so the hash chain remains verifiable across archived segments.
- **Entity Registry exemption**: the Persistent Entity Registry (canonical name ↔ token mappings used to unify name variants across documents) is long-lived by design and is **not** subject to the vault token TTL above.
- **On-demand erasure**: an authorized operator (bearing `OCU_AUDITOR_TOKEN`) can delete specific vault tokens immediately via `POST /api/vault/delete`, supporting data-subject erasure requests ahead of the automatic TTL.

You can adjust or disable any of this retention behavior at any time; it governs your own infrastructure, not a remote system.

---

## 8. Children's Data

OCULTAR is a developer infrastructure tool not directed at children. We do not knowingly process data submitted by or about children.

---

## 9. Changes to This Policy

Material changes will be noted in the [CHANGELOG](https://github.com/ocultar-dev/ocultar/blob/main/CHANGELOG.md) and reflected in the effective date above.

---

## 10. Contact

For privacy questions or data requests: **[edu@ocultar.dev](mailto:edu@ocultar.dev)**
