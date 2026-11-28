export const ROUTES_CONFIG = {
  siteUrl: "https://ocultar.dev",
  siteName: "OCULTAR",
  defaultOgImage: "https://ocultar.dev/og-default.png",
  routes: [
    {
      path: "/",
      title: "OCULTAR — Zero-Egress Local PII Redaction Firewall for AI",
      description:
        "Open-source, local-first PII detection and redaction firewall written in Go. Deterministic tokenization, AES-256-GCM vault, and MCP connectors for Claude, Mistral, and Goose.",
      ogType: "website",
      priority: "1.0",
      changefreq: "weekly",
    },
    {
      path: "/vs-microsoft-presidio",
      title: "OCULTAR vs Microsoft Presidio — Technical Comparison",
      description:
        "Objective engineering comparison of OCULTAR and Microsoft Presidio: Go binary vs Python/spaCy footprint, zero-egress fail-closed architecture, reversible vaults, and MCP support.",
      ogType: "article",
      priority: "0.9",
      changefreq: "monthly",
      article: {
        headline:
          "OCULTAR vs Microsoft Presidio: Architecture, Footprint, and MCP Comparison",
        datePublished: "2026-11-28",
        dateModified: "2026-11-28",
        section: "Comparisons",
      },
      breadcrumbs: [
        { name: "Home", path: "/" },
        { name: "OCULTAR vs Microsoft Presidio", path: "/vs-microsoft-presidio" },
      ],
      faq: [
        {
          question: "Does Microsoft Presidio have an official MCP server for Claude?",
          answer:
            "No. Microsoft Presidio does not ship an official first-party MCP server; community wrappers exist. OCULTAR publishes first-party MCP servers on PyPI for Claude (ocultar-claude-mcp), Mistral (ocultar-mistral-mcp), and Goose (ocultar-goose-mcp).",
        },
        {
          question: "Can OCULTAR restore redacted PII after an LLM responds?",
          answer:
            "Yes. OCULTAR replaces PII with deterministic HMAC-SHA256 tokens and stores originals in an AES-256-GCM encrypted local vault. Authorized callers with OCU_AUDITOR_TOKEN can reveal tokens via /api/reveal, and every reveal is recorded in an Ed25519-signed audit log.",
        },
        {
          question: "When is Microsoft Presidio a better fit than OCULTAR?",
          answer:
            "Choose Presidio if you are embedding PII detection directly inside a Python data pipeline, need image/OCR redaction out of the box (presidio-image-redactor), or require a permissive MIT license instead of AGPLv3.",
        },
        {
          question: "Does OCULTAR require a GPU or Python runtime?",
          answer:
            "No. OCULTAR's core Refinery is a compiled Go binary running deterministic regex, checksum, libphonenumber, and contextual heuristic shields locally with no Python or ML runtime required. An optional Tier 2 SLM sidecar can be attached when deeper contextual NER is needed.",
        },
      ],
    },
    {
      path: "/guides/redact-pii-claude-mcp",
      title: "Redact PII in Claude Desktop & Claude Code with MCP — OCULTAR",
      description:
        "Step-by-step developer guide to wiring OCULTAR's local MCP server to Claude Desktop and Claude Code so client PII, IBANs, and credentials stay on your machine.",
      ogType: "article",
      priority: "0.9",
      changefreq: "monthly",
      article: {
        headline:
          "How to Redact PII in Claude Desktop and Claude Code Using Local MCP",
        datePublished: "2026-11-28",
        dateModified: "2026-11-28",
        section: "Guides",
      },
      breadcrumbs: [
        { name: "Home", path: "/" },
        { name: "Docs", path: "/docs" },
        {
          name: "Redact PII in Claude with MCP",
          path: "/guides/redact-pii-claude-mcp",
        },
      ],
      howto: {
        name: "Configure OCULTAR MCP for Claude Desktop and Claude Code",
        description:
          "Run the local OCULTAR Refinery and connect ocultar-claude-mcp so PII is tokenized locally before reaching Claude.",
        steps: [
          {
            name: "Start the local OCULTAR Refinery",
            text: "Generate OCU_MASTER_KEY, OCU_SALT, and OCU_AUDITOR_TOKEN and launch ghcr.io/ocultar-dev/ocultar:latest on localhost:4141.",
          },
          {
            name: "Install ocultar-claude-mcp",
            text: "Install the PyPI package with pip install ocultar-claude-mcp (or run via uvx).",
          },
          {
            name: "Register the MCP server in Claude Desktop or Claude Code",
            text: "Add ocultar-pii to claude_desktop_config.json or run claude mcp add ocultar-pii -- ocultar-claude-mcp.",
          },
          {
            name: "Verify fail-closed local redaction",
            text: "Check GET /api/health and invoke refine_text with a sample payload containing an email and IBAN.",
          },
        ],
      },
    },
    {
      path: "/guides/local-pii-redaction-gdpr-eu-ai-act",
      title: "Local PII Redaction for GDPR & EU AI Act Compliance — OCULTAR",
      description:
        "Practical engineering checklist for using frontier LLMs under GDPR (Art. 5, 25, 30, 32, 35, Ch. V) and the EU AI Act (Art. 10, 12, 26, 50) using local zero-egress redaction.",
      ogType: "article",
      priority: "0.9",
      changefreq: "monthly",
      article: {
        headline:
          "Engineering Checklist: Local PII Redaction for GDPR and EU AI Act Compliance",
        datePublished: "2026-11-28",
        dateModified: "2026-11-28",
        section: "Compliance",
      },
      breadcrumbs: [
        { name: "Home", path: "/" },
        { name: "Docs", path: "/docs" },
        {
          name: "GDPR & EU AI Act Checklist",
          path: "/guides/local-pii-redaction-gdpr-eu-ai-act",
        },
      ],
    },
    {
      path: "/solutions",
      title: "Enterprise Solutions & Architecture — OCULTAR",
      description:
        "Multi-tier PII redaction, OpenAI-compatible Sombra gateway, Slack and SharePoint connectors, PostgreSQL HA vault, and Ed25519 audit logging for enterprise AI.",
      ogType: "website",
      priority: "0.8",
      changefreq: "monthly",
      breadcrumbs: [
        { name: "Home", path: "/" },
        { name: "Solutions", path: "/solutions" },
      ],
    },
    {
      path: "/docs",
      title: "Documentation — OCULTAR",
      description:
        "Setup guides, API reference, MCP extensions, architecture docs, and GDPR compliance mappings for OCULTAR.",
      ogType: "website",
      priority: "0.8",
      changefreq: "weekly",
      breadcrumbs: [
        { name: "Home", path: "/" },
        { name: "Documentation", path: "/docs" },
      ],
    },
    {
      path: "/calculator",
      title: "Cloud DLP vs Local Redaction ROI Calculator — OCULTAR",
      description:
        "Compare monthly and annual costs of Google Cloud DLP, AWS Comprehend, and Azure AI Language against running OCULTAR locally.",
      ogType: "website",
      priority: "0.7",
      changefreq: "monthly",
      breadcrumbs: [
        { name: "Home", path: "/" },
        { name: "ROI Calculator", path: "/calculator" },
      ],
    },
    {
      path: "/risk-assessment",
      title: "AI PII Exposure Risk Assessment — OCULTAR",
      description:
        "Inspect sample payloads for identifiability, financial sensitivity, and GDPR/EU AI Act regulatory findings before sending data to external LLMs.",
      ogType: "website",
      priority: "0.7",
      changefreq: "monthly",
      breadcrumbs: [
        { name: "Home", path: "/" },
        { name: "Risk Assessment", path: "/risk-assessment" },
      ],
    },
    {
      path: "/about",
      title: "About — OCULTAR",
      description:
        "Why Edu Trejos built OCULTAR: an open-source, AGPLv3, zero-egress PII redaction proxy written in Go.",
      ogType: "website",
      priority: "0.6",
      changefreq: "monthly",
      breadcrumbs: [
        { name: "Home", path: "/" },
        { name: "About", path: "/about" },
      ],
    },
    {
      path: "/privacy",
      title: "Privacy Policy — OCULTAR",
      description:
        "OCULTAR is self-hosted, zero-egress software with no telemetry, no cloud account, and an encrypted local vault under your control.",
      ogType: "website",
      priority: "0.5",
      changefreq: "yearly",
      breadcrumbs: [
        { name: "Home", path: "/" },
        { name: "Privacy Policy", path: "/privacy" },
      ],
    },
    {
      path: "/docs/guides/SETUP_GUIDE",
      title: "Setup Guide — OCULTAR Docs",
      description:
        "Install and run the OCULTAR PII Refinery with Docker or from Go source.",
      ogType: "article",
      priority: "0.7",
      changefreq: "monthly",
    },
    {
      path: "/docs/guides/FRENCH_FINANCE_QUICKSTART",
      title: "French Finance Quickstart — OCULTAR Docs",
      description:
        "Deploy OCULTAR with French financial PII rules (IBAN, SIRET, SIREN, NIR) and CNIL/RGPD alignment.",
      ogType: "article",
      priority: "0.6",
      changefreq: "monthly",
    },
    {
      path: "/docs/guides/DEVELOPER_GUIDE",
      title: "Developer Guide — OCULTAR Docs",
      description:
        "Integrate OCULTAR's Go Refinery API, vault, and custom shields into your engineering workflow.",
      ogType: "article",
      priority: "0.7",
      changefreq: "monthly",
    },
    {
      path: "/docs/guides/ENTERPRISE_SETUP_GUIDE",
      title: "Enterprise Setup Guide — OCULTAR Docs",
      description:
        "Configure PostgreSQL HA vaults, secrets management, and multi-node OCULTAR deployments.",
      ogType: "article",
      priority: "0.6",
      changefreq: "monthly",
    },
    {
      path: "/docs/guides/MCP_EXTENSIONS",
      title: "MCP Extensions (Claude, Mistral, Goose) — OCULTAR Docs",
      description:
        "Install and configure ocultar-claude-mcp, ocultar-mistral-mcp, and ocultar-goose-mcp.",
      ogType: "article",
      priority: "0.8",
      changefreq: "monthly",
    },
    {
      path: "/docs/guides/CONNECTORS_GUIDE",
      title: "Connectors Guide — OCULTAR Docs",
      description:
        "Connect Slack, SharePoint, REST APIs, and file pipelines through the OCULTAR Refinery.",
      ogType: "article",
      priority: "0.6",
      changefreq: "monthly",
    },
    {
      path: "/docs/guides/refinery_proxy_setup",
      title: "Refinery Proxy Setup — OCULTAR Docs",
      description:
        "Deploy the transparent fail-closed HTTP proxy in front of OpenAI, Anthropic, or internal LLMs.",
      ogType: "article",
      priority: "0.7",
      changefreq: "monthly",
    },
    {
      path: "/docs/guides/SOMBRA_GUIDE",
      title: "Sombra Gateway Guide — OCULTAR Docs",
      description:
        "Run the OpenAI-compatible Sombra gateway with SSE streaming, JWT auth, and multi-provider routing.",
      ogType: "article",
      priority: "0.7",
      changefreq: "monthly",
    },
    {
      path: "/docs/guides/TESTING_GUIDE",
      title: "Testing Guide — OCULTAR Docs",
      description:
        "Run unit, fail-closed, SSRF, and smoke test suites for OCULTAR.",
      ogType: "article",
      priority: "0.5",
      changefreq: "monthly",
    },
    {
      path: "/docs/guides/RELEASE_GUIDE",
      title: "Release Guide — OCULTAR Docs",
      description:
        "Build and release OCULTAR binaries, Docker images, and PyPI MCP packages.",
      ogType: "article",
      priority: "0.4",
      changefreq: "monthly",
    },
    {
      path: "/docs/api/API_REFERENCE",
      title: "API Reference — OCULTAR Docs",
      description:
        "HTTP endpoints for /api/health, /api/refine, /api/refine/file, /api/reveal, and /api/entities.",
      ogType: "article",
      priority: "0.7",
      changefreq: "monthly",
    },
    {
      path: "/docs/reference/ARCHITECTURE",
      title: "Architecture Reference — OCULTAR Docs",
      description:
        "Pipeline tiers (0.1 through 3), DuckDB/PostgreSQL vault encryption, and fail-closed proxy design.",
      ogType: "article",
      priority: "0.7",
      changefreq: "monthly",
    },
    {
      path: "/docs/reference/PII_DETECTION",
      title: "PII Detection Reference — OCULTAR Docs",
      description:
        "Supported PII entity types, checksum validators, and regional formats in OCULTAR.",
      ogType: "article",
      priority: "0.7",
      changefreq: "monthly",
    },
    {
      path: "/docs/reference/FAQ",
      title: "FAQ — OCULTAR Docs",
      description:
        "Frequently asked technical questions about OCULTAR's zero-egress architecture, vault, and licensing.",
      ogType: "article",
      priority: "0.6",
      changefreq: "monthly",
    },
    {
      path: "/docs/reference/PRODUCT_CONTEXT",
      title: "Product Context — OCULTAR Docs",
      description:
        "Overview of OCULTAR components: Refinery, Proxy, Sombra Gateway, and MCP extensions.",
      ogType: "article",
      priority: "0.5",
      changefreq: "monthly",
    },
    {
      path: "/docs/reference/EU_SOVEREIGN_PACK_V1",
      title: "EU Sovereign Pack v1 — OCULTAR Docs",
      description:
        "European identifier rulesets and sovereign deployment defaults in OCULTAR.",
      ogType: "article",
      priority: "0.6",
      changefreq: "monthly",
    },
    {
      path: "/docs/reference/ONBOARDING_GUIDE",
      title: "Onboarding Guide — OCULTAR Docs",
      description:
        "Onboarding checklist for teams deploying OCULTAR into staging and production.",
      ogType: "article",
      priority: "0.5",
      changefreq: "monthly",
    },
    {
      path: "/docs/compliance/GDPR_PRIVACY_BY_DESIGN",
      title: "GDPR Article 25 Privacy by Design — OCULTAR Docs",
      description:
        "Architecture-to-requirement mapping of OCULTAR against GDPR Article 25 and EU AI Act controls.",
      ogType: "article",
      priority: "0.7",
      changefreq: "monthly",
    },
    {
      path: "/docs/compliance/GDPR_FRENCH_FINANCE",
      title: "GDPR French Finance (DPO) — OCULTAR Docs",
      description:
        "DPO reference for deploying OCULTAR in regulated French financial workflows under RGPD and CNIL guidance.",
      ogType: "article",
      priority: "0.6",
      changefreq: "monthly",
    },
    {
      path: "/docs/security/SECURITY_MODEL",
      title: "Security Model — OCULTAR Docs",
      description:
        "Threat model, AES-256-GCM vault cryptography, SSRF defenses, and Ed25519 audit chain verification.",
      ogType: "article",
      priority: "0.7",
      changefreq: "monthly",
    },
    {
      path: "/docs/blog/going-fully-open-source",
      title: "OCULTAR Is Fully Open Source — OCULTAR Blog",
      description:
        "Why OCULTAR moved to AGPLv3 open source for its full PII redaction stack.",
      ogType: "article",
      priority: "0.6",
      changefreq: "yearly",
    },
    {
      path: "/docs/blog/zero-egress-supply-chain",
      title: "Zero-Egress Supply Chain — OCULTAR Blog",
      description:
        "Eliminating outbound telemetry and third-party data leaks in AI developer tooling.",
      ogType: "article",
      priority: "0.6",
      changefreq: "yearly",
    },
  ],
};
