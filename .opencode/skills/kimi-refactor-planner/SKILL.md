---
name: kimi-refactor-planner
description: Produce low-risk, test-first refactor plans with rollback-aware sequencing
---
## Purpose
Use this skill before large refactors to minimize churn and reduce token waste from failed iterations.

## Required output
- Refactor objective
- Files impacted
- Step order with checkpoints
- Test strategy per step
- Rollback plan for each risky step

## Execution rules
1. Keep changes in small reversible increments.
2. Preserve external interfaces unless explicitly requested.
3. Prefer mechanical renames and type-safe transformations.
4. Run diagnostics/tests between phases.

## Stop conditions
- Unexpected behavior change
- Test regressions without clear root cause
- Cross-module impact larger than scoped estimate
