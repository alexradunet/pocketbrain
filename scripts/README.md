# Scripts

Operational and setup scripts for PocketBrain.

## Layout

- `setup/` machine/bootstrap scripts
  - `install-debian-dev.sh` developer machine setup
  - `install-debian-runtime.sh` runtime host prerequisites (Docker + Compose)
- `ops/` operator scripts
  - `docker-logs.sh`
  - `docker-shell.sh`
  - `release.sh`
  - `backup.sh`
  - `restore.sh`

Use `make` targets at repository root as the primary command interface.
