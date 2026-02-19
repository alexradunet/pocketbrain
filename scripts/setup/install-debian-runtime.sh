#!/usr/bin/env bash
set -euo pipefail

sudo apt update
sudo apt install -y ca-certificates curl git

curl -fsSL https://bun.sh/install | bash

printf 'Runtime prerequisites installed. Next steps:\n'
printf '  1) cp .env.example .env\n'
printf '  2) adjust .env for your environment\n'
printf '  3) bun install\n'
printf '  4) bun run setup\n'
printf '  5) bun run start\n'
