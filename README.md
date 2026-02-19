# PocketBrain

PocketBrain is a Bun + OpenCode assistant runtime with SQLite-backed state and channel adapters.

## Start Here

- Documentation index: `docs/README.md`
- Canonical runbooks: `docs/runbooks/README.md`
- Skills catalog: `docs/setup/agent-skills.md`

## Quick Commands

```bash
make setup-dev
make setup-runtime
make start
make dev
make test
make logs
make release TAG=$(git rev-parse --short HEAD)
```

## Data Paths

- Runtime data root: `.data/` (via `DATA_DIR`)
- SQLite state: `.data/state.db`
- Vault: `.data/vault/`
- WhatsApp auth: `.data/whatsapp-auth/`

## Repository Layout

- `src/` application code
- `tests/` automated tests
- `scripts/` setup and operational scripts
- `docs/` architecture, setup, deploy, and runbooks
- `.agents/skills/` OpenCode-compatible skills
- `development/` repo contract and CI tooling
