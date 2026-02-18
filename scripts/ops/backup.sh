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

SERVICES_STOPPED=0
restore_services() {
  if [ "$SERVICES_STOPPED" -eq 1 ]; then
    $DOCKER_BIN compose -p "$RUNTIME_PROJECT" -f "$RUNTIME_COMPOSE_FILE" start tailscale pocketbrain syncthing >/dev/null 2>&1 || true
  fi
}
trap restore_services EXIT

$DOCKER_BIN compose -p "$RUNTIME_PROJECT" -f "$RUNTIME_COMPOSE_FILE" stop tailscale pocketbrain syncthing >/dev/null 2>&1 || true
SERVICES_STOPPED=1

tar -czf "$OUTPUT_FILE" -C "$APP_DIR" "${DATA_PATH#${APP_DIR}/}"

$DOCKER_BIN compose -p "$RUNTIME_PROJECT" -f "$RUNTIME_COMPOSE_FILE" start tailscale pocketbrain syncthing >/dev/null 2>&1 || true
SERVICES_STOPPED=0
trap - EXIT

printf 'Backup created: %s\n' "$OUTPUT_FILE"
