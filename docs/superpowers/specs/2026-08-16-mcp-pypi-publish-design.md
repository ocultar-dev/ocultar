# MCP Extensions: Automated PyPI Publishing from ocultar-dev/ocultar

**Date:** 2026-08-16
**Status:** Approved

## Problem

Three Python MCP packages are distributed from this repo — `ocultar-claude-mcp`
(`extensions/claude/`), `ocultar-goose-mcp` (`extensions/goose/`), and
`ocultar-mistral-mcp` (`extensions/mistral/`) — but PyPI is out of sync with
what's actually here, and ownership isn't aligned with the org either:

- `ocultar-claude-mcp` on PyPI is at `v0.1.2`, with its repo URL pointing at
  the old personal fork `github.com/Edu963/ocultar`. This repo has it at
  `v0.2.0` (16-char tokens, `github.com/ocultar-dev/ocultar` in
  `pyproject.toml`).
- There is no CI step anywhere that builds or publishes these packages —
  every past release (confirmed for all three: claude, goose, mistral, last
  released Apr 27–30, 2026) was a manual `build` + `twine upload` from a
  local machine.
- All three projects are listed as "Sole Owner" under a personal PyPI
  account, not any `ocultar` identity — a bus-factor risk independent of
  the publishing pipeline.

Goal: make `ocultar-dev/ocultar` the sole source of truth for these
packages, with releases flowing through CI instead of manual builds, and
ownership held by an org identity rather than one person.

## Design

### Part A — Ownership transfer to a PyPI Organization

Runs in parallel with Part B; neither blocks the other, since Trusted
Publishing (Part B) authorizes by GitHub repo/workflow, not by which PyPI
account owns the project.

1. Request a **PyPI Organization** named `ocultar` (or closest available)
   via *Your organizations* on pypi.org. This goes through PyPI's approval
   queue — no fixed SLA.
2. Once approved, for each of the three projects: *Manage → Collaborators* →
   add the `ocultar` org as **Owner** → confirm the org has access → then
   remove the personal account as owner. (Add-then-remove — PyPI requires
   at least one owner at all times.)

This step happens entirely on pypi.org; it isn't part of the implementation
plan below since there's no code or repo change involved, but it's tracked
here as a prerequisite alongside Part B.

### Part B — CI workflow

### Versioning

All three extensions stay version-locked together (they already are, at
`0.2.0`). One release bumps all three `pyproject.toml` files to the same
version number, even if only one package's code actually changed.

### CI workflow — `.github/workflows/mcp-publish.yml`

- **Trigger:** `push: tags: ["mcp-v*.*.*"]`. A dedicated prefix, distinct from
  the existing `v*.*.*` tags that `.github/workflows/release.yml` uses for
  the Go binaries — the two versioning tracks are independent (product is at
  `v1.15.0`, extensions at `v0.2.0`).
- **Jobs:** a matrix over the three extension directories
  (`extensions/claude`, `extensions/goose`, `extensions/mistral`). Each job:
  1. Checks out the repo.
  2. Sets up Python 3.12 (matching the `>=3.10` floor in each `pyproject.toml`).
  3. Runs `python -m build` inside the extension directory to produce a wheel
     + sdist.
  4. Publishes with `pypa/gh-action-pypi-publish`, using **PyPI Trusted
     Publishing (OIDC)** — no `PYPI_API_TOKEN` secret stored in the repo.
- **Permissions:** `id-token: write` (required for OIDC), `contents: read`.
- Failure in one matrix leg (e.g. mistral) must not block the others from
  publishing — `fail-fast: false`.

### One-time PyPI Trusted Publisher setup (manual, not part of this implementation)

Before the first tag push, on pypi.org, for each of the three existing
projects (`ocultar-claude-mcp`, `ocultar-goose-mcp`, `ocultar-mistral-mcp`):
*Settings → Publishing* → add Trusted Publisher — owner `ocultar-dev`, repo
`ocultar`, workflow `mcp-publish.yml`.

This can be done under the current personal ownership or after the Part A
transfer completes — either account configuring it works, since Trusted
Publishing keys off the GitHub repo/workflow, not the PyPI account identity.
It's a hard prerequisite regardless: tagging before it's done will fail the
workflow's auth step.

### Release process going forward

1. Bump the `version` field in all three `extensions/*/pyproject.toml`.
2. Commit.
3. `git tag mcp-v<version> && git push origin mcp-v<version>`.
4. CI builds and publishes all three packages.

The first tag (`mcp-v0.2.0`) both establishes the pipeline and resyncs PyPI
to the repo's current state — no separate manual republish is needed.

## Out of scope

- Fixing the `OCULTAR_URL` port inconsistency between
  `extensions/claude/README.md` (`:4141`) and
  `apps/web/public/content/guides/MCP_EXTENSIONS.md` (`:8080`) — unrelated
  to the publishing pipeline, noted here so it isn't lost.
- Independent per-package versioning (rejected — lockstep chosen for
  simplicity, matches current state).
- Changing the Go binary release workflow or its `v*.*.*` tag scheme.
- Actually performing the PyPI Organization request/transfer (Part A) or the
  Trusted Publisher form-filling (Part B prerequisite) — both are manual
  pypi.org actions outside this session's access; this spec documents them
  as required steps but the implementation plan only covers the workflow
  file itself.

## Testing

- Workflow can be validated with a dry run: build steps (`python -m build`)
  run locally / in CI without the publish step to confirm wheels produce
  cleanly before the first real tag.
- No unit tests apply — this is CI/release infrastructure, not
  application code.
