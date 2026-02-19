#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/../.." && pwd)"
APP_DIR="$REPO_ROOT"
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

tar -czf "$OUTPUT_FILE" -C "$APP_DIR" "${DATA_PATH#${APP_DIR}/}"

printf 'Backup created: %s\n' "$OUTPUT_FILE"
