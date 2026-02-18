# Developer Onboarding

OpenCode skill equivalent: `pocketbrain-dev-setup`.

This guide sets up a contributor environment from repository root.

## Prerequisites

- Debian/Ubuntu/macOS/Linux shell
- Bun 1.3.x
- Git

## Setup

```bash
make setup-dev
```

Or manually:

```bash
bun install
cp .env.example .env
bun run setup
```

## Development commands

```bash
make dev
make test
make build
```

## Optional dev-control container

```bash
docker compose -p pocketbrain-dev -f docker-compose.dev.yml up -d --build
docker compose -p pocketbrain-dev -f docker-compose.dev.yml exec -it dev-control sh
```

Inside `dev-control`, repo root is `/workspace`.
