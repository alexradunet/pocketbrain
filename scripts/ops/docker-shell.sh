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

$DOCKER_BIN compose -p "$RUNTIME_PROJECT" -f docker-compose.yml exec -it pocketbrain sh "$@"
