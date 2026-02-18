#!/usr/bin/env bash
set -eu

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/../.." && pwd)"
APP_DIR="$REPO_ROOT"
RUNTIME_PROJECT="${RUNTIME_PROJECT:-pocketbrain-runtime}"

cd "$APP_DIR"

DOCKER_BIN="docker"
if ! docker info >/dev/null 2>&1; then
  DOCKER_BIN="sudo -E docker"
fi

if [ "${1:-}" = "--tailscale" ]; then
  $DOCKER_BIN compose -p "$RUNTIME_PROJECT" -f docker-compose.yml logs -f tailscale
elif [ "${1:-}" = "--error" ]; then
  $DOCKER_BIN compose -p "$RUNTIME_PROJECT" -f docker-compose.yml logs -f pocketbrain | grep -i error
else
  $DOCKER_BIN compose -p "$RUNTIME_PROJECT" -f docker-compose.yml logs -f "$@"
fi
