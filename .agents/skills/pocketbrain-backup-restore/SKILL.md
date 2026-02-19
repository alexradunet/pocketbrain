---
name: pocketbrain-backup-restore
description: Run and verify PocketBrain backup and restore drills for runtime data durability.
compatibility: opencode
metadata:
  audience: operators
  scope: resilience
---

## What I do

- Create runtime backup artifacts
- Restore runtime state from backup
- Verify service and functional recovery

## When to use me

Use this weekly, before high-risk releases, or during recovery events.

## Canonical workflow

1. Create backup:

```bash
make backup
```

2. Validate artifact exists and is non-empty.

3. Restore from backup:

```bash
make restore FILE=<backup-file>
```

4. Verify health:

```bash
make logs
```

5. Verify function:
- assistant responds to one known test command
- expected data exists in `data/`

## Success criteria

- backup command exits 0
- restore command exits 0
- services recover healthy
- no data integrity anomalies observed
