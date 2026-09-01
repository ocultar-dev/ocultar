# MCP License Fix + Entity Registry & Sombra Query Tools Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Correct the license metadata across all three MCP extensions (Apache-2.0 → AGPLv3, matching the rest of the repo since 2026-06-28) and add two new capabilities — Entity Registry tools (Claude + Mistral only) and a `sombra_query` tool (all three) — bumping each package to `0.3.0`.

**Architecture:** Each of the three extension packages (`extensions/claude`, `extensions/goose`, `extensions/mistral`) gets its license metadata, version, Python tool implementation, packaging manifest, and README updated together as one coherent unit — a reviewer needs to see the whole package consistent, not partial. A fourth task updates the two shared docs (the cross-client guide and the changelog) that reference all three.

**Tech Stack:** Python 3.10+, `httpx` (async HTTP client, already a dependency), the `mcp` SDK's `Server`/`types.Tool` pattern already used by all three extensions.

## Global Constraints

- License identifier is **`AGPL-3.0-only`**, not `AGPL-3.0-or-later` — confirmed by inspecting `LICENSE` and `COMMERCIAL_LICENSE.md`: neither contains an "or (at your option) any later version" grant specific to this project (the phrase that does appear in `LICENSE:638` is boilerplate instructional text for *how to apply* the license, not a statement that Ocultar chose that option). The PyPI trove classifier is `License :: OSI Approved :: GNU Affero General Public License v3` (no "+"/"or later" variant).
- Every touched package's version goes `0.2.0` → `0.3.0`. This includes the version string embedded in `manifest.json`'s `mcp_config.args` (e.g. `"ocultar-claude-mcp@0.2.0"` → `"ocultar-claude-mcp@0.3.0"`) — easy to miss since it's a second occurrence of the version number in the same file.
- Entity Registry tools (`register_entity`, `list_entities`, `seed_entities`) are added to **Claude and Mistral only**, never Goose — matching Goose's own existing, deliberate exclusion of `reveal_tokens` ("auditor-only, not suitable for automated agent workflows" — same `OCULTAR_AUDITOR_TOKEN` gate applies here).
- `sombra_query` is added to **all three** extensions.
- `sombra_query` uses two new env vars, extension-wide: `OCULTAR_SOMBRA_URL` (default `http://localhost:8086`) and `OCULTAR_SOMBRA_TOKEN` (no default, required — Sombra's `extractActor` rejects any request with an empty/missing Bearer token outright, there is no unauthenticated fallback mode).
- `sombra_query` sends `multipart/form-data` (via `httpx`'s `data=` param, not `json=`) — Sombra's handler calls `r.ParseMultipartForm`, not a JSON decoder; sending JSON would not be parsed.
- `sombra_query`'s `model` parameter is **passed through with no client-side validation or hardcoded list** — Sombra's own policy check on the Go side is the source of truth, and rejects an invalid model with its own clear error.
- `sombra_query` has no file-upload support in this version — Sombra's handler already has a working "prompt-only" fallback path when no file/`source_id` is given, so this is a real supported mode, not a partial implementation.
- Preserve each file's existing local error-wrapping convention exactly — do not "fix" or unify them, that's out of scope: `extensions/claude/ocultar_claude_mcp.py`'s existing tools wrap connection errors as `raise RuntimeError(_connection_error(...))` (double-wrapped, but functionally harmless and pre-existing); `extensions/mistral/ocultar_mistral_mcp.py`'s existing tools call `raise _connection_error(...)` directly (no double-wrap). New code in each file must match that file's own existing convention, not the other file's.
- Entity Registry tools use a 15-second `httpx` timeout, matching `refine_text`/`reveal_tokens` in the same file (pure redaction-engine calls). `sombra_query` uses a 60-second timeout — it triggers an actual server-side LLM call via Sombra, not just local redaction, so the existing 15s budget elsewhere is not the right comparison.

---

## File Structure

- Modify: `extensions/claude/pyproject.toml`, `extensions/claude/ocultar_claude_mcp.py`, `extensions/claude/manifest.json`, `extensions/claude/README.md`
- Modify: `extensions/goose/pyproject.toml`, `extensions/goose/ocultar_mcp.py`, `extensions/goose/extension.yaml`, `extensions/goose/README.md`
- Modify: `extensions/mistral/pyproject.toml`, `extensions/mistral/ocultar_mistral_mcp.py`, `extensions/mistral/manifest.json`, `extensions/mistral/README.md`
- Modify: `apps/web/public/content/guides/MCP_EXTENSIONS.md`, `CHANGELOG.md`

No new files. No Go code touched.

---

### Task 1: Claude extension — license, version, Entity Registry + sombra_query tools

**Files:**
- Modify: `extensions/claude/pyproject.toml`
- Modify: `extensions/claude/ocultar_claude_mcp.py`
- Modify: `extensions/claude/manifest.json`
- Modify: `extensions/claude/README.md`

**Interfaces:**
- Produces: four new MCP tools (`register_entity`, `list_entities`, `seed_entities`, `sombra_query`) alongside the existing `refine_text`/`reveal_tokens`, and two new module-level env vars (`OCULTAR_SOMBRA_URL`, `OCULTAR_SOMBRA_TOKEN`). Task 4 (shared docs) references these tool names and env vars — they must match exactly.

- [ ] **Step 1: Update `extensions/claude/pyproject.toml`**

Change:
```toml
version = "0.2.0"
```
to:
```toml
version = "0.3.0"
```

Change:
```toml
license = { text = "Apache-2.0" }
```
to:
```toml
license = { text = "AGPL-3.0-only" }
```

Change:
```toml
    "License :: OSI Approved :: Apache Software License",
```
to:
```toml
    "License :: OSI Approved :: GNU Affero General Public License v3",
```

- [ ] **Step 2: Add the new module-level env vars to `extensions/claude/ocultar_claude_mcp.py`**

Find:
```python
OCULTAR_URL = os.environ.get("OCULTAR_URL", "http://localhost:4141").rstrip("/")
OCULTAR_API_KEY = os.environ.get("OCULTAR_API_KEY", "")
OCULTAR_AUDITOR_TOKEN = os.environ.get("OCULTAR_AUDITOR_TOKEN", "")
```
Replace with:
```python
OCULTAR_URL = os.environ.get("OCULTAR_URL", "http://localhost:4141").rstrip("/")
OCULTAR_API_KEY = os.environ.get("OCULTAR_API_KEY", "")
OCULTAR_AUDITOR_TOKEN = os.environ.get("OCULTAR_AUDITOR_TOKEN", "")
OCULTAR_SOMBRA_URL = os.environ.get("OCULTAR_SOMBRA_URL", "http://localhost:8086").rstrip("/")
OCULTAR_SOMBRA_TOKEN = os.environ.get("OCULTAR_SOMBRA_TOKEN", "")
```

- [ ] **Step 3: Add the `_sombra_headers` and `_sombra_connection_error` helpers**

Find:
```python
def _connection_error(endpoint: str) -> RuntimeError:
    return RuntimeError(
        f"Cannot connect to Ocultar at {OCULTAR_URL}{endpoint}. "
        "Start the Refinery first: `docker compose up` or "
        "`go run ./services/refinery/cmd/main.go --serve 8080`. "
        "Raw data withheld to preserve zero-egress guarantee."
    )
```
Replace with (adds two new functions immediately after):
```python
def _connection_error(endpoint: str) -> RuntimeError:
    return RuntimeError(
        f"Cannot connect to Ocultar at {OCULTAR_URL}{endpoint}. "
        "Start the Refinery first: `docker compose up` or "
        "`go run ./services/refinery/cmd/main.go --serve 8080`. "
        "Raw data withheld to preserve zero-egress guarantee."
    )


def _sombra_headers() -> dict[str, str]:
    if OCULTAR_SOMBRA_TOKEN:
        return {"Authorization": f"Bearer {OCULTAR_SOMBRA_TOKEN}"}
    return {}


def _sombra_connection_error() -> RuntimeError:
    return RuntimeError(
        f"Cannot connect to Sombra at {OCULTAR_SOMBRA_URL}/query. "
        "Start the Sombra gateway first: `go run ./apps/sombra` (default port 8086). "
        "Raw data withheld to preserve zero-egress guarantee."
    )
```

- [ ] **Step 4: Add the four new tool definitions to `list_tools()`**

Find the end of the `reveal_tokens` `types.Tool(...)` entry and the closing of the list:
```python
                    }
                },
                "required": ["tokens"],
            },
        ),
    ]
```
Replace with:
```python
                    }
                },
                "required": ["tokens"],
            },
        ),
        types.Tool(
            name="register_entity",
            title="Register a canonical PII entity",
            description=(
                "Pre-register a canonical name/identifier and its variants in the "
                "persistent Entity Registry so all variants map to the same token "
                "consistently across sessions (e.g. 'Alice', 'A. Martin', and "
                "'alice martin' all resolving to [PERSON_1]). Requires "
                "OCULTAR_AUDITOR_TOKEN to be set."
            ),
            inputSchema={
                "type": "object",
                "properties": {
                    "entity_type": {
                        "type": "string",
                        "description": "Entity category, e.g. 'PERSON' or 'ORG'.",
                    },
                    "canonical_name": {
                        "type": "string",
                        "description": "The canonical name to register, e.g. 'Alice Martin'.",
                    },
                    "variants": {
                        "type": "array",
                        "items": {"type": "string"},
                        "description": "Alternate forms that should resolve to the same token.",
                    },
                },
                "required": ["entity_type", "canonical_name"],
            },
        ),
        types.Tool(
            name="list_entities",
            title="List registered PII entities",
            description=(
                "List every entry in the persistent Entity Registry. Requires "
                "OCULTAR_AUDITOR_TOKEN to be set."
            ),
            readOnlyHint=True,
            inputSchema={"type": "object", "properties": {}},
        ),
        types.Tool(
            name="seed_entities",
            title="Bulk-register PII entities",
            description=(
                "Bulk-register multiple canonical entities in one call, e.g. from "
                "a CRM roster or patient list. Requires OCULTAR_AUDITOR_TOKEN to be set."
            ),
            inputSchema={
                "type": "object",
                "properties": {
                    "entities": {
                        "type": "array",
                        "items": {
                            "type": "object",
                            "properties": {
                                "entity_type": {"type": "string"},
                                "canonical_name": {"type": "string"},
                                "variants": {
                                    "type": "array",
                                    "items": {"type": "string"},
                                },
                            },
                            "required": ["entity_type", "canonical_name"],
                        },
                        "description": "List of entities to register.",
                    }
                },
                "required": ["entities"],
            },
        ),
        types.Tool(
            name="sombra_query",
            title="Ask a redacted question via the Sombra gateway",
            description=(
                "Send a prompt through the Ocultar Sombra gateway, which redacts "
                "PII, routes the request to the chosen LLM, and rehydrates the "
                "response. Requires the Sombra gateway to be running separately "
                "(`go run ./apps/sombra`, default port 8086) and "
                "OCULTAR_SOMBRA_TOKEN to be set."
            ),
            inputSchema={
                "type": "object",
                "properties": {
                    "prompt": {
                        "type": "string",
                        "description": "The question or instruction to send.",
                    },
                    "model": {
                        "type": "string",
                        "description": (
                            "Which model to route to, e.g. 'gemini-flash-latest', "
                            "'gpt-4o', 'gpt-4o-mini', 'mistral-large-latest', "
                            "'claude-sonnet-4-6'. Passed through to Sombra as-is."
                        ),
                    },
                    "connector": {
                        "type": "string",
                        "description": "Data connector to use. Defaults to 'file'.",
                    },
                },
                "required": ["prompt", "model"],
            },
        ),
    ]
```

- [ ] **Step 5: Extend the `call_tool` dispatcher**

Find:
```python
@app.call_tool()
async def call_tool(name: str, arguments: dict) -> list[types.TextContent]:
    if name == "refine_text":
        return await _refine_text(arguments)
    if name == "reveal_tokens":
        return await _reveal_tokens(arguments)
    raise ValueError(f"Unknown tool: {name}")
```
Replace with:
```python
@app.call_tool()
async def call_tool(name: str, arguments: dict) -> list[types.TextContent]:
    if name == "refine_text":
        return await _refine_text(arguments)
    if name == "reveal_tokens":
        return await _reveal_tokens(arguments)
    if name == "register_entity":
        return await _register_entity(arguments)
    if name == "list_entities":
        return await _list_entities(arguments)
    if name == "seed_entities":
        return await _seed_entities(arguments)
    if name == "sombra_query":
        return await _sombra_query(arguments)
    raise ValueError(f"Unknown tool: {name}")
```

- [ ] **Step 6: Add the four new implementation functions**

Find the end of `_reveal_tokens` and the start of `main`:
```python
    data = response.json()
    return [types.TextContent(type="text", text=json.dumps(data, ensure_ascii=False))]


async def main() -> None:
```
Replace with (inserts four new functions between them):
```python
    data = response.json()
    return [types.TextContent(type="text", text=json.dumps(data, ensure_ascii=False))]


async def _register_entity(arguments: dict) -> list[types.TextContent]:
    entity_type = arguments.get("entity_type", "")
    canonical_name = arguments.get("canonical_name", "")
    variants = arguments.get("variants", [])
    if not entity_type or not canonical_name:
        raise RuntimeError("entity_type and canonical_name are required.")

    if not OCULTAR_AUDITOR_TOKEN:
        raise RuntimeError(
            "OCULTAR_AUDITOR_TOKEN is not set. "
            "Entity registration is an auditor-only operation. "
            "Set the environment variable to enable it."
        )

    try:
        async with httpx.AsyncClient(timeout=15.0) as client:
            response = await client.post(
                f"{OCULTAR_URL}/api/entities",
                json={
                    "entity_type": entity_type,
                    "canonical_name": canonical_name,
                    "variants": variants,
                },
                headers=_auditor_headers(),
            )
            response.raise_for_status()
    except httpx.ConnectError:
        raise RuntimeError(_connection_error("/api/entities"))
    except httpx.TimeoutException:
        raise RuntimeError(
            "Ocultar /api/entities timed out (15 s). "
            "Raw data withheld to preserve zero-egress guarantee."
        )
    except httpx.HTTPStatusError as exc:
        if exc.response.status_code in (401, 403):
            raise RuntimeError(
                "Ocultar rejected the auditor token (401/403). "
                "Check OCULTAR_AUDITOR_TOKEN matches OCU_AUDITOR_TOKEN on the server."
            )
        raise RuntimeError(
            f"Ocultar /api/entities returned HTTP {exc.response.status_code}: "
            f"{exc.response.text[:300]}"
        )

    data = response.json()
    return [types.TextContent(type="text", text=json.dumps(data, ensure_ascii=False))]


async def _list_entities(arguments: dict) -> list[types.TextContent]:
    if not OCULTAR_AUDITOR_TOKEN:
        raise RuntimeError(
            "OCULTAR_AUDITOR_TOKEN is not set. "
            "Listing entities is an auditor-only operation. "
            "Set the environment variable to enable it."
        )

    try:
        async with httpx.AsyncClient(timeout=15.0) as client:
            response = await client.get(
                f"{OCULTAR_URL}/api/entities",
                headers=_auditor_headers(),
            )
            response.raise_for_status()
    except httpx.ConnectError:
        raise RuntimeError(_connection_error("/api/entities"))
    except httpx.TimeoutException:
        raise RuntimeError(
            "Ocultar /api/entities timed out (15 s). "
            "Raw data withheld to preserve zero-egress guarantee."
        )
    except httpx.HTTPStatusError as exc:
        if exc.response.status_code in (401, 403):
            raise RuntimeError(
                "Ocultar rejected the auditor token (401/403). "
                "Check OCULTAR_AUDITOR_TOKEN matches OCU_AUDITOR_TOKEN on the server."
            )
        raise RuntimeError(
            f"Ocultar /api/entities returned HTTP {exc.response.status_code}: "
            f"{exc.response.text[:300]}"
        )

    data = response.json()
    return [types.TextContent(type="text", text=json.dumps(data, ensure_ascii=False))]


async def _seed_entities(arguments: dict) -> list[types.TextContent]:
    entities = arguments.get("entities", [])
    if not entities:
        return [types.TextContent(type="text", text=json.dumps({"seeded": 0, "tokens": []}))]

    if not OCULTAR_AUDITOR_TOKEN:
        raise RuntimeError(
            "OCULTAR_AUDITOR_TOKEN is not set. "
            "Seeding entities is an auditor-only operation. "
            "Set the environment variable to enable it."
        )

    try:
        async with httpx.AsyncClient(timeout=15.0) as client:
            response = await client.post(
                f"{OCULTAR_URL}/api/entities/seed",
                json=entities,
                headers=_auditor_headers(),
            )
            response.raise_for_status()
    except httpx.ConnectError:
        raise RuntimeError(_connection_error("/api/entities/seed"))
    except httpx.TimeoutException:
        raise RuntimeError(
            "Ocultar /api/entities/seed timed out (15 s). "
            "Raw data withheld to preserve zero-egress guarantee."
        )
    except httpx.HTTPStatusError as exc:
        if exc.response.status_code in (401, 403):
            raise RuntimeError(
                "Ocultar rejected the auditor token (401/403). "
                "Check OCULTAR_AUDITOR_TOKEN matches OCU_AUDITOR_TOKEN on the server."
            )
        raise RuntimeError(
            f"Ocultar /api/entities/seed returned HTTP {exc.response.status_code}: "
            f"{exc.response.text[:300]}"
        )

    data = response.json()
    return [types.TextContent(type="text", text=json.dumps(data, ensure_ascii=False))]


async def _sombra_query(arguments: dict) -> list[types.TextContent]:
    prompt = arguments.get("prompt", "")
    model = arguments.get("model", "")
    connector = arguments.get("connector", "file")
    if not prompt or not model:
        raise RuntimeError("prompt and model are required.")

    if not OCULTAR_SOMBRA_TOKEN:
        raise RuntimeError(
            "OCULTAR_SOMBRA_TOKEN is not set. "
            "The Sombra gateway rejects every request with no Bearer token. "
            "Set the environment variable to enable this tool."
        )

    try:
        async with httpx.AsyncClient(timeout=60.0) as client:
            response = await client.post(
                f"{OCULTAR_SOMBRA_URL}/query",
                data={"connector": connector, "model": model, "prompt": prompt},
                headers=_sombra_headers(),
            )
            response.raise_for_status()
    except httpx.ConnectError:
        raise RuntimeError(_sombra_connection_error())
    except httpx.TimeoutException:
        raise RuntimeError(
            f"Sombra /query timed out (60 s) at {OCULTAR_SOMBRA_URL}. "
            "Raw data withheld to preserve zero-egress guarantee."
        )
    except httpx.HTTPStatusError as exc:
        if exc.response.status_code == 401:
            raise RuntimeError(
                "Sombra rejected the request (401 unauthorized). "
                "Check OCULTAR_SOMBRA_TOKEN is set to a non-empty value."
            )
        raise RuntimeError(
            f"Sombra /query returned HTTP {exc.response.status_code}: "
            f"{exc.response.text[:300]}"
        )

    return [types.TextContent(type="text", text=response.text)]


async def main() -> None:
```

Note: this file's existing convention double-wraps (`raise RuntimeError(_connection_error(...))`) — Step 6 matches that exactly on purpose, per Global Constraints.

- [ ] **Step 7: Verify the file compiles and imports cleanly**

Run: `python3 -m py_compile extensions/claude/ocultar_claude_mcp.py && echo OK`
Expected: `OK`, no syntax errors.

- [ ] **Step 8: Replace `extensions/claude/manifest.json` in full**

```json
{
  "manifest_version": "0.3",
  "name": "ocultar-pii",
  "display_name": "Ocultar PII Refinery",
  "version": "0.3.0",
  "description": "Zero-egress PII redaction for Claude workflows. Redacts names, emails, IBANs, SSNs, phone numbers, and credit cards before any AI processing. Runs entirely in your infrastructure — no data ever leaves your environment.",
  "long_description": "Ocultar is a local PII detection and redaction engine. This extension gives Claude six tools:\n\n- **refine_text** — redacts PII from any text before it reaches Claude, replacing sensitive values with deterministic tokens (e.g. `[EMAIL_9c8f7a1b]`).\n- **reveal_tokens** — de-tokenizes tokens back to plaintext for authorized callers (auditor-only, every call is logged in an immutable Ed25519-signed audit trail).\n- **register_entity** / **list_entities** / **seed_entities** — manage the persistent Entity Registry so name variants (e.g. 'Alice', 'A. Martin') resolve to the same token across sessions (auditor-only).\n- **sombra_query** — ask a redacted question through the Ocultar Sombra gateway, which redacts PII, routes to the chosen LLM, and rehydrates the response (requires Sombra running separately).\n\nAll processing happens on your machine via the locally running Ocultar Refinery (and Sombra, for sombra_query). If either service is unreachable, every tool fails closed — raw text is never forwarded. Zero-egress is an architectural property, not a policy.",
  "author": {
    "name": "Ocultar Security",
    "email": "edu@ocultar.dev",
    "url": "https://ocultar.dev"
  },
  "repository": {
    "type": "git",
    "url": "https://github.com/ocultar-dev/ocultar"
  },
  "homepage": "https://ocultar.dev",
  "documentation": "https://github.com/ocultar-dev/ocultar/blob/main/extensions/claude/README.md",
  "support": "https://github.com/ocultar-dev/ocultar/issues",
  "license": "AGPL-3.0-only",
  "keywords": ["pii", "privacy", "gdpr", "zero-egress", "security", "redaction", "compliance"],
  "privacy_policies": ["https://ocultar.dev/privacy"],
  "server": {
    "type": "python",
    "entry_point": "ocultar_claude_mcp.py",
    "mcp_config": {
      "command": "uvx",
      "args": ["ocultar-claude-mcp@0.3.0"],
      "env": {
        "OCULTAR_URL": "${user_config.ocultar_url}",
        "OCULTAR_API_KEY": "${user_config.ocultar_api_key}",
        "OCULTAR_AUDITOR_TOKEN": "${user_config.ocultar_auditor_token}",
        "OCULTAR_SOMBRA_URL": "${user_config.ocultar_sombra_url}",
        "OCULTAR_SOMBRA_TOKEN": "${user_config.ocultar_sombra_token}"
      }
    }
  },
  "user_config": {
    "ocultar_url": {
      "type": "string",
      "title": "Ocultar Refinery URL",
      "description": "URL of your locally running Ocultar Refinery",
      "required": true,
      "default": "http://localhost:4141"
    },
    "ocultar_api_key": {
      "type": "string",
      "title": "API Key",
      "description": "Ocultar API key (leave blank if not configured)",
      "required": false,
      "sensitive": true
    },
    "ocultar_auditor_token": {
      "type": "string",
      "title": "Auditor Token",
      "description": "Enables reveal_tokens and the Entity Registry tools (register_entity, list_entities, seed_entities). Must match OCU_AUDITOR_TOKEN on the server. Leave blank to disable these tools.",
      "required": false,
      "sensitive": true
    },
    "ocultar_sombra_url": {
      "type": "string",
      "title": "Ocultar Sombra Gateway URL",
      "description": "URL of your locally running Ocultar Sombra gateway (enables sombra_query)",
      "required": false,
      "default": "http://localhost:8086"
    },
    "ocultar_sombra_token": {
      "type": "string",
      "title": "Sombra Bearer Token",
      "description": "Enables sombra_query. Sombra rejects requests with no Bearer token — leave blank to disable this tool.",
      "required": false,
      "sensitive": true
    }
  },
  "tools": [
    {
      "name": "refine_text",
      "description": "Redact PII from text before sending to Claude"
    },
    {
      "name": "reveal_tokens",
      "description": "De-tokenize Ocultar tokens back to plaintext (auditor-only)"
    },
    {
      "name": "register_entity",
      "description": "Register a canonical PII entity and its variants (auditor-only)"
    },
    {
      "name": "list_entities",
      "description": "List registered PII entities (auditor-only)"
    },
    {
      "name": "seed_entities",
      "description": "Bulk-register PII entities (auditor-only)"
    },
    {
      "name": "sombra_query",
      "description": "Ask a redacted question via the Ocultar Sombra gateway"
    }
  ],
  "compatibility": {
    "platforms": ["darwin", "win32", "linux"],
    "runtimes": {
      "python": ">=3.10"
    }
  }
}
```

Note: `repository.url` and `documentation` change from `Edu963/ocultar` to `ocultar-dev/ocultar` in the same edit — this file was never caught by the earlier `pyproject.toml`-only repo-URL fix, same class of staleness.

- [ ] **Step 9: Validate the JSON parses**

Run: `python3 -c "import json; json.load(open('extensions/claude/manifest.json')); print('VALID')"`
Expected: `VALID`.

- [ ] **Step 10: Update `extensions/claude/README.md`**

Change the license footer:
```markdown
## License

Apache 2.0 — see [LICENSE](../../LICENSE)
```
to:
```markdown
## License

AGPLv3 — see [LICENSE](../../LICENSE). Commercial licensing available for organizations that cannot comply with AGPLv3's source-disclosure requirements — see [COMMERCIAL_LICENSE.md](../../COMMERCIAL_LICENSE.md).
```

Change the tools table:
```markdown
## Tools

| Tool | Description |
|------|-------------|
| `refine_text` | Redacts PII before sending text to Claude. Returns clean text + token map. |
| `reveal_tokens` | De-tokenizes tokens back to plaintext (auditor-only, requires `OCULTAR_AUDITOR_TOKEN`). |
```
to:
```markdown
## Tools

| Tool | Description |
|------|-------------|
| `refine_text` | Redacts PII before sending text to Claude. Returns clean text + token map. |
| `reveal_tokens` | De-tokenizes tokens back to plaintext (auditor-only, requires `OCULTAR_AUDITOR_TOKEN`). |
| `register_entity` | Registers a canonical PII entity and its variants so they resolve to the same token across sessions (auditor-only, requires `OCULTAR_AUDITOR_TOKEN`). |
| `list_entities` | Lists all registered PII entities (auditor-only, requires `OCULTAR_AUDITOR_TOKEN`). |
| `seed_entities` | Bulk-registers PII entities, e.g. from a CRM roster (auditor-only, requires `OCULTAR_AUDITOR_TOKEN`). |
| `sombra_query` | Asks a redacted question via the Ocultar Sombra gateway — redacts PII, routes to the chosen LLM, rehydrates the response (requires `OCULTAR_SOMBRA_TOKEN` and Sombra running separately). |
```

Change the environment variables table:
```markdown
## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `OCULTAR_URL` | Yes | URL of your local Ocultar Refinery (default: `http://localhost:4141`) |
| `OCULTAR_API_KEY` | No | Bearer token for Refinery auth |
| `OCULTAR_AUDITOR_TOKEN` | No | Enables `reveal_tokens` — must match `OCU_AUDITOR_TOKEN` on the server |
```
to:
```markdown
## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `OCULTAR_URL` | Yes | URL of your local Ocultar Refinery (default: `http://localhost:4141`) |
| `OCULTAR_API_KEY` | No | Bearer token for Refinery auth |
| `OCULTAR_AUDITOR_TOKEN` | No | Enables `reveal_tokens`, `register_entity`, `list_entities`, `seed_entities` — must match `OCU_AUDITOR_TOKEN` on the server |
| `OCULTAR_SOMBRA_URL` | No | URL of your local Ocultar Sombra gateway (default: `http://localhost:8086`) |
| `OCULTAR_SOMBRA_TOKEN` | No | Enables `sombra_query` — Sombra rejects requests with no Bearer token |
```

- [ ] **Step 11: Commit**

```bash
git add extensions/claude/pyproject.toml extensions/claude/ocultar_claude_mcp.py extensions/claude/manifest.json extensions/claude/README.md
git commit -m "feat(claude-mcp): license fix, Entity Registry and sombra_query tools, v0.3.0"
```

---

### Task 2: Goose extension — license, version, sombra_query only

**Files:**
- Modify: `extensions/goose/pyproject.toml`
- Modify: `extensions/goose/ocultar_mcp.py`
- Modify: `extensions/goose/extension.yaml`
- Modify: `extensions/goose/README.md`

**Interfaces:**
- Consumes: nothing from Task 1 (independent package).
- Produces: one new tool (`sombra_query`) and two new env vars (`OCULTAR_SOMBRA_URL`, `OCULTAR_SOMBRA_TOKEN`) — same names/semantics as Task 1's, referenced by Task 4.

- [ ] **Step 1: Update `extensions/goose/pyproject.toml`**

Change:
```toml
version = "0.2.0"
```
to:
```toml
version = "0.3.0"
```

Change:
```toml
license = { text = "Apache-2.0" }
```
to:
```toml
license = { text = "AGPL-3.0-only" }
```

Change:
```toml
    "License :: OSI Approved :: Apache Software License",
```
to:
```toml
    "License :: OSI Approved :: GNU Affero General Public License v3",
```

- [ ] **Step 2: Add the new module-level env vars to `extensions/goose/ocultar_mcp.py`**

Find:
```python
OCULTAR_URL = os.environ.get("OCULTAR_URL", "http://localhost:4141").rstrip("/")
OCULTAR_API_KEY = os.environ.get("OCULTAR_API_KEY", "")
```
Replace with:
```python
OCULTAR_URL = os.environ.get("OCULTAR_URL", "http://localhost:4141").rstrip("/")
OCULTAR_API_KEY = os.environ.get("OCULTAR_API_KEY", "")
OCULTAR_SOMBRA_URL = os.environ.get("OCULTAR_SOMBRA_URL", "http://localhost:8086").rstrip("/")
OCULTAR_SOMBRA_TOKEN = os.environ.get("OCULTAR_SOMBRA_TOKEN", "")
```

- [ ] **Step 3: Add the `sombra_query` tool definition to `list_tools()`**

Find:
```python
                    "text": {
                        "type": "string",
                        "description": "Raw text that may contain PII.",
                    }
                },
                "required": ["text"],
            },
        )
    ]
