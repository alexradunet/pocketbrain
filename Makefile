SHELL := /usr/bin/env bash
BINARY := pocketbrain

.PHONY: build dev test clean setup logs release shell doctor

build:
	go build -o $(BINARY) .

dev:
	go run . start

start:
	go run . start --headless

test:
	go test ./... -count=1

setup:
	go run . setup

clean:
	rm -f $(BINARY)
	go clean -cache

logs:
	./scripts/ops/runtime-logs.sh $(ARGS)

release:
	./scripts/ops/release.sh $(TAG)

shell:
	./scripts/ops/runtime-shell.sh $(ARGS)

doctor:
	./scripts/ops/doctor.sh $(ARGS)
