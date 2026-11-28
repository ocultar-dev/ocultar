import { Link } from "react-router-dom";
import { ContentLayout } from "@/components/site/ContentLayout";
import { CodeBlock } from "@/components/site/CodeBlock";
import { RepoCTA } from "@/components/site/RepoCTA";
import { useSeo } from "@/hooks/useSeo";

const COMPARISON_ROWS = [
  {
    dimension: "Core runtime",
    ocultar: "Compiled Go 1.26+ binary (Python 3.10+ only for optional MCP stdio shims)",
    presidio: "Python 3.11–3.14 + Flask + NLP engine (spaCy, Transformers, or Stanza)",
  },
  {
    dimension: "Deployment footprint",
    ocultar: "Single ~155 MB container for core Refinery/Proxy (Tiers 0.1–1.5 & 3); optional local SLM sidecar for Tier 2 NER",
    presidio: "Two containers (presidio-analyzer + presidio-anonymizer) plus downloaded spaCy model (en_core_web_lg) or Transformer weights",
  },
  {
    dimension: "Network egress & failure mode",
    ocultar: "Zero-egress by default; fail-closed (HTTP 500/429 on error, never forwards raw text); SSRF blocklist + loopback-only SLM guard",
    presidio: "Library / REST microservices; zero-egress and fail-closed enforcement must be implemented in the caller or gateway",
  },
  {
    dimension: "Speed & latency profile",
    ocultar: "Compiled Go regex, checksum, libphonenumber, and heuristic pipeline with no Python GIL or model cold start; optional Tier 2 SLM (~200 ms avg)",
    presidio: "Python runtime + spaCy/Transformer NER inference on every request; optional CUDA/MPS acceleration for heavy models",
  },
  {
    dimension: "Reversible tokenization",
    ocultar: "Built-in deterministic HMAC-SHA256 tokens ([EMAIL_bbbec93e]) backed by AES-256-GCM / HKDF-SHA256 local vault (DuckDB or PostgreSQL HA) and auditor-gated /api/reveal",
    presidio: "One-way anonymization by default (replace, mask, redact, hash, encrypt operators); cross-session reversible vault must be built externally",
  },
  {
    dimension: "MCP integration",
    ocultar: "First-party PyPI servers: ocultar-claude-mcp, ocultar-mistral-mcp, ocultar-goose-mcp",
    presidio: "No official first-party MCP server; community-maintained wrappers only",
  },
  {
    dimension: "Audit trail",
    ocultar: "Built-in SHA-256 hash-chained, Ed25519-signed append-only audit log (metadata only, zero plaintext PII)",
    presidio: "No built-in cryptographic audit log; bring your own logging pipeline",
  },
  {
    dimension: "Image / OCR redaction",
    ocultar: "Not built in (focuses on text, JSON, CSV, and file payloads via /api/refine/file)",
    presidio: "Dedicated presidio-image-redactor package/container with Tesseract OCR",
  },
  {
    dimension: "License",
    ocultar: "AGPLv3 (commercial license available)",
    presidio: "MIT",
  },
];

const OCULTAR_START = `export OCU_MASTER_KEY=$(openssl rand -hex 32)
export OCU_SALT=$(openssl rand -hex 16)
export OCU_AUDITOR_TOKEN=$(openssl rand -hex 24)

docker run --rm -p 4141:4141 \\
  -e OCU_MASTER_KEY -e OCU_SALT -e OCU_AUDITOR_TOKEN \\
  ghcr.io/ocultar-dev/ocultar:latest -serve 4141`;

const PRESIDIO_START = `# Presidio container images moved from mcr.microsoft.com to Data Privacy Stack GHCR
docker run -d -p 5002:3000 ghcr.io/data-privacy-stack/presidio-analyzer:latest
docker run -d -p 5001:3000 ghcr.io/data-privacy-stack/presidio-anonymizer:latest

# Or as Python packages (requires a spaCy language model download):
pip install presidio_analyzer presidio_anonymizer
python -m spacy download en_core_web_lg`;

