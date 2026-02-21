# ---- Build stage ----
FROM oven/bun:1 AS builder
WORKDIR /app
COPY package.json ./
RUN bun install
COPY src/ ./src/
COPY tsconfig.json ./
RUN bun build --compile --target=bun-linux-x64 src/index.ts --outfile dist/pocketbrain && \
    bun build --compile --target=bun-linux-x64 src/mcp-tools.ts --outfile dist/pocketbrain-mcp

# ---- Runtime stage ----
FROM debian:trixie-slim
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates curl jq iptables \
    && rm -rf /var/lib/apt/lists/*
RUN curl -fsSL https://tailscale.com/install.sh | sh
RUN mkdir -p /workspace
WORKDIR /app
COPY --from=builder /app/dist/pocketbrain /app/pocketbrain
COPY --from=builder /app/dist/pocketbrain-mcp /app/dist/pocketbrain-mcp
COPY scripts/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
ENV WORKSPACE_DIR=/workspace
VOLUME /workspace
ENTRYPOINT ["/entrypoint.sh"]
