---
name: pocketbrain-vault-autoconfig
description: Auto-adapt PocketBrain behavior to a specific Obsidian vault using live vault and .obsidian config.
compatibility: opencode
metadata:
  audience: runtime-agent
  scope: vault-operations
---

## What I do

- Read vault configuration using `vault_obsidian_config`.
- Infer vault conventions before creating or moving notes.
- Keep writes aligned with the user's existing folder and naming patterns.

## When to use me

Use this at session start when vault access is enabled, after vault import, or when the user says their structure changed.

## Workflow

1. Run `vault_obsidian_config` and capture daily/new-note/attachment settings.
2. Inspect current structure with `vault_list` (root and key folders from config) and `vault_search` for naming patterns.
3. Mirror existing conventions; do not enforce a fixed PARA/Johnny Decimal schema.
4. If config is missing or contradictory, ask the user one concise clarification question before bulk writes.
5. Re-check configuration after major vault migrations.

## Output expectations

- Explain chosen destination paths before creating new notes.
- Keep note placement and links consistent with the discovered vault conventions.
