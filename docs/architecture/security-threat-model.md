# PocketBrain Security Threat Model

## Scope

- Runtime application in `internal/`
- Runtime data in `.data/` (`DATA_DIR`)
- Runtime process: single Go binary managed by systemd
- Deployment on Debian VPS

## Assets

- User conversation data and memory facts (`state.db` tables: `memory`, `session`, `outbox`, `whitelist`).
- WhatsApp auth state (`DATA_DIR/whatsapp-auth`).
- Workspace content (`DATA_DIR/workspace`).
- Runtime credentials (provider API keys in `.env`).

## Trust Boundaries

1. External network -> WhatsApp edge.
2. Runtime process -> host filesystem (`.data/`).
3. Operator shell -> systemd service control.
4. Runtime process -> AI provider endpoints (Anthropic, OpenAI-compatible, Google).

## Primary Threats and Controls

### 1) Unauthorized messaging access

- Threat: untrusted user interacts with assistant channel.
- Current controls:
  - Whitelist gate in `whitelist` table.
  - Operator-managed phone-number whitelist via `WHATSAPP_WHITELIST_NUMBERS`.
  - Unknown users are rejected until explicitly whitelisted.
  - `/pair` self-service onboarding is disabled.
- Required controls:
  - Audit `whitelist` entries weekly.

### 2) Data loss or tampering in runtime state

- Threat: accidental deletion/corruption of `.data/` or rollback to stale state.
- Current controls:
  - Canonical data path config and persistent volume usage.
  - SQLite WAL mode for crash resilience.
  - VPS/provider snapshot and backup capabilities.
- Required controls:
  - Weekly backup/restore drill with documented evidence.
  - Immutable backup storage copy outside runtime host.

### 3) Secret leakage

- Threat: secrets committed or exposed in logs/process output.
- Current controls:
  - `.env` is gitignored and never bundled with application code.
  - Structured logging via `slog`.
- Required controls:
  - Never commit `.env` or raw credentials.
  - Redaction review for new logs touching auth/config values.

### 4) Supply chain drift

- Threat: unpinned dependencies introduce unreviewed changes.
- Current controls:
  - Dependencies pinned in `go.mod` and `go.sum`.
  - Single static binary with no runtime dependencies.
- Required controls:
  - Monthly dependency refresh window and regression run.
  - Critical CVE response within 48h (`govulncheck` if available).

### 5) False healthy deployments

- Threat: system reports healthy while key dependencies are broken.
- Current controls:
  - Runtime startup checks and structured logs.
  - CI quality gates: build, test with race detection, vet.
- Required controls:
  - Keep health checks strict and review when startup logic changes.
  - Include rollback-health validation on every release.

### 6) Capability drift beyond workspace-only profile

- Threat: tool or prompt changes accidentally enable system command behavior in chat runtime.
- Current controls:
  - Runtime prompt declares workspace-only capability boundaries.
  - Tool registry limited to workspace, memory, channel, and skill tools.
- Required controls:
  - Review tool registry whenever tool surface changes.
  - Review prompt boundary text on tool additions.

## Residual Risk Register

| ID | Risk | Current Posture | Residual Level | Mitigation Owner |
|---|---|---|---|---|
| RR-01 | WhatsApp provider/runtime external dependency outage | No local failover | Medium | Operations |
| RR-02 | Human error in VPS snapshot/restore selection | Manual provider workflow | Medium | Operations |
| RR-03 | Secret rotation lag | Policy added, enforcement manual | Medium | Security/Operations |

## Security Definition of Done

- Threat model reviewed quarterly.
- Residual risk register updated after each sev-1/sev-2 incident.
- Secret rotation and dependency cadence evidence attached to release notes.
