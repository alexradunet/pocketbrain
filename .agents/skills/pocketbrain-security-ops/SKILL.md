---
name: pocketbrain-security-ops
description: Apply PocketBrain security operations policy for secret rotation, dependency hygiene, and risk tracking.
compatibility: opencode
metadata:
  audience: operators
  scope: security
---

## What I do

- Enforce secret handling and rotation cadence
- Apply dependency/image update cadence and validation
- Keep residual risk tracking up to date

## In scope

- `TS_AUTHKEY`
- `WHITELIST_PAIR_TOKEN`
- OpenCode/provider credentials on runtime hosts and CI

## Secret rules

1. Store secrets only in runtime `.env` or CI secret store.
2. Never commit secrets to repository files or logs.
3. Rotate on cadence:
- `TS_AUTHKEY`: every 30 days
- `WHITELIST_PAIR_TOKEN`: every 30 days
- provider credentials: every 60 days or immediately after exposure
4. Rotate immediately after any auth/host compromise incident.

## Rotation procedure

1. Generate new secret in source system.
2. Update runtime `.env` and CI secret if applicable.
3. Restart runtime stack.
4. Validate health and one functional command path.
5. Record timestamp in release notes.

## Dependency/image cadence

- Weekly: vulnerability scan review
- Monthly: scheduled dependency/image refresh + full checks
- Critical CVEs: patch within 48 hours

## Update procedure

```bash
bun run typecheck
bun run test:processes
bun run test
bun run build
make release TAG=$(git rev-parse --short HEAD)
```

## Residual risk updates

Update risk records whenever:
- new external dependency is added
- security control changes
- incident reveals missed control

Required fields:
- risk ID
- description
- impact
- likelihood
- owner
- next mitigation action