```
Replace with:
```python
                    "text": {
                        "type": "string",
                        "description": "Raw text that may contain PII.",
                    }
                },
                "required": ["text"],
            },
        ),
        types.Tool(
            name="sombra_query",
            description=(
                "Send a prompt through the Ocultar Sombra gateway, which redacts "
                "PII, routes the request to the chosen LLM, and rehydrates the "
                "response. Requires the Sombra gateway to be running separately "
                "(`go run ./apps/sombra`, default port 8086) and "
                "OCULTAR_SOMBRA_TOKEN to be set."
            ),
            inputSchema={
                "type": "object",
                "properties": {
                    "prompt": {
                        "type": "string",
                        "description": "The question or instruction to send.",
                    },
                    "model": {
                        "type": "string",
                        "description": (
                            "Which model to route to, e.g. 'gemini-flash-latest', "
                            "'gpt-4o', 'gpt-4o-mini', 'mistral-large-latest', "
                            "'claude-sonnet-4-6'. Passed through to Sombra as-is."
                        ),
                    },
                    "connector": {
                        "type": "string",
                        "description": "Data connector to use. Defaults to 'file'.",
                    },
                },
                "required": ["prompt", "model"],
            },
        ),
    ]
```

- [ ] **Step 4: Add a `sombra_query` branch to `call_tool` and the new implementation function**

Find:
```python
@app.call_tool()
async def call_tool(name: str, arguments: dict) -> list[types.TextContent]:
    if name != "refine_text":
        raise ValueError(f"Unknown tool: {name}")

    text = arguments.get("text", "")
