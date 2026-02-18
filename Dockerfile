# =============================================================================
# PocketBrain Runtime Image
# =============================================================================
# Single-process container. Networking concerns are delegated to Docker Compose
# sidecars (for example, Tailscale) instead of in-container orchestration.
# =============================================================================

FROM oven/bun:1.2-alpine AS builder

WORKDIR /build

COPY package.json bun.lock ./
RUN bun install

COPY . .
RUN bun build --compile --minify --outfile pocketbrain ./src/index.ts

FROM oven/bun:1.2-alpine

ARG APP_VERSION=dev
ARG GIT_SHA=unknown

RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    && rm -rf /var/cache/apk/*

RUN mkdir -p /app /data /data/vault /tmp && \
    chown -R 1000:1000 /data /tmp

WORKDIR /app

COPY --from=builder /build/pocketbrain /app/pocketbrain
COPY --from=builder /build/.env.example /app/.env.example

RUN chmod +x /app/pocketbrain

USER 1000:1000

ENV DATA_DIR=/data \
    TMPDIR=/tmp \
    VAULT_PATH=/data/vault \
    APP_VERSION=${APP_VERSION} \
    GIT_SHA=${GIT_SHA}

VOLUME ["/data"]

HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD pgrep -f "/app/pocketbrain" > /dev/null || exit 1

ENTRYPOINT ["/app/pocketbrain"]
