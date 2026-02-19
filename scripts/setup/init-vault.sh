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
  "$TARGET_DIR/00_inbox/capture" \
  "$TARGET_DIR/00_inbox/to_process" \
  "$TARGET_DIR/01_daily_journal/daily" \
  "$TARGET_DIR/01_daily_journal/weekly" \
  "$TARGET_DIR/01_daily_journal/monthly" \
  "$TARGET_DIR/02_projects/active" \
  "$TARGET_DIR/02_projects/planning" \
  "$TARGET_DIR/02_projects/on_hold" \
  "$TARGET_DIR/02_projects/templates" \
  "$TARGET_DIR/03_areas/health" \
  "$TARGET_DIR/03_areas/work" \
  "$TARGET_DIR/03_areas/finance" \
  "$TARGET_DIR/03_areas/home" \
  "$TARGET_DIR/03_areas/relationships" \
  "$TARGET_DIR/03_areas/learning" \
  "$TARGET_DIR/04_resources/articles" \
  "$TARGET_DIR/04_resources/books" \
  "$TARGET_DIR/04_resources/reference" \
  "$TARGET_DIR/04_resources/snippets" \
  "$TARGET_DIR/04_resources/media" \
  "$TARGET_DIR/05_archive/projects" \
  "$TARGET_DIR/05_archive/areas" \
  "$TARGET_DIR/05_archive/resources" \
  "$TARGET_DIR/05_archive/journal" \
  "$TARGET_DIR/99_system/templates" \
  "$TARGET_DIR/99_system/prompts" \
  "$TARGET_DIR/99_system/workflows" \
  "$TARGET_DIR/99_system/config"

write_file "$TARGET_DIR/00_inbox/README.md" "# Inbox

## Purpose

Fast capture only. No organizing during capture.

## Folder layout

- \`capture/\` for raw incoming notes
- \`to_process/\` for notes waiting weekly review

## Naming

Use natural names, no numeric prefix required (example: \`meeting-notes-client-a.md\`).

## Rule

Process inbox weekly and move notes into Projects, Areas, Resources, or Archive."

write_file "$TARGET_DIR/01_daily_journal/README.md" "# Daily Journal

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

write_file "$TARGET_DIR/02_projects/README.md" "# Projects (PARA)

## Purpose

Short-term efforts with a clear outcome and finish line.

## Folder layout

- \`active/\` currently in progress
- \`planning/\` defined but not started
- \`on_hold/\` paused projects
- \`templates/\` reusable project templates

## Naming

Use clear slug names (example: \`website-redesign.md\`)."

write_file "$TARGET_DIR/03_areas/README.md" "# Areas (PARA)

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

write_file "$TARGET_DIR/04_resources/README.md" "# Resources (PARA)

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

write_file "$TARGET_DIR/05_archive/README.md" "# Archive (PARA)

## Purpose

Inactive material kept for history and lookup.

## Folder layout

- \`projects/\`
- \`areas/\`
- \`resources/\`
- \`journal/\`

## Rule

Move completed or inactive items here instead of deleting."

write_file "$TARGET_DIR/99_system/README.md" "# System

## Purpose

Vault operating rules, templates, prompts, and internal configuration notes.

## Folder layout

- \`templates/\`
- \`prompts/\`
- \`workflows/\`
- \`config/\`

## Naming

Use descriptive names by type (example: \`template-daily-note.md\`, \`workflow-weekly-review.md\`)."

printf 'Vault scaffold ready at: %s\n' "$TARGET_DIR"
if [ "$FORCE" != "--force" ]; then
  printf 'Existing README.md files were preserved. Use --force to rewrite them.\n'
fi
