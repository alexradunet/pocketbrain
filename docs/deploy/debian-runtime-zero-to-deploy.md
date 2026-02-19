# Debian Runtime Zero-to-Deploy

OpenCode skill equivalent: `pocketbrain-runtime-deploy`.

Deploy PocketBrain on a Debian host as an always-on Bun runtime.

For host hardening and production setup, see `docs/deploy/secure-vps-and-run-pocketbrain.md`.

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
ENABLE_WHATSAPP=true
```

Optional:

```dotenv
ENABLE_WHATSAPP=true
WHITELIST_PAIR_TOKEN=your-secure-token
OPENCODE_MODEL=provider/model
DATA_DIR=.data
WHATSAPP_AUTH_DIR=.data/whatsapp-auth
```

## 3) Start runtime stack

```bash
bun install
bun run start
```

## 4) Verify health

```bash
make logs
```

## 5) Always-on behavior

- Configure a systemd unit for `bun run start`.
- Enable the service on boot.

## 6) Update

```bash
git pull
bun install
bun run start
```

## 7) Managed release

```bash
make release TAG=$(git rev-parse --short HEAD)
```
