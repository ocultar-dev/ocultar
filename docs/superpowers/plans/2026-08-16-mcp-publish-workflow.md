# MCP Extensions PyPI Publish Workflow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a GitHub Actions workflow that builds and publishes all three MCP extension packages (`ocultar-claude-mcp`, `ocultar-goose-mcp`, `ocultar-mistral-mcp`) to PyPI via Trusted Publishing whenever a `mcp-v*.*.*` tag is pushed.

**Architecture:** A single workflow file, `.github/workflows/mcp-publish.yml`, with one job that fans out over a 3-entry build matrix (one entry per extension directory). Each matrix leg builds its own wheel/sdist with `python -m build` and publishes it independently via `pypa/gh-action-pypi-publish`, authenticating with OIDC (no stored PyPI token). `fail-fast: false` so one package's failure doesn't cancel the others mid-publish.

**Tech Stack:** GitHub Actions, Python 3.12, PyPA `build`, `pypa/gh-action-pypi-publish` (PyPI Trusted Publishing / OIDC).

## Global Constraints

- Trigger tag pattern is `mcp-v*.*.*` — must NOT overlap with the existing `v*.*.*` pattern used by `.github/workflows/release.yml` for Go binaries (per spec `docs/superpowers/specs/2026-08-16-mcp-pypi-publish-design.md`).
- All three packages are versioned in lockstep — the workflow builds and publishes all three matrix legs on every trigger, regardless of which package's code actually changed.
- No `PYPI_API_TOKEN` or any other PyPI secret is stored in the repo — publishing must use Trusted Publishing (OIDC), which requires top-level job `permissions: id-token: write`.
- `actions/checkout` is pinned to `v7.0.1` repo-wide as of this writing (every file under `.github/workflows/`) — match that pin for consistency; Dependabot (`.github/dependabot.yml`, `github-actions` ecosystem, directory `/`) already watches and bumps action versions repo-wide, so no new Dependabot config is needed for this file.
- This plan covers only the workflow file. It does NOT cover: requesting the `ocultar` PyPI Organization, transferring project ownership, or configuring the PyPI Trusted Publisher form for each project — those are manual pypi.org steps tracked in the spec's Part A and are hard prerequisites for the *first* tag push to succeed, but are not blocked by this plan and don't block it either.

---

## File Structure

- Create: `.github/workflows/mcp-publish.yml` — the entire deliverable. One file, one job, one 3-entry matrix.

No other files change. There is no application code, no tests directory, and no existing workflow file to modify.

---

### Task 1: Build matrix — checkout, Python setup, package build

**Files:**
- Create: `.github/workflows/mcp-publish.yml`

**Interfaces:**
- Produces: a workflow triggered on `push: tags: ["mcp-v*.*.*"]`, with a job `publish` that has a `strategy.matrix.include` list of three `{package, dir}` pairs:
  - `{package: ocultar-claude-mcp, dir: extensions/claude}`
  - `{package: ocultar-goose-mcp, dir: extensions/goose}`
  - `{package: ocultar-mistral-mcp, dir: extensions/mistral}`
  - Each matrix leg ends this task with a built wheel + sdist in `${{ matrix.dir }}/dist/`. Task 2 consumes `matrix.dir` and `matrix.package` to add the publish step.

- [ ] **Step 1: Create the workflow file with trigger, permissions, and matrix build steps**

Write `.github/workflows/mcp-publish.yml`:

```yaml
name: Publish MCP Extensions to PyPI

on:
  push:
    tags: ["mcp-v*.*.*"]

permissions:
  contents: read

jobs:
  publish:
    name: Publish ${{ matrix.package }}
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
      matrix:
        include:
          - package: ocultar-claude-mcp
            dir: extensions/claude
          - package: ocultar-goose-mcp
            dir: extensions/goose
          - package: ocultar-mistral-mcp
            dir: extensions/mistral
    steps:
      - name: Checkout repository
        uses: actions/checkout@v7.0.1

      - name: Set up Python
        uses: actions/setup-python@v7.0.0
        with:
          python-version: "3.12"

      - name: Install build tool
        run: python -m pip install --upgrade build

      - name: Build ${{ matrix.package }}
        working-directory: ${{ matrix.dir }}
        run: python -m build
```

- [ ] **Step 2: Validate the YAML parses**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/mcp-publish.yml'))" && echo VALID`
Expected: `VALID` printed, no exception.

