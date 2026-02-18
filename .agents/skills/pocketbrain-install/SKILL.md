---
name: pocketbrain-install
description: Install and deploy PocketBrain on Debian from zero to healthy always-on runtime.
compatibility: opencode
metadata:
  audience: operators
  scope: bootstrap
---

## Purpose

Use this for end-to-end first install on a Debian host.

## Required outcomes

1. Docker Engine + Compose plugin installed
2. `.env` created with required runtime values
3. Runtime stack starts successfully
4. `pocketbrain` and `syncthing` become healthy
5. Operator receives handoff commands for logs and updates

## Canonical workflow

1. Install runtime prerequisites:

```bash
make setup-runtime
```

2. Configure environment:

```bash
cp .env.example .env
```

Set required value:

```dotenv
TS_AUTHKEY=tskey-auth-...
```

3. Deploy runtime:

```bash
make up
```

4. Verify:

```bash
make ps
docker compose -p pocketbrain-runtime -f docker-compose.yml logs --tail=120 pocketbrain
docker compose -p pocketbrain-runtime -f docker-compose.yml logs --tail=120 syncthing
```

## Operational handoff

```bash
make ps
make logs
git pull && make up
```
