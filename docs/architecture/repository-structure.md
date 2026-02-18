# Repository Structure Contract 🧭

Canonical repository layout for PocketBrain.

## Quick Visual Map

```mermaid
flowchart TB
  Root[Repository Root]
  Root --> Src[src/ app code]
  Root --> Tests[tests/ automated tests]
  Root --> Scripts[scripts/ setup + runtime + ops]
  Root --> Docs[docs/ architecture + guides + runbooks]
  Root --> Skills[.agents/skills/ reusable workflows]
  Root --> Dev[development/ CI + contracts]
```

## Top-level folders

### `src/`

Application source code.

### `tests/`

Automated tests for the application.

### `scripts/`

Executable scripts grouped by purpose:
- `setup/` machine/bootstrap setup
- `runtime/` runtime/container boot scripts
- `ops/` release/backup/operational helpers

### `docs/`

Documentation and runbooks only.

### `development/`

Development and CI contract tooling.

### `.agents/skills/`

OpenCode/agent-compatible reusable skills (`SKILL.md` files).

## Placement checklist

1. Is it executable app code? -> `src/`
2. Is it a test? -> `tests/`
3. Is it an operator/setup/runtime script? -> `scripts/`
4. Is it documentation? -> `docs/`
5. Is it reusable agent workflow knowledge? -> `.agents/skills/`
6. Is it CI/repo-contract tooling? -> `development/`

## Command contract

- Repository root is the only canonical working directory.
- `make` is the primary command interface.
- No compatibility wrapper scripts at `scripts/*.sh`.

## Enforcement

- Structure rules are enforced by `development/ci/validate-structure.sh`.
- PRs run `.github/workflows/structure-contract.yml`.