```
Replace with:
```python
@app.call_tool()
async def call_tool(name: str, arguments: dict) -> list[types.TextContent]:
    if name == "sombra_query":
        return await _sombra_query(arguments)
    if name != "refine_text":
        raise ValueError(f"Unknown tool: {name}")

    text = arguments.get("text", "")
```

This leaves the existing inline `refine_text` body (the rest of `call_tool`, unchanged) exactly as-is — do not extract it into a `_refine_text` helper, that would be an unrelated refactor.

Then, find the end of `call_tool` (the existing inline `refine_text` implementation) and the start of `main`:
```python
    payload = json.dumps(
        {"cleanText": clean_text, "tokenMap": token_map},
        ensure_ascii=False,
    )
    return [types.TextContent(type="text", text=payload)]


async def main() -> None:
```
Replace with (inserts the new function between them):
```python
    payload = json.dumps(
        {"cleanText": clean_text, "tokenMap": token_map},
        ensure_ascii=False,
    )
    return [types.TextContent(type="text", text=payload)]


async def _sombra_query(arguments: dict) -> list[types.TextContent]:
    prompt = arguments.get("prompt", "")
    model = arguments.get("model", "")
    connector = arguments.get("connector", "file")
    if not prompt or not model:
        raise RuntimeError("prompt and model are required.")

    if not OCULTAR_SOMBRA_TOKEN:
        raise RuntimeError(
            "OCULTAR_SOMBRA_TOKEN is not set. "
            "The Sombra gateway rejects every request with no Bearer token. "
            "Set the environment variable to enable this tool."
        )

    try:
        async with httpx.AsyncClient(timeout=60.0) as client:
            response = await client.post(
                f"{OCULTAR_SOMBRA_URL}/query",
                data={"connector": connector, "model": model, "prompt": prompt},
                headers={"Authorization": f"Bearer {OCULTAR_SOMBRA_TOKEN}"},
            )
            response.raise_for_status()
    except httpx.ConnectError:
        raise RuntimeError(
            f"Cannot connect to Sombra at {OCULTAR_SOMBRA_URL}/query. "
            "Start the Sombra gateway first: `go run ./apps/sombra` (default port 8086). "
            "Raw data withheld to preserve zero-egress guarantee."
        )
    except httpx.TimeoutException:
        raise RuntimeError(
            f"Sombra /query timed out (60 s) at {OCULTAR_SOMBRA_URL}. "
            "Raw data withheld to preserve zero-egress guarantee."
        )
    except httpx.HTTPStatusError as exc:
        if exc.response.status_code == 401:
            raise RuntimeError(
                "Sombra rejected the request (401 unauthorized). "
                "Check OCULTAR_SOMBRA_TOKEN is set to a non-empty value."
            )
        raise RuntimeError(
            f"Sombra /query returned HTTP {exc.response.status_code}: "
            f"{exc.response.text[:300]}"
        )

    return [types.TextContent(type="text", text=response.text)]


