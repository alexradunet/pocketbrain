# Agent Skills

PocketBrain supports dynamic skills that can be loaded, created, and installed from GitHub.

## Skill tools

The AI assistant has access to these skill management tools:

- `skill_list` — list all installed skills
- `skill_load` — load a skill by name
- `skill_create` — create a new skill
- `skill_install` — install a skill from a GitHub repository

## Skill location

Skills are stored in the workspace under `.data/workspace/skills/`.

## Usage

Ask the assistant to manage skills:

- "List my skills"
- "Create a skill for daily standup reports"
- "Install the skill from github.com/user/repo"

## Authoring rules

Skills are markdown files with frontmatter metadata. See `internal/skills/` for the implementation.
