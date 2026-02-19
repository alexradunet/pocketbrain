---
name: pocketbrain-dev-setup
description: Set up and validate a PocketBrain developer environment from repository root.
compatibility: opencode
metadata:
  audience: contributors
  scope: development
---

## What I do

- Guide contributor machine setup and validation
- Keep onboarding aligned with canonical local workflow

## When to use me

Use this when onboarding a developer machine or fixing a broken local setup.

## Canonical references

- Primary workflow: `docs/runbooks/dev-setup.md`
- Additional context: `docs/setup/developer-onboarding.md`

## Troubleshooting

- `bun: command not found`: re-open shell and ensure Bun is in PATH.
- `.env` missing: create from `.env.example`.
- test/build errors: run `bun install --frozen-lockfile` and retry.
