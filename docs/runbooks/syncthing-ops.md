# Syncthing Operations Runbook

Use this runbook to connect PocketBrain to Syncthing REST safely.

## 1) Configure environment

Set the following in `.env`:

```dotenv
SYNCTHING_ENABLED=true
SYNCTHING_BASE_URL=http://127.0.0.1:8384
SYNCTHING_API_KEY=replace-with-syncthing-api-key
SYNCTHING_TIMEOUT_MS=5000
SYNCTHING_VAULT_FOLDER_ID=vault
SYNCTHING_MUTATION_TOOLS_ENABLED=true
SYNCTHING_ALLOWED_FOLDER_IDS=vault
```

Security guidance:

- Keep `SYNCTHING_BASE_URL` on localhost.
- Do not expose Syncthing GUI/API publicly.
- Keep mutation allowlist minimal.

## 2) Restart runtime

```bash
sudo systemctl restart pocketbrain
```

## 3) Verify with doctor

```bash
make doctor ARGS="--non-interactive"
```

Expected Syncthing checks:

- API key present
- API ping succeeds
- configured folder ID resolves
- mutation policy/allowlist coherent

## 4) Available Syncthing tools

Read-only:

- `syncthing_health`
- `syncthing_status`
- `syncthing_folder_status`
- `syncthing_folder_errors`

Guarded mutation:

- `syncthing_scan_folder`

`syncthing_scan_folder` is blocked unless:

- `SYNCTHING_MUTATION_TOOLS_ENABLED=true`
- target folder ID is in `SYNCTHING_ALLOWED_FOLDER_IDS`