async def main() -> None:
```

Note: this file has no `_connection_error`/`_sombra_headers` helper functions (unlike claude/mistral) — its existing style inlines the headers dict and error messages directly. `_sombra_query` above matches that inline style on purpose, per Global Constraints (follow each file's own convention).

- [ ] **Step 5: Verify the file compiles**

Run: `python3 -m py_compile extensions/goose/ocultar_mcp.py && echo OK`
Expected: `OK`.

- [ ] **Step 6: Update `extensions/goose/extension.yaml` in full**

```yaml
id: ocultar-pii
name: Ocultar PII Refinery
description: >
  Zero-egress PII protection for Goose AI workflows.
  Redacts names, IBANs, emails, phone numbers, and financial identifiers
  before any upstream processing. Runs entirely in your infrastructure —
  no data leaves your environment.
version: 0.3.0
author: ocultar-dev
license: AGPL-3.0-only
repository: https://github.com/ocultar-dev/ocultar
tags:
  - security
  - privacy
  - pii
  - finance
  - zero-egress
  - gdpr
transport: stdio
command: python
args:
  - extensions/goose/ocultar_mcp.py
env:
  OCULTAR_URL:
    description: URL of your local Ocultar instance
    default: http://localhost:4141
    required: true
  OCULTAR_API_KEY:
    description: Your Ocultar API key
    required: false
  OCULTAR_SOMBRA_URL:
    description: URL of your local Ocultar Sombra gateway (enables sombra_query)
    default: http://localhost:8086
    required: false
  OCULTAR_SOMBRA_TOKEN:
    description: Bearer token for the Sombra gateway (enables sombra_query)
    required: false
