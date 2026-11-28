import { Link } from "react-router-dom";
import { ContentLayout } from "@/components/site/ContentLayout";
import { CodeBlock } from "@/components/site/CodeBlock";
import { RepoCTA } from "@/components/site/RepoCTA";
import { useSeo } from "@/hooks/useSeo";

const GDPR_CHECKLIST = [
  {
    article: "Art. 5(1)(c) & Art. 25(1)",
    title: "Data minimisation & privacy by design",
    verify: "Ensure prompts sent to external LLM APIs contain only the minimum fields required for the task—no raw names, emails, IBANs, SSNs, or national IDs.",
    control: "Route traffic through the OCULTAR Proxy or Sombra Gateway. redactBody() runs synchronously before any upstream socket opens, replacing PII with HMAC-SHA256 tokens ([EMAIL_bbbec93e]).",
  },
  {
    article: "Art. 25(2)",
    title: "Privacy by default (fail-closed)",
    verify: "Confirm that a redaction crash, vault write failure, or SLM timeout aborts the request instead of falling back to sending raw text.",
    control: "OCULTAR returns HTTP 500 on Refinery/vault/SLM errors and HTTP 429 on queue saturation. There is no runtime flag to bypass redaction on a live proxy.",
  },
  {
    article: "Art. 5(1)(e)",
    title: "Storage limitation & retention",
    verify: "Define a bounded TTL for any local token-to-plaintext mapping and verify automatic purge.",
    control: "The local DuckDB/PostgreSQL vault runs a background retention sweep (default TTL: 90 days). Raw prompts are never stored as whole documents—only individual encrypted token mappings.",
  },
  {
    article: "Art. 32(1)(a)–(b)",
    title: "Encryption at rest, key separation & network boundaries",
    verify: "Store encryption keys separately from vault files, isolate environments with distinct salts, and block SSRF exfiltration.",
    control: "AES-256-GCM ciphertext at rest with HKDF-SHA256 derivation (OCU_MASTER_KEY + OCU_SALT injected from your secrets manager). SSRF blocklist rejects RFC 1918, loopback, link-local, and DNS-rebinding targets; SLM_SIDECAR_URL rejects non-loopback hosts by default.",
  },
  {
    article: "Art. 5(2) & Art. 30",
    title: "Accountability & records of processing",
    verify: "Maintain tamper-evident records showing when data was masked or revealed, by whom, without leaking plaintext PII into log aggregators.",
    control: "Set OCU_AUDIT_PRIVATE_KEY to enable the SHA-256 hash-chained, Ed25519-signed append-only audit log. Entries record actor, action (refine/reveal), token IDs, and timestamp—never plaintext PII.",
  },
  {
    article: "Chapter V (Art. 44–49)",
    title: "International data transfers",
    verify: "Keep re-identification material inside EU/EEA infrastructure and verify that no raw personal data crosses into third-country LLM endpoints.",
    control: "Host the Refinery, vault.db (or PostgreSQL HA), and OCU_MASTER_KEY inside your EU/EEA VPC. Enforce per-connector model allowlists in Sombra and verify outbound payloads via packet capture.",
  },
  {
    article: "Art. 35",
    title: "Data Protection Impact Assessment (DPIA)",
    verify: "Document data flows, residual re-identification risk (e.g., rare quasi-identifiers in free text), and automated test evidence for your DPO.",
    control: "Attach OCULTAR's GDPR Art. 25 mapping, fail-closed test suite results, and SSRF test results directly to your DPIA annex.",
  },
];

const EU_AI_ACT_CHECKLIST = [
  {
    article: "Art. 10",
    title: "Data & data governance",
    scope: "High-risk AI systems",
    control: "Stripping direct identifiers before inference prevents customer PII from leaking into downstream model prompts, evaluations, or provider retention buffers.",
  },
  {
    article: "Art. 12",
    title: "Record-keeping",
    scope: "High-risk AI systems",
    control: "Ed25519-signed, hash-chained audit logs provide verifiable event records of every refine and reveal operation for post-incident investigation.",
  },
  {
    article: "Art. 26",
    title: "Deployer obligations",
    scope: "Deployers of high-risk systems",
    control: "Technical input-data controls: connector policy matrices, per-connector model allowlists in Sombra, and least-privilege separation of OCU_AUDITOR_TOKEN.",
  },
  {
    article: "Art. 50",
    title: "Transparency obligations",
    scope: "All user-facing AI systems",
    control: "Not solved by a redaction proxy. Informing users that they are interacting with an AI system and labelling synthetic content remain application-layer obligations.",
  },
];

