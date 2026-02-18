# Agent Skills Catalog

PocketBrain externalizes repeatable operational workflows as skills so OpenCode agents can load them on demand.

## Skill location

- Project-local skills: `.agents/skills/<name>/SKILL.md`

## Current skills

- `pocketbrain-install` - zero-to-healthy first install on Debian
- `pocketbrain-runtime-deploy` - runtime deployment and health verification
- `pocketbrain-dev-setup` - contributor machine setup and dev validation
- `pocketbrain-release-ops` - release checklist and managed deployment
- `pocketbrain-backup-restore` - backup and restore drills
- `pocketbrain-incident-response` - first-response triage and recovery
- `pocketbrain-security-ops` - secret rotation, dependency hygiene, and residual risk process
- `pocketbrain-ci-e2e` - CI E2E secret setup and validation behavior

## Usage

In OpenCode, ask explicitly for the skill you want to apply, for example:

- "Use `pocketbrain-runtime-deploy` to deploy this host"
- "Use `pocketbrain-release-ops` for this release"
- "Use `pocketbrain-incident-response` to triage this outage"

## Authoring rules

- Skill file must be named `SKILL.md`
- Frontmatter requires `name` and `description`
- Skill name must match directory name and use lowercase hyphenated format
