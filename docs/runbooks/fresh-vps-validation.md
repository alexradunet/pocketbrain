# Fresh VPS Validation Runbook

Use this checklist after reinstalling PocketBrain on a fresh VPS to verify vault-backed PocketBrain setup and recovery behavior.

## 1) Clone and install prerequisites

```bash
git clone https://github.com/alexradunet/pocketbrain.git
cd pocketbrain
make setup-runtime
bun install
```

## 2) Configure environment

```bash
cp .env.example .env
```

Set at minimum:

- `VAULT_ENABLED=true`
- `VAULT_PATH=.data/vault` (or your synced mount path)
- `SYNCTHING_ENABLED=true` and `SYNCTHING_*` values (if using Syncthing)
- `ENABLE_WHATSAPP=true` (if WhatsApp channel is required)

## 3) Initialize vault scaffold

```bash
make vault-init VAULT_DIR=.data/vault
```

Verify the PocketBrain vault home exists:

- `.data/vault/99-system/99-pocketbrain`
- `.data/vault/99-system/99-pocketbrain/.agents/skills`

## 4) Start PocketBrain

```bash
make start
```

Confirm logs show healthy startup without OpenCode config/plugin path errors.

## 5) Validate OpenCode config in vault

Confirm these exist after startup:

- `.data/vault/99-system/99-pocketbrain/opencode.json`
- `.data/vault/99-system/99-pocketbrain/.agents/skills/`

## 6) Run doctor checks

```bash
make doctor
```

Expected checks include:

- vault path writable
- PocketBrain vault home writable
- PocketBrain runtime skills dir writable

## 7) Validate code health (recommended)

```bash
bun run typecheck
bun run test
```

## 8) Functional smoke tests

From chat, run lightweight checks:

- call `vault_obsidian_config`
- append to daily note via `vault_daily`

Confirm resulting files are written in expected vault locations.

## 9) Reinstall resilience drill

Simulate a fresh host while keeping vault sync content:

1. Stop service/runtime.
2. Remove local runtime directories except synced vault mount.
3. Re-clone repo and repeat steps 1, 2, and 4.

Success criteria:

- PocketBrain starts without manual recreation of config/skills/process files.
- Vault-backed PocketBrain home under `99-system/99-pocketbrain` is reused automatically.
