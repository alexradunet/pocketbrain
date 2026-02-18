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
- `pocketbrain` (assistant runtime)
- `syncthing` (vault synchronization)

## Dev-control stack (optional)

```bash
docker compose -p pocketbrain-dev -f docker-compose.dev.yml up -d --build
docker compose -p pocketbrain-dev -f docker-compose.dev.yml exec -it dev-control sh
```

Inside `dev-control`, repository root is `/workspace`.

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
