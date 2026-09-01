# Ocultar

[![AGPL v3](https://img.shields.io/badge/license-AGPL%20v3-blue.svg)](LICENSE) [![Commercial License](https://img.shields.io/badge/license-Commercial-green.svg)](COMMERCIAL_LICENSE.md)
[![Go 1.24+](https://img.shields.io/badge/go-1.24%2B-00ADD8.svg)](https://go.dev)
[![Docker](https://img.shields.io/badge/docker-ghcr.io%2Focultar--dev%2Focultar-blue?logo=docker)](https://github.com/ocultar-dev/ocultar/pkgs/container/ocultar)
[![Release](https://img.shields.io/github/v/release/ocultar-dev/ocultar)](https://github.com/ocultar-dev/ocultar/releases/latest)

Ocultar is an open-source local PII/PHI masking engine for AI workflows.

It runs as a local HTTP sidecar. Send it text before it reaches a cloud LLM; it returns the
same text with every piece of personal data replaced by a deterministic, reversible token
(`[EMAIL_9c8f7a1b2d3e4f50]`, `[PERSON_3a12b4cd91e7f6a0]`, …). Originals are encrypted and stored in a
local vault. Callers with the auditor token can restore them.

No PII ever reaches the upstream model.

---

## Quick start — Docker

```bash
export OCU_MASTER_KEY=$(openssl rand -hex 32)
export OCU_SALT=$(openssl rand -hex 16)
export OCU_AUDITOR_TOKEN=$(openssl rand -hex 24)

docker run --rm -p 4141:4141 \
  -e OCU_MASTER_KEY \
  -e OCU_SALT \
  -e OCU_AUDITOR_TOKEN \
  ghcr.io/ocultar-dev/ocultar:latest -serve 4141
```

## Quick start — build from source

```bash
CGO_ENABLED=1 go build -o ocultar ./services/refinery/cmd/

OCU_MASTER_KEY=$(openssl rand -hex 32) \
OCU_SALT=$(openssl rand -hex 16) \
OCU_AUDITOR_TOKEN=$(openssl rand -hex 24) \
./ocultar -serve 4141
```

---

## API reference

### `GET /api/health`

Returns engine status. No authentication required.

```json
{
  "status": "healthy",
  "version": "1.15.0",
  "vault": { "status": "online" },
  "slm":   { "status": "online", "circuit": "closed" }
}
```

---

### `POST /api/refine` and `POST /api/refine/file`

Mask PII in text, JSON, or uploaded files. No authentication required.

**`POST /api/refine` Request body**: raw text string or any JSON value.
**`POST /api/refine/file` Request body**: `multipart/form-data` with a `file` field (max 10MB).

**Response**:

```json
{
  "refined": "{\"message\":\"Hello [PERSON_3a12b4cd91e7f6a0], your order [EMAIL_9c8f7a1b2d3e4f50] is ready.\"}",
  "report": {
    "hits": 2,
    "types": ["PERSON", "EMAIL"]
  }
}
```

> `refined` is a JSON-encoded string — parse it once to get the masked payload.

---

### `POST /api/reveal`

Restore vault tokens back to originals.

**Authentication**: `Authorization: Bearer <OCU_AUDITOR_TOKEN>` header required.
Returns `403` if `OCU_AUDITOR_TOKEN` is not set on the server.

**Request body**:

```json
{ "tokens": ["[PERSON_3a12b4cd91e7f6a0]", "[EMAIL_9c8f7a1b2d3e4f50]"] }
```

**Response**:

```json
{
  "results": {
    "[PERSON_3a12b4cd91e7f6a0]": "Alice Martin",
    "[EMAIL_9c8f7a1b2d3e4f50]": "alice@example.com"
  }
}
```

---

### `GET /api/entities` · `POST /api/entities` · `POST /api/entities/seed`

Manage the persistent entity registry (pre-seed canonical names so all variants map to the
same token). Requires `Authorization: Bearer <OCU_AUDITOR_TOKEN>`.

---

## Architecture

Every request runs through one linear pipeline, in this order (see
`services/refinery/pkg/refinery/refinery.go`, `RefineString`):

| Tier | Shield | What it does |
|------|--------|---------------|
| 0.1 | Base64 Evasion | Recursively decodes Base64/JWT/URL-encoded payloads and rescans the decoded content |
| 0 | Dictionary | VIP/org names from `configs/protected_entities.json` |
| 0.5 | Entity Registry Pre-Pass | Replaces all registered entity variants by direct string match *before* NER runs, so known identities are masked even when the model misses them |
| 1 | Rule Engine | EMAIL, SSN, IBAN, credit cards, 50+ national ID formats |
| 1.1 | Phone Shield | libphonenumber validation |
| 1.2 | Address Shield | Heuristic street address parser (EN/FR/ES/DE) |
| 1.5 | Contextual (Greeting & Signature) | Names in greetings, signatures, interrogative sentences |
| 2 | AI NER | SLM sidecar named-entity recognition (mandatory phase when Tier 2 is enabled) |
| 2.5 | Boundary Artifact Cleanup | Absorbs 1-3 char orphaned fragments left adjacent to tokens by SLM sub-word tokenization |
| 3 | Structural Heuristics | Generalized multilingual rules — titles, possessives, semantic triggers (e.g. `DIVORCE`, `HOSPITAL`) |

Tiers 0.1–1.5 and 3 always run and never leave the machine. Tier 2 is optional — see below.

### Tier 2 — SLM-based NER (higher recall, configurable endpoint)

Sends text to a local AI sidecar for named-entity recognition. The scanner is always
initialized but produces no results unless a compatible sidecar is running at `SLM_SIDECAR_URL`.
Point it at a [privacy-filter](https://huggingface.co/openai/privacy-filter) or llama.cpp instance to activate NER.

```bash
SLM_SIDECAR_URL=http://localhost:8085 ./ocultar -serve 4141
```

Use `SLM_ADAPTER=openai-chat` for a llama.cpp / Qwen endpoint, or leave unset for the
privacy-filter protocol (default). `NewRemoteScanner` refuses to dial a non-loopback
`SLM_SIDECAR_URL` unless `OCU_ALLOW_REMOTE_SLM=true` is explicitly set — Tier 2 sends
raw, un-redacted text to that endpoint, so this guard keeps a misconfigured URL from
silently exfiltrating PII off-host.

---

## Sombra Gateway

`apps/sombra` is a separate agentic LLM gateway (default port 8086) — an OpenAI-compatible
`/v1/chat/completions` proxy with real SSE streaming, JWT Bearer auth, circuit-breaking,
and its own Ed25519-signed audit log. It runs the same detection pipeline as the proxy but
is built for gateway-style deployments (Slack app connector, multi-provider routing) rather
than a single fixed upstream. See `docs/guides/architecture/sombra.md` for the full picture.

---

## MCP integrations

OCULTAR ships as an MCP server for common AI coding/agent clients, published on PyPI:
`ocultar-claude-mcp`, `ocultar-goose-mcp`, `ocultar-mistral-mcp`. Each exposes tools to
refine text and manage the entity registry (`register_entity`, `sombra_query`) directly
from the client. See `apps/web/public/content/guides/MCP_EXTENSIONS.md` for setup.

---

## Privacy model

- **Zero-egress design.** Masked tokens (`[EMAIL_9c8f7a1b2d3e4f50]`, …) are the only data forwarded to the upstream model. Raw text is not transmitted.
- **Local vault only.** The mapping of each token back to its original value is stored in an encrypted DuckDB vault (`vault.db`) on the local filesystem using AES-256-GCM with HKDF-SHA256. The vault file is never transmitted.
- **Data retention.** Enforces GDPR Art. 5(1)(e) storage limitation by automatically purging expired vault tokens (default TTL: 90 days) via a background retention sweep.
- **Token mapping retention.** Each detected PII value is encrypted and stored in the vault keyed by its token (e.g. `[EMAIL_9c8f7a1b2d3e4f50]` → ciphertext). The refinery does not store the raw, unmasked prompt as a whole — only the individual token-to-plaintext mappings. `/api/reveal` decrypts mappings for tokens it's given; it does not reconstruct or diff the original prompt. If reveal access is not desired, do not configure `OCU_AUDITOR_TOKEN` — without an auditor token the endpoint returns `403`.
- **Fail-closed design.** If the refinery encounters an error or is unavailable, the gateway returns a `5xx` error and stops — it does not forward raw text as a fallback.

---

## Configuration

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `OCU_MASTER_KEY` | Yes | — (server refuses to start if unset) | 32+ byte AES key material for HKDF |
| `OCU_SALT` | Yes (production) | built-in default | Per-deployment HKDF salt |
| `OCU_AUDITOR_TOKEN` | Yes | — | Bearer token for `/api/reveal` and `/api/entities` |
| `OCU_VAULT_PATH` | No | `vault.db` | DuckDB vault file path |
| `SLM_SIDECAR_URL` | No | `http://localhost:8085` | Tier 2 NER sidecar endpoint |
| `SLM_ADAPTER` | No | `privacy-filter` | Sidecar protocol: `privacy-filter` or `openai-chat` |

---

## Building from source

Requires Go 1.24+ with CGO enabled (DuckDB and libphonenumber need a C compiler).

```bash
git clone https://github.com/ocultar-dev/ocultar.git
cd ocultar
make build
```

Run tests:

```bash
CGO_ENABLED=1 go test ./...
```

---

## License

AGPLv3 — see [LICENSE](LICENSE). Commercial licensing available, see [COMMERCIAL_LICENSE.md](COMMERCIAL_LICENSE.md).

