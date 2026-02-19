# PocketBrain Security Threat Model

## Scope

- Runtime application in `src/`
- Runtime data in `data/` (`DATA_DIR`)
- Runtime process managed by Bun/service manager
- Operational scripts in `scripts/ops/` and provider snapshot tooling

## Assets

- User conversation data and memory facts (`state.db` tables: `memory`, `kv`, `outbox`, `whitelist`).
- WhatsApp auth state (`DATA_DIR/whatsapp-auth`).
- Vault content (`DATA_DIR/vault`).
- Runtime credentials (`TS_AUTHKEY`, `WHITELIST_PAIR_TOKEN`, provider auth via OpenCode config).

## Trust Boundaries

1. External network -> WhatsApp/Tailscale edge.
2. Runtime process -> host filesystem (`data/`).
3. Operator shell -> scripts that control release/runtime + VPS/provider backup tooling.
4. OpenCode runtime -> model provider endpoints.

## Primary Threats and Controls

### 1) Unauthorized messaging access

- Threat: untrusted user interacts with assistant channel.
- Current controls:
  - Whitelist gate in `whitelist` table.
  - Pair-token flow (`/pair <token>`) with timing-safe comparison.
- Required controls:
  - Rotate `WHITELIST_PAIR_TOKEN` monthly or after suspected exposure.
  - Audit `whitelist` entries weekly.

### 2) Data loss or tampering in runtime state

- Threat: accidental deletion/corruption of `/data` or rollback to stale state.
- Current controls:
  - Canonical data path config and persistent volume usage.
  - VPS/provider snapshot and backup capabilities.
- Required controls:
  - Weekly backup/restore drill via VPS/provider workflow with documented evidence.
  - Immutable backup storage copy outside runtime host.

### 3) Secret leakage

- Threat: secrets committed or exposed in logs/process output.
- Current controls:
  - `.env` is never bundled with application code.
  - Structured logging for app flow.
- Required controls:
  - Never commit `.env` or raw credentials.
  - Redaction review for new logs touching auth/config values.

### 4) Supply chain drift

- Threat: unpinned dependencies introduce unreviewed changes.
- Current controls:
  - App dependencies pinned in `package.json` and `bun.lock`.
  - Runtime dependency versions pinned in lockfiles where applicable.
- Required controls:
  - Monthly dependency refresh window and regression run.
  - Critical CVE response within 48h.

### 5) False healthy deployments

- Threat: system reports healthy while key dependencies are broken.
- Current controls:
  - Runtime startup checks and structured logs.
  - Release script waits for healthy services.
- Required controls:
  - Keep health checks strict and review when startup logic changes.
  - Include rollback-health validation on every release.

## Residual Risk Register

| ID | Risk | Current Posture | Residual Level | Mitigation Owner |
|---|---|---|---|---|
| RR-01 | WhatsApp provider/runtime external dependency outage | No local failover | Medium | Operations |
| RR-02 | Human error in manual restore selection | Scripted restore, no guardrail by environment | Medium | Operations |
| RR-03 | Secret rotation lag | Policy added, enforcement manual | Medium | Security/Operations |
| RR-04 | E2E model-path test skipped when secret missing | CI has optional gate | Low-Medium | DevOps |

## Security Definition of Done

- Threat model reviewed quarterly.
- Residual risk register updated after each sev-1/sev-2 incident.
- Secret rotation and dependency cadence evidence attached to release notes.
