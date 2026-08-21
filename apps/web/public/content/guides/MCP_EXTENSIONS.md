# OCULTAR | MCP Extensions

OCULTAR ships three MCP (Model Context Protocol) extensions that plug directly into AI clients. Every extension exposes `refine_text`; the Claude and Mistral extensions additionally expose `reveal_tokens` and the Entity Registry tools (`register_entity`, `list_entities`, `seed_entities`); all three expose `sombra_query` for redacted queries through the Sombra gateway. Every tool enforces the zero-egress guarantee at the protocol level: if the service it depends on (the local Refinery, or Sombra for `sombra_query`) is unreachable, the tool fails closed and refuses to forward any data.

---

## Supported Clients

| Extension | Client | Package |
|---|---|---|
| **ocultar-claude-mcp** | Claude Desktop · Claude Code CLI | `pip install ocultar-claude-mcp` |
| **ocultar-goose-mcp** | Goose AI | `pip install ocultar-goose-mcp` |
| **ocultar-mistral-mcp** | Mistral Le Chat | `pip install ocultar-mistral-mcp` |

All three packages require **Python 3.10+** and the OCULTAR Refinery running locally on port 4141.

---

## Prerequisites

Start the Refinery before connecting any MCP client:

```bash
docker compose up
```

Or manually:

```bash
go run ./services/refinery/cmd/main.go --serve 4141
```

---

## Tools

### `refine_text`

Available in all three extensions.

Redacts PII from text before it reaches the AI model. Returns the cleaned text with all PII replaced by deterministic tokens, and a map of each token to its PII type.

**Input**
```json
{ "text": "Jean Dupont, jean.dupont@banque.fr, IBAN FR76 3000 6000 0112 3456 7890 189" }
```

**Output**
```json
{
  "cleanText": "[PERSON_a1b2c3d4e5f6a7b8], [EMAIL_9c8f7a1b2d3e4f50], IBAN [IBAN_7f3e9a2b1c4d5e60]",
  "tokenMap": {
    "[PERSON_a1b2c3d4e5f6a7b8]": "PERSON",
    "[EMAIL_9c8f7a1b2d3e4f50]": "EMAIL",
    "[IBAN_7f3e9a2b1c4d5e60]": "IBAN"
  }
}
```

Safe to expose to any AI session. No PII ever leaves your infrastructure.

### `reveal_tokens`

Available in the Claude and Mistral extensions only — not Goose, which deliberately omits auditor-only operations as unsuitable for automated agent workflows.

De-tokenizes specific tokens back to plaintext. Requires `OCULTAR_AUDITOR_TOKEN` — auditor-only. Every call is recorded in the immutable Ed25519-signed audit log with actor identity and timestamp.

**Input**
```json
{ "tokens": ["[EMAIL_9c8f7a1b2d3e4f50]", "[IBAN_7f3e9a2b1c4d5e60]"] }
```

### `register_entity`, `list_entities`, `seed_entities`

Available in the Claude and Mistral extensions only, for the same reason as `reveal_tokens` above — same `OCULTAR_AUDITOR_TOKEN` gate.

Manage the persistent Entity Registry so all variants of a name or identifier (e.g. "Alice", "A. Martin", "alice martin") resolve to the same token across files, prompts, and sessions.

**`register_entity` input**
```json
{ "entity_type": "PERSON", "canonical_name": "Alice Martin", "variants": ["Alice", "A. Martin"] }
```

**`register_entity` output**
```json
{ "canonical_token": "[PERSON_1]" }
```

**`seed_entities` input** — bulk-register from a roster:
```json
{ "entities": [{ "entity_type": "PERSON", "canonical_name": "Alice Martin", "variants": ["Alice"] }] }
```

**`list_entities`** takes no input and returns every registered entity.

### `sombra_query`

Available in all three extensions. Requires the Sombra gateway running separately (`go run ./apps/sombra`, default port 8086) and `OCULTAR_SOMBRA_TOKEN` set — Sombra rejects any request with no Bearer token.

Sends a prompt through Sombra, which redacts PII, routes the request to the chosen LLM, and rehydrates the response. Unlike `refine_text` (which only redacts text you hand it), this makes an actual LLM call on your behalf.

**Input**
```json
{ "prompt": "Summarize the attached invoice", "model": "gemini-flash-latest", "connector": "file" }
```

`model` is required — no default — pick from the models your Sombra deployment has configured (e.g. `gemini-flash-latest`, `gpt-4o`); it's passed through to Sombra as-is, and Sombra's own policy check rejects an unsupported value with a clear error. `connector` defaults to `"file"` if omitted. This version does not support file uploads through the MCP tool; the prompt itself carries the full query context.

---

## Claude Desktop

**Install:**
```bash
pip install ocultar-claude-mcp
```

