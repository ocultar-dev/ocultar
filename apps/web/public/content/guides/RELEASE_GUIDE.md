# OCULTAR | Release & Distribution Guide

This guide explains how to build and distribute OCULTAR binaries or Docker images to clients.

---

## 1. Build the Release

**CI (tagged releases):** pushing a `v*.*.*` tag triggers `.github/workflows/release.yml`, which cross-compiles the refinery server and proxy for linux/amd64, darwin/arm64, and windows/amd64, generates a `SHA256SUMS-<os>-<arch>.txt` and a CycloneDX SBOM, and publishes everything as GitHub Release assets. It builds two raw binaries per platform (`ocultar-<os>-<arch>`, `ocultar-proxy-<os>-<arch>`) — no client-facing archive with a GUI installer.

**Local packaging:** `tools/scripts/release.sh` builds `apps/proxy`, `services/refinery/cmd`, `services/refinery/cmd/riskreport`, `apps/sombra`, and `apps/slm-engine`, copies `configs/*.yaml` and `.env.example`, and tars the result into `ocultar-release-v<date>.tar.gz`:

```bash
./tools/scripts/release.sh
```

Output in `dist/release_<date>/`, archived as `ocultar-release-v<date>.tar.gz` in the repo root.

> [!NOTE]
> OCULTAR runs all PII detection locally — local SLM-based NER, on-premise vault, zero internet required at runtime. The only external dependency is the one-time model download from HuggingFace on first run.
> Release artifacts (`dist/`, `*.tar.gz`) are **not tracked in version control** (`.gitignore`). Don't force-add them.

---

## 2. Distribution Options

### Option A: Docker (Recommended for All Clients)

Docker standardizes the environment: clients don't need Go, Python, or any dependency.

**What to send:**
1. This repo (or the release binaries from the GitHub Releases page)
2. The Setup Guide (`apps/web/public/content/guides/SETUP_GUIDE.md`, published at `/docs/guides/SETUP_GUIDE` on the docs site)

**Docker image tarball:**
```bash
docker build -t ocultar-api .
docker save ocultar-api > ocultar_v$(date +%Y%m%d).tar
```

The client loads it with:
```bash
docker load -i ocultar_v20260301.tar
docker compose up -d
```

*Works on Windows (WSL2), macOS, and Linux without modification.*

---

### Option B: Native Bare-Metal Binaries

For developers or security researchers who want to run the raw binary without Docker.

> **Note:** OCULTAR uses DuckDB (via CGO), so cross-compilation requires a C toolchain.

These are the same source paths `.github/workflows/release.yml` builds — the refinery server from `./services/refinery/cmd`, the proxy from `./apps/proxy`.

#### Linux (native, from a Linux machine)
```bash
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
  go build -o dist/ocultar-linux-amd64 ./services/refinery/cmd
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
  go build -o dist/ocultar-proxy-linux-amd64 ./apps/proxy
```

#### Windows (from Linux, using mingw-w64)
```bash
# Install cross-compiler first:
sudo apt-get install gcc-mingw-w64

# Build:
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc \
  go build -o dist/ocultar-windows-amd64.exe ./services/refinery/cmd
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc \
  go build -o dist/ocultar-proxy-windows-amd64.exe ./apps/proxy
```

#### macOS (Silicon / Intel)
Cross-compiling for macOS requires Apple's proprietary SDKs. Three practical options:

1. **Build on an actual Mac** — easiest:
   ```bash
   CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 \
     go build -o dist/ocultar-darwin-arm64 ./services/refinery/cmd
   CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 \
     go build -o dist/ocultar-proxy-darwin-arm64 ./apps/proxy
   ```
2. **GitHub Actions** — set up a macOS runner in CI/CD to produce the binary automatically
3. **Recommend Docker** — Docker Desktop runs natively on Mac, sidestepping this entirely

---

## 3. Go Workspace Structure

The project uses a `go.work` file to manage a multi-module workspace:

```
go.work
├── apps/proxy       (github.com/ocultar-dev/ocultar-proxy)        ← transparent proxy
├── apps/sombra      (github.com/ocultar-dev/ocultar/apps/sombra)  ← Sombra gateway
├── apps/slm-engine  (github.com/ocultar-dev/ocultar/apps/slm-engine)
├── services/refinery (github.com/ocultar-dev/ocultar)
├── services/vault    (github.com/ocultar-dev/ocultar/vault)
├── internal/pii
└── pkg/gateway
```

There is no root `go.mod`, so `go build ./...` from the repo root fails (`directory prefix . does not contain modules listed in go.work`). Build each module individually, or use `make build`.

---

## 4. Proxy Distribution

The `docker-compose.proxy.yml` file is the deployment unit for the proxy mode. Include it alongside the binary for clients who want transparent LLM API interception:

```bash
# Proxy cluster startup:
docker compose -f docker-compose.proxy.yml up -d
```

See the Setup Guide (`apps/web/public/content/guides/SETUP_GUIDE.md`) for the full proxy deployment guide.
