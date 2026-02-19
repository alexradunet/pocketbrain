# Scripts

Executable helper scripts. `make` at repository root is the canonical interface.

## Layout

- `setup/` machine/bootstrap scripts
  - `install-debian-dev.sh` developer machine setup
  - `install-debian-runtime.sh` runtime host prerequisites
  - `init-vault.sh` create vault scaffold plus `99-system/99-pocketbrain`
- `ops/` operator scripts
  - `doctor.sh`
  - `runtime-logs.sh`
  - `runtime-shell.sh`
  - `release.sh`
