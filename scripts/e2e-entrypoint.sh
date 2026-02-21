#!/usr/bin/env sh
# E2E entrypoint — seeds the workspace data then starts PocketBrain with MockChannel.
# No Tailscale, no WhatsApp auth.
set -eu

WORKSPACE_DIR="${WORKSPACE_DIR:-/workspace}"

mkdir -p "${WORKSPACE_DIR}/data/chats/main" \
         "${WORKSPACE_DIR}/data/ipc" \
         "${WORKSPACE_DIR}/data/logs"

# Write minimal AGENTS.md so the agent has instructions in the test environment
cat > "${WORKSPACE_DIR}/data/chats/main/AGENTS.md" << 'EOF'
# E2E Test Environment

You are PocketBrain running in an automated end-to-end test suite.
Respond to every message helpfully and concisely.
Do not ask for clarification — just answer directly.
EOF

# Seed chat config so PocketBrain's message loop
# recognises the mock JID immediately on startup.
bun run /app/scripts/e2e-seed.ts

exec bun run /app/src/index.ts
