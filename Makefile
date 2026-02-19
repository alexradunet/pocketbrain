SHELL := /usr/bin/env bash

.PHONY: setup-dev setup-runtime setup dev start test typecheck build logs release backup restore shell

setup-dev:
	./scripts/setup/install-debian-dev.sh

setup-runtime:
	./scripts/setup/install-debian-runtime.sh

setup:
	bun run setup

dev:
	bun run dev

start:
	bun run start

test:
	bun run test

typecheck:
	bun run typecheck

build:
	bun run build

logs:
	./scripts/ops/runtime-logs.sh $(ARGS)

release:
	./scripts/ops/release.sh $(TAG)

backup:
	./scripts/ops/backup.sh

restore:
	@if [ -z "$(FILE)" ]; then echo "Usage: make restore FILE=<backup-tar.gz>"; exit 1; fi
	./scripts/ops/restore.sh "$(FILE)"

shell:
	./scripts/ops/runtime-shell.sh $(ARGS)
