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
APP_DIR="$REPO_ROOT"
DATA_PATH="${DATA_PATH:-$APP_DIR/data}"

cd "$APP_DIR"

rm -rf "$DATA_PATH"
mkdir -p "$DATA_PATH"

tar -xzf "$BACKUP_FILE" -C "$APP_DIR"

printf 'Restore completed from: %s\n' "$BACKUP_FILE"
