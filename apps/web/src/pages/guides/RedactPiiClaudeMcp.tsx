import { Link } from "react-router-dom";
import { ContentLayout } from "@/components/site/ContentLayout";
import { CodeBlock } from "@/components/site/CodeBlock";
import { RepoCTA } from "@/components/site/RepoCTA";
import { useSeo } from "@/hooks/useSeo";

const START_REFINERY = `# Generate persistent keys once (store them in your secrets manager — rotating them invalidates existing vault tokens)
export OCU_MASTER_KEY=$(openssl rand -hex 32)
export OCU_SALT=$(openssl rand -hex 16)
export OCU_AUDITOR_TOKEN=$(openssl rand -hex 24)

docker run --rm -p 127.0.0.1:4141:4141 \\
  -e OCU_MASTER_KEY \\
  -e OCU_SALT \\
  -e OCU_AUDITOR_TOKEN \\
  ghcr.io/ocultar-dev/ocultar:latest -serve 4141`;

const HEALTH_CHECK = `curl -s http://127.0.0.1:4141/api/health
# {"slm":{"circuit":"closed","status":"online"},"status":"healthy","vault":{"status":"online"},"version":"1.16.0"}`;

const INSTALL_MCP = `# Option A: install with pip (Python 3.10+)
pip install ocultar-claude-mcp

# Option B: run ephemerally with uvx (no global install)
uvx ocultar-claude-mcp --help`;

const CLAUDE_DESKTOP_JSON = `{
  "mcpServers": {
    "ocultar-pii": {
      "command": "ocultar-claude-mcp",
      "env": {
        "OCULTAR_URL": "http://127.0.0.1:4141"
      }
    }
  }
}`;

const CLAUDE_DESKTOP_UVX_JSON = `{
  "mcpServers": {
    "ocultar-pii": {
      "command": "uvx",
      "args": ["ocultar-claude-mcp"],
      "env": {
        "OCULTAR_URL": "http://127.0.0.1:4141",
        "OCULTAR_AUDITOR_TOKEN": "paste-your-OCU_AUDITOR_TOKEN-only-for-auditor-profiles"
      }
    }
  }
}`;

const CLAUDE_CODE_CLI = `# Register globally for Claude Code (read-only redaction profile — no auditor reveal)
claude mcp add ocultar-pii \\
  -e OCULTAR_URL=http://127.0.0.1:4141 \\
  -- ocultar-claude-mcp

# Verify registration
claude mcp list`;

const CLAUDE_CODE_PROJECT_JSON = `{
  "mcpServers": {
    "ocultar-pii": {
      "command": "ocultar-claude-mcp",
      "env": {
        "OCULTAR_URL": "http://127.0.0.1:4141"
      }
    }
  }
}`;

const VERIFY_CURL = `printf 'Contact Jean Dupont at jean.dupont@banque.fr, IBAN FR76 3000 6000 0112 3456 7890 189. api_key=sk_live_9876543210 password=hunter2' \\
  | curl -s --data-binary @- http://127.0.0.1:4141/api/refine | jq .`;

const VERIFY_OUTPUT = `{
  "refined": "Contact [PERSON_f0d48c8f] [EMAIL_bbbec93e], IBAN [IBAN_72eb0d24]. [SECRET_4a91c0de] [CREDENTIAL_8e12b7a1]",
  "report": {
    "mode": "serve",
    "files_scanned": 1,
    "total_pii_count": 5,
    "blocking": true
  }
}`;

const REVEAL_EXAMPLE = `# Only works when OCULTAR_AUDITOR_TOKEN matches OCU_AUDITOR_TOKEN on the server
curl -s -X POST http://127.0.0.1:4141/api/reveal \\
  -H "Authorization: Bearer $OCU_AUDITOR_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{"tokens":["[EMAIL_bbbec93e]","[IBAN_72eb0d24]"]}'`;

