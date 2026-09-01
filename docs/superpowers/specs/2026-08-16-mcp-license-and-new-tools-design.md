# MCP Extensions: License Correction + Entity Registry & Sombra Query Tools

**Date:** 2026-08-16
**Status:** Approved

## Problem

Two unrelated gaps surfaced while auditing the three MCP extension packages
(`ocultar-claude-mcp`, `ocultar-goose-mcp`, `ocultar-mistral-mcp`) against
the current state of `ocultar-dev/ocultar`:

1. **Stale license metadata.** The whole repo relicensed from Apache 2.0 to
   AGPLv3 (with a commercial-license option, see `COMMERCIAL_LICENSE.md`) on
   2026-06-28 (`c97f03a`). All three extensions still declare `Apache-2.0` in
   `pyproject.toml`, `manifest.json`/`extension.yaml`, and their README
   footers — two days *before* the relicense, they were last bumped to
   `0.2.0` still under the old license. Confirmed with the project owner:
   AGPLv3 is intentional and correct for the whole repo, including these
   packages — this is a metadata bug, not a licensing decision to make.

2. **Functional gap since Sombra went fully open source.** The MCP tool
   surface (`refine_text`, `reveal_tokens`) only ever talks to the Refinery.
   Auditing `apps/sombra` (now public, was previously closed-source) and the
   Refinery's own HTTP API turned up two capabilities with real callable use
   cases and zero MCP coverage:
   - The **Entity Registry** (`GET/POST /api/entities`,
     `POST /api/entities/seed`) — already on the Refinery, already
     documented in `docs/api-reference.md`, unrelated to Sombra's
     open-sourcing but never exposed as an MCP tool either.
   - Sombra's **`/query`** endpoint — a connector-based agentic query
     (fetch data from a source, redact, route to an LLM, rehydrate the
     answer) — the one capability genuinely unique to Sombra with no
     Refinery equivalent.

   Sombra's other endpoints (`/v1/chat/completions`, `/v1/slack/events`) are
   transport-layer infrastructure a client points *at* or a webhook target,
   not something an MCP tool calls — out of scope, nothing to add there.

## Design

### Part A — License correction (all 3 extensions)

- `pyproject.toml`: `license = { text = "Apache-2.0" }` →
  `license = { text = "AGPL-3.0-or-later" }`; classifier
  `License :: OSI Approved :: Apache Software License` →
  `License :: OSI Approved :: GNU Affero General Public License v3 or later (AGPLv3+)`.
- `extensions/claude/manifest.json` and `extensions/mistral/manifest.json`:
  `"license": "Apache-2.0"` → `"license": "AGPL-3.0-or-later"`.
- `extensions/goose/extension.yaml`: `license: Apache-2.0` →
  `license: AGPL-3.0-or-later`.
- `extensions/claude/README.md` and `extensions/mistral/README.md` license
  footer: `Apache 2.0 — see [LICENSE](../../LICENSE)` →
  `AGPLv3 — see [LICENSE](../../LICENSE). Commercial licensing available, see [COMMERCIAL_LICENSE.md](../../COMMERCIAL_LICENSE.md).`
  `extensions/goose/README.md` has no license footer today — add one in the
  same form for consistency with the other two.
- No change to `LICENSE` or `COMMERCIAL_LICENSE.md` themselves — those are
  already correct at the repo root; this is purely propagating the existing,
  already-decided repo license into the three package manifests.

### Part B — Entity Registry tools (Claude + Mistral extensions only)

Not added to the Goose extension: it already deliberately excludes
`reveal_tokens` as "auditor-only, not suitable for automated agent
workflows" (its own README's words). These three tools share the exact same
`OCULTAR_AUDITOR_TOKEN` auth gate as `reveal_tokens`, so the same reasoning
excludes them from Goose too.

All three call the Refinery on the existing `OCULTAR_URL` (no new service,
no new env var), reusing the existing `_auditor_headers()` helper, and hard
fail with a clear error if `OCULTAR_AUDITOR_TOKEN` is unset — same pattern
`_reveal_tokens` already uses.

- **`register_entity`** — `POST {OCULTAR_URL}/api/entities`
  - Input: `entity_type` (string, required), `canonical_name` (string,
    required), `variants` (array of strings, optional).
  - Request body: `{"entity_type": ..., "canonical_name": ..., "variants": [...]}`.
  - Output: the response JSON as-is, e.g. `{"canonical_token": "[PERSON_1]"}`.

- **`list_entities`** — `GET {OCULTAR_URL}/api/entities`
  - No input.
  - Output: the response JSON array as-is.

