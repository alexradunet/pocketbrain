#!/usr/bin/env bash
set -eu

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/../.." && pwd)"
APP_DIR="$REPO_ROOT"
RUNTIME_PROJECT="${RUNTIME_PROJECT:-pocketbrain-runtime}"
RUNTIME_COMPOSE_FILE="${RUNTIME_COMPOSE_FILE:-docker-compose.yml}"
RUNTIME_IMAGE="${RUNTIME_IMAGE:-${RUNTIME_PROJECT}-pocketbrain}"

cd "$APP_DIR"

if [ ! -f ".env" ] && [ -f ".env.example" ]; then
  cp .env.example .env
fi

IMAGE_TAG="${1:-latest}"
APP_VERSION="${APP_VERSION:-$IMAGE_TAG}"
GIT_SHA="${GIT_SHA:-$(git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null || printf 'unknown')}"

DOCKER_BIN="docker"
if ! docker info >/dev/null 2>&1; then
  DOCKER_BIN="sudo -E docker"
fi

export APP_VERSION
export GIT_SHA
$DOCKER_BIN compose -p "$RUNTIME_PROJECT" -f "$RUNTIME_COMPOSE_FILE" build
$DOCKER_BIN tag "${RUNTIME_IMAGE}:latest" "${RUNTIME_IMAGE}:${IMAGE_TAG}"

printf 'Built image %s:%s\n' "$RUNTIME_IMAGE" "$IMAGE_TAG"
