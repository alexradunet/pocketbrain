# PocketBrain

PocketBrain is a Bun-based assistant runtime with:
- OpenCode SDK integration
- WhatsApp adapter
- SQLite state
- Syncthing-backed vault sync
- Docker-first runtime deployment

## Repository layout

- `src/` application code
- `tests/` automated tests
- `scripts/` setup, runtime, and operator scripts
- `docs/` architecture, deploy guides, and runbooks
- `development/` CI tooling and repository contract checks

## Canonical command interface

Use `make` from repository root:

```bash
make setup-dev
make test
make build
make up
make ps
make logs
make release TAG=$(git rev-parse --short HEAD)
```

## Runtime quick start

```bash
cp .env.example .env
# set TS_AUTHKEY in .env
make up
make ps
```

## Developer quick start

```bash
bun install
bun run setup
make dev
```

For detailed instructions:
- `docs/setup/developer-onboarding.md`
- `docs/deploy/debian-runtime-zero-to-deploy.md`

For OpenCode skill-driven workflows:
- `docs/setup/agent-skills.md`
