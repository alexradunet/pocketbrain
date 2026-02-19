# Release Operations Runbook

Use this runbook for every staging or production release candidate.

## 1) Pre-release checks

```bash
bun run typecheck
bun run test:processes
bun run test
bun run build
```

## 2) Execute release script

```bash
make release TAG=$(git rev-parse --short HEAD)
```

## 3) Verify runtime

```bash
make logs
```

Validation checklist:
- startup logs include expected commit/tag context
- no repeated startup/runtime errors
- one end-to-end command path succeeds (`/new` or `/remember`)

## 4) Release record

Capture:
- tag
- git SHA
- key changes
- known risks
- verification evidence