- [ ] **Step 3: Confirm each extension actually builds with this exact command sequence**

Run for each of the three directories (using a scratch output dir so nothing pollutes the repo):

```bash
for dir in extensions/claude extensions/goose extensions/mistral; do
  echo "=== $dir ==="
  python3 -m build --outdir /tmp/mcp-plan-check/$(basename "$dir") "$dir"
done
```

Expected: all three print `Successfully built <name>-0.2.0.tar.gz and <name>-0.2.0-py3-none-any.whl` with no errors. `ocultar-claude-mcp` is already confirmed to build cleanly; this step additionally verifies `ocultar-goose-mcp` and `ocultar-mistral-mcp`.

Clean up afterward: `rm -rf /tmp/mcp-plan-check`

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/mcp-publish.yml
git commit -m "ci: add MCP extensions build matrix for PyPI publish workflow"
```

---

### Task 2: Publish step via PyPI Trusted Publishing

**Files:**
- Modify: `.github/workflows/mcp-publish.yml` (adds `id-token: write` permission and a publish step; everything from Task 1 stays unchanged)

**Interfaces:**
- Consumes: `matrix.dir` and `matrix.package` from Task 1's matrix definition — no new matrix entries, no renaming.
- Produces: a complete, mergeable workflow. Nothing downstream in this plan consumes this task's output — it's the final task.

- [ ] **Step 1: Add job-level `id-token: write` permission and the publish step**

Edit `.github/workflows/mcp-publish.yml`: add `id-token: write` to the `publish` job's permissions (job-level, alongside the existing top-level `contents: read`), and append the publish step after the build step:

```yaml
jobs:
  publish:
    name: Publish ${{ matrix.package }}
    runs-on: ubuntu-latest
    permissions:
      id-token: write
    strategy:
      fail-fast: false
      matrix:
        include:
          - package: ocultar-claude-mcp
            dir: extensions/claude
          - package: ocultar-goose-mcp
            dir: extensions/goose
          - package: ocultar-mistral-mcp
            dir: extensions/mistral
    steps:
      - name: Checkout repository
        uses: actions/checkout@v7.0.1

      - name: Set up Python
        uses: actions/setup-python@v7.0.0
        with:
          python-version: "3.12"

      - name: Install build tool
        run: python -m pip install --upgrade build

      - name: Build ${{ matrix.package }}
        working-directory: ${{ matrix.dir }}
        run: python -m build

      - name: Publish ${{ matrix.package }} to PyPI
        uses: pypa/gh-action-pypi-publish@v1.14.2
        with:
          packages-dir: ${{ matrix.dir }}/dist
```

(Job-level `permissions:` overrides the top-level default for this job only — this is standard GitHub Actions behavior, not a workaround.)

- [ ] **Step 2: Validate the YAML parses**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/mcp-publish.yml'))" && echo VALID`
Expected: `VALID` printed, no exception.

- [ ] **Step 3: Review the publish step's inputs against Task 1's matrix**

Confirm by inspection (no tool runs this, it's a manual read-through):
- `packages-dir: ${{ matrix.dir }}/dist` resolves to `extensions/claude/dist`, `extensions/goose/dist`, `extensions/mistral/dist` for the three legs respectively — matching where Task 1's `python -m build` (run with `working-directory: ${{ matrix.dir }}`) writes its output (`build`'s default `--outdir` is `<cwd>/dist`).
- No `password` or `repository-url` input is set on the publish step — Trusted Publishing needs neither; supplying a password input would make the action ignore OIDC and fail without a token.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/mcp-publish.yml
git commit -m "ci: publish MCP extensions to PyPI via trusted publishing"
```

---

## Post-Plan Prerequisites (not implementation tasks — tracked here so they aren't lost)

Before pushing the first `mcp-v0.2.0` tag, on pypi.org, for each of the three projects (`ocultar-claude-mcp`, `ocultar-goose-mcp`, `ocultar-mistral-mcp`):

*Settings → Publishing* → add Trusted Publisher — owner `ocultar-dev`, repo `ocultar`, workflow `mcp-publish.yml`.

Tagging before this is done will fail the workflow's OIDC auth step (not a bug in the workflow — PyPI will reject the identity until the publisher is registered). The `ocultar` PyPI Organization request (spec Part A) is independent of this and does not need to finish first.
