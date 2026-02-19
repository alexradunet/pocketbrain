#!/usr/bin/env bash
set -euo pipefail

SERVICE_NAME="${SERVICE_NAME:-pocketbrain}"
LOG_FILE="${LOG_FILE:-./.data/pocketbrain.log}"

if command -v journalctl >/dev/null 2>&1 && systemctl list-unit-files "${SERVICE_NAME}.service" >/dev/null 2>&1; then
  exec sudo journalctl -u "${SERVICE_NAME}.service" -f -n 200
fi

if [ -f "$LOG_FILE" ]; then
  exec tail -n 200 -f "$LOG_FILE"
fi

printf 'No systemd service or log file found.\n' >&2
printf 'Run with one of:\n' >&2
printf '  bun run dev\n' >&2
printf '  bun run start\n' >&2
