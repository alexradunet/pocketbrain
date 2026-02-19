# Runbooks

Canonical operational workflows for PocketBrain.

## Available runbooks

- `docs/runbooks/runtime-install.md` - first-time Debian install to healthy runtime
- `docs/runbooks/runtime-deploy.md` - runtime deploy/update/verify workflow
- `docs/runbooks/dev-setup.md` - contributor machine setup and validation
- `docs/runbooks/release-ops.md` - release preflight, deploy, and verification
- `docs/runbooks/incident-response.md` - first-response triage and recovery workflow
- `docs/runbooks/security-ops.md` - secret rotation, dependency hygiene, and risk updates
- `docs/runbooks/doctor.md` - runtime diagnostics and guided repair workflow
- `docs/runbooks/syncthing-ops.md` - syncthing API setup, health checks, and safe scan operations
- `docs/runbooks/ci-e2e.md` - CI OpenCode model-path E2E setup and troubleshooting

## Rules

- Runbooks are the source of truth for operational steps.
- Skills should reference runbooks and keep only task framing + success criteria.
- If workflow commands change, update runbooks first.
