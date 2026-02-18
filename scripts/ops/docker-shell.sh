#!/usr/bin/env bash
set -eu

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/../.." && pwd)"
APP_DIR="$REPO_ROOT/pocketbrain"
RUNTIME_PROJECT="${RUNTIME_PROJECT:-pocketbrain-runtime}"
DEV_PROJECT="${DEV_PROJECT:-pocketbrain-dev}"

cd "$APP_DIR"

DOCKER_BIN="docker"
if ! docker info >/dev/null 2>&1; then
  DOCKER_BIN="sudo -E docker"
fi

if [ "${1:-}" = "--dev" ]; then
  shift
  $DOCKER_BIN compose -p "$DEV_PROJECT" -f docker-compose.dev.yml exec -it dev-control sh "$@"
  exit 0
fi

$DOCKER_BIN compose -p "$RUNTIME_PROJECT" -f docker-compose.yml exec -it pocketbrain sh "$@"
