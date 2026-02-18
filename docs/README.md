# Documentation

This folder contains all documentation for the PocketBrain workspace.

## Sections

- `architecture/` - repository layout contracts and architectural guidance.
- `runbooks/` - deployment and operations guides.

## Key Documents

- `product-readiness-plan.md` - phased execution plan for product hardening.
- `architecture/security-threat-model.md` - threat model and residual risk register.
- `runbooks/release-checklist.md` - mandatory release gate checklist.
- `runbooks/security-operations-policy.md` - secrets rotation and dependency cadence.
- `runbooks/backup-restore-drill.md` - backup/restore operational drill.
- `runbooks/incident-response.md` - first-response and recovery actions.
- `runbooks/postmortem-template.md` - incident retrospective template.
- `runbooks/ci-e2e-enablement.md` - enabling model-path e2e checks in CI.

## Rule

When adding a new doc, place it in `docs/` (or a subfolder) instead of keeping docs in ad-hoc locations at the repository root.
