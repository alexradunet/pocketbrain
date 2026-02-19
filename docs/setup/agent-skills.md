# Agent Skills Catalog

PocketBrain externalizes repeatable workflows as OpenCode skills.

## Skill location

- Active runtime skills: `$OPENCODE_CONFIG_DIR/.agents/skills/<name>/SKILL.md`
- Default runtime path when vault is enabled: `VAULT_PATH/99-system/99-pocketbrain/.agents/skills/`
- Repository copies under `.agents/skills/` are treated as bundled defaults and can be seeded into the runtime path.

## Current skills

- `pocketbrain-install` - zero-to-healthy first install on Debian
- `pocketbrain-runtime-deploy` - runtime deployment and health verification
- `pocketbrain-dev-setup` - contributor machine setup and dev validation
- `pocketbrain-release-ops` - release checklist and managed deployment
- `pocketbrain-incident-response` - first-response triage and recovery
- `pocketbrain-security-ops` - secret rotation, dependency hygiene, and residual risk process
- `pocketbrain-ci-e2e` - CI E2E secret setup and validation behavior
- `pocketbrain-vault-autoconfig` - adapt to each vault's Obsidian settings and conventions

## Usage

Ask explicitly for the skill to apply, for example:

- "Use `pocketbrain-runtime-deploy` to deploy this host"
- "Use `pocketbrain-release-ops` for this release"
- "Use `pocketbrain-incident-response` to triage this outage"

Each skill references canonical runbooks in `docs/runbooks/`.

## Authoring rules

- Skill file must be named `SKILL.md`
- Frontmatter requires `name` and `description`
- Skill name must match directory name and use lowercase hyphenated format
