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
  $DOCKER_BIN compose -p "$DEV_PROJECT" -f docker-compose.dev.yml logs -f "$@"
  exit 0
fi

if [ "${1:-}" = "--tailscale" ]; then
  $DOCKER_BIN compose -p "$RUNTIME_PROJECT" -f docker-compose.yml logs -f pocketbrain | grep -i tailscale
elif [ "${1:-}" = "--error" ]; then
  $DOCKER_BIN compose -p "$RUNTIME_PROJECT" -f docker-compose.yml logs -f pocketbrain | grep -i error
else
  $DOCKER_BIN compose -p "$RUNTIME_PROJECT" -f docker-compose.yml logs -f "$@"
fi
