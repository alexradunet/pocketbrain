#!/usr/bin/env bash
set -eu

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/../.." && pwd)"
APP_DIR="$REPO_ROOT/pocketbrain"
DEV_PROJECT="${DEV_PROJECT:-pocketbrain-dev}"
DEV_COMPOSE_FILE="${DEV_COMPOSE_FILE:-docker-compose.dev.yml}"
RUNTIME_PROJECT="${RUNTIME_PROJECT:-pocketbrain-runtime}"

TAG="${1:-}"

cd "$APP_DIR"

DOCKER_BIN="docker"
if ! docker info >/dev/null 2>&1; then
  DOCKER_BIN="sudo -E docker"
fi

$DOCKER_BIN compose -p "$DEV_PROJECT" -f "$DEV_COMPOSE_FILE" up -d --build dev-control

if [ -n "$TAG" ]; then
  $DOCKER_BIN compose -p "$DEV_PROJECT" -f "$DEV_COMPOSE_FILE" exec -T dev-control sh -lc "cd /workspace && RUNTIME_PROJECT='$RUNTIME_PROJECT' ./scripts/release.sh '$TAG'"
else
  $DOCKER_BIN compose -p "$DEV_PROJECT" -f "$DEV_COMPOSE_FILE" exec -T dev-control sh -lc "cd /workspace && RUNTIME_PROJECT='$RUNTIME_PROJECT' ./scripts/release.sh"
fi