tools:
  - name: refine_text
    description: >
      Redacts PII from text locally before AI processing.
      Returns clean text and a token map for re-hydration.
  - name: sombra_query
    description: >
      Ask a redacted question via the Ocultar Sombra gateway — redacts PII,
      routes to the chosen LLM, and rehydrates the response.
```

- [ ] **Step 7: Validate the YAML parses**

Run: `python3 -c "import yaml; yaml.safe_load(open('extensions/goose/extension.yaml')); print('VALID')"`
Expected: `VALID`.

- [ ] **Step 8: Update `extensions/goose/README.md`**

Change the available-tools section and its note:
```markdown
### Available tools

| Tool | Description |
|---|---|
| `refine_text` | Redacts PII from text before AI processing |

> **Note:** `reveal_tokens` (de-tokenization) is intentionally omitted from this extension.
> Token reveal is an auditor-only operation and is not suitable for automated agent workflows.
> Use the Claude MCP extension if you need reveal access.
```
to:
```markdown
### Available tools

| Tool | Description |
|---|---|
| `refine_text` | Redacts PII from text before AI processing |
| `sombra_query` | Asks a redacted question via the Ocultar Sombra gateway — redacts PII, routes to the chosen LLM, rehydrates the response. Requires `OCULTAR_SOMBRA_TOKEN` and Sombra running separately (`go run ./apps/sombra`). |

> **Note:** `reveal_tokens` (de-tokenization) and the Entity Registry tools
> (`register_entity`, `list_entities`, `seed_entities`) are intentionally
> omitted from this extension. They are auditor-only operations and not
> suitable for automated agent workflows. Use the Claude or Mistral MCP
> extension if you need them.
```

Add a license footer at the end of the file (the file currently has none):
```markdown

## License

AGPLv3 — see [LICENSE](../../LICENSE). Commercial licensing available for organizations that cannot comply with AGPLv3's source-disclosure requirements — see [COMMERCIAL_LICENSE.md](../../COMMERCIAL_LICENSE.md).
```

- [ ] **Step 9: Commit**

```bash
git add extensions/goose/pyproject.toml extensions/goose/ocultar_mcp.py extensions/goose/extension.yaml extensions/goose/README.md
git commit -m "feat(goose-mcp): license fix and sombra_query tool, v0.3.0"
```

---

### Task 3: Mistral extension — license, version, Entity Registry + sombra_query tools

**Files:**
- Modify: `extensions/mistral/pyproject.toml`
- Modify: `extensions/mistral/ocultar_mistral_mcp.py`
- Modify: `extensions/mistral/manifest.json`
- Modify: `extensions/mistral/README.md`

**Interfaces:**
- Consumes: nothing from Tasks 1–2 (independent package). Uses the identical tool names/schemas as Task 1 for consistency across clients — but this is a fresh implementation in this file, not a shared import.
- Produces: same four new tools as Task 1 (`register_entity`, `list_entities`, `seed_entities`, `sombra_query`), referenced by Task 4.

- [ ] **Step 1: Update `extensions/mistral/pyproject.toml`**

Change:
```toml
version = "0.2.0"
```
to:
```toml
version = "0.3.0"
```

Change:
```toml
license = { text = "Apache-2.0" }
```
to:
```toml
license = { text = "AGPL-3.0-only" }
```

Change:
```toml
    "License :: OSI Approved :: Apache Software License",
```
to:
```toml
    "License :: OSI Approved :: GNU Affero General Public License v3",
```

- [ ] **Step 2: Add the new module-level env vars to `extensions/mistral/ocultar_mistral_mcp.py`**

Find:
```python
OCULTAR_URL = os.environ.get("OCULTAR_URL", "http://localhost:8080").rstrip("/")
OCULTAR_API_KEY = os.environ.get("OCULTAR_API_KEY", "")
OCULTAR_AUDITOR_TOKEN = os.environ.get("OCULTAR_AUDITOR_TOKEN", "")
```
Replace with:
```python
OCULTAR_URL = os.environ.get("OCULTAR_URL", "http://localhost:8080").rstrip("/")
OCULTAR_API_KEY = os.environ.get("OCULTAR_API_KEY", "")
OCULTAR_AUDITOR_TOKEN = os.environ.get("OCULTAR_AUDITOR_TOKEN", "")
OCULTAR_SOMBRA_URL = os.environ.get("OCULTAR_SOMBRA_URL", "http://localhost:8086").rstrip("/")
OCULTAR_SOMBRA_TOKEN = os.environ.get("OCULTAR_SOMBRA_TOKEN", "")
```

- [ ] **Step 3: Add the `_sombra_headers` and `_sombra_connection_error` helpers**

Find:
```python
def _connection_error(endpoint: str) -> RuntimeError:
    return RuntimeError(
        f"Cannot connect to Ocultar at {OCULTAR_URL}{endpoint}. "
        "Start the Refinery first: `docker compose up` or "
        "`go run ./services/refinery/cmd/main.go --serve 8080`. "
        "Raw data withheld to preserve zero-egress guarantee."
    )
```
Replace with:
```python
def _connection_error(endpoint: str) -> RuntimeError:
    return RuntimeError(
        f"Cannot connect to Ocultar at {OCULTAR_URL}{endpoint}. "
        "Start the Refinery first: `docker compose up` or "
        "`go run ./services/refinery/cmd/main.go --serve 8080`. "
        "Raw data withheld to preserve zero-egress guarantee."
    )


def _sombra_headers() -> dict[str, str]:
    if OCULTAR_SOMBRA_TOKEN:
        return {"Authorization": f"Bearer {OCULTAR_SOMBRA_TOKEN}"}
    return {}


def _sombra_connection_error() -> RuntimeError:
    return RuntimeError(
        f"Cannot connect to Sombra at {OCULTAR_SOMBRA_URL}/query. "
        "Start the Sombra gateway first: `go run ./apps/sombra` (default port 8086). "
        "Raw data withheld to preserve zero-egress guarantee."
    )
```

- [ ] **Step 4: Add the four new tool definitions to `list_tools()`**

Find:
```python
                    }
                },
                "required": ["tokens"],
            },
        ),
    ]
```
Replace with:
```python
                    }
                },
                "required": ["tokens"],
            },
        ),
        types.Tool(
            name="register_entity",
            title="Register a canonical PII entity",
            description=(
                "Pre-register a canonical name/identifier and its variants in the "
                "persistent Entity Registry so all variants map to the same token "
                "consistently across sessions (e.g. 'Alice', 'A. Martin', and "
                "'alice martin' all resolving to [PERSON_1]). Requires "
                "OCULTAR_AUDITOR_TOKEN to be set."
            ),
            inputSchema={
                "type": "object",
                "properties": {
                    "entity_type": {
                        "type": "string",
                        "description": "Entity category, e.g. 'PERSON' or 'ORG'.",
                    },
                    "canonical_name": {
                        "type": "string",
                        "description": "The canonical name to register, e.g. 'Alice Martin'.",
                    },
                    "variants": {
                        "type": "array",
                        "items": {"type": "string"},
                        "description": "Alternate forms that should resolve to the same token.",
                    },
                },
                "required": ["entity_type", "canonical_name"],
            },
        ),
        types.Tool(
            name="list_entities",
            title="List registered PII entities",
            description=(
                "List every entry in the persistent Entity Registry. Requires "
                "OCULTAR_AUDITOR_TOKEN to be set."
            ),
            readOnlyHint=True,
            inputSchema={"type": "object", "properties": {}},
        ),
        types.Tool(
            name="seed_entities",
            title="Bulk-register PII entities",
            description=(
                "Bulk-register multiple canonical entities in one call, e.g. from "
                "a CRM roster or patient list. Requires OCULTAR_AUDITOR_TOKEN to be set."
            ),
            inputSchema={
                "type": "object",
                "properties": {
                    "entities": {
                        "type": "array",
                        "items": {
                            "type": "object",
                            "properties": {
                                "entity_type": {"type": "string"},
                                "canonical_name": {"type": "string"},
                                "variants": {
                                    "type": "array",
                                    "items": {"type": "string"},
                                },
                            },
                            "required": ["entity_type", "canonical_name"],
                        },
                        "description": "List of entities to register.",
                    }
                },
                "required": ["entities"],
            },
        ),
        types.Tool(
            name="sombra_query",
            title="Ask a redacted question via the Sombra gateway",
            description=(
                "Send a prompt through the Ocultar Sombra gateway, which redacts "
                "PII, routes the request to the chosen LLM, and rehydrates the "
                "response. Requires the Sombra gateway to be running separately "
                "(`go run ./apps/sombra`, default port 8086) and "
                "OCULTAR_SOMBRA_TOKEN to be set."
            ),
            inputSchema={
                "type": "object",
                "properties": {
                    "prompt": {
                        "type": "string",
                        "description": "The question or instruction to send.",
                    },
                    "model": {
                        "type": "string",
                        "description": (
                            "Which model to route to, e.g. 'gemini-flash-latest', "
                            "'gpt-4o', 'gpt-4o-mini', 'mistral-large-latest', "
                            "'claude-sonnet-4-6'. Passed through to Sombra as-is."
                        ),
                    },
                    "connector": {
                        "type": "string",
                        "description": "Data connector to use. Defaults to 'file'.",
                    },
                },
                "required": ["prompt", "model"],
            },
        ),
    ]
