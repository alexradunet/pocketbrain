# Fresh VPS Validation Runbook

Use this checklist after reinstalling PocketBrain on a fresh VPS to verify setup and recovery behavior.

## 1) Clone and build

```bash
git clone https://github.com/alexradunet/pocketbrain.git
cd pocketbrain
go build -o pocketbrain .
```

## 2) Configure environment

```bash
./pocketbrain setup
```

Set at minimum:

- `PROVIDER` and API key for your chosen provider
- `WORKSPACE_PATH=.data/workspace`
- `TAILSCALE_ENABLED=true` and `TS_AUTHKEY=...` (for embedded tailnet connectivity)
- `TAILDRIVE_ENABLED=true` and `TAILDRIVE_*` values (for Taildrive workspace sharing)
- `ENABLE_WHATSAPP=true` (if WhatsApp channel is required)

Alternative (manual): create `.env` from `.env.example` and fill values directly.

## 3) Start PocketBrain

```bash
./pocketbrain start --headless
```

Confirm logs show healthy startup without errors.

## 4) Validate data directories

Confirm these exist after startup:

- `.data/state.db`
- `.data/workspace/`
- `.data/whatsapp-auth/` (if WhatsApp enabled)

## 5) Run health checks

```bash
journalctl -u pocketbrain -f
sudo systemctl status pocketbrain
```

## 6) Validate code health (recommended)

```bash
go vet ./...
go test ./... -count=1
```

## 7) Functional smoke tests

From chat, run lightweight checks:

- Send a message and confirm a response
- Use `/new` to start a fresh session
- Use `/remember` to save a memory fact

## 8) Reinstall resilience drill

Simulate a fresh host while keeping data:

1. Stop service/runtime.
2. Remove local runtime directories except synced workspace.
3. Re-clone repo and repeat steps 1-3.

Success criteria:

- PocketBrain starts without manual recreation of config files.
- SQLite state is recreated on fresh start.
