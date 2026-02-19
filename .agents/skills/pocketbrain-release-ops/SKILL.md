---
name: pocketbrain-release-ops
description: Execute a safe PocketBrain release with preflight checks, health verification, and rollback readiness.
compatibility: opencode
metadata:
  audience: maintainers
  scope: release
---

## What I do

- Apply release checklist gates
- Execute the canonical release workflow
- Verify post-release health and behavior

## When to use me

Use this for every production or staging release candidate.

## Canonical references

- Primary workflow: `docs/runbooks/release-ops.md`
- Deploy context: `docs/runbooks/runtime-deploy.md`

## Release record

Capture:
- tag
- git SHA
- key changes
- known risks
- verification evidence
