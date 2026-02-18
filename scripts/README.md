# Instance Scripts

This folder contains scripts used to manage and inspect PocketBrain instances.

## Layout

- `runtime/` - scripts used by runtime/container boot flow.
- `ops/` - operator helper scripts for build, logs, and shell access.

## Current scripts

- `runtime/docker-entrypoint.sh` - container entrypoint used at runtime.
- `ops/docker-build.sh` - image build helper.
- `ops/docker-logs.sh` - follow and filter container logs.
- `ops/docker-shell.sh` - open an interactive shell in the running container.
- `ops/release.sh` - typecheck, test, deploy, health-check, and rollback runtime releases.
- `ops/dev-release.sh` - run the same release flow from inside the dev-control container.

Root-level `scripts/docker-*.sh` files are compatibility wrappers that delegate to the structured locations above.
Root-level `scripts/release.sh` delegates to `ops/release.sh`.
Root-level `scripts/dev-release.sh` delegates to `ops/dev-release.sh`.

`install-debian.sh` is retained as a compatibility wrapper and now delegates to `development/setup/install-debian.sh`.