**Configure** — add to `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or `%APPDATA%\Claude\claude_desktop_config.json` (Windows):

```json
{
  "mcpServers": {
    "ocultar-pii": {
      "command": "ocultar-claude-mcp",
      "env": {
        "OCULTAR_URL": "http://localhost:4141",
        "OCULTAR_API_KEY": "your-api-key",
        "OCULTAR_AUDITOR_TOKEN": "your-auditor-token",
        "OCULTAR_SOMBRA_URL": "http://localhost:8086",
        "OCULTAR_SOMBRA_TOKEN": "your-sombra-token"
      }
    }
  }
}
```

## Claude Code (CLI)

```bash
claude mcp add ocultar-pii -- ocultar-claude-mcp
```

Or add the same block above to `.claude/settings.json`.

---

## Goose AI

**Install:**
```bash
pip install ocultar-goose-mcp
```

**Configure in Goose:**
1. Open Goose Settings
2. Add Extension → Command-line Extension
3. Name: `ocultar-pii`
4. Command: `ocultar-goose-mcp`
5. Environment: `OCULTAR_URL=http://localhost:4141`, and optionally `OCULTAR_SOMBRA_URL=http://localhost:8086` + `OCULTAR_SOMBRA_TOKEN=your-sombra-token` to enable `sombra_query`

---

## Mistral Le Chat

Optimised for French and EU compliance: SIRET/SIREN, IBAN, French phone numbers, RGPD Article 25, CNIL requirements.

**Install:**
```bash
pip install ocultar-mistral-mcp
```

Or with `uvx` (no install needed):
```bash
uvx ocultar-mistral-mcp
```

**Configure** — in Mistral Le Chat, open **Settings → Tools → MCP Servers** and add:

```json
{
  "mcpServers": {
    "ocultar-pii": {
      "command": "ocultar-mistral-mcp",
      "env": {
        "OCULTAR_URL": "http://localhost:4141",
        "OCULTAR_API_KEY": "your-api-key",
        "OCULTAR_AUDITOR_TOKEN": "your-auditor-token",
        "OCULTAR_SOMBRA_URL": "http://localhost:8086",
        "OCULTAR_SOMBRA_TOKEN": "your-sombra-token"
      }
    }
  }
}
```

With `uvx`:
```json
{
  "mcpServers": {
    "ocultar-pii": {
      "command": "uvx",
      "args": ["ocultar-mistral-mcp"],
      "env": {
        "OCULTAR_URL": "http://localhost:4141",
        "OCULTAR_API_KEY": "your-api-key",
        "OCULTAR_AUDITOR_TOKEN": "your-auditor-token",
        "OCULTAR_SOMBRA_URL": "http://localhost:8086",
        "OCULTAR_SOMBRA_TOKEN": "your-sombra-token"
      }
    }
  }
}
```

### Why This Matters for French Enterprises

Sending customer data to any external AI API — including Mistral's cloud — without redaction constitutes an RGPD violation under Article 25 (Privacy by Design). The CNIL has issued enforcement guidance specifically targeting AI pipeline data flows.

OCULTAR ensures that:
- No raw PII ever reaches Le Chat's API endpoint
- SIRET, SIREN, IBAN, and French address formats are detected and tokenized
- Every vault access is logged in a tamper-evident, Ed25519-signed audit trail
- You remain the data controller — OCULTAR is a local processor under your full control

---

## Environment Variables

All three extensions share the same environment variables:

| Variable | Required | Default | Description |
|---|---|---|---|
| `OCULTAR_URL` | Yes | `http://localhost:4141` | URL of your local OCULTAR Refinery |
| `OCULTAR_API_KEY` | No | — | Bearer token for Refinery authentication |
| `OCULTAR_AUDITOR_TOKEN` | No | — | Enables `reveal_tokens`, `register_entity`, `list_entities`, `seed_entities` (Claude/Mistral only) — must match `OCU_AUDITOR_TOKEN` on the server |
| `OCULTAR_SOMBRA_URL` | No | `http://localhost:8086` | URL of your local OCULTAR Sombra gateway |
| `OCULTAR_SOMBRA_TOKEN` | No | — | Enables `sombra_query` (all three extensions) — Sombra rejects requests with no Bearer token |

---

## Security Model

- `refine_text` is safe to expose to any AI session — it only sends text to the local Refinery, which runs on `localhost`. No telemetry, no remote calls.
- `reveal_tokens` and the Entity Registry tools require `OCULTAR_AUDITOR_TOKEN`. Every call is logged with actor identity, timestamp, and Ed25519 signature in the tamper-proof audit trail.
- `sombra_query` requires `OCULTAR_SOMBRA_TOKEN` — Sombra rejects any request with no Bearer token. It's also the one tool whose answer contains rehydrated (real) PII by design: Sombra redacts before routing to the external LLM, but returns the fully rehydrated response to this MCP host, since that's the trusted local caller the query was made on behalf of. The redaction guarantee covers what reaches the external LLM, not the answer that comes back here.
- The Refinery vault uses AES-256-GCM with HKDF-SHA256 key derivation — tokens are useless without the master key.
- **Fail-closed guarantee:** if the service a tool depends on is unreachable for any reason, that tool returns an MCP error and refuses to forward raw data or vault contents to the caller.

---

## License

AGPLv3 — see [LICENSE](https://github.com/ocultar-dev/ocultar/blob/main/LICENSE). Commercial licensing available — see [COMMERCIAL_LICENSE.md](https://github.com/ocultar-dev/ocultar/blob/main/COMMERCIAL_LICENSE.md).
