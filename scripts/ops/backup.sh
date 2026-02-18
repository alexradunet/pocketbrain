#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/../.." && pwd)"
APP_DIR="$REPO_ROOT"
RUNTIME_PROJECT="${RUNTIME_PROJECT:-pocketbrain-runtime}"
RUNTIME_COMPOSE_FILE="${RUNTIME_COMPOSE_FILE:-docker-compose.yml}"
DATA_PATH="${DATA_PATH:-$APP_DIR/data}"
BACKUP_DIR="${BACKUP_DIR:-$APP_DIR/backups}"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
OUTPUT_FILE="${1:-$BACKUP_DIR/pocketbrain-backup-${TIMESTAMP}.tar.gz}"

cd "$APP_DIR"

if [ ! -d "$DATA_PATH" ]; then
  printf 'Data path not found: %s\n' "$DATA_PATH" >&2
  exit 1
fi

mkdir -p "$BACKUP_DIR"

DOCKER_BIN="docker"
if ! docker info >/dev/null 2>&1; then
  DOCKER_BIN="sudo -E docker"
fi

$DOCKER_BIN compose -p "$RUNTIME_PROJECT" -f "$RUNTIME_COMPOSE_FILE" stop pocketbrain syncthing >/dev/null 2>&1 || true

tar -czf "$OUTPUT_FILE" -C "$APP_DIR" "${DATA_PATH#${APP_DIR}/}"

$DOCKER_BIN compose -p "$RUNTIME_PROJECT" -f "$RUNTIME_COMPOSE_FILE" start pocketbrain syncthing >/dev/null 2>&1 || true

printf 'Backup created: %s\n' "$OUTPUT_FILE"
