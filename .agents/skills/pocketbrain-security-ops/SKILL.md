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
- Apply dependency hygiene and validation
- Keep residual risk tracking up to date

## In scope

- `WHITELIST_PAIR_TOKEN`
- OpenCode/provider credentials on runtime hosts and CI

## Canonical references

- Primary workflow: `docs/runbooks/security-ops.md`
- Security model: `docs/architecture/security-threat-model.md`

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