const BENCHMARK_SNIPPET = `# Measure local OCULTAR p50/p99 on your own payload with hyperfine or curl
PAYLOAD='Call Jean Dupont at jean.dupont@banque.fr, IBAN FR76 3000 6000 0112 3456 7890 189.'

hyperfine --warmup 10 --runs 200 \\
  "curl -s -X POST http://127.0.0.1:4141/api/refine --data-binary '$PAYLOAD' > /dev/null"`;

const FAQS = [
  {
    q: "Does Microsoft Presidio have an official MCP server for Claude?",
    a: "No. Presidio does not publish a first-party MCP server; integrations with Claude or other MCP hosts rely on third-party wrappers or external AI gateways. OCULTAR maintains first-party MCP packages on PyPI for Claude (ocultar-claude-mcp), Mistral (ocultar-mistral-mcp), and Goose (ocultar-goose-mcp).",
  },
  {
    q: "Can OCULTAR restore redacted PII after an LLM responds?",
    a: "Yes. Every detected value is replaced with a deterministic HMAC-SHA256 token and encrypted into a local AES-256-GCM vault. Callers holding OCU_AUDITOR_TOKEN can restore specific tokens through POST /api/reveal, and every reveal is recorded in an Ed25519-signed audit log.",
  },
  {
    q: "When is Microsoft Presidio a better fit than OCULTAR?",
    a: "Choose Presidio if you are embedding PII detection directly inside a Python ETL or Spark pipeline, need built-in image/OCR redaction (presidio-image-redactor), or require a permissive MIT license instead of AGPLv3.",
  },
  {
    q: "Does OCULTAR require a GPU or Python runtime?",
    a: "No. OCULTAR's core Refinery is a compiled Go binary executing regex, checksum, libphonenumber, and contextual heuristic shields locally. An optional Tier 2 SLM sidecar can be enabled via SLM_SIDECAR_URL when you want neural NER on top of deterministic shields.",
  },
];

