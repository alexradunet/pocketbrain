SHELL := /usr/bin/env bash

.PHONY: setup-dev setup-runtime dev test build up down ps logs release dev-release backup restore shell shell-dev

setup-dev:
	./scripts/setup/install-debian-dev.sh

setup-runtime:
	./scripts/setup/install-debian-runtime.sh

dev:
	bun run dev

test:
	bun run test

build:
	bun run build

up:
	docker compose -p pocketbrain-runtime -f docker-compose.yml up -d --build

down:
	docker compose -p pocketbrain-runtime -f docker-compose.yml down

ps:
	docker compose -p pocketbrain-runtime -f docker-compose.yml ps

logs:
	./scripts/ops/docker-logs.sh $(ARGS)

release:
	./scripts/ops/release.sh $(TAG)

dev-release:
	./scripts/ops/dev-release.sh $(TAG)

backup:
	./scripts/ops/backup.sh

restore:
	@if [ -z "$(FILE)" ]; then echo "Usage: make restore FILE=<backup-tar.gz>"; exit 1; fi
	./scripts/ops/restore.sh "$(FILE)"

shell:
	./scripts/ops/docker-shell.sh $(ARGS)

shell-dev:
	./scripts/ops/docker-shell.sh --dev $(ARGS)
