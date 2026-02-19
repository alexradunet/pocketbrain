SHELL := /usr/bin/env bash

.PHONY: setup-dev setup-runtime setup dev start test typecheck build logs release shell

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

shell:
	./scripts/ops/runtime-shell.sh $(ARGS)
