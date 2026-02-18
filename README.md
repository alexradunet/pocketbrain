# PocketBrain Workspace

This repository is organized as a workspace with clear ownership boundaries:

- `development/` - development-only tooling, bootstrap scripts, and local utilities.
- `docs/` - architecture, decisions, and operational documentation.
- `pocketbrain/` - application source code and app-level runtime assets.
- `scripts/` - operational scripts to build, run, and manage PocketBrain instances (`ops/` and `runtime/`).

Use this layout contract when adding new files. If a file does not clearly fit one of these folders, document the exception in `docs/architecture/repository-structure.md`.
