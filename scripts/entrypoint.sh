#!/usr/bin/env sh
set -eu

WORKSPACE_DIR="${WORKSPACE_DIR:-/workspace}"
TAILSCALE_SOCKET="/var/run/tailscale/tailscaled.sock"
TAILSCALE_STATE="${WORKSPACE_DIR}/data/tailscale.state"

mkdir -p "${WORKSPACE_DIR}/store" "${WORKSPACE_DIR}/groups" "${WORKSPACE_DIR}/data"

# Optional env file mount for runtime setups that inject credentials via file.
if [ -f "${WORKSPACE_DIR}/env-dir/env" ]; then
  set -a
  . "${WORKSPACE_DIR}/env-dir/env"
  set +a
fi

if command -v tailscaled >/dev/null 2>&1; then
  mkdir -p /var/run/tailscale
  tailscaled --state="${TAILSCALE_STATE}" --socket="${TAILSCALE_SOCKET}" >/tmp/tailscaled.log 2>&1 &

  if [ -n "${TS_AUTHKEY:-}" ]; then
    i=0
    while [ $i -lt 30 ]; do
      if tailscale --socket="${TAILSCALE_SOCKET}" status >/dev/null 2>&1; then
        break
      fi
      i=$((i + 1))
      sleep 1
    done
    tailscale --socket="${TAILSCALE_SOCKET}" up --authkey="${TS_AUTHKEY}" --hostname="${TS_HOSTNAME:-pocketbrain}" || true
  fi
fi

exec /app/pocketbrain
