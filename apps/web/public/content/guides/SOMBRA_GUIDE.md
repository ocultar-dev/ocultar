# OCULTAR Sombra Gateway — Setup & Usage Guide

> **Audience:** Developers and operators who need an intelligent AI routing layer on top of OCULTAR — going beyond the raw proxy to support connectors, multi-provider routing, and structured query workflows.

---

## Table of Contents

1. [What Is Sombra — and When to Use It](#1-what-is-sombra--and-when-to-use-it)
2. [Prerequisites](#2-prerequisites)
3. [Running Sombra](#3-running-sombra)
4. [Configuration: Environment Variables, Not YAML](#4-configuration-environment-variables-not-yaml)
5. [Connectors](#5-connectors)
6. [Multi-Model Routing](#6-multi-model-routing)
7. [The `/query` Endpoint](#7-the-query-endpoint)
8. [How Sombra Integrates with OCULTAR](#8-how-sombra-integrates-with-ocultar)
9. [Quick Test](#9-quick-test)
10. [Troubleshooting](#10-troubleshooting)
11. [Entity Registry API](#11-entity-registry-api)

---

## 1. What Is Sombra — and When to Use It

**OCULTAR Proxy** (`apps/proxy`) protects data in transit — it redacts PII from payloads before they reach an LLM, and rehydrates tokens in the response, for a single upstream API.

**Sombra** (`apps/sombra`) is the agentic layer above the proxy, **in the same repo** — it is not a separate project to clone. It adds:

| Capability | OCULTAR Proxy | Sombra Gateway |
|---|---|---|
| Transparent HTTP PII redaction | ✅ | ✅ (built-in) |
| Re-hydration of LLM responses | ✅ | ✅ (built-in) |
| Multi-LLM routing (OpenAI, Gemini, Claude, Mistral, local mock) | ❌ | ✅ |
| File connector (ingest an uploaded document before querying) | ❌ | ✅ |
| Slack connector (`/v1/slack/events` webhook) | ❌ | ✅ |
| Persistent Entity Registry API | ❌ | ✅ |
| Single `/query` endpoint orchestrating everything | ❌ | ✅ |

**Use the raw OCULTAR proxy when:**
- You only need transparent PII scrubbing for a single upstream API
- You want zero-config sidecar deployment alongside an existing LLM client

**Use Sombra when:**
- You want to route queries across multiple AI providers
- You need to redact an uploaded file before querying the LLM
- You want a single orchestrated endpoint that handles: fetch → redact → route → respond → rehydrate

---

## 2. Prerequisites

| Requirement | Details |
|---|---|
| **Go** | 1.25+ (matches the repo's `go.work`) |
| **CGO enabled** | Sombra links the vault's DuckDB driver |
| **`OCU_MASTER_KEY`** | 32-byte AES key — same key used by the proxy if you want shared rehydration |
| **`OCU_JWT_SECRET`** | HS256 secret for Bearer token validation. Generate: `openssl rand -hex 32`. If unset, Sombra logs a warning and accepts any Bearer value as actor identity (dev only — insecure). |
| **LLM API key(s)** | At least one of `OPENAI_API_KEY`, `GEMINI_API_KEY`, `ANTHROPIC_API_KEY`, `MISTRAL_API_KEY` for the model(s) you intend to route to |

---

## 3. Running Sombra

Sombra is already part of this repo's Go workspace (`go.work` lists `./apps/sombra`) — there is no separate repo to clone and no `go work use` step.

```bash
export OCU_MASTER_KEY="your-key"
export OCU_SALT="your-salt"
export OPENAI_API_KEY="sk-..."

go run ./apps/sombra
```

Sombra listens on `:8086` by default (override with `SOMBRA_PORT`).

---

## 4. Configuration: Environment Variables, Not YAML

Sombra has **no YAML config file**. Its model list and connector are hardcoded in `apps/sombra/main.go` and its runtime behavior is controlled entirely by environment variables:

| Variable | Purpose |
|---|---|
| `SOMBRA_PORT` | Listen port (default `8086`) |
| `OCU_VAULT_PATH` | Vault file path — **defaults to `sombra_vault.db`**, a different file from the proxy's default `vault.db`. Set this to the same path for both services if you want tokens shared. |
| `OCU_MASTER_KEY` / `OCU_SALT` | Key derivation, shared with the proxy |
| `OCU_JWT_SECRET` | Bearer auth secret |
| `OCU_SOMBRA_ALLOW_DEGRADED_NER` | Opt out of fail-closed behavior if the Tier 2 SLM sidecar is unreachable (see `CLAUDE.md`) |
| `SLM_SIDECAR_URL` | Tier 2 AI NER sidecar endpoint (via `configs/config.yaml`'s `slm_sidecar_url`, falling back to this env var) |
| `GEMINI_API_KEY`, `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `MISTRAL_API_KEY` | Provider credentials for the hardcoded model list (§6) |
| `SOMBRA_MOCK_AI_URL` | If set, registers an additional `mock-ai` model pointing at this URL — for demos/local testing without a real provider key |
| `SLACK_SIGNING_SECRET` | Required to accept `/v1/slack/events` webhooks |
| `OCULTAR_DEBUG` | Set to `true` to include `ai_saw`, `prompt_redacted`, and `original_prompt` in `/query` response metadata (debug only — do not enable in production) |

To add a new model or connector, edit `apps/sombra/main.go` directly and rebuild — there is currently no config-file-driven way to do this.

---

## 5. Connectors

Only one connector is registered by default: **`file`**, backed by `apps/sombra/pkg/connector/file_connector.go`. A `/query` request uploads a file via multipart form data; Sombra reads it, redacts it, and includes it as context for the model.

```bash
curl -X POST http://localhost:8086/query \
  -H "Authorization: Bearer <actor-token>" \
  -F "connector=file" \
  -F "model=gpt-4o" \
  -F "prompt=Summarise the key health risk factors in this dataset." \
  -F "file=@/data/patient_records.csv"
```

An API connector implementation (`apps/sombra/pkg/connector/api_connector.go`, `NewAPIConnector`) exists in the codebase but is **not wired up in the stock binary** — using it requires registering it yourself in `main.go`.

Slack is handled separately, not through the generic connector interface: `POST /v1/slack/events` is a dedicated webhook route (`g.HandleSlackEvent`) gated on `SLACK_SIGNING_SECRET` — see `CLAUDE.md`.

---

## 6. Multi-Model Routing

The model list is hardcoded in `apps/sombra/main.go` and registered at startup:

| `model` value | Provider | Credential |
|---|---|---|
| `gemini-flash-latest` | Gemini 2.0 Flash | `GEMINI_API_KEY` |
| `gpt-4o` | OpenAI | `OPENAI_API_KEY` |
| `gpt-4o-mini` | OpenAI | `OPENAI_API_KEY` |
| `mistral-large-latest` | Mistral (OpenAI-compatible endpoint) | `MISTRAL_API_KEY` |
| `claude-sonnet-4-6` | Anthropic | `ANTHROPIC_API_KEY` |
| `mock-ai` | Local mock, only if `SOMBRA_MOCK_AI_URL` is set | none |

A `/query` request selects one of these by name in the `model` field. There is no separate routing-rules config — routing is just "the model you named."

**All routes share the same OCULTAR redaction pipeline** — regardless of which LLM is targeted, PII is stripped before leaving your infrastructure.

---

## 7. The `/query` Endpoint

```
POST http://localhost:8086/query
Content-Type: multipart/form-data
Authorization: Bearer <user-id-or-token>
```

This is a **form** request, not JSON — it needs to carry an optional file upload.

### Form Fields

| Field | Required | Description |
|---|---|---|
| `connector` | ✅ | Connector name — currently only `file` is registered by default |
| `model` | ✅ | Model name from the table in §6 |
| `prompt` | recommended | User prompt |
| `source_id` | optional | Passed to the connector's `FetchRequest` |
| `file` | optional | Multipart file upload — read, redacted, and used as data context |

Any other form field is forwarded to the connector as an arbitrary parameter. If no file and no `source_id` are given, the connector fetch is treated as empty and the prompt alone carries the context.

### Response Schema

```json
{
  "response": "The AI's answer (PII rehydrated)",
  "metadata": {
    "model": "gpt-4o",
    "connector": "file",
    "pii_was_redacted": true
  }
}
```

| Field | Description |
|---|---|
| `response` | The LLM's answer with original PII values restored from the vault |
| `metadata.model` | The model that handled the request |
| `metadata.connector` | The connector used |
| `metadata.pii_was_redacted` | `true` if at least one PII token appears in the redacted data or prompt |

If `OCULTAR_DEBUG=true`, the response also includes `metadata.ai_saw` (the exact redacted prompt+context sent to the model), `metadata.prompt_redacted`, and `metadata.original_prompt`. **Never enable this in production** — it deliberately surfaces post-redaction content for debugging.

---

## 8. How Sombra Integrates with OCULTAR

```
Client → POST /query (multipart form)
  → Sombra fetches connector data (file upload, if any)
  → Sombra pre-scrubs emails/account numbers, then runs the same refinery pipeline as the proxy
  → Sombra forwards redacted data + prompt to the selected model
  → Sombra receives the model's response
  → Sombra calls refinery.DecryptToken() for each token in the response
  → Client receives the rehydrated response
```

### Vault Sharing

Sombra and the proxy do **not** share a vault file by default — Sombra defaults to `sombra_vault.db`, the proxy defaults to `vault.db`. To let one service rehydrate tokens the other created, set `OCU_VAULT_PATH` to the **same path** for both, and use the **same** `OCU_MASTER_KEY` and `OCU_SALT` (both derive the AES key via HKDF-SHA256 from those two values).

---

## 9. Quick Test

```bash
curl -s -X POST http://localhost:8086/query \
  -H "Authorization: Bearer dev-actor" \
  -F "connector=file" \
  -F "model=gpt-4o" \
  -F 'prompt=Hello, my email is test@example.com. What is 2+2?' | jq .
```

Expected: `response` contains the answer, and `metadata.pii_was_redacted` is `true`. The email never reached OpenAI.

---

## 10. Troubleshooting

| Symptom | Likely Cause | Fix |
|---|---|---|
| `unauthorized: invalid or missing token` | No `Authorization: Bearer ...` header, or `OCU_JWT_SECRET` is set and the token doesn't validate | Send a Bearer token; check `OCU_JWT_SECRET` matches how the token was signed |
| `missing 'connector' parameter` | `connector` form field omitted | `connector` is required on every `/query` request |
| `unknown connector: "..."` | Connector name doesn't match a registered connector | Only `file` is registered by default (§5) |
| `401 Unauthorized` from the LLM provider | API key env var not set | `export OPENAI_API_KEY=sk-...` (or the matching var for your model) before running Sombra |
| `ai model request failed` (502) | Model routing/provider call failed | Check the provider's API key and that `model` matches a name from §6 |
| Response contains raw tokens `[EMAIL_…]` instead of plaintext | Rehydration vault path mismatch | Ensure Sombra's and the proxy's `OCU_VAULT_PATH` point to the same file |
| HKDF key mismatch (rehydration silently returns tokens) | Different `OCU_SALT` between Sombra and the proxy | Use the same `OCU_SALT` env var for both services |

---

## 11. Entity Registry API

The Sombra gateway exposes a persistent entity registry that collapses all name variants for a known identity into a single canonical token. This eliminates token fragmentation across sessions and documents.

**Default port:** `8086` (set via `SOMBRA_PORT` env var).

### `POST /v1/entities` — Register a single entity

```bash
curl -X POST http://localhost:8086/v1/entities \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <actor-token>" \
  -d '{
    "entity_type":    "PERSON",
    "canonical_name": "John Doe",
    "variants":       ["John", "Doe", "J. Doe"]
  }'
```

**Response:**
```json
{ "canonical_token": "[PERSON_1]" }
```

Registration is idempotent — sending the same `canonical_name` again merges new variants and returns the existing token. Supported `entity_type` values: `PERSON`, `ORGANIZATION` (and any type whose name should become the token prefix).

### `POST /v1/entities/seed` — Bulk seed from a roster

Accepts a flat JSON array or a `{"entities": [...]}` wrapper. Designed for startup seeding from a CRM, patient roster, or employee directory.

```bash
curl -X POST http://localhost:8086/v1/entities/seed \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-token>" \
  -d '[
    {
      "entity_type":    "PERSON",
      "canonical_name": "Jane Smith",
      "variants":       ["Jane", "Smith", "Dr. Smith"]
    },
    {
      "entity_type":    "ORGANIZATION",
      "canonical_name": "Acme Corporation",
      "variants":       ["Acme", "Acme Corp"]
    }
  ]'
```

**Response:**
```json
{ "seeded": 2, "tokens": ["[PERSON_1]", "[ORGANIZATION_1]"] }
```

Seeding is safe to re-run — duplicate `canonical_name` values are skipped without error.

### `GET /v1/entities` — List all registered entities

```bash
curl http://localhost:8086/v1/entities \
  -H "Authorization: Bearer <actor-token>"
```

**Response:**
```json
[
  {
    "id": "PERSON_1",
    "entity_type": "PERSON",
    "canonical_name": "John Doe",
    "variants": ["Doe", "J. Doe", "John"]
  },
  {
    "id": "ORGANIZATION_1",
    "entity_type": "ORGANIZATION",
    "canonical_name": "Acme Corporation",
    "variants": ["Acme", "Acme Corp"]
  }
]
```

### How it integrates with the refinery

Once entities are registered, the refinery automatically resolves any matching name fragment to the canonical token **before** the HMAC-SHA256 hash path. No configuration change is required — routing is automatic based on the token suffix format.

For the full guide including token format details, database schema, and use-case examples (healthcare, legal, HR, CRM), see the [Entity Registry Guide](ENTITY_REGISTRY_GUIDE.md).
