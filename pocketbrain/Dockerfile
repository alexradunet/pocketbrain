# =============================================================================
# PocketBrain - Secure Docker Image with Tailscale
# =============================================================================
# 
# This image contains ONLY PocketBrain. 
# File sync is handled by Syncthing (separate container).
#
# Features:
# - Multi-stage build for minimal attack surface
# - Tailscale userspace networking (no privileged mode)
# - Non-root execution
# - Read-only root filesystem
# =============================================================================

# -----------------------------------------------------------------------------
# Stage 1: Builder
# -----------------------------------------------------------------------------
FROM oven/bun:1.2-alpine AS builder

WORKDIR /build

# Install dependencies
COPY package.json bun.lock ./
RUN bun install --frozen-lockfile

# Copy source and compile
COPY . .
RUN bun build --compile --minify --outfile pocketbrain ./src/index.ts

# -----------------------------------------------------------------------------
# Stage 2: Runtime
# -----------------------------------------------------------------------------
FROM oven/bun:1.2-alpine

ARG APP_VERSION=dev
ARG GIT_SHA=unknown

COPY --from=tailscale/tailscale:v1.78 /usr/local/bin/tailscale /usr/local/bin/tailscale
COPY --from=tailscale/tailscale:v1.78 /usr/local/bin/tailscaled /usr/local/bin/tailscaled

# Install minimal runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    su-exec \
    && rm -rf /var/cache/apk/*

RUN BUN_INSTALL=/usr/local bun add -g opencode-ai@1.1.53

# Create directories
# /app - application code (read-only)
# /data - persistent data (SQLite, vault, state)
# /tmp - temporary files
RUN mkdir -p /app /data /data/vault /tmp /var/run/tailscale && \
    chown -R 1000:1000 /data /tmp && \
    chmod 755 /var/run/tailscale

WORKDIR /app

# Copy binary
COPY --from=builder /build/pocketbrain /app/pocketbrain
COPY --from=builder /build/.env.example /app/.env.example
COPY --chown=1000:1000 scripts/docker-entrypoint.sh /entrypoint.sh

RUN chmod +x /entrypoint.sh /app/pocketbrain

# Use non-root user
USER 1000:1000

# Environment defaults
ENV DATA_DIR=/data \
    TMPDIR=/tmp \
    TS_STATE_DIR=/data/tailscale \
    TS_SOCKET=/var/run/tailscale/tailscaled.sock \
    TS_USERSPACE=true \
    VAULT_PATH=/data/vault \
    APP_VERSION=${APP_VERSION} \
    GIT_SHA=${GIT_SHA}

# No ports exposed - all access via Tailscale
# Syncthing handles file sync separately

# Mark volumes
VOLUME ["/data"]

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD pgrep -f "/app/pocketbrain" > /dev/null && \
        tailscale status > /dev/null 2>&1 || exit 1

ENTRYPOINT ["/entrypoint.sh"]
