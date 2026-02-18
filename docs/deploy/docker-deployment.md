# Docker Deployment

PocketBrain runtime runs as a Docker Compose stack from repository root.

## Runtime stack

```bash
cp .env.example .env
# set TS_AUTHKEY
make up
make ps
```

Services:
- `tailscale` (network sidecar for tailnet connectivity)
- `pocketbrain` (assistant runtime)
- `syncthing` (vault synchronization)

Required runtime secret:
- `TS_AUTHKEY` in `.env` for the `tailscale` service.

## Operations

```bash
make logs
make release TAG=$(git rev-parse --short HEAD)
make backup
make restore FILE=backups/<file>.tar.gz
```

## Notes

- Runtime data is stored in `data/` by default.
- Services use `restart: unless-stopped`.
- Docker daemon is expected to be enabled on boot.
