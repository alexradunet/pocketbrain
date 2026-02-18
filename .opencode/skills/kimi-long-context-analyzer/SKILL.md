---
name: kimi-long-context-analyzer
description: Analyze large code/docs contexts with structured extraction optimized for Kimi K2.5
---
## Purpose
Use this skill when the task requires reading large amounts of code or documentation before action.

## Input format
- Goal: one sentence
- Scope: explicit paths/modules
- Constraints: what must not change
- Output schema: bullets under Findings, Evidence, Risks, Actions

## Method
1. Chunk source material by subsystem.
2. Extract facts, assumptions, and unknowns separately.
3. Link every conclusion to evidence.
4. Return a concise action plan with validation steps.

## Guardrails
- Never infer behavior without evidence.
- Separate observed facts from hypotheses.
- Flag version-sensitive guidance explicitly.
