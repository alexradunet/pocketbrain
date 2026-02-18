# PocketBrain Product Readiness Plan

## 1) Objective

Prepare PocketBrain for reliable, secure, repeatable deployment and day-2 operations by closing architecture, documentation, development, and deployment gaps.

## 2) Scope and Success Criteria

### Scope

- Application runtime in `pocketbrain/`
- Workspace-level operational scripts in `scripts/`
- Documentation in `docs/`
- CI/CD workflows in `.github/workflows/`

### Product-Readiness Exit Criteria

1. Runtime persistence is correct and survives restart/redeploy.
2. CI gates enforce typecheck, tests, and build before merge.
3. Release process is deterministic with tested rollback.
4. Observability and incident response are documented and usable.
5. Security baseline (dependency hygiene, config validation, secrets handling) is enforced.

## 3) Current-State Findings (Evidence-Backed)

### Architecture and Runtime

- Data path mismatch risk:
  - DB is created under `process.cwd()/.data` in `pocketbrain/src/store/db.ts`.
  - Compose mounts persistent volume at `/data` in `pocketbrain/docker-compose.yml`.
  - This creates risk of ephemeral runtime state if app writes `.data` instead of `/data`.
- Health checks are weak for readiness confidence:
  - Health logic allows Tailscale check to pass with `|| true` in both `pocketbrain/docker-compose.yml` and `pocketbrain/Dockerfile`.
  - This can produce false positives.

### Development Quality

- `bun run typecheck` passes.
- `bun build --compile --minify --outfile ...` succeeds.
- `bun test` currently fails in DB tests (`tests/store/db.test.ts`) with readonly DB errors.

### CI/CD and Governance

- Only structure contract workflow exists: `.github/workflows/structure-contract.yml`.
- No mandatory CI jobs for typecheck/tests/build/security scans.

### Documentation and Operations

- Good deployment docs exist (`pocketbrain/DOCKER.md`, `docs/runbooks/debian-zero-to-deploy.md`).
- Missing consolidated product docs set:
  - No architecture decision log set (ADRs) for key runtime decisions.
  - No SLO/SLI targets.
  - No incident response matrix and on-call runbook.
  - No formal release checklist and definition of done.

## 4) Workstreams

1. Runtime Correctness
2. Engineering Quality Gates
3. Release and Deployment Reliability
4. Observability and Operations
5. Security and Compliance Baseline
6. Product Documentation Pack

## 5) Phased Execution Plan

## Phase 0 - Blockers (Week 1)

### Goal

Eliminate correctness risks that block trustworthy releases.

### Deliverables

- Align all runtime state paths to one canonical persistent location (`/data` in containers, explicit local default for dev).
- Fix deterministic DB tests.
- Tighten health checks to reflect actual app readiness.

### Tasks

1. **Canonicalize persistence paths**
   - Define one source of truth for data root in config.
   - Update DB, WhatsApp auth, vault, temp skill install paths to use that source.
   - Update docs and `.env.example` to match runtime behavior.
2. **Stabilize failing tests**
   - Fix readonly DB behavior in `tests/store/db.test.ts` setup/cleanup.
   - Ensure test suite is repeatable across local and CI.
3. **Harden health checks**
   - Require app process readiness and critical dependency readiness.
   - Remove permissive success conditions that can mask failures.

### Acceptance Criteria

- Restart test proves state persistence across container restart.
- `bun test` passes in clean environment.
- Health shows `unhealthy` when critical dependency is down.

## Phase 1 - Engineering Quality Gates (Week 2)

### Goal

Make quality enforcement automatic and non-optional.

### Deliverables

- CI workflows for typecheck, unit tests, optional gated e2e, and Docker build.
- Dependency and image version pinning policy.
- Config validation with fail-fast startup.

### Tasks

1. Add CI workflow matrix in `.github/workflows/`:
   - Install deps
   - `bun run typecheck`
   - `bun test` (excluding env-dependent e2e unless configured)
   - Docker build smoke
2. Replace floating `latest` where runtime critical (`package.json`, image tags) with pinned versions.
3. Add explicit configuration validation for required/optional env vars.

### Acceptance Criteria

- PR merge blocked when any quality job fails.
- Runtime startup fails with clear error when required config is invalid.
- Build + test status visible and reproducible in CI logs.

## Phase 2 - Release and Deployment Reliability (Week 3)

### Goal

Make release outcomes predictable, auditable, and reversible.

### Deliverables

- Hardened release pipeline built on `scripts/ops/release.sh` and `scripts/ops/dev-release.sh`.
- Explicit release checklist and rollback verification protocol.
- Artifact/version traceability from commit to deployed container.

### Tasks

