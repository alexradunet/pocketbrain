---
name: pocketbrain-runtime-deploy
description: Deploy PocketBrain runtime on Debian with Docker Compose and verify healthy always-on services.
compatibility: opencode
metadata:
  audience: operators
  scope: runtime
---

## What I do

- Install runtime prerequisites on Debian
- Configure runtime environment
- Start and verify PocketBrain and Syncthing services
- Hand off update and health commands

## When to use me

Use this for first-time runtime deployment or redeployment on a server.

## Canonical workflow

1. Install prerequisites:

```bash
make setup-runtime
```

2. Configure environment:

```bash
cp .env.example .env
```

Set at minimum:

```dotenv
TS_AUTHKEY=tskey-auth-...
```

3. Start runtime:

```bash
make up
```

4. Verify health:

```bash
make ps
docker compose -p pocketbrain-runtime -f docker-compose.yml logs --tail=120 pocketbrain
docker compose -p pocketbrain-runtime -f docker-compose.yml logs --tail=120 syncthing
```

5. Confirm always-on behavior:
- Docker service enabled on boot
- Compose services use `restart: unless-stopped`

## Operational handoff

```bash
make ps
make logs
git pull && make up
```
