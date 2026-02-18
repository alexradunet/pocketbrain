#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 1 ]; then
  printf 'Usage: %s <backup-tar.gz>\n' "${0##*/}" >&2
  exit 1
fi

BACKUP_FILE="$1"
if [ ! -f "$BACKUP_FILE" ]; then
  printf 'Backup file not found: %s\n' "$BACKUP_FILE" >&2
  exit 1
fi

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/../.." && pwd)"
APP_DIR="$REPO_ROOT/pocketbrain"
RUNTIME_PROJECT="${RUNTIME_PROJECT:-pocketbrain-runtime}"
RUNTIME_COMPOSE_FILE="${RUNTIME_COMPOSE_FILE:-docker-compose.yml}"
DATA_PATH="${DATA_PATH:-$APP_DIR/data}"

cd "$APP_DIR"

DOCKER_BIN="docker"
if ! docker info >/dev/null 2>&1; then
  DOCKER_BIN="sudo -E docker"
fi

$DOCKER_BIN compose -p "$RUNTIME_PROJECT" -f "$RUNTIME_COMPOSE_FILE" down >/dev/null 2>&1 || true

rm -rf "$DATA_PATH"
mkdir -p "$DATA_PATH"

tar -xzf "$BACKUP_FILE" -C "$APP_DIR"

$DOCKER_BIN compose -p "$RUNTIME_PROJECT" -f "$RUNTIME_COMPOSE_FILE" up -d pocketbrain syncthing

printf 'Restore completed from: %s\n' "$BACKUP_FILE"
