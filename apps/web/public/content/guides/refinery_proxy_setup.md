# OCULTAR Proxy — 1-Click Deployment Guide

Deploying the OCULTAR Proxy is designed to be frictionless, taking less than 10 minutes to secure your AI pipeline.

## Prerequisites

- **OS**: Linux, macOS, or Windows (with WSL 2)
- **Docker**: Docker Refinery + Compose plugin (or Docker Desktop on Windows/macOS)
- No host-level Go, Python, or ML libraries required — everything runs in containers

---

## Phase 1: Environment Setup (2 Minutes)

**1. Unpack the archive**

If you received a distribution tarball:
```bash
# Create and enter the directory FIRST to avoid collisions
mkdir ocultar
cd ocultar
tar -xzf ../ocultar-*.tar.gz
```

If deploying from source:
```bash
git clone https://github.com/ocultar-dev/ocultar.git
cd ocultar
```

**2. Create your environment file**

```bash
cp .env.example .env
```

**3. Configure your secrets**

Open `.env` and set the following values:

```bash
# REQUIRED — AES-256 master encryption key (min 32 chars).
# ⚠️  Never change this after first run: it invalidates all vault entries.
export OCU_MASTER_KEY=$(openssl rand -hex 32)

# REQUIRED — Per-deployment HKDF salt.
# ⚠️  Also invalidates vault if changed after first run.
export OCU_SALT=$(openssl rand -hex 16)

# Your upstream LLM API URL (the service you want to protect).
# Examples: https://api.openai.com  |  http://your-internal-llm:11434
# NOTE: docker-compose.yml and docker-compose.proxy.yml both hardcode
# OCU_PROXY_TARGET to the bundled echo-upstream mock (they ignore this .env
# value) — that's intentional for the zero-config demo/smoke-test path. To
# point at a real upstream, override the ocultar-proxy service's environment
# in your own compose file, or run the built binary/container directly with
# this variable set.
export OCU_PROXY_TARGET=https://api.openai.com

# OPTIONAL — Proxy listener port (default: 8081).
export OCU_PROXY_PORT=8081

```

> A one-liner that fills in the keys automatically:
> ```bash
> sed -i "s/replace-with-a-secure-32-byte-key/$(openssl rand -hex 32)/" .env
> sed -i "s/ocultar-kdf-salt-placeholder/$(openssl rand -hex 16)/" .env
> ```
> (On macOS replace `-i` with `-i ''`)

---

## Phase 2: Start the Cluster (4–5 Minutes)

```bash
docker compose up -d --build
```

This starts `echo-upstream` and `ocultar-proxy` — Tier 1 (regex) detection only, no AI model download. To add Tier 2 AI NER:

```bash
# piiranha sidecar (openai/privacy-filter, downloads ~500 MB on first run)
OCU_PILOT_MODE=0 docker compose --profile ai up -d --build

# OR local Qwen1.5-1.8B via llama.cpp (no download; needs
# models/qwen-1.5b-q4_k_m.gguf already present)
docker compose --profile qwen up -d --build
```

Watch startup progress:
```bash
docker compose logs -f
```

Expected final output:
```
ocultar-proxy  | [+] All pre-flight checks passed! Starting Proxy.
ocultar-proxy  | [INFO] OCULTAR proxy listening on :8081
```

> **Subsequent starts are instant** — the model is cached in the `slm_data` Docker volume.

---

## Phase 3: Validation (1 Minute)

```bash
bash tools/scripts/scripts/smoke_test.sh
```

Expected output:
```
[+] Proxy is healthy!
[*] Running smoke test with leaky payload...
[+] SUCCESS: PII successfully intercepted and redacted!
```

**Your applications should now point to `http://localhost:${OCU_PROXY_PORT:-8081}`** instead of the upstream API directly. All JSON payloads will be automatically scrubbed before forwarding.

---

## What Each Port Does

| Port | Service | Purpose |
|---|---|---|
| `${OCU_PROXY_PORT:-8081}` (host) | `ocultar-proxy` | Transparent PII proxy — point your app here |
| `9090` (host) | `ocultar-proxy` | Prometheus metrics |
| `8086` (`--profile ai` only) | `privacy-filter` | Tier 2 AI NER sidecar (piiranha) |
| `8080` (`--profile qwen` only) | `llama-qwen` | Tier 2 AI NER sidecar (local Qwen1.5-1.8B) |

> The Dashboard (`/index.html`) is part of the **standalone binary** deployment, not the proxy stack. See [`ENTERPRISE_SETUP_GUIDE.md`](./ENTERPRISE_SETUP_GUIDE.md) for the full feature walkthrough.

---

## Shut Down

```bash
docker compose down
```

Vault data is persisted in the `vault-data` Docker volume and survives restarts.

To wipe everything (including the vault):
```bash
docker compose down -v
```
---

## Maintenance & Integrity

### Key Rotation Strategy
The `OCU_MASTER_KEY` and `OCU_SALT` are the root of your Sovereign Vault's security.
* **⚠️ WARNING**: Changing either of these values after data has been vaulted will invalidate all existing tokens.
* **Fallback Behavior**: In the event of a key rotation, OCULTAR will not crash but will return **un-hydrated tokens** (e.g., `[PERSON_a1b2c3d4e5f6a7b8]`) instead of original PII. This prevents data leaks while alerting you to the key mismatch.
* **Best Practice**: Backup your `.env` file securely. If you must rotate keys, you should clear the vault (`rm vault.db`) to ensure fresh deterministic mappings.
### Encryption & Security Protocol
All Sovereign Data Objects (SDOs) are protected using industry-standard authenticated encryption:
1. **Key Derivation**: On boot, the `OCU_MASTER_KEY` and `OCU_SALT` are passed through **HKDF-SHA256** to derive the 32-byte AES refinery key. 
2. **Deterministic Hashing**: PII is hashed using **HMAC-SHA256** to create a unique vault index.
3. **Authenticated Encryption**: SDOs are encrypted using **AES-256-GCM**. Each record includes a unique cryptographically secure random nonce, ensuring that the same PII encrypted twice produces different ciphertexts.
4. **Data-at-Rest**: Only the hashed index and the GCM-protected ciphertext are stored in the vault. **Raw PII never touches the disk.**
