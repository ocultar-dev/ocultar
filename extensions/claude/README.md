# Ocultar PII Refinery — Claude MCP Extension

[![PyPI](https://img.shields.io/pypi/v/ocultar-claude-mcp)](https://pypi.org/project/ocultar-claude-mcp/)

mcp-name: io.github.ocultar-dev/ocultar-pii

Zero-egress PII protection for Claude AI workflows.
Runs entirely in your infrastructure — no data ever leaves your environment.

## Tools

| Tool | Description |
|------|-------------|
| `refine_text` | Redacts PII before sending text to Claude. Returns clean text + token map. |
| `reveal_tokens` | De-tokenizes tokens back to plaintext (auditor-only, requires `OCULTAR_AUDITOR_TOKEN`). |
| `register_entity` | Registers a canonical PII entity and its variants so they resolve to the same token across sessions (auditor-only, requires `OCULTAR_AUDITOR_TOKEN`). |
| `list_entities` | Lists all registered PII entities (auditor-only, requires `OCULTAR_AUDITOR_TOKEN`). |
| `seed_entities` | Bulk-registers PII entities, e.g. from a CRM roster (auditor-only, requires `OCULTAR_AUDITOR_TOKEN`). |
| `sombra_query` | Asks a redacted question via the Ocultar Sombra gateway — redacts PII, routes to the chosen LLM, rehydrates the response (requires `OCULTAR_SOMBRA_TOKEN` and Sombra running separately). |

## Prerequisites

- Ocultar Refinery running locally:
  ```bash
  docker compose up
  ```
- Python 3.10+

## Installation

```bash
pip install ocultar-claude-mcp
```

## Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or
`%APPDATA%\Claude\claude_desktop_config.json` (Windows):

```json
{
  "mcpServers": {
    "ocultar-pii": {
      "command": "ocultar-claude-mcp",
      "env": {
        "OCULTAR_URL": "http://localhost:4141",
        "OCULTAR_API_KEY": "your-api-key"
      }
    }
  }
}
```

## Claude Code (CLI)

```bash
claude mcp add ocultar-pii -- ocultar-claude-mcp
```

Or add to `.claude/settings.json`:

```json
{
  "mcpServers": {
    "ocultar-pii": {
      "command": "ocultar-claude-mcp",
      "env": {
        "OCULTAR_URL": "http://localhost:4141",
        "OCULTAR_API_KEY": "your-api-key"
      }
    }
  }
}
```

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `OCULTAR_URL` | Yes | URL of your local Ocultar Refinery (default: `http://localhost:4141`) |
| `OCULTAR_API_KEY` | No | Bearer token for Refinery auth |
| `OCULTAR_AUDITOR_TOKEN` | No | Enables `reveal_tokens`, `register_entity`, `list_entities`, `seed_entities` — must match `OCU_AUDITOR_TOKEN` on the server |
| `OCULTAR_SOMBRA_URL` | No | URL of your local Ocultar Sombra gateway (default: `http://localhost:8086`) |
| `OCULTAR_SOMBRA_TOKEN` | No | Enables `sombra_query` — Sombra rejects requests with no Bearer token |

## Usage

Once connected, Claude will automatically call `refine_text` when you ask it to handle
sensitive data. You can also ask explicitly:

> "Refine this before processing: John Smith's email is john@example.com, SSN 123-45-6789"

Claude returns:
```json
{
  "cleanText": "John [NAME_a1b2c3d4e5f6a7b8]'s email is [EMAIL_9c8f7a1b2d3e4f50], SSN [SSN_3a1b2c4d5e6f7081]",
  "tokenMap": {
    "[NAME_a1b2c3d4e5f6a7b8]": "NAME",
    "[EMAIL_9c8f7a1b2d3e4f50]": "EMAIL",
    "[SSN_3a1b2c4d5e6f7081]": "SSN"
  }
}
```

For authorized workflows that need to restore PII after AI processing:

> "Reveal these tokens: [EMAIL_9c8f7a1b2d3e4f50], [SSN_3a1b2c4d5e6f7081]"

This call is recorded in the immutable Ed25519-signed audit log.

## Why Zero-Egress?

The Ocultar Refinery runs entirely on your machine. The MCP server communicates only
with `localhost` — no telemetry, no cloud calls, no supply chain attack surface.
If the Refinery is unreachable, both tools fail closed: raw PII is never forwarded.

## Security Model

- `refine_text` is safe to expose to any Claude session
- `reveal_tokens` requires `OCULTAR_AUDITOR_TOKEN` and every call is logged with actor, timestamp, and Ed25519 signature in the audit trail
- The Refinery's vault uses AES-256-GCM with HKDF-SHA256 key derivation — tokens are useless without the master key

## License

AGPLv3 — see [LICENSE](../../LICENSE). Commercial licensing available for organizations that cannot comply with AGPLv3's source-disclosure requirements — see [COMMERCIAL_LICENSE.md](../../COMMERCIAL_LICENSE.md).
