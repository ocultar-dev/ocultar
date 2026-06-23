# Compliance Posture

**Effective date:** 23 June 2026
**Contact:** [edu@ocultar.dev](mailto:edu@ocultar.dev)

This is a one-page, honest statement of where OCULTAR stands today. For the
underlying data-handling guarantees, see [`PRIVACY.md`](PRIVACY.md); for the
vulnerability-disclosure process, see [`SECURITY.md`](SECURITY.md); for
detailed, control-by-control mappings used by procurement/security teams,
see [`docs/security_readiness/`](docs/security_readiness/) (CSA CAIQ v4.1,
SIG Lite, VSAQ).

## Certifications

**No SOC 2 or ISO 27001 certification today.** Both require a third-party
audit, which OCULTAR has not yet undergone — no code change makes this
"done," only an external auditor's report does. This is a roadmap item, not
a committed date.

What you *can* verify yourself in the meantime: OCULTAR is open source
(Apache 2.0), so every control claim below is checkable directly against
the code that's actually running, not just a vendor's word for it.

## Data Minimization & Storage Limitation (GDPR Art. 5(1)(e))

Implemented and on by default:

- **Vault tokens** (encrypted PII behind a `[TYPE_token]`) are purged 90
  days after creation, enforced by a background sweep
  (`vault.RunRetentionLoop`). Configurable via `vault_retention_days` in
  `configs/config.yaml`.
- **Audit logs** rotate at 50MB and rotated archives are deleted after 365
  days, with the immutable hash chain's tamper-evidence preserved across
  the rotation boundary via a signed checkpoint.
- **On-demand erasure**: `POST /api/vault/delete` lets an authorized
  operator delete specific tokens ahead of the TTL, for data-subject
  erasure requests.

Full detail in [`PRIVACY.md` §7](PRIVACY.md#7-data-retention-and-deletion).

## Zero-Egress Enforcement

The "no raw PII leaves the deployment" claim is **code-enforced, not just
operator discipline**: `NewRemoteScanner` refuses to start if
`SLM_SIDECAR_URL` doesn't resolve to a loopback address
(`services/refinery/pkg/inference/remote.go`), unless an operator
explicitly opts out via `OCU_ALLOW_REMOTE_SLM=true` — a deliberate,
loud, single-flag override, not a silent default.

## Audit Trail Integrity

The immutable audit log (`services/refinery/pkg/audit/immutable.go`) is
Ed25519-signed and SHA-256 hash-chained. It records actor, action, token
ID, and timestamp — **never plaintext PII**. A machine-readable snapshot of
active policies, enabled detection tiers, and the audit chain is available
at `GET /api/compliance/evidence` for integration into platforms like Vanta
or Drata.

## Data Processing Agreement

OCULTAR runs entirely within your own infrastructure — you are the data
controller, OCULTAR is your local data processor (see
[`PRIVACY.md` §5](PRIVACY.md#5-your-role-as-data-controller)). A DPA
template is available on request: [edu@ocultar.dev](mailto:edu@ocultar.dev).

## Vulnerability Disclosure

Bounded SLA: acknowledgment within 48 hours, fix target within 30 days for
Critical/High severity, 90 days for Medium/Low. Full process in
[`SECURITY.md`](SECURITY.md).

## Questions

For anything not covered here — a specific control mapping, a security
questionnaire, or a DPA — email [edu@ocultar.dev](mailto:edu@ocultar.dev).