```

Note: `readOnlyHint` is set only on `list_entities` (`True`, since it's genuinely read-only), matching how `refine_text`/`reveal_tokens` already only ever set it to `True` in this file — the three mutating tools (`register_entity`, `seed_entities`, `sombra_query`) omit the field entirely rather than setting it to `False`, consistent with Task 1's claude.py.

- [ ] **Step 5: Extend the `call_tool` dispatcher**

Find:
```python
@app.call_tool()
async def call_tool(name: str, arguments: dict) -> list[types.TextContent]:
    if name == "refine_text":
        return await _refine_text(arguments)
    if name == "reveal_tokens":
        return await _reveal_tokens(arguments)
    raise ValueError(f"Unknown tool: {name}")
```
Replace with:
```python
@app.call_tool()
async def call_tool(name: str, arguments: dict) -> list[types.TextContent]:
    if name == "refine_text":
        return await _refine_text(arguments)
    if name == "reveal_tokens":
        return await _reveal_tokens(arguments)
    if name == "register_entity":
        return await _register_entity(arguments)
    if name == "list_entities":
        return await _list_entities(arguments)
    if name == "seed_entities":
        return await _seed_entities(arguments)
    if name == "sombra_query":
        return await _sombra_query(arguments)
    raise ValueError(f"Unknown tool: {name}")
```

- [ ] **Step 6: Add the four new implementation functions**

Find the end of `_reveal_tokens` and the start of `main`:
```python
    data = response.json()
    return [types.TextContent(type="text", text=json.dumps(data, ensure_ascii=False))]


async def main() -> None:
```
Replace with:
```python
    data = response.json()
    return [types.TextContent(type="text", text=json.dumps(data, ensure_ascii=False))]


async def _register_entity(arguments: dict) -> list[types.TextContent]:
    entity_type = arguments.get("entity_type", "")
    canonical_name = arguments.get("canonical_name", "")
    variants = arguments.get("variants", [])
    if not entity_type or not canonical_name:
        raise RuntimeError("entity_type and canonical_name are required.")

    if not OCULTAR_AUDITOR_TOKEN:
        raise RuntimeError(
            "OCULTAR_AUDITOR_TOKEN is not set. "
            "Entity registration is an auditor-only operation. "
            "Set the environment variable to enable it."
        )

    try:
        async with httpx.AsyncClient(timeout=15.0) as client:
            response = await client.post(
                f"{OCULTAR_URL}/api/entities",
                json={
                    "entity_type": entity_type,
                    "canonical_name": canonical_name,
                    "variants": variants,
                },
                headers=_auditor_headers(),
            )
            response.raise_for_status()
    except httpx.ConnectError:
        raise _connection_error("/api/entities")
    except httpx.TimeoutException:
        raise RuntimeError(
            "Ocultar /api/entities timed out (15 s). "
            "Raw data withheld to preserve zero-egress guarantee."
        )
    except httpx.HTTPStatusError as exc:
        if exc.response.status_code in (401, 403):
            raise RuntimeError(
                "Ocultar rejected the auditor token (401/403). "
                "Check OCULTAR_AUDITOR_TOKEN matches OCU_AUDITOR_TOKEN on the server."
            )
        raise RuntimeError(
            f"Ocultar /api/entities returned HTTP {exc.response.status_code}: "
            f"{exc.response.text[:300]}"
        )

    data = response.json()
    return [types.TextContent(type="text", text=json.dumps(data, ensure_ascii=False))]


async def _list_entities(arguments: dict) -> list[types.TextContent]:
    if not OCULTAR_AUDITOR_TOKEN:
        raise RuntimeError(
            "OCULTAR_AUDITOR_TOKEN is not set. "
            "Listing entities is an auditor-only operation. "
            "Set the environment variable to enable it."
        )

    try:
        async with httpx.AsyncClient(timeout=15.0) as client:
            response = await client.get(
                f"{OCULTAR_URL}/api/entities",
                headers=_auditor_headers(),
            )
            response.raise_for_status()
    except httpx.ConnectError:
        raise _connection_error("/api/entities")
    except httpx.TimeoutException:
        raise RuntimeError(
            "Ocultar /api/entities timed out (15 s). "
            "Raw data withheld to preserve zero-egress guarantee."
        )
    except httpx.HTTPStatusError as exc:
        if exc.response.status_code in (401, 403):
            raise RuntimeError(
                "Ocultar rejected the auditor token (401/403). "
                "Check OCULTAR_AUDITOR_TOKEN matches OCU_AUDITOR_TOKEN on the server."
            )
        raise RuntimeError(
            f"Ocultar /api/entities returned HTTP {exc.response.status_code}: "
            f"{exc.response.text[:300]}"
        )

    data = response.json()
    return [types.TextContent(type="text", text=json.dumps(data, ensure_ascii=False))]


async def _seed_entities(arguments: dict) -> list[types.TextContent]:
    entities = arguments.get("entities", [])
    if not entities:
        return [types.TextContent(type="text", text=json.dumps({"seeded": 0, "tokens": []}))]

    if not OCULTAR_AUDITOR_TOKEN:
        raise RuntimeError(
            "OCULTAR_AUDITOR_TOKEN is not set. "
            "Seeding entities is an auditor-only operation. "
            "Set the environment variable to enable it."
        )

    try:
        async with httpx.AsyncClient(timeout=15.0) as client:
            response = await client.post(
                f"{OCULTAR_URL}/api/entities/seed",
                json=entities,
                headers=_auditor_headers(),
            )
            response.raise_for_status()
    except httpx.ConnectError:
        raise _connection_error("/api/entities/seed")
    except httpx.TimeoutException:
        raise RuntimeError(
            "Ocultar /api/entities/seed timed out (15 s). "
            "Raw data withheld to preserve zero-egress guarantee."
        )
    except httpx.HTTPStatusError as exc:
        if exc.response.status_code in (401, 403):
            raise RuntimeError(
                "Ocultar rejected the auditor token (401/403). "
                "Check OCULTAR_AUDITOR_TOKEN matches OCU_AUDITOR_TOKEN on the server."
            )
        raise RuntimeError(
            f"Ocultar /api/entities/seed returned HTTP {exc.response.status_code}: "
            f"{exc.response.text[:300]}"
        )

    data = response.json()
    return [types.TextContent(type="text", text=json.dumps(data, ensure_ascii=False))]


async def _sombra_query(arguments: dict) -> list[types.TextContent]:
    prompt = arguments.get("prompt", "")
    model = arguments.get("model", "")
    connector = arguments.get("connector", "file")
    if not prompt or not model:
        raise RuntimeError("prompt and model are required.")

    if not OCULTAR_SOMBRA_TOKEN:
        raise RuntimeError(
            "OCULTAR_SOMBRA_TOKEN is not set. "
            "The Sombra gateway rejects every request with no Bearer token. "
            "Set the environment variable to enable this tool."
        )

    try:
        async with httpx.AsyncClient(timeout=60.0) as client:
            response = await client.post(
                f"{OCULTAR_SOMBRA_URL}/query",
                data={"connector": connector, "model": model, "prompt": prompt},
                headers=_sombra_headers(),
            )
            response.raise_for_status()
    except httpx.ConnectError:
        raise _sombra_connection_error()
    except httpx.TimeoutException:
        raise RuntimeError(
            f"Sombra /query timed out (60 s) at {OCULTAR_SOMBRA_URL}. "
            "Raw data withheld to preserve zero-egress guarantee."
        )
    except httpx.HTTPStatusError as exc:
        if exc.response.status_code == 401:
            raise RuntimeError(
                "Sombra rejected the request (401 unauthorized). "
                "Check OCULTAR_SOMBRA_TOKEN is set to a non-empty value."
            )
        raise RuntimeError(
            f"Sombra /query returned HTTP {exc.response.status_code}: "
            f"{exc.response.text[:300]}"
        )

    return [types.TextContent(type="text", text=response.text)]


