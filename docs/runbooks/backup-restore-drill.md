# Backup and Restore Drill

## Goal

Verify that PocketBrain runtime state can be backed up and restored within the expected recovery window.

## Preconditions

- Runtime stack is running under project `pocketbrain-runtime`.
- `pocketbrain/.env` is configured.
- At least one known test message/session exists.

## Drill Steps

1. Create a backup:
   - `./scripts/ops/backup.sh`
   - Note the backup file path.
2. Validate backup artifact exists and is non-empty.
3. Simulate recovery event:
   - Stop stack and restore from backup:
   - `./scripts/ops/restore.sh <backup-file>`
4. Verify service health:
   - `docker compose -p pocketbrain-runtime -f pocketbrain/docker-compose.yml ps`
5. Verify functional recovery:
   - Confirm PocketBrain replies to one test message.
   - Confirm expected vault/data files exist in `pocketbrain/data`.

## Success Criteria

- Backup command exits 0 and produces `.tar.gz` artifact.
- Restore command exits 0 and stack returns to healthy.
- Test message flow works after restore.
- No data integrity anomalies found in restored runtime.

## Drill Frequency

- Run weekly in active development.
- Run before any high-risk migration or major release.

## Drill Record Template

- Date/Time:
- Operator:
- Backup file:
- Restore duration:
- Health verification result:
- Functional verification result:
- Issues found and follow-up actions:
