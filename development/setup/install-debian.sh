#!/usr/bin/env bash
set -eu

sudo apt update
sudo apt install -y zip unzip git curl

curl -fsSL https://bun.com/install | bash

git clone https://github.com/CefBoud/PocketBrain.git
cd PocketBrain

export BUN_INSTALL="$HOME/.bun"
export PATH="$BUN_INSTALL/bin:$PATH"

bun install
bun run setup

bun run dev