export default function VsPresidio() {
  useSeo({ path: "/vs-microsoft-presidio" });

  return (
    <ContentLayout
      eyebrow="Technical Comparison"
      title="OCULTAR vs Microsoft Presidio"
      lead="Both tools detect and mask PII locally without calling a cloud DLP API. They diverge on runtime architecture, deployment footprint, reversible tokenization, and first-party LLM/MCP integration."
      meta="Updated Nov 2026 · Covers OCULTAR v1.16 (Go) vs Presidio (Data Privacy Stack / Python)"
      breadcrumbs={[
        { label: "Home", to: "/" },
        { label: "OCULTAR vs Microsoft Presidio" },
      ]}
    >
      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-foreground">
          The short version
        </h2>
        <p>
          <strong className="text-foreground">Microsoft Presidio</strong> (now maintained under the{" "}
          <code className="font-mono text-[13px] text-primary">data-privacy-stack</code> GitHub organization) is a
          mature Python framework for PII detection and anonymization. It excels inside Python data pipelines,
          notebooks, and batch jobs where spaCy or Hugging Face models already fit your stack.
        </p>
        <p>
          <strong className="text-foreground">OCULTAR</strong> is a compiled Go zero-egress firewall built
          specifically for LLM workflows. It ships as a single local binary or container with deterministic,
          reversible tokenization, an encrypted local vault, a fail-closed HTTP proxy, an OpenAI-compatible
          gateway (Sombra), and first-party MCP servers for Claude, Mistral, and Goose.
        </p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-foreground">
          Side-by-side comparison
        </h2>
        <div className="overflow-x-auto rounded-xl border border-border bg-surface/60">
          <table className="w-full border-collapse text-left text-[13.5px]">
            <thead>
              <tr className="border-b border-border bg-surface-elevated/80">
                <th className="py-3.5 px-4 font-mono text-[11px] uppercase tracking-wider text-muted-foreground">
                  Dimension
                </th>
                <th className="py-3.5 px-4 font-mono text-[11px] uppercase tracking-wider text-primary">
                  OCULTAR
                </th>
                <th className="py-3.5 px-4 font-mono text-[11px] uppercase tracking-wider text-foreground">
                  Microsoft Presidio
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {COMPARISON_ROWS.map((row) => (
                <tr key={row.dimension} className="align-top">
                  <td className="py-3.5 px-4 font-medium text-foreground whitespace-nowrap">
                    {row.dimension}
                  </td>
                  <td className="py-3.5 px-4 text-foreground/90">{row.ocultar}</td>
                  <td className="py-3.5 px-4 text-muted-foreground">{row.presidio}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-foreground">
          1. Deployment footprint
        </h2>
        <p>
          Presidio splits detection and redaction into two Flask microservices (
          <code className="font-mono text-[13px] text-foreground">presidio-analyzer</code> and{" "}
          <code className="font-mono text-[13px] text-foreground">presidio-anonymizer</code>). Even when installed
          via <code className="font-mono text-[13px] text-foreground">pip</code>, the default NLP engine pulls
          spaCy and a multi-hundred-megabyte language model (
          <code className="font-mono text-[13px] text-foreground">en_core_web_lg</code>).
        </p>
        <p>
          OCULTAR's core Refinery and Proxy compile to static Go binaries. The Docker image is ~155 MB and runs
          Tiers 0.1–1.5 and Tier 3 (Base64/JWT evasion decoding, custom dictionary, entity registry, 50+ national ID
          and financial regex/checksum rules, libphonenumber, address heuristics, greeting/signature detection) with
          no Python interpreter and no ML runtime. If you need neural NER for unstructured names, you opt into the
          local Tier 2 SLM sidecar via <code className="font-mono text-[13px] text-foreground">SLM_SIDECAR_URL</code>.
        </p>
        <div className="grid grid-cols-1 gap-4">
          <CodeBlock code={OCULTAR_START} language="bash" filename="OCULTAR — single container" />
          <CodeBlock code={PRESIDIO_START} language="bash" filename="Presidio — two containers or Python + spaCy" />
        </div>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-foreground">
          2. Zero-egress & fail-closed architecture
        </h2>
        <p>
          Presidio is a detection/anonymization component; it does not enforce how your application handles timeouts,
          crashes, or outbound network traffic. If your wrapper catches an exception from the analyzer and falls back
          to sending the original prompt, raw PII leaks to the upstream LLM.
        </p>
        <p>
          OCULTAR enforces fail-closed behavior at the proxy and MCP boundary:
        </p>
        <ul className="list-disc pl-6 space-y-2">
          <li>
            <strong className="text-foreground">Synchronous blocking pipeline:</strong>{" "}
            <code className="font-mono text-[13px] text-foreground">redactBody()</code> must succeed and persist
            tokens to the encrypted vault before an upstream socket is opened. Any error returns HTTP 500 (or 429 on
            queue saturation) and aborts the request.
          </li>
          <li>
            <strong className="text-foreground">Loopback guard for Tier 2 NER:</strong>{" "}
            <code className="font-mono text-[13px] text-foreground">NewRemoteScanner</code> refuses to dial a
            non-loopback <code className="font-mono text-[13px] text-foreground">SLM_SIDECAR_URL</code> unless{" "}
            <code className="font-mono text-[13px] text-foreground">OCU_ALLOW_REMOTE_SLM=true</code> is explicitly
            set, preventing accidental exfiltration of raw text to a misconfigured host.
          </li>
          <li>
            <strong className="text-foreground">SSRF protection:</strong> the proxy blocks RFC 1918, loopback,
            link-local, and DNS-rebinding targets on upstream requests.
          </li>
        </ul>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-foreground">
          3. Reversible tokenization vs. one-way anonymization
        </h2>
        <p>
          In LLM workflows, one-way redaction (replacing every name with{" "}
          <code className="font-mono text-[13px] text-foreground">&lt;PERSON&gt;</code>) destroys coreference: the
          model cannot tell whether two mentions refer to the same customer, and you cannot restore real values in
          the model's reply.
        </p>
        <p>
          OCULTAR generates deterministic deployment-keyed HMAC-SHA256 tokens (for example,{" "}
          <code className="font-mono text-[13px] text-primary">[PERSON_f0d48c8f]</code> and{" "}
          <code className="font-mono text-[13px] text-primary">[IBAN_72eb0d24]</code>) and stores the mapping in a
          local AES-256-GCM encrypted vault (DuckDB by default, PostgreSQL HA for multi-node deployments) with an
          automatic 90-day retention sweep. Pre-seeding an Entity Registry ensures{" "}
          <code className="font-mono text-[13px] text-foreground">"Alice"</code>,{" "}
          <code className="font-mono text-[13px] text-foreground">"A. Martin"</code>, and{" "}
          <code className="font-mono text-[13px] text-foreground">"Alice Martin"</code> all collapse to the same
          token across sessions.
        </p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-foreground">
          4. Speed & benchmarking on your own payloads
        </h2>
        <p>
          We do not publish synthetic cross-tool benchmark tables without identical hardware and corpora.
          Architecturally, OCULTAR's Tier 0.1–1.5 and Tier 3 shields execute inside a compiled Go process with
          pre-compiled regular expressions and zero Python interpreter or transformer cold-start overhead. When Tier
          2 SLM NER is enabled, the local model adds roughly ~200 ms per call depending on hardware.
        </p>
        <p>
          Measure latency directly against your own document distribution:
        </p>
        <CodeBlock code={BENCHMARK_SNIPPET} language="bash" filename="benchmark-local.sh" />
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-foreground">
          5. MCP integration for Claude, Mistral, and Goose
        </h2>
        <p>
          Presidio does not ship an official Model Context Protocol server. OCULTAR publishes three first-party PyPI
          packages (<code className="font-mono text-[13px] text-primary">ocultar-claude-mcp</code>,{" "}
          <code className="font-mono text-[13px] text-primary">ocultar-mistral-mcp</code>, and{" "}
          <code className="font-mono text-[13px] text-primary">ocultar-goose-mcp</code>) that talk exclusively to
          your local Refinery over <code className="font-mono text-[13px] text-foreground">localhost</code> and fail
          closed if the Refinery is unreachable. See the{" "}
          <Link
            to="/guides/redact-pii-claude-mcp"
            className="text-primary hover:underline underline-offset-4"
          >
            Claude Desktop & Claude Code MCP setup guide
          </Link>{" "}
          for copy-pasteable configs.
        </p>
      </section>

      <section className="space-y-4 rounded-xl border border-border bg-surface/50 p-6">
        <h2 className="text-xl font-semibold text-foreground">
          When Microsoft Presidio is the better choice
        </h2>
        <ul className="list-disc pl-5 space-y-2 text-[14.5px]">
          <li>
            <strong className="text-foreground">Embedded Python ETL / Spark jobs:</strong> if you want to import a
            Python library in-process without running a separate HTTP sidecar.
          </li>
          <li>
            <strong className="text-foreground">Image and screenshot redaction:</strong> Presidio's{" "}
            <code className="font-mono text-[13px] text-foreground">presidio-image-redactor</code> handles bounding
            boxes and Tesseract OCR out of the box.
          </li>
          <li>
            <strong className="text-foreground">Permissive MIT licensing requirement:</strong> if your legal policy
            prohibits AGPLv3 dependencies and you do not want a commercial license.
          </li>
          <li>
            <strong className="text-foreground">Existing spaCy/Transformers recognizers:</strong> if your team has
            already invested in custom Python recognizers tuned on internal corpora.
          </li>
        </ul>
      </section>

      <section className="space-y-5">
        <h2 className="text-2xl font-semibold text-foreground">
          Frequently asked questions
        </h2>
        <div className="divide-y divide-border rounded-xl border border-border bg-surface/40">
          {FAQS.map((item) => (
            <div key={item.q} className="p-5 sm:p-6">
              <h3 className="text-[15px] font-semibold text-foreground">{item.q}</h3>
              <p className="mt-2 text-[14px] leading-relaxed text-muted-foreground">{item.a}</p>
            </div>
          ))}
        </div>
      </section>

      <RepoCTA
        heading="Test OCULTAR against your own payloads"
        description="Single Go binary or Docker container. No cloud account, no telemetry, AGPLv3 open source."
      />
    </ContentLayout>
  );
}