const HARDENED_RUN = `# Production-style local/VPC launch with audit signing enabled
export OCU_MASTER_KEY=$(openssl rand -hex 32)
export OCU_SALT=$(openssl rand -hex 16)
export OCU_AUDITOR_TOKEN=$(openssl rand -hex 24)
export OCU_AUDIT_PRIVATE_KEY=$(openssl rand -hex 32)

docker run -d --name ocultar-refinery \\
  -p 127.0.0.1:4141:4141 \\
  -v ocultar-vault:/data \\
  -e OCU_MASTER_KEY \\
  -e OCU_SALT \\
  -e OCU_AUDITOR_TOKEN \\
  -e OCU_AUDIT_PRIVATE_KEY \\
  -e OCU_AUDIT_LOG_PATH=/data/audit.log \\
  ghcr.io/ocultar-dev/ocultar:latest -serve 4141`;

const VERIFY_BOUNDARY = `# 1. Confirm /api/reveal rejects unauthenticated callers (must return 401/403)
curl -i -s -X POST http://127.0.0.1:4141/api/reveal \\
  -H "Content-Type: application/json" \\
  -d '{"tokens":["[EMAIL_bbbec93e]"]}' | head -n 5

# 2. Confirm fail-closed proxy refuses to forward when Refinery is unavailable
# (Run the repository's automated fail-closed and SSRF suites in CI)
go test ./services/refinery/pkg/proxy/... ./services/refinery/pkg/refinery/... ./apps/proxy/...`;

