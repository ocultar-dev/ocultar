# OCULTAR | Developer Guide

> **Audience:** Go contributors, security researchers, and integrators embedding OCULTAR as a library.

---

## Table of Contents

1. [Module Structure](#1-module-structure)
2. [Development Environment](#2-development-environment)
3. [Running Tests](#3-running-tests)
4. [Package Overview](#4-package-overview)
5. [Extending the Refinery](#5-extending-the-refinery)
   - [Adding a Detection Rule (no recompile)](#51-adding-a-detection-rule-no-recompile)
   - [Adding a New Detection Tier (code)](#52-adding-a-new-detection-tier-code)
   - [Adding a Vault Backend](#53-adding-a-vault-backend)
6. [Embedding OCULTAR as a Go Library](#6-embedding-ocultar-as-a-go-library)
7. [Coding Conventions](#7-coding-conventions)
8. [Agentic Governance & Orchestration](#8-agentic-governance--orchestration)
9. [SLM AI Relay (Ollama Proxy)](#9-slm-ai-relay-ollama-proxy)
10. [CI / PR Checklist](#10-ci--pr-checklist)

---

## 1. Module Structure

OCULTAR uses a **Go Workspace** (`go.work`) — there is no root `go.mod`; each module below is independently versioned and listed in `go.work`'s `use ()` block:

```
ocultar/
├── go.work                              ← workspace definition (no root go.mod)
│
├── apps/
│   ├── proxy/            main.go        ← reverse-proxy entrypoint (module: ocultar-proxy)
│   ├── slm-engine/       main.go        ← local SLM sidecar (module: .../apps/slm-engine)
│   │                     pkg/inference/
│   └── sombra/           main.go        ← agentic LLM gateway (module: .../apps/sombra)
│
├── services/
│   ├── refinery/                        ← core detection/redaction engine (module: github.com/ocultar-dev/ocultar)
│   │   ├── cmd/main/       main.go      ← Refinery CLI + `--serve` HTTP/dashboard server
│   │   ├── cmd/riskreport/ main.go      ← standalone report generator
│   │   ├── internal/handlers/           ← HTTP route registration
│   │   └── pkg/
│   │       ├── config/    config.go     ← settings, regex/dict rules, fail-closed startup
│   │       ├── refinery/  refinery.go   ← core redaction pipeline (RefineString)
│   │       │             phone_parser.go, address_parser.go
│   │       ├── proxy/     proxy_handler.go ← reverse-proxy handler used by apps/proxy
│   │       ├── audit/                   ← immutable (Ed25519 hash-chained) + basic audit loggers
│   │       ├── inference/               ← RemoteScanner (Tier 2 SLM sidecar client)
│   │       ├── connector/               ← Slack, SharePoint connectors
│   │       ├── reporter/                ← HTML risk-report generation
│   │       └── license/                 ← license stub (all features always enabled)
│   └── vault/                           ← encrypted token storage (module: .../ocultar/vault)
│       vault.go        ← Provider interface + factory (New)
│       duckdb_provider.go, postgres_provider.go, entity_registry.go, retention.go
│
├── internal/pii/                        ← shared PII type registry (module: .../internal/pii)
├── pkg/gateway/                         ← shared gateway client logic (module: .../pkg/gateway)
│
├── configs/
│   ├── config.yaml                      ← Runtime config (regexes, dicts, vault)
│   └── protected_entities.json          ← Tier 0 Dictionary Shield terms (required at startup)
│
└── scripts/                             ← dev/benchmark/release scripts, e.g. orchestrate.sh
```

There is no top-level `pkg/config`, `pkg/refinery`, `pkg/vault`, `pkg/proxy`, `cmd/`, or `documentation/` directory — those paths were stale references to a pre-workspace layout.

### `go.work` contents

```go
go 1.25.8

use (
    ./apps/proxy
    ./apps/sombra
    ./apps/slm-engine
    ./services/refinery
    ./services/vault
    ./internal/pii
    ./pkg/gateway
)
```

---

## 2. Development Environment

### Prerequisites

| Tool | Version | Notes |
|---|---|---|
| **Go** | 1.25+ | `go version` to verify — `go.work` pins the workspace toolchain |
| **GCC / CGO** | Any modern | Required — DuckDB uses CGO. `gcc --version` to verify. |
| **Docker + Compose** | Latest | Only needed to run the proxy or full stack. |
| **Python** | 3.9+ | Optional — only for the audit/analysis scripts in the root. |

### Built-in Regex Rules

| Type | Pattern | Description |
|---|---|---|
| `EMAIL` | `(?i)[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}` | Standard email addresses |
| `URL` | `(?i)https?://[^\s/$.?#].[^\s]*|www\.[^\s/$.?#].[^\s]*` | HTTP/S URLs and bare `www.` domains |
| `SSN` | `\b\d{3}-\d{2}-\d{4}\b` | US Social Security Numbers |
| `CREDENTIAL` | `(?i)\bpassword\s*[:=]\s*[^\s,]+` | In-line passwords |
| `SECRET` | `(?i)\b(?:secret|key|token|api_key|auth_token|access_token|client_secret|private_key|refresh_token)\b\s*[:=]\s*[^\s,]+` | In-line secrets/keys |

### Clone and Build

```bash
# Clone — Sombra is part of this monorepo (apps/sombra), no separate clone needed
git clone https://github.com/ocultar-dev/ocultar.git
cd ocultar

# Verify the workspace (requires CGO — see prerequisites above)
CGO_ENABLED=1 go build ./...
```

### First-Run Requirements

OCULTAR's startup is **fail-closed** — it will immediately abort if the Dictionary Shield file is missing:

```bash
# configs/protected_entities.json must exist and be non-empty before running
# The file is a simple JSON array of strings:
cat configs/protected_entities.json
# Example: ["internal-term-1", "internal-term-2"]
```

For development you can use the placeholder array already in the repository.

### Environment Setup

```bash
# Minimum required variable
export OCU_MASTER_KEY="dev-only-key-do-not-use-in-production"

# For proxy development
export OCU_PROXY_TARGET="http://localhost:11434"  # e.g. a local Ollama instance
export OCU_PROXY_PORT="8080"

# Optional: SLM sidecar for Tier 2 NER
export SLM_SIDECAR_URL="http://localhost:8085"
```

---

## 3. Running Tests

Each module is tested from its own directory (this is a multi-module workspace, not a single `go test ./...` from root):

```bash
# Run the core suite (requires CGO_ENABLED=1 + a writable tmp dir for DuckDB)
make test    # == CGO_ENABLED=1 go test ./internal/pii/... ./services/refinery/... ./services/vault/...

# Run a specific package/module
cd services/refinery && CGO_ENABLED=1 go test ./pkg/refinery/...
cd services/refinery && CGO_ENABLED=1 go test ./pkg/proxy/...
cd services/vault && CGO_ENABLED=1 go test ./...
cd apps/proxy && CGO_ENABLED=1 go test ./...

# Run with race detector (recommended for proxy/refinery concurrency tests)
cd services/refinery && CGO_ENABLED=1 go test -race ./...

# Run fail-closed proxy tests specifically
cd services/refinery && CGO_ENABLED=1 go test -v ./pkg/proxy/ -run TestFailClosed

# Run a single test by name
cd services/refinery && CGO_ENABLED=1 go test ./... -run TestName
```

### Key Test Files

| File | What it covers |
|---|---|
| `services/refinery/pkg/refinery/refinery_test.go` | Core redaction correctness (email, phone, IBAN, address, base64, nested JSON) |
| `services/refinery/pkg/refinery/phone_parser_test.go` | International phone number parsing edge cases |
| `services/refinery/pkg/refinery/address_parser_test.go` | European/LATAM address heuristics |
| `services/refinery/pkg/proxy/proxy_test.go` | End-to-end proxy redaction with a mock upstream |
| `services/refinery/pkg/proxy/fail_closed_test.go` | Ensures refinery errors return 4xx/5xx and never forward un-redacted data |
| `services/vault/vault_test.go` | StoreToken idempotency, GetToken lookup, CountAll |

---

## 4. Package Overview

```
services/refinery/pkg/config   ──► loaded once at startup ──► pkg/refinery reads config.Global
                                                          ──► services/vault reads cfg.VaultBackend
services/vault                 ──► Provider interface ──► duckdbProvider (default)
                                                     ──► postgresProvider
services/refinery/pkg/refinery ──► RefineString / ProcessInterface
                                ──► depends on: pkg/config, services/vault, pkg/license
                                ──► optional: AuditLogger, AIScanner (injected post-construction)
services/refinery/pkg/proxy    ──► http.Handler wrapping pkg/refinery
                                ──► depends on: pkg/refinery, services/vault
apps/proxy                     ──► thin entrypoint that constructs and runs services/refinery/pkg/proxy's Handler
```

**Dependency rules (enforced by import graph):**
- `pkg/config` has **zero** internal dependencies (it only uses stdlib).
- `services/vault` depends on `pkg/config` and `pkg/license` — never on `pkg/refinery`.
- `pkg/refinery` depends on `pkg/config`, `services/vault`, `pkg/license` — never on `pkg/proxy`.
- `pkg/proxy` sits at the top of the refinery stack; it may import `pkg/refinery` and `services/vault`. `apps/proxy` (the deployable binary) sits above that again.

---

## 5. Extending the Refinery

### 5.1 Adding a Detection Rule (no recompile)

The fastest path — no Go code required. Edit `configs/config.yaml`:

```yaml
# Custom regex rule
regexes:
  - type: PASSPORT_NUMBER
    pattern: '\b[A-Z]{1,2}[0-9]{6,9}\b'

# Custom dictionary rule (Tier 0 — exact match)
dictionaries:
  - type: SECRET_PROJECT
    terms:
      - "Project Nightshade"
      - "Operation Dusk"
```

Restart the binary. Changes take effect at next startup (no recompilation).

---

### 5.2 Adding a New Detection Tier (code)

To add a new Tier inside `RefineString` in `services/refinery/pkg/refinery/refinery.go`:

1. **Write a parser function** following the existing pattern:
   ```go
   // ParseAndReplaceMyPII returns match index pairs (start, end) for each detected entity.
   func ParseAndReplaceMyPII(input string) [][]int { ... }
   ```

2. **Call `parseAndReplaceWithErr`** in `RefineString` at the appropriate tier position:
   ```go
   // TIER X: My Custom Shield
   refined, err = parseAndReplaceWithErr(refined, ParseAndReplaceMyPII, func(match string) (string, error) {
       return e.getOrSetSecureToken(match, "MY_TYPE", actor)
   })
   if err != nil {
       return "", err
   }
   ```

3. **Add tests** in `pkg/refinery/refinery_test.go` following the pattern in `TestRefineString`.

> **Important:** Never return partial output when `err != nil`. The caller always discards output on error.

---

### 5.3 Adding a Vault Backend

1. Create `services/vault/my_provider.go` implementing the `Provider` interface:
   ```go
   type myProvider struct { /* ... */ }

   func (p *myProvider) StoreToken(hash, token, encryptedPII string) (bool, error) { ... }
   func (p *myProvider) GetToken(hash string) (string, bool)                        { ... }
   func (p *myProvider) CountAll() int64                                             { ... }
   func (p *myProvider) Close() error                                                { ... }
   ```

2. Add a new `case` to the `New()` factory in `services/vault/vault.go`:
   ```go
   case "mybackend":
       return newMyProvider(cfg.MyDSN)
   ```

3. Add a `MyDSN string \`yaml:"my_dsn"\`` field to `config.Settings`.

4. Add test coverage in `services/vault/vault_test.go`.

---

## 6. Embedding OCULTAR as a Go Library

You can import the refinery directly into your own Go service:

```go
import (
    "github.com/ocultar-dev/ocultar/pkg/config"
    "github.com/ocultar-dev/ocultar/pkg/refinery"
    "github.com/ocultar-dev/ocultar/vault"
    "crypto/sha256"
)

func main() {
    // 1. Load configuration (fail-closed: will fatal if protected_entities.json is missing)
    config.Load()

    // 2. Derive the 32-byte AES key
    rawKey := []byte("my-secret-master-key")
    hash := sha256.Sum256(rawKey)
    masterKey := hash[:]

    // 3. Open the vault
    v, err := vault.New(config.Global, "vault.db")
    if err != nil {
        panic(err)
    }
    defer v.Close()

    // 4. Construct the refinery
    eng, err := refinery.NewRefinery(v, masterKey)
    if err != nil {
        panic(err)
    }

    // 5. Refine a string
    refined, err := eng.RefineString("Call me at john@example.com", "system", nil)
    if err != nil {
        panic(err)
    }
    // refined → "Call me at [EMAIL_9c8f7a1b2d3e4f50]"
    fmt.Println(refined)
}
```

Note: `github.com/ocultar-dev/ocultar/pkg/config` and `.../pkg/refinery` resolve inside the `services/refinery` module, whose module path is `github.com/ocultar-dev/ocultar`. The vault is a **separate module** (`services/vault`, module path `github.com/ocultar-dev/ocultar/vault` — no `/pkg/` segment).

> Add `github.com/ocultar-dev/ocultar` to your `go.mod`:
> ```bash
> go get github.com/ocultar-dev/ocultar@latest
> ```

---

## 7. Coding Conventions

| Area | Convention |
|---|---|
| **Error handling** | Always return errors; never panic in `pkg/` packages (only `main` and config loading may `log.Fatal`). |
| **Fail-closed** | Refinery errors must **block** processing — never forward partially-processed data. |
| **Thread safety** | All shared state (e.g. `Hits` map) must be protected by a mutex. Use `atomic.Int64` for counters. |
| **No side effects in tests** | Tests must not rely on disk state. Use `:memory:` for vault and `config.InitDefaults()`. |
| **Token format** | `[TYPE_16HEXCHARS]` where `16HEXCHARS` is the first 16 hex characters of the HMAC-SHA256 of the original PII. Never change this format — it breaks existing vaults. |
| **Logging** | Use `log/slog` for structured JSON logging. `slog.SetDefault` is initialized at startup. |
| **Imports** | `stdlib` → `external` → `internal` (the standard Go import grouping). |

---

## 8. Agentic Governance & Orchestration

Ocultar development is supported by a 16-step **Continuous AI Orchestrator**. This system ensures that all code changes follow security and compliance best practices.

### Key Skills for Developers
- **`refinery-rule-generator`**: Use this when adding new PII detection rules. It automates the Go regex and config generation.
- **`sombra-gateway-policy-enforcer`**: An architectural linter. If you add new routes to Sombra, this skill will verify they are "Fail-Closed".
- **`change-impact-visualizer`**: Run this before opening a PR to generate a compliance impact summary for the reviewers.
- **`red-team-evasion-scanner`**: Proactively stress-tests your changes against obfuscation bypasses (Base64, URL-encoding).

### How to trigger
Skills are triggered automatically by the agent during the task lifecycle. Developers can also run the full suite manually via the orchestration pipeline:

```bash
./scripts/orchestrate.sh
```

### Security Gates
The orchestrator executes the following functional gates located in `tools/scripts/`:
- **`run_secret_scanner.sh`**: Scans for hardcoded API keys, tokens, and credentials using high-entropy regex pathfinding.
- **`run_arch_linter.sh`**: Enforces strict package boundaries (e.g., preventing `gateway` from directly importing `internal/pii`).
- **`run_zero_egress_validator.sh`**: Validates configuration manifests to ensure `ALLOW_ALL` policies are never committed.

---

## 9. Tier 2 NER Adapter Selection

OCULTAR routes Tier 2 AI NER scans to a sidecar over HTTP (`SLM_SIDECAR_URL`). The protocol used to talk to that sidecar is selected via `SLM_ADAPTER` (`TIER2_ENGINE` is a deprecated alias — still read, but logs a `[DEPRECATED]` warning).

### Architecture

```
Refinery → RemoteScanner (HTTP, services/refinery/pkg/inference/remote.go) → sidecar → inference backend
```

The refinery's `RemoteScanner` is decoupled from the inference backend behind the sidecar's HTTP contract.

### Supported Adapters (`SLM_ADAPTER`)

| Value | Sidecar | What runs the model | Model source |
|---|---|---|---|
| `privacy-filter` (default, or unset) | `apps/slm-engine` (Go, no CGO) → separate Python HF service | `openai/privacy-filter` token classifier via HuggingFace Transformers (Python, `scripts/serve_privacy_filter.py`) | `PRIVACY_FILTER_MODEL_PATH` |
| `openai-chat` | none — refinery talks directly to an OpenAI-chat-compatible endpoint | e.g. the `llama-qwen` service in `docker-compose.yml` (`ghcr.io/ggml-org/llama.cpp:server`, prebuilt image — not a Go CGO build) | model baked into that server's own image/args |

`apps/slm-engine` itself only implements `privacy-filter` — passing any other `SLM_ADAPTER` to it fails startup with `unsupported SLM_ADAPTER`. `openai-chat` is handled by the refinery/proxy directly against `SLM_SIDECAR_URL`, bypassing `apps/slm-engine` entirely.

### Adapter: privacy-filter (default)

`openai/privacy-filter` is a bidirectional token classifier (Apache 2.0, ~1.5B params). Because it is Python-native (HuggingFace Transformers), run it as a separate Python service using the provided Dockerfile or script, and point the sidecar at it:

```bash
# To run locally via script:
pip install -r apps/slm-engine/python/requirements.txt
export PRIVACY_FILTER_MODEL_PATH=openai/privacy-filter # Or a local fine-tuned model path
python scripts/serve_privacy_filter.py   # listens on :8086

# Start the Go sidecar pointing at it (SLM_ADAPTER=privacy-filter is the default, shown here for clarity)
export SLM_ADAPTER=privacy-filter
export PYTHON_SIDECAR_URL=http://localhost:8086
go run ./apps/slm-engine
```

No CGO required for the Go sidecar in this mode. The Python service must expose the same HTTP contract:
- `POST /scan  {"text":"..."}  →  {"PERSON":["John"],"EMAIL":["j@x.com"]}`
- `GET  /health               →  {"status":"ok"}`

---

## 10. CI / PR Checklist

Before opening a PR:

This is a Go workspace (no root `go.mod`) — `go test`/`go build`/`go vet` with a bare `./...` fail from repo root (`directory prefix . does not contain modules listed in go.work`). Run them per-module, or via `make`:

```bash
# 1. All tests pass (make test covers internal/pii, services/refinery, services/vault)
make test
cd services/refinery && CGO_ENABLED=1 go test -race ./...
cd apps/proxy && CGO_ENABLED=1 go test -race ./...

# 2. No compilation errors
make build

# 3. Vet clean (per module)
cd services/refinery && go vet ./...
cd apps/proxy && go vet ./...

# 4. Doc links valid
bash tools/scripts/scripts/check_docs.sh

# 5. Smoke test passes (requires Docker)
docker compose -f docker-compose.proxy.yml up -d
bash tools/scripts/scripts/smoke_test.sh
docker compose -f docker-compose.proxy.yml down
```

**PR requirements:**
- New detection tiers must include at least 3 test cases (true positive, false positive, boundary).
- New vault backends must implement all 4 `Provider` methods with test coverage.
- Changes to `RefineString`'s tier order must include a comment explaining the security rationale.
- Secrets must never appear in test fixtures — use placeholder-only test strings.
