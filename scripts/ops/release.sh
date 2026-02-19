#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/../.." && pwd)"
TAG="${1:-$(git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null || printf 'local')}"

cd "$REPO_ROOT"

if [ ! -f ".env" ]; then
  printf '.env missing in %s\n' "$REPO_ROOT" >&2
  exit 1
fi

bun install
bun run typecheck
bun run test
bun run build

printf 'Release %s validated.\n' "$TAG"
printf 'Next step: restart the runtime process with your service manager.\n'
