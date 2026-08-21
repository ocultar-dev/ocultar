#!/usr/bin/env python3
"""Ocultar PII Refinery — Mistral Le Chat MCP Extension (stdio transport).

Two tools:
  refine_text   — redacts PII before sending text to Mistral Le Chat
  reveal_tokens — de-tokenizes specific tokens back to plaintext (auditor-only)

Zero-egress guarantee: if Ocultar is unreachable both tools return an MCP error
and refuse to forward raw text or vault data to the caller.
"""

import asyncio
import json
import os
import re

import httpx
import mcp.server.stdio
import mcp.types as types
from mcp.server import Server

OCULTAR_URL = os.environ.get("OCULTAR_URL", "http://localhost:4141").rstrip("/")
OCULTAR_API_KEY = os.environ.get("OCULTAR_API_KEY", "")
OCULTAR_AUDITOR_TOKEN = os.environ.get("OCULTAR_AUDITOR_TOKEN", "")
OCULTAR_SOMBRA_URL = os.environ.get("OCULTAR_SOMBRA_URL", "http://localhost:8086").rstrip("/")
OCULTAR_SOMBRA_TOKEN = os.environ.get("OCULTAR_SOMBRA_TOKEN", "")

# Matches tokens like [EMAIL_9c8f7a1b2d3e4f50] produced by the Ocultar tokenizer
_TOKEN_RE = re.compile(r"\[([A-Z_]+)_([a-f0-9]{8,16}|\d+)\]")

app = Server("ocultar-pii")


def _auth_headers() -> dict[str, str]:
    if OCULTAR_API_KEY:
        return {"Authorization": f"Bearer {OCULTAR_API_KEY}"}
    return {}


def _auditor_headers() -> dict[str, str]:
    if OCULTAR_AUDITOR_TOKEN:
        return {"Authorization": f"Bearer {OCULTAR_AUDITOR_TOKEN}"}
    return {}


def _connection_error(endpoint: str) -> RuntimeError:
    return RuntimeError(
        f"Cannot connect to Ocultar at {OCULTAR_URL}{endpoint}. "
        "Start the Refinery first: `docker compose up` or "
        "`go run ./services/refinery/cmd/main.go --serve 4141`. "
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


@app.list_tools()
async def list_tools() -> list[types.Tool]:
    return [
        types.Tool(
            name="refine_text",
            title="Redact PII from text",
            description=(
                "Redact PII from text using the local Ocultar Refinery before "
                "sending it to Mistral Le Chat or any other AI model. Returns the "
                "cleaned text with all PII replaced by deterministic tokens "
                "(e.g. [EMAIL_9c8f7a1b]) and a map of each token to its PII type. "
                "All processing is local — no data leaves your infrastructure. "
                "Use this tool before processing any text that may contain names, "
                "emails, phone numbers, IBANs, SIRET/SIREN numbers, credit cards, "
                "or addresses. Optimised for French and EU regulatory requirements "
                "(RGPD, CNIL, DSP2)."
            ),
            readOnlyHint=True,
            inputSchema={
                "type": "object",
                "properties": {
                    "text": {
                        "type": "string",
                        "description": "Raw text that may contain PII.",
                    }
                },
                "required": ["text"],
            },
        ),
        types.Tool(
            name="reveal_tokens",
            title="Reveal original PII from tokens",
            description=(
                "De-tokenize Ocultar PII tokens back to their original plaintext "
                "values. Requires OCULTAR_AUDITOR_TOKEN to be set — this is an "
                "auditor-only operation that is logged in the immutable audit trail. "
                "Use only when the authorized caller explicitly needs to retrieve "
                "the original PII values after AI processing is complete."
            ),
            readOnlyHint=True,
            inputSchema={
                "type": "object",
                "properties": {
                    "tokens": {
                        "type": "array",
                        "items": {"type": "string"},
                        "description": (
                            "List of Ocultar tokens to de-tokenize, "
                            "e.g. ['[EMAIL_9c8f7a1b]', '[IBAN_3a1b2c4d]']."
                        ),
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


async def _refine_text(arguments: dict) -> list[types.TextContent]:
    text = arguments.get("text", "")
    if not text.strip():
        return [types.TextContent(type="text", text=json.dumps({"cleanText": "", "tokenMap": {}}))]

    try:
        async with httpx.AsyncClient(timeout=15.0) as client:
            response = await client.post(
                f"{OCULTAR_URL}/api/refine",
                content=text.encode("utf-8"),
                headers=_auth_headers(),
            )
            response.raise_for_status()
    except httpx.ConnectError:
        raise _connection_error("/api/refine")
    except httpx.TimeoutException:
        raise RuntimeError(
            "Ocultar /api/refine timed out (15 s). "
            "Raw text withheld to preserve zero-egress guarantee."
        )
    except httpx.HTTPStatusError as exc:
        raise RuntimeError(
            f"Ocultar /api/refine returned HTTP {exc.response.status_code}: "
            f"{exc.response.text[:300]}"
        )

    data = response.json()
    clean_text: str = data.get("refined", "")
    token_map: dict[str, str] = {
        m.group(0): m.group(1) for m in _TOKEN_RE.finditer(clean_text)
    }

    payload = json.dumps(
        {"cleanText": clean_text, "tokenMap": token_map},
        ensure_ascii=False,
    )
    return [types.TextContent(type="text", text=payload)]


async def _reveal_tokens(arguments: dict) -> list[types.TextContent]:
    tokens: list[str] = arguments.get("tokens", [])
    if not tokens:
        return [types.TextContent(type="text", text=json.dumps({"results": {}}))]

    if not OCULTAR_AUDITOR_TOKEN:
        raise RuntimeError(
            "OCULTAR_AUDITOR_TOKEN is not set. "
            "Token reveal is an auditor-only operation. "
            "Set the environment variable to enable it."
        )

    try:
        async with httpx.AsyncClient(timeout=15.0) as client:
            response = await client.post(
                f"{OCULTAR_URL}/api/reveal",
                json={"tokens": tokens},
                headers=_auditor_headers(),
            )
            response.raise_for_status()
    except httpx.ConnectError:
        raise _connection_error("/api/reveal")
    except httpx.TimeoutException:
        raise RuntimeError(
            "Ocultar /api/reveal timed out (15 s). "
            "Token data withheld to preserve zero-egress guarantee."
        )
    except httpx.HTTPStatusError as exc:
        if exc.response.status_code in (401, 403):
            raise RuntimeError(
                "Ocultar rejected the auditor token (401/403). "
                "Check OCULTAR_AUDITOR_TOKEN matches OCU_AUDITOR_TOKEN on the server."
            )
        raise RuntimeError(
            f"Ocultar /api/reveal returned HTTP {exc.response.status_code}: "
            f"{exc.response.text[:300]}"
        )

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
    async with mcp.server.stdio.stdio_server() as (read_stream, write_stream):
        await app.run(
            read_stream,
            write_stream,
            app.create_initialization_options(),
        )


def main_sync() -> None:
    """Entrypoint for the ocultar-mistral-mcp console script."""
    asyncio.run(main())


if __name__ == "__main__":
    main_sync()