1. Add pre-release checks (typecheck/test/build) to CI and release docs.
2. Ensure deployment uses immutable version tags and records git SHA.
3. Add rollback validation step (not just rollback action).
4. Add post-deploy smoke checks for core flows.

### Acceptance Criteria

- Release dry-run and real release follow same documented path.
- Rollback drill demonstrates recovery within target time.
- Deployment metadata (version/SHA/time) is queryable from logs.

## Phase 3 - Observability and Operations (Week 4)

### Goal

Enable fast detection, diagnosis, and recovery in production.

### Deliverables

- Structured logs with correlation context.
- Minimal operational telemetry strategy (log-based first, metrics next).
- Incident response runbook and escalation path.

### Tasks

1. Standardize logging fields across startup, message handling, heartbeat, and plugin actions.
2. Define operational signals:
   - Message processing success/failure rate
   - Heartbeat failure streak
   - Outbox backlog size/retry depth
3. Create incident playbooks:
   - App not starting
   - Tailscale auth/network failure
   - DB corruption or permission issues
   - WhatsApp reconnect loops

### Acceptance Criteria

- On-call can diagnose top 3 incident types from runbook + logs only.
- MTTD/MTTR baselines defined and tracked.

## Phase 4 - Security and Documentation Completion (Week 5)

### Goal

Finalize production baseline and handoff quality.

### Deliverables

- Threat model and security hardening checklist.
- Secrets handling and rotation guide.
- Complete docs pack for engineering and operations.

### Tasks

1. Document trust boundaries and threat scenarios.
2. Define dependency update cadence and vulnerability response process.
3. Create or complete docs:
   - Architecture overview
   - ADR set for major design decisions
   - Release checklist
   - Operational runbook index
   - Security baseline guide

### Acceptance Criteria

- New engineer can deploy and operate from docs without tribal knowledge.
- Security checklist completed with explicit residual risk log.

## 6) Prioritized Backlog (Execution-Ready)

| ID | Priority | Item | Owner | Depends On | Done When |
|---|---|---|---|---|---|
| PB-001 | P0 | Canonical data path and persistence fix | Backend | - | Restart preserves DB/auth/vault state |
| PB-002 | P0 | Fix readonly DB tests | Backend/QA | PB-001 | `bun test` fully green |
| PB-003 | P0 | Real readiness/liveness checks | Backend/DevOps | PB-001 | Dependency outage marks service unhealthy |
| PB-004 | P1 | CI quality gates (typecheck/test/build) | DevOps | PB-002 | Required checks block bad merges |
| PB-005 | P1 | Version pinning policy implementation | Backend | PB-004 | No runtime-critical `latest` remains |
| PB-006 | P1 | Config schema and fail-fast validation | Backend | PB-001 | Invalid env fails startup clearly |
| PB-007 | P2 | Release hardening + rollback drill | DevOps | PB-004 | Rollback test passes in drill |
| PB-008 | P2 | Observability baseline + incident playbooks | Backend/DevOps | PB-003 | Top incidents diagnosable by runbook |
| PB-009 | P2 | Backup/restore automation and drill | DevOps/QA | PB-001 | Restore succeeds on clean instance |
| PB-010 | P3 | Full documentation pack + ADRs | Architect/Tech Writer | PB-001..009 | Docs cover build/deploy/operate/security |

## 7) Verification Gates

- **Gate A (End Phase 0):** Runtime correctness proven (persistence + health + tests).
- **Gate B (End Phase 1):** CI enforcement active and reliable.
- **Gate C (End Phase 3):** Operational drill passed with runbook-only response.
- **Gate D (End Phase 4):** Security and documentation sign-off complete.

No phase advances without passing its gate.

## 8) Risk Register and Mitigation

| Risk | Impact | Mitigation |
|---|---|---|
| Data loss from path misalignment | High | PB-001 first, persistence tests mandatory |
| False healthy status | High | PB-003, strict readiness criteria |
| Regressions from missing CI | High | PB-004 required checks |
| Drift between docs and runtime | Medium | Doc update in same PR as behavior change |
| Unbounded dependency drift | Medium | PB-005 pinning + update cadence |

## 9) Suggested Team Cadence

- Weekly readiness review (30 minutes) against PB-001..PB-010.
- Every PR maps to one backlog ID and one acceptance criterion.
- End of each phase includes demo + checklist sign-off.

## 10) Definition of Ready and Done

### Definition of Ready

- Backlog item has owner, dependencies, acceptance criteria, and verification command(s).

### Definition of Done

- Code/documentation merged.
- Acceptance criteria satisfied.
- Verification evidence attached (command output, screenshots, logs, or drill notes).
- Related runbook/docs updated.
