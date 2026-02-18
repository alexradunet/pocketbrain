# Debian Runtime Zero-to-Deploy

OpenCode skill equivalent: `pocketbrain-runtime-deploy`.

Deploy PocketBrain on a Debian host as an always-on Docker runtime.

## 1) Install prerequisites

```bash
make setup-runtime
```

## 2) Configure environment

```bash
cp .env.example .env
```

Set at minimum:

```dotenv
TS_AUTHKEY=tskey-auth-...
```

Optional:

```dotenv
ENABLE_WHATSAPP=true
WHITELIST_PAIR_TOKEN=your-secure-token
OPENCODE_MODEL=provider/model
DATA_PATH=./data
```

## 3) Start runtime stack

```bash
make up
```

## 4) Verify health

```bash
make ps
docker compose -p pocketbrain-runtime -f docker-compose.yml logs --tail=120 pocketbrain
docker compose -p pocketbrain-runtime -f docker-compose.yml logs --tail=120 syncthing
```

## 5) Always-on behavior

- Docker daemon is enabled on boot by setup script.
- Services use `restart: unless-stopped`.

## 6) Update

```bash
git pull
make up
```

## 7) Managed release

```bash
make release TAG=$(git rev-parse --short HEAD)
```
