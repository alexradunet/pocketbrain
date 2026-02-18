#!/usr/bin/env bash
set -eu

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/../.." && pwd)"
APP_DIR="$REPO_ROOT/pocketbrain"
RUNTIME_PROJECT="${RUNTIME_PROJECT:-pocketbrain-runtime}"
RUNTIME_COMPOSE_FILE="${RUNTIME_COMPOSE_FILE:-docker-compose.yml}"
RUNTIME_IMAGE="${RUNTIME_IMAGE:-${RUNTIME_PROJECT}-pocketbrain}"

TAG="${1:-$(git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null || printf 'local')}"
TIMEOUT_SECONDS="${TIMEOUT_SECONDS:-180}"
ROLLBACK_TIMEOUT_SECONDS="${ROLLBACK_TIMEOUT_SECONDS:-120}"

cd "$APP_DIR"

if [ ! -f ".env" ]; then
  printf '.env missing in %s\n' "$APP_DIR" >&2
  exit 1
fi

DOCKER_BIN="docker"
if ! docker info >/dev/null 2>&1; then
  DOCKER_BIN="sudo -E docker"
fi

PREVIOUS_IMAGE_ID="$($DOCKER_BIN image inspect "${RUNTIME_IMAGE}:latest" --format '{{.Id}}' 2>/dev/null || true)"

wait_for_health() {
  SERVICE_NAME="$1"
  MAX_WAIT_SECONDS="$2"
  START_TS="$(date +%s)"

  while true; do
    CONTAINER_ID="$($DOCKER_BIN compose -p "$RUNTIME_PROJECT" -f "$RUNTIME_COMPOSE_FILE" ps -q "$SERVICE_NAME" 2>/dev/null || true)"
    STATUS=""
    if [ -n "$CONTAINER_ID" ]; then
      STATUS="$($DOCKER_BIN inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$CONTAINER_ID" 2>/dev/null || true)"
    fi

    if [ "$STATUS" = "healthy" ]; then
      return 0
    fi

    if [ "$STATUS" = "unhealthy" ]; then
      return 1
    fi

    NOW_TS="$(date +%s)"
    if [ $((NOW_TS - START_TS)) -ge "$MAX_WAIT_SECONDS" ]; then
      return 1
    fi

    sleep 5
  done
}

$DOCKER_BIN compose -p "$RUNTIME_PROJECT" -f "$RUNTIME_COMPOSE_FILE" down --remove-orphans >/dev/null 2>&1 || true

bun run typecheck
bun run test
bun run build

export APP_VERSION="$TAG"
export GIT_SHA="$TAG"
$DOCKER_BIN compose -p "$RUNTIME_PROJECT" -f "$RUNTIME_COMPOSE_FILE" up -d --build pocketbrain syncthing

NEW_IMAGE_ID="$($DOCKER_BIN image inspect "${RUNTIME_IMAGE}:latest" --format '{{.Id}}' 2>/dev/null || true)"
if [ -n "$NEW_IMAGE_ID" ]; then
  $DOCKER_BIN tag "$NEW_IMAGE_ID" "${RUNTIME_IMAGE}:${TAG}"
fi

if wait_for_health "pocketbrain" "$TIMEOUT_SECONDS" && wait_for_health "syncthing" "$TIMEOUT_SECONDS"; then
  printf 'Release %s deployed and healthy\n' "$TAG"
  exit 0
fi

printf 'Release %s failed health check, rolling back\n' "$TAG" >&2

if [ -n "$PREVIOUS_IMAGE_ID" ]; then
  $DOCKER_BIN tag "$PREVIOUS_IMAGE_ID" "${RUNTIME_IMAGE}:latest"
  $DOCKER_BIN compose -p "$RUNTIME_PROJECT" -f "$RUNTIME_COMPOSE_FILE" up -d pocketbrain syncthing

  if wait_for_health "pocketbrain" "$ROLLBACK_TIMEOUT_SECONDS" && wait_for_health "syncthing" "$ROLLBACK_TIMEOUT_SECONDS"; then
    printf 'Rollback applied and healthy\n' >&2
  else
    printf 'Rollback applied but services are not healthy\n' >&2
  fi
else
  printf 'No previous image available for rollback\n' >&2
fi

printf 'Last 80 logs from pocketbrain:\n' >&2
$DOCKER_BIN compose -p "$RUNTIME_PROJECT" -f "$RUNTIME_COMPOSE_FILE" logs --tail=80 pocketbrain >&2 || true

exit 1
