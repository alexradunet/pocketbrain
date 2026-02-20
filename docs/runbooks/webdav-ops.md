# WebDAV Operations Runbook

PocketBrain exposes the workspace directory as a WebDAV endpoint. System-level Tailscale (or any VPN/network layer) handles connectivity and auth — the WebDAV server itself has no authentication.

## Prerequisites

- PocketBrain running with `WEBDAV_ENABLED=true`
- Network connectivity to the server (e.g. via Tailscale, VPN, or LAN)

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `WEBDAV_ENABLED` | `false` | Enable the WebDAV workspace server |
| `WEBDAV_ADDR` | `0.0.0.0:6060` | Listen address for the WebDAV server |

## Testing with curl

```bash
# Directory listing (PROPFIND)
curl -X PROPFIND http://localhost:6060/

# Read a file
curl http://localhost:6060/inbox/test.md

# Upload a file
curl -T myfile.txt http://localhost:6060/myfile.txt

# Create a directory
curl -X MKCOL http://localhost:6060/new-folder/

# Delete a file
curl -X DELETE http://localhost:6060/myfile.txt
```

## Troubleshooting

| Symptom | Check |
|---------|-------|
| Connection refused | Is `WEBDAV_ENABLED=true`? Did PocketBrain start successfully? |
| Cannot reach from other device | Check firewall rules and network connectivity (Tailscale, VPN, etc.) |
| Files not appearing | Check workspace path is correct and has files |
| Port conflict | Change `WEBDAV_ADDR` to a different port |

## Obsidian Sync

A custom Obsidian sync plugin (separate project) can consume the WebDAV URL from mobile/desktop to keep the vault in sync. Point it at `http://<host>:6060/` where `<host>` is the Tailscale IP or hostname of the PocketBrain server.
