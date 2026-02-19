# Taildrive Operations Runbook

Taildrive shares the PocketBrain vault folder as a WebDAV endpoint accessible to all devices on the tailnet.

## Prerequisites

- Tailscale installed and running on the VPS (`tailscale status` shows "online")
- Taildrive enabled in your Tailscale ACL policy

## ACL Policy

Add the following to your Tailscale ACL policy (https://login.tailscale.com/admin/acls):

```json
{
  "grants": [
    {
      "src": ["autogroup:member"],
      "dst": ["autogroup:self"],
      "app": {
        "tailscale.com/cap/drive": [
          {
            "shares": ["*"],
            "access": "rw"
          }
        ]
      }
    }
  ]
}
```

Adjust `src`/`dst` to match your tailnet's security requirements.

## Creating the Share

PocketBrain auto-creates the share on startup when `TAILDRIVE_ENABLED=true` and `TAILDRIVE_AUTO_SHARE=true`.

To create manually:

```bash
tailscale drive share vault /home/debian/pocketbrain/.data/vault
```

## Verifying the Share

```bash
# List shares
tailscale drive list

# Expected output:
# vault  /home/debian/pocketbrain/.data/vault
```

## WebDAV URL Format

Once shared, the vault is available at:

```
http://100.100.100.100:8080/<tailnet>/<machine>/vault/
```

Where:
- `100.100.100.100:8080` is the Taildrive local proxy (on each accessing device)
- `<tailnet>` is your tailnet name (e.g., `tail1234.ts.net`)
- `<machine>` is the VPS hostname (e.g., `pocketbrain`)

Example:
```
http://100.100.100.100:8080/tail1234.ts.net/pocketbrain/vault/
```

## Testing WebDAV Access (from another tailnet device)

```bash
# Directory listing
curl -X PROPFIND http://100.100.100.100:8080/<tailnet>/<machine>/vault/

# Read a file
curl http://100.100.100.100:8080/<tailnet>/<machine>/vault/inbox/test.md
```

## Removing a Share

```bash
tailscale drive unshare vault
```

## Troubleshooting

| Symptom | Check |
|---------|-------|
| `tailscale drive list` empty | Is `TAILDRIVE_ENABLED=true`? Did PocketBrain start successfully? |
| `tailscale drive share` fails | Check ACL policy grants Taildrive access |
| WebDAV 403 | ACL policy missing or `src`/`dst` too restrictive |
| WebDAV connection refused | Tailscale not running on the accessing device |
| Files not appearing | Check vault path is correct and has files |

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `TAILDRIVE_ENABLED` | `false` | Enable Taildrive bootstrap check |
| `TAILDRIVE_SHARE_NAME` | `vault` | Name of the Taildrive share |
| `TAILDRIVE_AUTO_SHARE` | `true` | Auto-create share if missing on startup |

## Obsidian Sync Plugin

A custom Obsidian sync plugin (separate project) consumes the Taildrive WebDAV URL from mobile/desktop to keep the vault in sync. See the plugin repo for setup instructions.
