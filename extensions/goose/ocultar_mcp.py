#!/usr/bin/env python3
"""Ocultar PII Refinery — Goose MCP Extension (stdio transport).

Fail-closed design: if Ocultar is unreachable this server returns an MCP
error and refuses to forward the raw text to the caller.
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
OCULTAR_SOMBRA_URL = os.environ.get("OCULTAR_SOMBRA_URL", "http://localhost:8086").rstrip("/")
OCULTAR_SOMBRA_TOKEN = os.environ.get("OCULTAR_SOMBRA_TOKEN", "")

# Matches tokens like [EMAIL_9c8f7a1b2d3e4f50] produced by the Ocultar tokenizer
_TOKEN_RE = re.compile(r"\[([A-Z_]+)_([a-f0-9]{8,16}|\d+)\]")

app = Server("ocultar-pii")


@app.list_tools()
async def list_tools() -> list[types.Tool]:
    return [
        types.Tool(
            name="refine_text",
            description=(
                "Redact PII from text using the local Ocultar Refinery. "
                "Returns the cleaned text with all PII replaced by deterministic "
                "tokens (e.g. [EMAIL_9c8f7a1b]) and a map of each token to its "
                "PII type. All processing happens locally — no data leaves your "
                "infrastructure."
            ),
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


@app.call_tool()
async def call_tool(name: str, arguments: dict) -> list[types.TextContent]:
    if name == "sombra_query":
        return await _sombra_query(arguments)
    if name != "refine_text":
        raise ValueError(f"Unknown tool: {name}")

    text = arguments.get("text", "")
    if not text.strip():
        payload = json.dumps({"cleanText": "", "tokenMap": {}}, ensure_ascii=False)
        return [types.TextContent(type="text", text=payload)]

    headers: dict[str, str] = {}
    if OCULTAR_API_KEY:
        headers["Authorization"] = f"Bearer {OCULTAR_API_KEY}"

    try:
        async with httpx.AsyncClient(timeout=15.0) as client:
            response = await client.post(
                f"{OCULTAR_URL}/api/refine",
                content=text.encode("utf-8"),
                headers=headers,
            )
            response.raise_for_status()
    except httpx.ConnectError:
        raise RuntimeError(
            f"Cannot connect to Ocultar at {OCULTAR_URL}. "
            "Start the Refinery first: "
            "`docker run --rm -p 4141:4141 -e OCU_MASTER_KEY=<key> -e OCU_SALT=<salt> "
            "-e OCU_AUDITOR_TOKEN=<token> ghcr.io/ocultar-dev/ocultar:latest -serve 4141`. "
            "Raw text withheld — fail-closed."
        )
    except httpx.TimeoutException:
        raise RuntimeError(
            f"Ocultar request timed out (15 s) at {OCULTAR_URL}. "
            "Raw text withheld to preserve zero-egress design."
        )
    except httpx.HTTPStatusError as exc:
        raise RuntimeError(
            f"Ocultar returned HTTP {exc.response.status_code}: "
            f"{exc.response.text[:300]}"
        )

    data = response.json()
    clean_text: str = data.get("refined", "")

    # Build token map from the redacted output: token → PII type
    # e.g. {"[EMAIL_9c8f7a1b]": "EMAIL", "[SSN_3a1b2c4d]": "SSN"}
    token_map: dict[str, str] = {
        m.group(0): m.group(1) for m in _TOKEN_RE.finditer(clean_text)
    }

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
    async with mcp.server.stdio.stdio_server() as (read_stream, write_stream):
        await app.run(
            read_stream,
            write_stream,
            app.create_initialization_options(),
        )


def main_sync() -> None:
    """Entrypoint for the ocultar-goose-mcp console script."""
    asyncio.run(main())


if __name__ == "__main__":
    main_sync()
