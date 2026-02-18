# Repository Structure Contract

This document defines the canonical workspace layout and where new files belong.

## Top-level folders

### `development/`

Purpose: local development support.

Put here:

- Environment bootstrap scripts.
- Developer-only helper scripts.
- Local utilities that are not required to operate a running instance.

Do not put here:

- Production/runtime operational scripts.
- App source code.

### `docs/`

Purpose: documentation only.

Put here:

- Architecture notes.
- Runbooks and troubleshooting guides.
- ADR-style decisions.

### `pocketbrain/`

Purpose: application code and app-scoped assets.

Put here:

- `src/`, `tests/`, package manifests, and app runtime config.
- App-specific Docker config already scoped to the app.

### `scripts/`

Purpose: operational scripts for managing PocketBrain instances.

Put here:

- `runtime/` for runtime/container boot scripts.
- `ops/` for operator tooling (build, logs, shell, diagnostics).
- Compatibility wrappers at `scripts/*.sh` only for stable entry points.

Do not put here:

- Developer machine bootstrap scripts (place those in `development/`).

## Placement decision checklist

When adding a file, ask:

1. Is this used to run/manage a deployed instance? -> `scripts/`
2. Is this used to setup or improve local development? -> `development/`
3. Is this executable app code or tests? -> `pocketbrain/`
4. Is this documentation? -> `docs/`

## Current migration decision

- Debian bootstrap script moved to `development/setup/install-debian.sh`.
- `scripts/install-debian.sh` is kept as a compatibility wrapper and delegates to the new location.
- Docker operational scripts moved under `scripts/ops/` and `scripts/runtime/`.

## Enforcement

- PRs run `.github/workflows/structure-contract.yml`.
- Rule set is implemented in `development/ci/validate-structure.sh`.