export default function RedactPiiClaudeMcp() {
  useSeo({ path: "/guides/redact-pii-claude-mcp" });

  return (
    <ContentLayout
      eyebrow="Developer Guide · MCP"
      title="Redact PII in Claude Desktop and Claude Code with Local MCP"
      lead="Wire OCULTAR's local MCP server (ocultar-claude-mcp) to Claude Desktop or Claude Code so names, emails, IBANs, and credentials are tokenized on localhost before reaching external model APIs."
      meta="Requires Docker (or Go 1.26+) · Python 3.10+ · ~5 minutes"
      breadcrumbs={[
        { label: "Home", to: "/" },
        { label: "Docs", to: "/docs" },
        { label: "Redact PII in Claude with MCP" },
      ]}
    >
      <section className="rounded-xl border border-primary/30 bg-primary/5 p-6 space-y-3">
        <p className="font-mono text-[11px] uppercase tracking-[0.18em] text-primary">
          Security boundary — read before deploying
        </p>
        <p className="text-[14.5px] text-foreground/90">
          Know what an MCP tool can and cannot intercept. If you paste raw client data directly into the Claude
          chat input box, that message is sent to Anthropic's API <em>before</em> Claude decides to invoke any MCP
          tool.
        </p>
        <ul className="list-disc pl-5 space-y-1.5 text-[14px] text-muted-foreground">
          <li>
            <strong className="text-foreground">Safe with MCP:</strong> calling{" "}
            <code className="font-mono text-[13px] text-foreground">refine_text</code> from local scripts/workflows
            before passing text to an agent, pre-seeding entities via{" "}
            <code className="font-mono text-[13px] text-foreground">register_entity</code>, or routing prompts
            through <code className="font-mono text-[13px] text-foreground">sombra_query</code> (which redacts
            locally before dialing the upstream LLM).
          </li>
          <li>
            <strong className="text-foreground">Hard network-level guarantee:</strong> when you need every outbound
            HTTP request scrubbed regardless of user habits, place the{" "}
            <Link
              to="/docs/guides/refinery_proxy_setup"
              className="text-primary hover:underline underline-offset-4"
            >
              OCULTAR HTTP Proxy
            </Link>{" "}
            or{" "}
            <Link
              to="/docs/guides/SOMBRA_GUIDE"
              className="text-primary hover:underline underline-offset-4"
            >
              Sombra Gateway
            </Link>{" "}
            in the egress path.
          </li>
        </ul>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-foreground">
          How the local MCP loop works
        </h2>
        <p>
          <code className="font-mono text-[13px] text-primary">ocultar-claude-mcp</code> runs locally over stdio as a
          child process of Claude Desktop or Claude Code. It makes zero outbound internet calls—every tool call talks
          strictly to your local OCULTAR Refinery on{" "}
          <code className="font-mono text-[13px] text-foreground">http://127.0.0.1:4141</code> (and optionally
          Sombra on port <code className="font-mono text-[13px] text-foreground">8086</code>). If the Refinery is
          unreachable, the MCP server fails closed with an error and refuses to pass data through.
        </p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-foreground">
          Step 1: Start the local OCULTAR Refinery
        </h2>
        <p>
          Bind the container to <code className="font-mono text-[13px] text-foreground">127.0.0.1:4141</code> so
          the Refinery is reachable only from your machine:
        </p>
        <CodeBlock code={START_REFINERY} language="bash" filename="Terminal" />
        <p>Confirm the engine and local encrypted vault are online:</p>
        <CodeBlock code={HEALTH_CHECK} language="bash" filename="Verify health" />
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-foreground">
          Step 2: Install ocultar-claude-mcp
        </h2>
        <p>
          The Claude MCP extension is published on PyPI and requires Python 3.10+.
        </p>
        <CodeBlock code={INSTALL_MCP} language="bash" filename="Install MCP server" />
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-foreground">
          Step 3: Configure Claude Desktop
        </h2>
        <p>
          Add the <code className="font-mono text-[13px] text-foreground">ocultar-pii</code> server to your Claude
          Desktop config file, then restart Claude Desktop:
        </p>
        <ul className="list-disc pl-6 space-y-1 text-[14px]">
          <li>
            <strong className="text-foreground">macOS:</strong>{" "}
            <code className="font-mono text-[12.5px] text-foreground">
              ~/Library/Application Support/Claude/claude_desktop_config.json
            </code>
          </li>
          <li>
            <strong className="text-foreground">Windows:</strong>{" "}
            <code className="font-mono text-[12.5px] text-foreground">
              %APPDATA%\Claude\claude_desktop_config.json
            </code>
          </li>
          <li>
            <strong className="text-foreground">Linux:</strong>{" "}
            <code className="font-mono text-[12.5px] text-foreground">
              ~/.config/Claude/claude_desktop_config.json
            </code>
          </li>
        </ul>
        <CodeBlock
          code={CLAUDE_DESKTOP_JSON}
          language="json"
          filename="claude_desktop_config.json (standard developer profile — no reveal)"
        />
        <p>
          If you prefer <code className="font-mono text-[13px] text-foreground">uvx</code> without a persistent
          install, or need auditor-only tools (
          <code className="font-mono text-[13px] text-foreground">reveal_tokens</code>,{" "}
          <code className="font-mono text-[13px] text-foreground">register_entity</code>,{" "}
          <code className="font-mono text-[13px] text-foreground">seed_entities</code>):
        </p>
        <CodeBlock
          code={CLAUDE_DESKTOP_UVX_JSON}
          language="json"
          filename="claude_desktop_config.json (uvx + auditor profile)"
        />
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-foreground">
          Step 4: Configure Claude Code (CLI)
        </h2>
        <p>
          Register the MCP server directly from the Claude Code CLI:
        </p>
        <CodeBlock code={CLAUDE_CODE_CLI} language="bash" filename="Claude Code CLI" />
        <p>
          Or check a project-scoped <code className="font-mono text-[13px] text-foreground">.mcp.json</code> into
          your repository (never commit{" "}
          <code className="font-mono text-[13px] text-foreground">OCULTAR_AUDITOR_TOKEN</code> or API keys):
        </p>
        <CodeBlock code={CLAUDE_CODE_PROJECT_JSON} language="json" filename=".mcp.json" />
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-foreground">
          Step 5: Verify redaction of PII and credentials
        </h2>
        <p>
          Before running inside Claude, test the local Refinery directly with a sample containing a person name,
          email, IBAN, and key-value credentials:
        </p>
        <CodeBlock code={VERIFY_CURL} language="bash" filename="Test /api/refine" />
        <CodeBlock code={VERIFY_OUTPUT} language="json" filename="Expected response" />
        <p className="text-[14px]">
          <strong className="text-foreground">Note on API keys and secrets:</strong> Tier 1 detects{" "}
          <code className="font-mono text-[13px] text-foreground">SECRET</code> and{" "}
          <code className="font-mono text-[13px] text-foreground">CREDENTIAL</code> assignments matching{" "}
          <code className="font-mono text-[13px] text-foreground">key=value</code>,{" "}
          <code className="font-mono text-[13px] text-foreground">token: value</code>,{" "}
          <code className="font-mono text-[13px] text-foreground">secret=value</code>,{" "}
          <code className="font-mono text-[13px] text-foreground">password=value</code>, and AWS secret formats.
          Bare unstructured token strings without a key label should be added to your custom regex rules or Tier 0
          dictionary if your stack uses proprietary prefixes.
        </p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-foreground">
          Step 6: Least-privilege token reveal & Entity Registry
        </h2>
        <p>
          By design, <code className="font-mono text-[13px] text-foreground">refine_text</code> requires no
          auditor privileges, while{" "}
          <code className="font-mono text-[13px] text-foreground">reveal_tokens</code> and the Entity Registry tools
          (<code className="font-mono text-[13px] text-foreground">register_entity</code>,{" "}
          <code className="font-mono text-[13px] text-foreground">list_entities</code>,{" "}
          <code className="font-mono text-[13px] text-foreground">seed_entities</code>) are gated by{" "}
          <code className="font-mono text-[13px] text-foreground">OCULTAR_AUDITOR_TOKEN</code>.
        </p>
        <p>
          Leave <code className="font-mono text-[13px] text-foreground">OCULTAR_AUDITOR_TOKEN</code> unset in
          day-to-day Claude sessions so the model cannot call{" "}
          <code className="font-mono text-[13px] text-foreground">reveal_tokens</code> on its own. When an
          authorized engineer or service needs to rehydrate tokens locally, call{" "}
          <code className="font-mono text-[13px] text-foreground">/api/reveal</code> directly—every call is
          appended to the Ed25519-signed audit log:
        </p>
        <CodeBlock code={REVEAL_EXAMPLE} language="bash" filename="Authorized local reveal" />
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-foreground">
          Troubleshooting
        </h2>
        <div className="space-y-3">
          <div className="rounded-lg border border-border bg-surface/50 p-4">
            <h3 className="font-mono text-[13px] font-semibold text-foreground">
              "Cannot connect to Ocultar at http://127.0.0.1:4141/api/refine"
            </h3>
            <p className="mt-1.5 text-[14px]">
              Expected fail-closed behavior. The MCP server will not bypass redaction if the Refinery container is
              stopped. Start the container and re-run <code className="font-mono text-[12.5px]">GET /api/health</code>.
            </p>
          </div>
          <div className="rounded-lg border border-border bg-surface/50 p-4">
            <h3 className="font-mono text-[13px] font-semibold text-foreground">
              HTTP 403 on reveal_tokens or register_entity
            </h3>
            <p className="mt-1.5 text-[14px]">
              Verify <code className="font-mono text-[12.5px]">OCULTAR_AUDITOR_TOKEN</code> in your MCP config
              matches <code className="font-mono text-[12.5px]">OCU_AUDITOR_TOKEN</code> on the Refinery server.
            </p>
          </div>
          <div className="rounded-lg border border-border bg-surface/50 p-4">
            <h3 className="font-mono text-[13px] font-semibold text-foreground">
              Log warning: "Tier 2 SLM unavailable, degrading to Tier 1"
            </h3>
            <p className="mt-1.5 text-[14px]">
              Normal when running the Refinery standalone without the optional SLM sidecar. Tiers 0.1–1.5 and Tier
              3 still run locally. To enable neural NER, launch the local{" "}
              <code className="font-mono text-[12.5px]">privacy-filter</code> or{" "}
              <code className="font-mono text-[12.5px]">llama.cpp</code> sidecar and set{" "}
              <code className="font-mono text-[12.5px]">SLM_SIDECAR_URL=http://127.0.0.1:8085</code>.
            </p>
          </div>
        </div>
      </section>

      <RepoCTA
        heading="Wire OCULTAR into Claude, Mistral, or Goose"
        description="First-party MCP servers on PyPI backed by a local Go Refinery. Zero telemetry, fail-closed by default."
        secondaryCommand={{
          label: "PyPI",
          command: "pip install ocultar-claude-mcp",
        }}
      />
    </ContentLayout>
  );
}