async def main() -> None:
```

Note: this file's existing convention does **not** double-wrap (`raise _connection_error(...)` directly) — Step 6 matches that exactly, unlike Task 1's claude file. Do not make these two files' error-wrapping style match each other.

- [ ] **Step 7: Verify the file compiles**

Run: `python3 -m py_compile extensions/mistral/ocultar_mistral_mcp.py && echo OK`
Expected: `OK`.

- [ ] **Step 8: Replace `extensions/mistral/manifest.json` in full**

```json
{
  "manifest_version": "0.3",
  "name": "ocultar-pii",
  "display_name": "Ocultar PII Refinery",
  "version": "0.3.0",
  "description": "Zero-egress PII redaction for Mistral Le Chat workflows. Redacts names, emails, IBANs, SIRET/SIREN, phone numbers, and credit cards before any AI processing. Runs entirely in your infrastructure — no data ever leaves your environment.",
  "long_description": "Ocultar est un moteur local de détection et de rédaction des DCP (Données à Caractère Personnel). Cette extension donne à Mistral Le Chat six outils :\n\n- **refine_text** — rédacte les DCP de tout texte avant qu'il n'atteigne Le Chat, en remplaçant les valeurs sensibles par des jetons déterministes (ex. `[EMAIL_9c8f7a1b]`). Optimisé pour le RGPD, la CNIL, les numéros SIRET/SIREN, IBAN, et les adresses françaises.\n- **reveal_tokens** — re-tokenise les jetons en clair pour les auditeurs autorisés (chaque appel est enregistré dans une piste d'audit immuable signée Ed25519).\n- **register_entity** / **list_entities** / **seed_entities** — gèrent le registre d'entités persistant afin que les variantes d'un nom (ex. 'Alice', 'A. Martin') se résolvent vers le même jeton d'une session à l'autre (auditeurs uniquement).\n- **sombra_query** — pose une question rédigée via la passerelle Ocultar Sombra, qui rédacte les DCP, achemine la requête vers le LLM choisi, puis réhydrate la réponse (nécessite Sombra en cours d'exécution séparément).\n\nTout le traitement se fait sur votre machine via le Refinery Ocultar local (et Sombra, pour sombra_query). Si l'un ou l'autre service est injoignable, chaque outil échoue en mode fermé — le texte brut n'est jamais transmis. Zéro-égression est une garantie architecturale, pas une politique.",
  "author": {
    "name": "Ocultar Security",
    "email": "edu@ocultar.dev",
    "url": "https://ocultar.dev"
  },
  "repository": {
    "type": "git",
    "url": "https://github.com/ocultar-dev/ocultar"
  },
  "homepage": "https://ocultar.dev",
  "documentation": "https://github.com/ocultar-dev/ocultar/blob/main/extensions/mistral/README.md",
  "support": "https://github.com/ocultar-dev/ocultar/issues",
  "license": "AGPL-3.0-only",
  "keywords": ["pii", "dcp", "privacy", "rgpd", "gdpr", "zero-egress", "security", "redaction", "compliance", "france", "mistral"],
  "privacy_policies": ["https://ocultar.dev/privacy"],
  "server": {
    "type": "python",
    "entry_point": "ocultar_mistral_mcp.py",
    "mcp_config": {
      "command": "uvx",
      "args": ["ocultar-mistral-mcp@0.3.0"],
      "env": {
        "OCULTAR_URL": "${user_config.ocultar_url}",
        "OCULTAR_API_KEY": "${user_config.ocultar_api_key}",
        "OCULTAR_AUDITOR_TOKEN": "${user_config.ocultar_auditor_token}",
        "OCULTAR_SOMBRA_URL": "${user_config.ocultar_sombra_url}",
        "OCULTAR_SOMBRA_TOKEN": "${user_config.ocultar_sombra_token}"
      }
    }
  },
  "user_config": {
    "ocultar_url": {
      "type": "string",
      "title": "Ocultar Refinery URL",
      "description": "URL of your locally running Ocultar Refinery",
      "required": true,
      "default": "http://localhost:8080"
    },
    "ocultar_api_key": {
      "type": "string",
      "title": "API Key",
      "description": "Ocultar API key (leave blank if not configured)",
      "required": false,
      "sensitive": true
    },
    "ocultar_auditor_token": {
      "type": "string",
      "title": "Auditor Token",
      "description": "Enables reveal_tokens and the Entity Registry tools (register_entity, list_entities, seed_entities). Must match OCU_AUDITOR_TOKEN on the server. Leave blank to disable these tools.",
      "required": false,
      "sensitive": true
    },
    "ocultar_sombra_url": {
      "type": "string",
      "title": "Ocultar Sombra Gateway URL",
      "description": "URL of your locally running Ocultar Sombra gateway (enables sombra_query)",
      "required": false,
      "default": "http://localhost:8086"
    },
    "ocultar_sombra_token": {
      "type": "string",
      "title": "Sombra Bearer Token",
      "description": "Enables sombra_query. Sombra rejects requests with no Bearer token — leave blank to disable this tool.",
      "required": false,
      "sensitive": true
    }
  },
  "tools": [
    {
      "name": "refine_text",
      "description": "Redact PII / DCP from text before sending to Mistral Le Chat"
    },
    {
      "name": "reveal_tokens",
      "description": "De-tokenize Ocultar tokens back to plaintext (auditor-only)"
    },
    {
      "name": "register_entity",
      "description": "Register a canonical PII entity and its variants (auditor-only)"
    },
    {
      "name": "list_entities",
      "description": "List registered PII entities (auditor-only)"
    },
    {
      "name": "seed_entities",
      "description": "Bulk-register PII entities (auditor-only)"
    },
    {
      "name": "sombra_query",
      "description": "Ask a redacted question via the Ocultar Sombra gateway"
    }
  ],
  "compatibility": {
    "platforms": ["darwin", "win32", "linux"],
    "runtimes": {
      "python": ">=3.10"
    }
  }
}
```

- [ ] **Step 9: Validate the JSON parses**

Run: `python3 -c "import json; json.load(open('extensions/mistral/manifest.json')); print('VALID')"`
Expected: `VALID`.

- [ ] **Step 10: Update `extensions/mistral/README.md`**

Change the license footer:
```markdown
## License

Apache 2.0 — see [LICENSE](../../LICENSE)
```
to:
```markdown
## License

AGPLv3 — see [LICENSE](../../LICENSE). Commercial licensing available for organizations that cannot comply with AGPLv3's source-disclosure requirements — see [COMMERCIAL_LICENSE.md](../../COMMERCIAL_LICENSE.md).
```

Change the tools table:
```markdown
## Tools

| Tool | Description |
|------|-------------|
| `refine_text` | Redacts PII / DCP before sending text to Le Chat. Returns clean text + token map. |
| `reveal_tokens` | De-tokenizes tokens back to plaintext (auditor-only, requires `OCULTAR_AUDITOR_TOKEN`). |
```
to:
```markdown
## Tools

| Tool | Description |
|------|-------------|
| `refine_text` | Redacts PII / DCP before sending text to Le Chat. Returns clean text + token map. |
| `reveal_tokens` | De-tokenizes tokens back to plaintext (auditor-only, requires `OCULTAR_AUDITOR_TOKEN`). |
| `register_entity` | Registers a canonical PII entity and its variants so they resolve to the same token across sessions (auditor-only, requires `OCULTAR_AUDITOR_TOKEN`). |
| `list_entities` | Lists all registered PII entities (auditor-only, requires `OCULTAR_AUDITOR_TOKEN`). |
| `seed_entities` | Bulk-registers PII entities, e.g. from a CRM roster (auditor-only, requires `OCULTAR_AUDITOR_TOKEN`). |
| `sombra_query` | Asks a redacted question via the Ocultar Sombra gateway — redacts PII, routes to the chosen LLM, rehydrates the response (requires `OCULTAR_SOMBRA_TOKEN` and Sombra running separately). |
```

Change the environment variables table:
```markdown
## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `OCULTAR_URL` | Yes | URL of your local Ocultar Refinery (default: `http://localhost:4141`) |
| `OCULTAR_API_KEY` | No | Bearer token for Refinery auth |
| `OCULTAR_AUDITOR_TOKEN` | No | Enables `reveal_tokens` — must match `OCU_AUDITOR_TOKEN` on the server |
```
to:
```markdown
## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `OCULTAR_URL` | Yes | URL of your local Ocultar Refinery (default: `http://localhost:4141`) |
| `OCULTAR_API_KEY` | No | Bearer token for Refinery auth |
| `OCULTAR_AUDITOR_TOKEN` | No | Enables `reveal_tokens`, `register_entity`, `list_entities`, `seed_entities` — must match `OCU_AUDITOR_TOKEN` on the server |
| `OCULTAR_SOMBRA_URL` | No | URL of your local Ocultar Sombra gateway (default: `http://localhost:8086`) |
| `OCULTAR_SOMBRA_TOKEN` | No | Enables `sombra_query` — Sombra rejects requests with no Bearer token |
```

- [ ] **Step 11: Commit**

```bash
git add extensions/mistral/pyproject.toml extensions/mistral/ocultar_mistral_mcp.py extensions/mistral/manifest.json extensions/mistral/README.md
git commit -m "feat(mistral-mcp): license fix, Entity Registry and sombra_query tools, v0.3.0"
```

---

### Task 4: Shared docs — MCP_EXTENSIONS.md guide and CHANGELOG.md

**Files:**
- Modify: `apps/web/public/content/guides/MCP_EXTENSIONS.md`
- Modify: `CHANGELOG.md`

**Interfaces:**
- Consumes: the exact tool names, env var names, and per-client availability decided in Tasks 1–3 (`register_entity`/`list_entities`/`seed_entities` on Claude+Mistral only; `sombra_query` on all three; `OCULTAR_SOMBRA_URL`/`OCULTAR_SOMBRA_TOKEN` env vars).
- Produces: nothing consumed by a later task — this is the last task in the plan.

- [ ] **Step 1: Update the intro paragraph in `apps/web/public/content/guides/MCP_EXTENSIONS.md`**

Find:
```markdown
# OCULTAR | MCP Extensions

