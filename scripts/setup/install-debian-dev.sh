#!/usr/bin/env bash
set -euo pipefail

sudo apt update
sudo apt install -y ca-certificates curl gnupg lsb-release git zip unzip

curl -fsSL https://bun.sh/install | bash

export BUN_INSTALL="${HOME}/.bun"
export PATH="${BUN_INSTALL}/bin:${PATH}"

bun install

if [ ! -f .env ] && [ -f .env.example ]; then
  cp .env.example .env
fi

printf 'Developer setup completed. Next steps:\n'
printf '  1) bun run setup\n'
printf '  2) bun run dev\n'
