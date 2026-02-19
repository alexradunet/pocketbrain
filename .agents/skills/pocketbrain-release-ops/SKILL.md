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
- Run managed release workflow
- Validate runtime health and post-deploy behavior

## When to use me

Use this for every production or staging release candidate.

## Canonical workflow

1. Pre-release checks:

```bash
bun run typecheck
bun run test:processes
bun run test
bun run build
```

2. Deploy release:

```bash
make release TAG=$(git rev-parse --short HEAD)
```

3. Validate:
- `pocketbrain` and `syncthing` are healthy
- startup logs include expected version/git SHA
- one end-to-end command path works (`/new` or `/remember`)

4. Rollback readiness:
- identify previous known-good tag
- ensure auto-rollback path remains available in `scripts/ops/release.sh`

## Release record

Capture:
- tag
- git SHA
- key changes
- known risks
- verification evidence