OCULTAR ships three MCP (Model Context Protocol) extensions that plug directly into AI clients. Each extension exposes two tools — `refine_text` and `reveal_tokens` — and enforces the zero-egress guarantee at the protocol level: if the local Refinery is unreachable, both tools fail closed and refuse to forward any data.
```
Replace with:
```markdown
# OCULTAR | MCP Extensions

OCULTAR ships three MCP (Model Context Protocol) extensions that plug directly into AI clients. Every extension exposes `refine_text`; the Claude and Mistral extensions additionally expose `reveal_tokens` and the Entity Registry tools (`register_entity`, `list_entities`, `seed_entities`); all three expose `sombra_query` for redacted queries through the Sombra gateway. Every tool enforces the zero-egress guarantee at the protocol level: if the service it depends on (the local Refinery, or Sombra for `sombra_query`) is unreachable, the tool fails closed and refuses to forward any data.
```

- [ ] **Step 2: Rewrite the `## Tools` section**

Find:
```markdown
## Tools

Both tools are available in all three extensions.

### `refine_text`
```
Replace with:
```markdown
## Tools

### `refine_text`

Available in all three extensions.
```

Find the end of the existing `reveal_tokens` subsection:
```markdown
### `reveal_tokens`

De-tokenizes specific tokens back to plaintext. Requires `OCULTAR_AUDITOR_TOKEN` — auditor-only. Every call is recorded in the immutable Ed25519-signed audit log with actor identity and timestamp.

**Input**
```json
{ "tokens": ["[EMAIL_9c8f7a1b2d3e4f50]", "[IBAN_7f3e9a2b1c4d5e60]"] }
```

---

## Claude Desktop
```
Replace with:
```markdown
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

`model` is passed through to Sombra as-is — Sombra's own policy check rejects an unsupported value with a clear error. `connector` defaults to `"file"` if omitted. This version does not support file uploads through the MCP tool; the prompt itself carries the full query context.

---

## Claude Desktop
```

- [ ] **Step 3: Update the per-client env var blocks (Claude Desktop, Claude Code, Goose, Mistral Le Chat sections)**

For each of the four `env` JSON/YAML blocks in the file (Claude Desktop, Claude Code CLI, Goose, Mistral Le Chat — both the plain and `uvx` variants for Mistral), add the two new variables alongside the existing ones. E.g. the Claude Desktop block:

Find (appears twice, identically, for Claude Desktop and Claude Code — apply to both):
```json
      "env": {
        "OCULTAR_URL": "http://localhost:8080",
        "OCULTAR_API_KEY": "your-api-key"
      }
```
Replace both occurrences with:
```json
      "env": {
        "OCULTAR_URL": "http://localhost:8080",
        "OCULTAR_API_KEY": "your-api-key",
        "OCULTAR_AUDITOR_TOKEN": "your-auditor-token",
        "OCULTAR_SOMBRA_URL": "http://localhost:8086",
        "OCULTAR_SOMBRA_TOKEN": "your-sombra-token"
      }
```

Find the Goose section:
```markdown
4. Command: `ocultar-goose-mcp`
5. Environment: `OCULTAR_URL=http://localhost:8080`
```
Replace with:
```markdown
4. Command: `ocultar-goose-mcp`
5. Environment: `OCULTAR_URL=http://localhost:8080`, and optionally `OCULTAR_SOMBRA_URL=http://localhost:8086` + `OCULTAR_SOMBRA_TOKEN=your-sombra-token` to enable `sombra_query`
```

Find both Mistral Le Chat JSON blocks (plain and `uvx`):
```json
      "env": {
        "OCULTAR_URL": "http://localhost:8080",
        "OCULTAR_API_KEY": "your-api-key"
      }
```
Replace both occurrences with the same five-key block used for Claude above.

- [ ] **Step 4: Update the `## Environment Variables` table**

Find:
```markdown
| Variable | Required | Default | Description |
|---|---|---|---|
| `OCULTAR_URL` | Yes | `http://localhost:8080` | URL of your local OCULTAR Refinery |
| `OCULTAR_API_KEY` | No | — | Bearer token for Refinery authentication |
| `OCULTAR_AUDITOR_TOKEN` | No | — | Enables `reveal_tokens` — must match `OCU_AUDITOR_TOKEN` on the server |
```
Replace with:
```markdown
| Variable | Required | Default | Description |
|---|---|---|---|
| `OCULTAR_URL` | Yes | `http://localhost:8080` | URL of your local OCULTAR Refinery |
| `OCULTAR_API_KEY` | No | — | Bearer token for Refinery authentication |
| `OCULTAR_AUDITOR_TOKEN` | No | — | Enables `reveal_tokens`, `register_entity`, `list_entities`, `seed_entities` (Claude/Mistral only) — must match `OCU_AUDITOR_TOKEN` on the server |
| `OCULTAR_SOMBRA_URL` | No | `http://localhost:8086` | URL of your local OCULTAR Sombra gateway |
| `OCULTAR_SOMBRA_TOKEN` | No | — | Enables `sombra_query` (all three extensions) — Sombra rejects requests with no Bearer token |
```

- [ ] **Step 5: Update the `## Security Model` section**

Find:
```markdown
## Security Model

- `refine_text` is safe to expose to any AI session — it only sends text to the local Refinery, which runs on `localhost`. No telemetry, no remote calls.
- `reveal_tokens` requires `OCULTAR_AUDITOR_TOKEN`. Every call is logged with actor identity, timestamp, and Ed25519 signature in the tamper-proof audit trail.
- The Refinery vault uses AES-256-GCM with HKDF-SHA256 key derivation — tokens are useless without the master key.
- **Fail-closed guarantee:** if the Refinery is unreachable for any reason, both tools return an MCP error and refuse to forward raw data or vault contents to the caller.
```
Replace with:
```markdown
## Security Model

- `refine_text` is safe to expose to any AI session — it only sends text to the local Refinery, which runs on `localhost`. No telemetry, no remote calls.
- `reveal_tokens` and the Entity Registry tools require `OCULTAR_AUDITOR_TOKEN`. Every call is logged with actor identity, timestamp, and Ed25519 signature in the tamper-proof audit trail.
- `sombra_query` requires `OCULTAR_SOMBRA_TOKEN` — Sombra rejects any request with no Bearer token.
- The Refinery vault uses AES-256-GCM with HKDF-SHA256 key derivation — tokens are useless without the master key.
- **Fail-closed guarantee:** if the service a tool depends on is unreachable for any reason, that tool returns an MCP error and refuses to forward raw data or vault contents to the caller.
```

- [ ] **Step 6: Update the license footer**

Find:
```markdown
## License

Apache 2.0
```
Replace with:
```markdown
## License

AGPLv3 — see [LICENSE](https://github.com/ocultar-dev/ocultar/blob/main/LICENSE). Commercial licensing available — see [COMMERCIAL_LICENSE.md](https://github.com/ocultar-dev/ocultar/blob/main/COMMERCIAL_LICENSE.md).
```

- [ ] **Step 7: Add a CHANGELOG.md entry**

Find:
```markdown
# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.15.0] - 2026-06-26
```
Replace with:
```markdown
# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **MCP Entity Registry tools**: `register_entity`, `list_entities`, `seed_entities` added to the Claude and Mistral MCP extensions (auditor-only, matching `reveal_tokens`'s existing gate).
- **MCP `sombra_query` tool**: added to all three MCP extensions (Claude, Goose, Mistral) — sends a prompt through the Sombra gateway for redact-route-rehydrate in one call. Requires `OCULTAR_SOMBRA_TOKEN`.

### Fixed
- **MCP extension license metadata**: all three MCP packages (`ocultar-claude-mcp`, `ocultar-goose-mcp`, `ocultar-mistral-mcp`) were still declaring `Apache-2.0` after the repo-wide relicense to AGPLv3 on 2026-06-28 — corrected to `AGPL-3.0-only` across `pyproject.toml`, `manifest.json`/`extension.yaml`, and README files. `extensions/{claude,mistral}/manifest.json` also still pointed at the old `Edu963/ocultar` personal fork — corrected to `ocultar-dev/ocultar`.

## [1.15.0] - 2026-06-26
```

- [ ] **Step 8: Commit**

```bash
git add apps/web/public/content/guides/MCP_EXTENSIONS.md CHANGELOG.md
git commit -m "docs: document MCP license fix and new Entity Registry / sombra_query tools"
```