- **`seed_entities`** — `POST {OCULTAR_URL}/api/entities/seed`
  - Input: `entities` (array of `{entity_type, canonical_name, variants?}`
    objects, required).
  - Request body: sent as a bare JSON array (matches what the Refinery
    handler accepts per `docs/api-reference.md`).
  - Output: the response JSON as-is, e.g.
    `{"seeded": 12, "tokens": ["[PERSON_1]", "[PERSON_2]"]}`.

Error handling for all three mirrors `_reveal_tokens`: connect error →
remediation message pointing at starting the Refinery; timeout → clear
zero-egress-preserving message; HTTP error → surface status + body,
special-casing 401/403 with the "check `OCULTAR_AUDITOR_TOKEN` matches
`OCU_AUDITOR_TOKEN`" message `_reveal_tokens` already uses.

### Part C — `sombra_query` tool (all 3 extensions)

New env vars, both extension-wide (not just this tool):
- `OCULTAR_SOMBRA_URL` — default `http://localhost:8086`.
- `OCULTAR_SOMBRA_TOKEN` — no default; required. Sombra's `extractActor`
  rejects any request with an empty/missing `Authorization: Bearer` value
  outright (`apps/sombra/pkg/handler/handler.go:453-463`), so there is no
  "unauthenticated but works" mode to fall back to, unlike `OCULTAR_API_KEY`
  on the Refinery side which is optional.

- **`sombra_query`** — `POST {OCULTAR_SOMBRA_URL}/query` as
  `multipart/form-data` (matches `r.ParseMultipartForm` on the Go side —
  sending JSON would not be parsed).
  - Input: `connector` (string, optional, default `"file"`), `model`
    (string, required, **no client-side validation** — passed through
    verbatim so the Python list can't drift from Sombra's Go-side router
    registration; an invalid value surfaces as Sombra's own 403 "connector
    policy forbids sending data to model" error), `prompt` (string,
    required).
  - No file upload in this version — Sombra's handler already falls back to
    "treat the prompt as the whole data context" when no file/`source_id`
    is provided (`apps/sombra/pkg/handler/handler.go:169-177`), so
    prompt-only mode is a real, working mode server-side, not a workaround.
  - Auth: `Authorization: Bearer {OCULTAR_SOMBRA_TOKEN}` via a new
    `_sombra_headers()` helper (parallel to `_auditor_headers()`), and the
    tool hard-fails with a clear message if `OCULTAR_SOMBRA_TOKEN` is unset
    — same "explain what to set and why" style as the audit-token gate.
  - Output: the response body as-is (Sombra returns the redacted-and-
    rehydrated LLM answer as plain text or JSON depending on content type —
    passed through as `TextContent` either way, matching how `refine_text`
    passes through the Refinery's response).

New connection-error helper `_sombra_connection_error()`, parallel to the
existing `_connection_error()`, pointing at `go run ./apps/sombra` (default
port 8086) as the remediation instead of the Refinery's `--serve 8080`.

### Part D — Versioning

All three extensions: `0.2.0` → `0.3.0` (new tools = minor bump per SemVer,
not a patch). The license correction rides in the same release — there is
no reason to ship it as a separate patch version first. This becomes the
first `mcp-v0.3.0` tag once the PyPI Trusted Publisher setup (tracked in
`docs/superpowers/specs/2026-08-16-mcp-pypi-publish-design.md`) is done.

`CHANGELOG.md` gets a new entry documenting both the license correction and
the two new tools.

## Out of scope

- File upload support for `sombra_query` — a real feature, deliberately
  deferred; prompt-only mode is fully functional on its own.
- Any change to `LICENSE` or `COMMERCIAL_LICENSE.md` content — already
  correct, this only propagates the existing decision into package metadata.
- Any change to Sombra's or the Refinery's Go-side handlers — both already
  expose everything these tools need; this is client-side wiring only.
- Adding `/v1/chat/completions` or `/v1/slack/events` as MCP tools —
  transport-layer/webhook endpoints, not callable capabilities.
- Entity Registry tools for the Goose extension — excluded per the same
  reasoning that already excludes `reveal_tokens` there.

## Testing

- No Go code changes — nothing for `govulncheck`/`go test` to cover.
- Python side has no existing test suite in any of the three extensions
  (confirmed: no `test_*.py` or `pytest` config in `extensions/`). Consistent
  with the existing codebase, verification is manual: exercise each new tool
  against a running Refinery/Sombra instance and confirm the request/response
  shapes match this spec, plus a syntax/import sanity check
  (`python3 -m py_compile`) on each modified file.
