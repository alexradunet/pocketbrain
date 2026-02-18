---
name: kimi-research-router
description: Route coding tasks to Kimi-first subagents for low-cost exploration and synthesis
---
## Purpose
Use this skill to decide when a task should go to a Kimi-powered subagent before using expensive frontier models.

## Prefer Kimi first
- Repo exploration: file discovery, grep-heavy investigation, dependency tracing
- Long-context reading: multi-file synthesis, RFC digestion, issue-thread summarization
- Drafting implementation plans and risk checklists

## Escalate away from Kimi when
- Safety-critical decisions need strongest reliability checks
- Repeated failed fixes require a second-opinion architecture review
- You need state-of-the-art deep debugging after multiple attempts

## Delegation template
1. Define objective and success criteria in one paragraph.
2. Bound scope with exact directories/files.
3. Ask for structured output: findings, risks, next actions.
4. Require citation of file paths and lines for every claim.

## Cost policy
- Do search/synthesis on Kimi first.
- Keep expensive models for final review or hard blockers.
