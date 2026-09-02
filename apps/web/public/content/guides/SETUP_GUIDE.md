# OCULTAR | Setup & Usage Guide

This guide covers how to run OCULTAR locally, package it for a client, and use the dashboard.

---

## Quick Start (5 minutes)

The fastest path to a working demo. You only need [Docker](https://docs.docker.com/get-docker/) and Docker Compose v2 — no secrets manager, no Go toolchain.

```bash
git clone https://github.com/ocultar-dev/ocultar
cd ocultar
docker compose up --build
```

First build takes ~5 minutes (compiling Go + DuckDB/CGO). Every subsequent start is instant.

**What's running:**

| Service | URL | What it does |
|---------|-----|--------------|
| `ocultar-proxy` | http://localhost:8081 | PII detection + redaction proxy |
| `echo-upstream` | http://localhost:8082 | Mock AI API (reflects the request back) |
| Prometheus metrics | http://localhost:9090/metrics | Live tier detection counts and per-tier latency (`ocultar_refinery_detections_total`, `ocultar_refinery_tier_duration_seconds`) |

**Test it:**

```bash
curl -s http://localhost:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o","messages":[{"role":"user","content":"My email is alice@example.com and SSN is 123-45-6789"}]}'
```

The echo response shows `[EMAIL_...]` and `[SSN_...]` tokens — the originals never reached the upstream.

**Add AI-powered Tier 2 NER** (downloads ~500 MB model on first run):

```bash
docker compose --profile ai up --build
```

**Use real API keys:**

```bash
cp .env.example .env   # edit with your keys and upstream URL
docker compose up --build
```

> **Security note:** The default demo keys in `.env.example` are intentionally insecure. Always set `OCU_MASTER_KEY` and `OCU_SALT` to high-entropy values before connecting real data.

---

## Part 1: For the Sender (Packaging for a Client)

**Your Goal:** Package OCULTAR for a client deployment.

### Step 1 — Build the release

Open a terminal in the `ocultar/` folder and run:

```bash
./tools/scripts/release.sh
```

This builds `apps/proxy`, `services/refinery/cmd` (the `--serve` dashboard/API binary), `services/refinery/cmd/riskreport`, `apps/sombra`, and `apps/slm-engine`, copies `configs/*.yaml` and `.env.example`, and tars the result into `ocultar-release-v<date>.tar.gz` in the repo root.

Alternatively, GitHub Actions' `release.yml` builds cross-platform binaries automatically on any `v*.*.*` tag push and attaches them to the GitHub Release.

### Step 2 — Send the package

Send the archive (or the GitHub Release binaries) to your client, along with this guide.

---

## Part 2: For the Receiver (Running OCULTAR)

**Your Goal:** Start OCULTAR on your computer (Windows, Mac, or Linux) and test it.

The easiest path is Docker — see **Quick Start** at the top of this guide (`docker compose up --build`), which requires no Go toolchain and no manual key setup beyond `.env`.

To run a downloaded binary directly instead:

### Prerequisites

- The `ocultar` (or `ocultar-<os>-<arch>`) binary for your platform, from the release archive.
- `configs/protected_entities.json` and `configs/config.yaml` alongside the binary (fail-closed startup aborts if the dictionary file is missing).
- `OCU_MASTER_KEY` and `OCU_SALT` set to real values — see the Security note in Quick Start above.

### Step 1 — Extract the archive

Extract the distribution archive and open the extracted folder in a terminal.

### Step 2 — Set your keys and start the dashboard/API server

```bash
export OCU_MASTER_KEY="$(openssl rand -hex 32)"
export OCU_SALT="$(openssl rand -hex 16)"
./ocultar --serve 3030
```

> **First run note:** If Tier 2 AI NER is enabled, the sidecar pulls its model on first run — see the Tier 2 sections above. The `--serve` binary itself starts instantly.

### Step 3 — Open the Dashboard

Once the server is running, open your web browser and go to:

👉 **http://localhost:3030/index.html**

You should see the OCULTAR Live Dashboard with the input panel on the left.

---

## Part 3: Using the Dashboard

### The Interface

| Area | What it does |
|---|---|
| **Sidebar** | Navigate between Live Refinery, Identity Vault (future), Audit Logs (future) |
| **System Status** (bottom-left) | Shows Lead Shield (regex refinery) status and refinery version |
| **Metrics Bar** | Live count of entities scrubbed, Privacy ROI, Vault reuse rate, Deep Scan health |

### The Refinery Controls

- **Lead Shield (Regex)** — Toggle for structural PII detection (emails, phones, IBANs, addresses, URLs).
- **Deep AI Scrub (SLM)** — Toggle for contextual NER (names in prose, company names). Requires local AI to be running.
- **Operational Controls** — Direct access to Regex Enforcement, Identity Dictionaries, and Risk Compliance data.

### Step-by-Step: Run a Test

1. In the **Raw Input (Liability)** box on the left, paste any text containing personal data. Example:

   ```
   From: sarah.connor@cyberdyne.com
   Tel: +33 6 12 34 56 78
   IBAN: DE89370400440532013000
   Regards, Sarah Connor
   ```

2. Click **Execute Redaction**.

3. The **Clean Asset** panel on the right will instantly show redacted output:

   ```
   From: [EMAIL_a1b2c3d4e5f6a7b8]
   Tel: [PHONE_9f8e7d6c3a2b1c90]
   IBAN: [IBAN_12ab34cd56ef7809]
   Regards, [PERSON_5e4f3a2b6c7d8e90]
   ```

4. The **Detection Attribution** panel appears below the output. For each entity found it shows:

   | Column | What it means |
   |--------|--------------|
   | Entity type | `EMAIL`, `PHONE`, `SSN`, `PERSON`, etc. |
   | Tier · Method | Which pipeline tier caught it and how (e.g. `Tier 1 · Rule Engine`, `Tier 1.5 · Greeting Shield`, `Tier 2 · AI NER`) |
   | `@offset` | Character position in the original text |
   | Confidence | Detection certainty (100% for rule-based, lower for contextual) |

   Example output for the text above:

   | Entity | Tier | Method | Location | Confidence |
   |--------|------|--------|----------|------------|
   | EMAIL | Tier 1 | Rule Engine | @6-30 | 100% |
   | PHONE | Tier 1.1 | Phone Shield | @36-55 | 100% |
   | IBAN | Tier 1 | Rule Engine · Validated | @62-84 | 100% |
   | PERSON | Tier 1.5 | Greeting Shield | @94-106 | 100% |

5. The **Secure Audit Trail** (right panel) shows the live Ed25519-signed log of vault events.

### Testing with Large Files

To process a large `.csv` or `.json` file directly through the refinery (bypassing the browser):

```bash
curl -F "file=@my_large_data.csv" http://localhost:8081/api/refine/file > cleaned_data.csv
```

OCULTAR streams the cleaned output directly into `cleaned_data.csv`.

---

## Part 4: Shut Down

When finished, run from your terminal (inside the `ocultar` folder):

```bash
docker compose down
```

This safely stops all containers and frees resources.

---

## Troubleshooting

| Symptom | Fix |
|---|---|
| Browser shows nothing at `localhost:3030` | Confirm the `--serve 3030` process is still running and check its terminal output for a fatal startup error (e.g. missing `configs/protected_entities.json`). |
| Dashboard loads but refinement fails | If Tier 2 AI NER is enabled, check the sidecar's logs (`docker compose logs privacy-filter` for the Docker path). |

### SharePoint Connector Environment Variables

- `MS_TENANT_ID`: Microsoft Entra (Azure AD) Tenant ID.
- `MS_CLIENT_ID`: Application (client) ID.
- `MS_CLIENT_SECRET`: Client secret.
- `MS_SHAREPOINT_SITE_ID`: (Optional) Target SharePoint site ID.
- `OCU_SALT`: (Required for Production) per-deployment salt string for HKDF key derivation — any high-entropy string works (`openssl rand -hex 16`), it does not have to be hex.
