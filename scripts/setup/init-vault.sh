#!/usr/bin/env bash
set -euo pipefail

TARGET_DIR="${1:-vault}"
FORCE="${2:-}"

write_file() {
  local file="$1"
  local content="$2"

  if [ -f "$file" ] && [ "$FORCE" != "--force" ]; then
    return 0
  fi

  cat >"$file" <<EOF
$content
EOF
}

mkdir -p \
  "$TARGET_DIR/00-inbox/capture" \
  "$TARGET_DIR/00-inbox/to-process" \
  "$TARGET_DIR/01-daily-journey/daily" \
  "$TARGET_DIR/01-daily-journey/weekly" \
  "$TARGET_DIR/01-daily-journey/monthly" \
  "$TARGET_DIR/02-projects/active" \
  "$TARGET_DIR/02-projects/planning" \
  "$TARGET_DIR/02-projects/on-hold" \
  "$TARGET_DIR/02-projects/templates" \
  "$TARGET_DIR/03-areas/health" \
  "$TARGET_DIR/03-areas/work" \
  "$TARGET_DIR/03-areas/finance" \
  "$TARGET_DIR/03-areas/home" \
  "$TARGET_DIR/03-areas/relationships" \
  "$TARGET_DIR/03-areas/learning" \
  "$TARGET_DIR/04-resources/articles" \
  "$TARGET_DIR/04-resources/books" \
  "$TARGET_DIR/04-resources/reference" \
  "$TARGET_DIR/04-resources/snippets" \
  "$TARGET_DIR/04-resources/media" \
  "$TARGET_DIR/05-archive/projects" \
  "$TARGET_DIR/05-archive/areas" \
  "$TARGET_DIR/05-archive/resources" \
  "$TARGET_DIR/05-archive/journal" \
  "$TARGET_DIR/99-system/templates" \
  "$TARGET_DIR/99-system/prompts" \
  "$TARGET_DIR/99-system/workflows" \
  "$TARGET_DIR/99-system/config" \
  "$TARGET_DIR/99-system/99-pocketbrain/.agents/skills" \
  "$TARGET_DIR/99-system/99-pocketbrain/processes" \
  "$TARGET_DIR/99-system/99-pocketbrain/knowledge" \
  "$TARGET_DIR/99-system/99-pocketbrain/runbooks" \
  "$TARGET_DIR/99-system/99-pocketbrain/config"

write_file "$TARGET_DIR/00-inbox/README.md" "# Inbox

## Purpose

Fast capture only. No organizing during capture.

## Folder layout

- \`capture/\` for raw incoming notes
- \`to-process/\` for notes waiting weekly review

## Naming

Use natural names, no numeric prefix required (example: \`meeting-notes-client-a.md\`).

## Rule

Process inbox weekly and move notes into Projects, Areas, Resources, or Archive."

write_file "$TARGET_DIR/01-daily-journey/README.md" "# Daily Journey

## Purpose

Chronological personal notes and reflections.

## Folder layout

- \`daily/\` day notes
- \`weekly/\` weekly reviews
- \`monthly/\` monthly reviews

## Naming

- Daily: \`YYYY-MM-DD.md\`
- Weekly: \`YYYY-Www.md\`
- Monthly: \`YYYY-MM.md\`"

write_file "$TARGET_DIR/02-projects/README.md" "# Projects (PARA)

## Purpose

Short-term efforts with a clear outcome and finish line.

## Folder layout

- \`active/\` currently in progress
- \`planning/\` defined but not started
- \`on-hold/\` paused projects
- \`templates/\` reusable project templates

## Naming

Use clear slug names (example: \`website-redesign.md\`)."

write_file "$TARGET_DIR/03-areas/README.md" "# Areas (PARA)

## Purpose

Long-term responsibilities to maintain over time.

## Folder layout

- \`health/\`
- \`work/\`
- \`finance/\`
- \`home/\`
- \`relationships/\`
- \`learning/\`

## Naming

Use domain-first names (example: \`health-routine.md\`, \`work-goals-2026.md\`)."

write_file "$TARGET_DIR/04-resources/README.md" "# Resources (PARA)

## Purpose

Reference material and reusable knowledge.

## Folder layout

- \`articles/\`
- \`books/\`
- \`reference/\`
- \`snippets/\`
- \`media/\`

## Naming

Use topic-focused names (example: \`sqlite-indexing-notes.md\`)."

write_file "$TARGET_DIR/05-archive/README.md" "# Archive (PARA)

## Purpose

Inactive material kept for history and lookup.

## Folder layout

- \`projects/\`
- \`areas/\`
- \`resources/\`
- \`journal/\`

## Rule

Move completed or inactive items here instead of deleting."

write_file "$TARGET_DIR/99-system/README.md" "# System

## Purpose

Vault operating rules, templates, prompts, and internal configuration notes.

## Folder layout

- \`templates/\`
- \`prompts/\`
- \`workflows/\`
- \`config/\`
- \`99-pocketbrain/\` synced PocketBrain skills, configuration, and process knowledge

## Naming

Use descriptive names by type (example: \`template-daily-note.md\`, \`workflow-weekly-review.md\`)."

write_file "$TARGET_DIR/99-system/99-pocketbrain/README.md" "# PocketBrain

## Purpose

Portable PocketBrain home inside the vault for synced OpenCode config, skills, and process knowledge.

## Folder layout

- \`.agents/skills/\` OpenCode skills used by PocketBrain
- \`processes/\` reusable operating procedures
- \`knowledge/\` source notes PocketBrain should reference
- \`runbooks/\` PocketBrain-specific operational guides
- \`config/\` non-secret PocketBrain config files

## Notes

- Runtime caches and machine-local state stay outside the vault.
- Secrets should remain in machine-local env files, not inside the vault."

printf 'Vault scaffold ready at: %s\n' "$TARGET_DIR"
if [ "$FORCE" != "--force" ]; then
  printf 'Existing README.md files were preserved. Use --force to rewrite them.\n'
fi