export default function GdprEuAiActRedaction() {
  useSeo({ path: "/guides/local-pii-redaction-gdpr-eu-ai-act" });

  return (
    <ContentLayout
      eyebrow="Compliance Engineering"
      title="Local PII Redaction Checklist for GDPR & the EU AI Act"
      lead="A practical engineering checklist for calling frontier LLMs while keeping raw personal data inside your infrastructure. Maps concrete GDPR and EU AI Act controls to verifiable OCULTAR configurations."
      meta="Updated Nov 2026 · Engineering reference, not legal advice"
      breadcrumbs={[
        { label: "Home", to: "/" },
        { label: "Docs", to: "/docs" },
        { label: "GDPR & EU AI Act Checklist" },
      ]}
    >
      <section className="rounded-xl border border-border bg-surface/60 p-6 space-y-3">
        <p className="font-mono text-[11px] uppercase tracking-[0.18em] text-primary">
          Important distinction · GDPR Recital 26
        </p>
        <h2 className="text-lg font-semibold text-foreground">
          Pseudonymisation is not anonymisation
        </h2>
        <p className="text-[14.5px]">
          Reversible tokenization (replacing{" "}
          <code className="font-mono text-[13px] text-foreground">jean.dupont@banque.fr</code> with{" "}
          <code className="font-mono text-[13px] text-primary">[EMAIL_bbbec93e]</code> while storing the encrypted
          mapping in <code className="font-mono text-[13px] text-foreground">vault.db</code>) is{" "}
          <strong className="text-foreground">pseudonymisation</strong> under GDPR Art. 4(5). Because you hold the
          vault and <code className="font-mono text-[13px] text-foreground">OCU_MASTER_KEY</code>, the dataset
          remains personal data in your hands.
        </p>
        <p className="text-[14.5px]">
          What changes at the boundary is that the external LLM provider never receives the additional information
          needed to re-identify the data subject. That directly supports Art. 25 privacy by design, Art. 32
          security, and Chapter V transfer risk assessments—but you still need a valid legal basis, a DPIA where
          applicable, and a review by your DPO or legal counsel.
        </p>
      </section>

      <section className="space-y-5">
        <h2 className="text-2xl font-semibold text-foreground">
          1. GDPR engineering checklist
        </h2>
        <div className="space-y-4">
          {GDPR_CHECKLIST.map((item) => (
            <div
              key={item.article}
              className="rounded-xl border border-border bg-surface/40 p-5 sm:p-6 space-y-3"
            >
              <div className="flex flex-wrap items-center gap-3">
                <span className="rounded border border-primary/30 bg-primary/10 px-2 py-0.5 font-mono text-[11px] text-primary">
                  {item.article}
                </span>
                <h3 className="text-[16px] font-semibold text-foreground">{item.title}</h3>
              </div>
              <p className="text-[14px]">
                <strong className="text-foreground">What to verify:</strong> {item.verify}
              </p>
              <p className="text-[14px]">
                <strong className="text-foreground">OCULTAR control:</strong> {item.control}
              </p>
            </div>
          ))}
        </div>
      </section>

      <section className="space-y-5">
        <h2 className="text-2xl font-semibold text-foreground">
          2. EU AI Act (Regulation 2024/1689) checklist
        </h2>
        <p>
          Where OCULTAR sits in front of an external LLM API or internal agent gateway, it maps to the following
          technical controls—and leaves non-proxy obligations explicitly called out:
        </p>
        <div className="overflow-x-auto rounded-xl border border-border bg-surface/60">
          <table className="w-full border-collapse text-left text-[13.5px]">
            <thead>
              <tr className="border-b border-border bg-surface-elevated/80">
                <th className="py-3 px-4 font-mono text-[11px] uppercase tracking-wider text-muted-foreground">
                  Article
                </th>
                <th className="py-3 px-4 font-mono text-[11px] uppercase tracking-wider text-muted-foreground">
                  Scope
                </th>
                <th className="py-3 px-4 font-mono text-[11px] uppercase tracking-wider text-primary">
                  Engineering control
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {EU_AI_ACT_CHECKLIST.map((row) => (
                <tr key={row.article} className="align-top">
                  <td className="py-3.5 px-4 font-medium text-foreground whitespace-nowrap">
                    <span className="font-mono text-primary">{row.article}</span>
                    <div className="text-[12.5px] text-muted-foreground font-normal mt-0.5">
                      {row.title}
                    </div>
                  </td>
                  <td className="py-3.5 px-4 text-muted-foreground whitespace-nowrap">
                    {row.scope}
                  </td>
                  <td className="py-3.5 px-4 text-foreground/90">{row.control}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-foreground">
          3. Hardened deployment baseline
        </h2>
        <p>
          Inject keys from your secrets manager (Vault, AWS Secrets Manager, Doppler) rather than committing them
          to compose files, bind listeners to loopback or a private VPC interface, and mount persistent storage for
          the encrypted vault and signed audit log:
        </p>
        <CodeBlock code={HARDENED_RUN} language="bash" filename="Hardened Refinery launch" />
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-foreground">
          4. Evidence to attach to your security review
        </h2>
        <p>
          Run these checks in CI or staging and archive the output for your DPO or vendor security review:
        </p>
        <CodeBlock code={VERIFY_BOUNDARY} language="bash" filename="Verification checks" />
        <p className="text-[14px]">
          For the full article-by-article DPA annex, read{" "}
          <Link
            to="/docs/compliance/GDPR_PRIVACY_BY_DESIGN"
            className="text-primary hover:underline underline-offset-4"
          >
            GDPR Article 25 — Privacy by Design
          </Link>{" "}
          and{" "}
          <Link
            to="/docs/security/SECURITY_MODEL"
            className="text-primary hover:underline underline-offset-4"
          >
            Security Model
          </Link>
          .
        </p>
      </section>

      <RepoCTA
        heading="Deploy a zero-egress boundary in your VPC"
        description="Self-hosted Go binary with AES-256-GCM vault, fail-closed proxy, and Ed25519-signed audit log. AGPLv3 open source."
      />
    </ContentLayout>
  );
}
