# OCULTAR is fully open source now — including the parts we were going to charge for

> **Audience:** Developers, security engineers, and anyone piping data into a third-party AI model who has ever paused before hitting send.

I wanted an AI model to read my medical records. I wasn't comfortable handing them over to do it.

That's the whole origin story. Not a market gap I spotted in a deck — a thing I actually wanted to do and couldn't do safely. Every option I looked at meant sending raw, identifiable data to a company I had no relationship with, no contract with, and no way to audit. So I built the thing I wanted: a proxy that sits between me and the model, strips out anything identifying before it leaves my machine, and puts it back afterward so the response still makes sense.

That's OCULTAR. And as of this release, all of it — including the parts of the roadmap we'd originally planned to gate behind a paid tier — is open source under AGPLv3.

## What it actually does

OCULTAR sits between your application and any LLM provider — OpenAI, Gemini, Anthropic, whatever you're calling. It's a drop-in reverse proxy, not a new SDK to learn:

```js
const response = await openai.chat.completions.create({
  baseURL: "https://ocultar.your-vpc", // ← only change; OCULTAR forwards to whatever OCU_PROXY_TARGET points at
  model: "gpt-4o",
  messages: [{ role: "user", content: userMessage }],
});
```

Before your text reaches the model, it runs through a five-tier detection pipeline — regex rules, entropy scoring, libphonenumber validation, an address parser, a local NER model for the stuff regex can't catch — and every match gets replaced with a deterministic token like `[EMAIL_9c8f7a1b1234abcd]`. Same input, same token, every time, so you can still reason about identity across a conversation without ever exposing it. The real value is encrypted separately in a local vault (AES-256-GCM), and the token rehydrates back to plaintext only for callers who are actually authorized to see it.

Nothing raw crosses the boundary. If the vault or the detection engine errors, the request gets blocked — fail-closed, not fail-open. That was true on day one and it's a line we won't move.

## What's new: everything is in

A few things that used to sit behind a "contact sales" link are just... in the repo now:

- **Sombra**, the agentic gateway — connector-based queries that fetch data from a source, redact it, route it to an LLM, and rehydrate the answer, all without the raw data ever leaving your infrastructure.
- **The Entity Registry** — collapse "John", "J. Doe", and "John Doe" across a whole document set into one canonical token, instead of tokenizing each fragment separately.
- **MCP integrations** for Claude, Goose, and Mistral's Le Chat — `pip install ocultar-claude-mcp` (or `-goose-mcp`, `-mistral-mcp`) and your agent gets `refine_text`, `reveal_tokens`, entity registry tools, and a `sombra_query` tool for free.
- **Deep-scan NER, SIEM-compatible audit forwarding, Ed25519-signed immutable audit logs, PostgreSQL HA vault** — governance features that used to be the enterprise pitch. No license key, no gating.

The reasoning is simple: a privacy tool people can't fully audit isn't actually a privacy tool, it's a trust exercise. AGPLv3 plus a commercial license option (for teams who need to embed it without the copyleft obligations) felt like the honest way to keep building this without asking anyone to take our word for it.

## Try it

```bash
git clone https://github.com/ocultar-dev/ocultar
cd ocultar && docker compose up
```

Docker Compose gets you the proxy, the refinery, and a local vault running in under five minutes, no cloud account, no API key required to start. Full setup guide and architecture docs are on the [GitHub repo](https://github.com/ocultar-dev/ocultar).

If you're building anything that pipes user data — support tickets, medical records, financial statements, whatever — into a model you don't control, I'd like to know if this is useful to you, and I'd like to know where it breaks. Both are useful. [Get in touch](/about).
