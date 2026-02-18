# Release Checklist

Use this checklist for every release candidate.

## Pre-Release

- [ ] Working tree reviewed and intentional changes only.
- [ ] `bun run typecheck` passes.
- [ ] `bun run test:processes` passes.
- [ ] `bun run test` passes.
- [ ] `bun run build` passes.
- [ ] Backup completed (`./scripts/ops/backup.sh`) and artifact path recorded.
- [ ] Config/secrets reviewed for target environment.

## Deployment

- [ ] Run release flow (`./scripts/release.sh <tag>` or dev-control equivalent).
- [ ] Confirm `pocketbrain` health is `healthy`.
- [ ] Confirm `syncthing` health is `healthy`.
- [ ] Confirm startup logs show expected version and git SHA.

## Post-Deploy Validation

- [ ] Verify one command path (`/new` or `/remember`) end-to-end.
- [ ] Verify no repeating runtime errors in last 100 logs.
- [ ] Verify outbox behavior for proactive message enqueue path.

## Rollback Preparedness

- [ ] Previous known-good image/tag identified.
- [ ] Rollback path tested or ready (`scripts/ops/release.sh` auto-rollback).

## Release Record

- [ ] Release notes updated with:
  - Tag
  - Git SHA
  - Key changes
  - Known risks
  - Verification evidence
