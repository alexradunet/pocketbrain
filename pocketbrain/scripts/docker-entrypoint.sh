#!/bin/sh
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log() {
    echo -e "${GREEN}[PocketBrain]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[PocketBrain]${NC} $1"
}

error() {
    echo -e "${RED}[PocketBrain]${NC} $1"
}

info() {
    echo -e "${BLUE}[PocketBrain]${NC} $1"
}

DATA_DIR="${DATA_DIR:-/data}"
TS_STATE_DIR="${TS_STATE_DIR:-$DATA_DIR/tailscale}"
TS_HOSTNAME="${TS_HOSTNAME:-pocketbrain}"
TS_AUTHKEY="${TS_AUTHKEY:-}"
TS_USERSPACE="${TS_USERSPACE:-true}"
TS_EXTRA_ARGS="${TS_EXTRA_ARGS:-}"

mkdir -p "$DATA_DIR" "$TS_STATE_DIR" "$DATA_DIR/vault"

log "Starting Tailscale..."

if [ "$TS_USERSPACE" = "true" ]; then
    log "Using userspace networking mode (no root required)"
    /usr/local/bin/tailscaled \
        --tun=userspace-networking \
        --socks5-server=localhost:1080 \
        --state="$TS_STATE_DIR/tailscaled.state" \
        --socket="$TS_SOCKET" \
        2>&1 &
    TAILSCALED_PID=$!
    log "tailscaled started (PID: $TAILSCALED_PID)"
else
    log "Using standard networking mode (requires NET_ADMIN)"
    /usr/local/bin/tailscaled \
        --state="$TS_STATE_DIR/tailscaled.state" \
        --socket="$TS_SOCKET" \
        2>&1 &
    TAILSCALED_PID=$!
fi

sleep 2
log "Authenticating with Tailscale..."

if [ -n "$TS_AUTHKEY" ]; then
    if /usr/local/bin/tailscale --socket="$TS_SOCKET" up \
        --authkey="$TS_AUTHKEY" \
        --hostname="$TS_HOSTNAME" \
        --accept-routes \
        $TS_EXTRA_ARGS 2>&1; then
        log "Authenticated with Tailscale (authkey)"
    else
        error "Failed to authenticate with Tailscale"
        error "Check that your TS_AUTHKEY is valid and not expired"
        exit 1
    fi
else
    warn "No TS_AUTHKEY provided - starting interactive authentication"
    warn "Run: docker logs <container> | grep -A5 'To authenticate'"
    /usr/local/bin/tailscale --socket="$TS_SOCKET" up \
        --hostname="$TS_HOSTNAME" \
        --accept-routes \
        $TS_EXTRA_ARGS 2>&1 || true
fi

MAX_RETRIES=30
RETRY_COUNT=0
while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    if /usr/local/bin/tailscale --socket="$TS_SOCKET" status > /dev/null 2>&1; then
        break
    fi
    RETRY_COUNT=$((RETRY_COUNT + 1))
    sleep 1
done

if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
    error "Tailscale failed to connect after ${MAX_RETRIES}s"
    error "Check logs above for authentication URL"
    exit 1
fi

TAILNET_NAME=$(/usr/local/bin/tailscale --socket="$TS_SOCKET" status --json 2>/dev/null | grep -o '"MagicDNSSuffix":"[^"]*"' | cut -d'"' -f4 || echo "unknown")
TAILSCALE_IP=$(/usr/local/bin/tailscale --socket="$TS_SOCKET" ip -4 2>/dev/null || echo "unknown")

echo ""
log "Tailscale connected"
log "Hostname: ${TS_HOSTNAME}.${TAILNET_NAME}"
log "IP: ${TAILSCALE_IP}"
echo ""

log "File sync via Syncthing"
log "Access Syncthing UI: http://localhost:8384"
log "Vault location: /data/vault"
echo ""

info "Security: sandboxed container, tailscale networking, non-root user"
log "Starting PocketBrain..."
log "Data directory: $DATA_DIR"
echo ""

export TAILSCALE_ENABLED=true
export ALL_PROXY=socks5://localhost:1080

exec /app/pocketbrain "$@"
