# Security Operations Policy

## 1) Secrets Handling and Rotation

## In Scope

- `TS_AUTHKEY`
- `WHITELIST_PAIR_TOKEN`
- OpenCode/provider credentials used by runtime host

## Rules

1. Store secrets only in runtime `.env` or CI secret store.
2. Never commit secrets to repository files or logs.
3. Rotate all runtime secrets on this cadence:
   - `TS_AUTHKEY`: every 30 days
   - `WHITELIST_PAIR_TOKEN`: every 30 days
   - Provider credentials: every 60 days or immediately after exposure event
4. Rotate immediately after any incident involving auth or host compromise.

## Rotation Procedure

1. Generate new secret/token in source system.
2. Update runtime `.env` (and CI secret where applicable).
3. Restart runtime stack.
4. Validate health and one functional command path.
5. Record rotation timestamp in release notes.

## 2) Dependency and Image Update Cadence

## Cadence

- Weekly: vulnerability scan review (dependencies + container images).
- Monthly: scheduled dependency/image refresh with full process test suite.
- Immediate: patch critical vulnerabilities (high/critical CVEs) within 48 hours.

## Update Procedure

1. Update pinned versions in `package.json`, `bun.lock`, and compose image tags.
2. Run:
   - `bun run typecheck`
   - `bun run test:processes`
   - `bun run test`
   - `bun run build`
3. Run runtime release script and health verification.
4. Document changes and known impact.

## 3) Residual Risk Tracking Process

Every phase/release must update residual risk entries when:

- A new external dependency is added.
- A security control changes.
- An incident reveals a missed control.

Risk record minimum fields:

- Risk ID
- Description
- Impact
- Likelihood
- Owner
- Next mitigation action
