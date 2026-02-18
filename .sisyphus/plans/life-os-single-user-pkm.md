# Single-User Life-OS PKM Plan (Markdown-First)

## TL;DR

> **Quick Summary**: Evolve PocketBrain into a stronger single-user Life-OS by improving markdown discoverability (content search, links/backlinks, tags) and adding a safe vault-authored skill workflow while keeping runtime stable.
>
> **Deliverables**:
> - Full-text vault search + content-match tool output
> - WikiLink parsing + backlinks lookup
> - Tag extraction + tag search workflow
> - Vault skill-source + reviewed promotion pipeline to runtime skills path
> - Tests + docs for Obsidian/VSCode interoperability
>
> **Estimated Effort**: Large
> **Parallel Execution**: YES - 3 waves
> **Critical Path**: 1 -> 6 -> 9 -> 12

---

## Context

### Original Request
Perform a fresh architecture analysis and produce a plan for a single-user Life-OS PKM where markdown files remain easy to use in Obsidian, VSCode, and standard editors, with optional skills stored in vault.

### Interview Summary
**Key Discussions**:
- Single-user is intentional; multi-user isolation is explicitly out of scope.
- Primary objective is local-first markdown interoperability and long-term ownership.
- Skills-in-vault is desired, but runtime stability and security must be preserved.
- GPT-5.3 preference is for this planning/coding session; PocketBrain runtime model behavior stays unchanged in this scope.

**Research Findings**:
- Vault operations exist and are centralized in `src/vault/vault-service.ts` and `src/adapters/plugins/vault.plugin.ts`.
- Runtime currently assumes skills under `.agents/skills` in `src/adapters/plugins/install-skill.plugin.ts` and `src/core/prompt-builder.ts`.
- Config accepts provider/model format via `OPENCODE_MODEL` in `src/config.ts`.

### Metis Review
Metis agent call could not be completed under current tool constraints, so this plan includes an explicit fallback gap pass:
- Added strict guardrails for single-user scope and markdown-first ownership.
- Added explicit scope lock to avoid unintended runtime model-policy changes.
- Added explicit anti-scope-creep rules (no multi-user, no web UI in this plan).

---

## Work Objectives

### Core Objective
Make PocketBrain a robust single-user markdown-native Life-OS with stronger discovery and reliable skill management, while preserving compatibility with Obsidian/VSCode and current runtime architecture.

### Concrete Deliverables
- Content-aware vault search (filename + content mode).
- WikiLink and backlink capability for markdown notes.
- Tag indexing/search capability.
- Vault skill source directory with reviewed promotion to runtime skills directory.
- Runtime model behavior remains unchanged unless explicitly requested later.

### Definition of Done
- [ ] `bun test` passes.
- [ ] `bun run typecheck` passes.
- [ ] Vault operations still work with plain markdown editors.
- [ ] Skill promotion flow works without direct runtime loading from unreviewed vault files.
- [ ] Existing model config behavior remains unchanged.

### Must Have
- Local-first markdown files remain canonical user data.
- Single-user assumptions preserved.
- New functionality verified with automated tests and agent-executed QA scenarios.

### Must NOT Have (Guardrails)
- No multi-user/tenant implementation in this plan.
- No migration away from markdown files as source-of-truth for notes.
- No auto-execution/auto-loading of arbitrary unreviewed skill files from synced vault paths.

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** for acceptance checks. All checks are command/tool executable.

### Test Decision
- **Infrastructure exists**: YES
- **Automated tests**: YES (Tests-after)
- **Framework**: bun:test + TypeScript typecheck

### QA Policy
Evidence path root: `.sisyphus/evidence/`

- **CLI/Module checks**: Bash (`bun test`, `bun run typecheck`, targeted test files)
- **Vault behavior checks**: Bash + tests against `VaultService`
- **Config/model checks**: Bash + config unit tests

Each task includes at least one happy-path and one negative-path scenario.

---

## Execution Strategy

### Parallel Execution Waves

Wave 1 (foundation, start immediately):
- Task 1: Vault search contract expansion (content search mode)
- Task 2: WikiLink parser utilities + tests
- Task 3: Tag parser/indexing utilities + tests
- Task 4: Skill-path config contract (vault source + runtime target)
- Task 5: Scope lock check for model-configuration stability

Wave 2 (feature integration after Wave 1):
- Task 6: Implement content search in `VaultService`
- Task 7: Extend vault plugin with link/backlink and tag tools
- Task 8: Skill promotion workflow (vault source -> `.agents/skills`)
- Task 9: Prompt/rules updates for governed skill flow
- Task 10: Repository-level integration tests for new PKM/skill workflows

Wave 3 (hardening and docs after Wave 2):
- Task 11: Security hardening for skill import/promotion and path safety
- Task 12: Runbook + user docs (Obsidian/VSCode usage + skills lifecycle)
- Task 13: End-to-end validation and release readiness sweep

Wave FINAL (independent review, parallel):
- F1: Plan compliance audit (oracle)
- F2: Code quality review (unspecified-high)
- F3: Real scenario QA replay (unspecified-high)
- F4: Scope fidelity check (deep)

Critical Path: 1 -> 6 -> 9 -> 12
Parallel Speedup: ~60% vs sequential
Max Concurrent: 5

### Dependency Matrix
- **1**: blocked by none -> blocks 6, 7, 10
- **2**: blocked by none -> blocks 7, 10
- **3**: blocked by none -> blocks 7, 10
- **4**: blocked by none -> blocks 8, 9, 11
- **5**: blocked by none -> blocks 9, 12
- **6**: blocked by 1 -> blocks 10, 13
- **7**: blocked by 1,2,3 -> blocks 10, 13
- **8**: blocked by 4 -> blocks 11, 13
- **9**: blocked by 4,5 -> blocks 12, 13
- **10**: blocked by 1,2,3,6,7 -> blocks 13
- **11**: blocked by 4,8 -> blocks 13
- **12**: blocked by 5,9 -> blocks 13
- **13**: blocked by 6,7,8,9,10,11,12 -> blocks Final Wave

### Agent Dispatch Summary
- **Wave 1**: T1 `quick`, T2 `quick`, T3 `quick`, T4 `unspecified-high`, T5 `quick`
- **Wave 2**: T6 `unspecified-high`, T7 `unspecified-high`, T8 `unspecified-high`, T9 `quick`, T10 `deep`
- **Wave 3**: T11 `deep`, T12 `writing`, T13 `unspecified-high`
- **FINAL**: F1 `oracle`, F2 `unspecified-high`, F3 `unspecified-high`, F4 `deep`

---

## TODOs

- [ ] 1. Expand vault search contract for content-aware querying

  **What to do**:
  - Define `vault_search` contract updates for search modes (`name`, `content`, `both`).
  - Keep backward compatibility for existing filename-only calls.
  - Add tests for request parsing and output shape.

  **Must NOT do**:
  - Do not remove existing filename behavior.

  **Recommended Agent Profile**:
  - **Category**: `quick` (small contract/test changes)
  - **Skills**: `pocketbrain-dev-setup`

  **Parallelization**:
  - Can Run In Parallel: YES (Wave 1)
  - Blocks: 6, 7, 10
  - Blocked By: None

  **References**:
  - `src/adapters/plugins/vault.plugin.ts:118` - Existing `vault_search` tool contract and output behavior.
  - `src/vault/vault-service.ts:157` - Current filename-only search implementation.
  - `tests/vault/vault-service.test.ts:192` - Existing search test style.

  **Acceptance Criteria**:
  - [ ] Contract supports mode selector while defaulting to current behavior.
  - [ ] Targeted tests for mode parsing/output pass.

  **QA Scenarios**:
  - Scenario: Default filename search still works
    Tool: Bash
    Steps: Run `bun test tests/vault/vault-service.test.ts`
    Expected: Existing search tests remain green
    Evidence: `.sisyphus/evidence/task-1-name-search.txt`
  - Scenario: Invalid mode rejected gracefully
    Tool: Bash
    Steps: Run new targeted unit test for invalid mode
    Expected: Deterministic error/validation output
    Evidence: `.sisyphus/evidence/task-1-invalid-mode.txt`

- [ ] 2. Add WikiLink parser utilities for markdown notes

  **What to do**:
  - Implement parser utility for `[[Note]]` and `[[Note|Alias]]` extraction.
  - Normalize links for case-insensitive lookup.
  - Add parser unit tests.

  **Must NOT do**:
  - Do not mutate note files during parsing.

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: `pocketbrain-dev-setup`

  **Parallelization**:
  - Can Run In Parallel: YES (Wave 1)
  - Blocks: 7, 10
  - Blocked By: None

  **References**:
  - `src/vault/vault-service.ts:36` - Vault domain class where link features will integrate.
  - `tests/vault/vault-service.test.ts:40` - Existing test file conventions.

  **Acceptance Criteria**:
  - [ ] Parser handles plain link, alias link, duplicate links.
  - [ ] Parser ignores malformed link syntax without crash.

  **QA Scenarios**:
  - Scenario: Valid links parsed
    Tool: Bash
    Steps: Run targeted parser test file
    Expected: `[[A]]`, `[[B|C]]` extracted correctly
    Evidence: `.sisyphus/evidence/task-2-happy.txt`
  - Scenario: Malformed link is ignored
    Tool: Bash
    Steps: Run malformed-input unit test
    Expected: No throw, empty/partial safe parse
    Evidence: `.sisyphus/evidence/task-2-negative.txt`

- [ ] 3. Add tag extraction utilities and index primitives

  **What to do**:
  - Implement markdown tag extraction (`#tag`, `#tag/subtag`).
  - Add in-memory/index primitives ready for vault integration.
  - Add parser/index tests.

  **Must NOT do**:
  - Do not treat headings as tags (`# Title`).

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: `pocketbrain-dev-setup`

  **Parallelization**:
  - Can Run In Parallel: YES (Wave 1)
  - Blocks: 7, 10
  - Blocked By: None

  **References**:
  - `src/vault/vault-service.ts:157` - Search-related methods where tag index hooks can attach.
  - `tests/vault/vault-service.test.ts:192` - Existing search-oriented assertions.

  **Acceptance Criteria**:
  - [ ] Tags parsed from note text with expected normalization.
  - [ ] Heading lines are not misclassified as tags.

  **QA Scenarios**:
  - Scenario: Tag list extracted from markdown body
    Tool: Bash
    Steps: Run targeted tag parser tests
    Expected: Exact expected tag set
    Evidence: `.sisyphus/evidence/task-3-happy.txt`
  - Scenario: Heading not treated as tag
    Tool: Bash
    Steps: Run negative parser test
    Expected: `# My Title` excluded from tag set
    Evidence: `.sisyphus/evidence/task-3-negative.txt`

- [ ] 4. Introduce skill-path config contract (vault source + runtime target)

  **What to do**:
  - Extend app config with optional vault skill-source path and promotion target policy.
  - Keep `.agents/skills` as runtime target default.
  - Add config validation tests.

  **Must NOT do**:
  - Do not silently break current `.agents/skills` workflow.

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: `pocketbrain-dev-setup`

  **Parallelization**:
  - Can Run In Parallel: YES (Wave 1)
  - Blocks: 8, 9, 11
  - Blocked By: None

  **References**:
  - `src/config.ts:59` - Schema and validation pattern.
  - `tests/core/config.test.ts:39` - Env-driven config testing style.
  - `src/adapters/plugins/install-skill.plugin.ts:9` - Current hardcoded runtime skill path.

  **Acceptance Criteria**:
  - [ ] New config keys validate correctly.
  - [ ] Defaults preserve existing behavior.

  **QA Scenarios**:
  - Scenario: Defaults remain backward compatible
    Tool: Bash
    Steps: Run `bun test tests/core/config.test.ts`
    Expected: Existing tests pass
    Evidence: `.sisyphus/evidence/task-4-happy.txt`
  - Scenario: Invalid skill path config fails validation
    Tool: Bash
    Steps: Run targeted invalid-config test
    Expected: Zod error thrown as expected
    Evidence: `.sisyphus/evidence/task-4-negative.txt`

- [ ] 5. Scope-lock runtime model behavior (no policy changes)

  **What to do**:
  - Confirm existing model parsing/validation remains unchanged while implementing PKM features.
  - Ensure no unintended edits to runtime model policy docs/config/tests.
  - Keep provider/model parsing behavior intact.

  **Must NOT do**:
  - Do not introduce new runtime model restrictions or defaults in this scope.

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: `pocketbrain-dev-setup`

  **Parallelization**:
  - Can Run In Parallel: YES (Wave 1)
  - Blocks: 9, 12
  - Blocked By: None

  **References**:
  - `src/config.ts:64` - `OPENCODE_MODEL` validation.
  - `tests/core/config.test.ts:75` - Existing model value assertions.
  - `src/core/runtime-provider.ts:68` - provider/model split logic.

  **Acceptance Criteria**:
  - [ ] Existing model-related tests continue passing.
  - [ ] No runtime model-policy behavior changes are introduced.

  **QA Scenarios**:
  - Scenario: Existing model config tests remain green
    Tool: Bash
    Steps: Run `bun test tests/core/config.test.ts`
    Expected: PASS
    Evidence: `.sisyphus/evidence/task-5-happy.txt`
  - Scenario: Invalid model format still rejected
    Tool: Bash
    Steps: Run invalid-format test case
    Expected: Throw/validation error
    Evidence: `.sisyphus/evidence/task-5-negative.txt`

- [ ] 6. Implement content-aware vault search in VaultService

  **What to do**:
  - Extend `VaultService` search logic to support content scanning mode.
  - Return stable file metadata for content matches.
  - Keep path traversal protections intact.

  **Must NOT do**:
  - Do not bypass `resolvePathWithinVault` constraints.

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: `pocketbrain-dev-setup`

  **Parallelization**:
  - Can Run In Parallel: YES (Wave 2)
  - Blocks: 10, 13
  - Blocked By: 1

  **References**:
  - `src/vault/vault-service.ts:157` - Current filename search entrypoint.
  - `src/vault/vault-service.ts:248` - Recursive listing helper.
  - `tests/vault/vault-service.test.ts:192` - Existing search tests to extend.

  **Acceptance Criteria**:
  - [ ] Content mode returns matches where query exists in markdown body.
  - [ ] Filename mode unchanged.

  **QA Scenarios**:
  - Scenario: Content match found in note body
    Tool: Bash
    Steps: Run updated vault search tests
    Expected: body-only match returns file
    Evidence: `.sisyphus/evidence/task-6-happy.txt`
  - Scenario: Path traversal input blocked
    Tool: Bash
    Steps: Run traversal negative test
    Expected: empty result/null-safe behavior
    Evidence: `.sisyphus/evidence/task-6-negative.txt`

- [ ] 7. Add link/backlink and tag tools in vault plugin

  **What to do**:
  - Extend vault plugin with tools for backlinks and tag queries.
  - Wire plugin to new parser/search utilities.
  - Keep tool output concise and deterministic.

  **Must NOT do**:
  - Do not remove existing vault tools.

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: `pocketbrain-dev-setup`

  **Parallelization**:
  - Can Run In Parallel: YES (Wave 2)
  - Blocks: 10, 13
  - Blocked By: 1,2,3

  **References**:
  - `src/adapters/plugins/vault.plugin.ts:50` - Existing tool registration structure.
  - `src/core/prompt-builder.ts:110` - Tool list instructions shown to model.
  - `tests/vault/vault-service.test.ts:192` - Search behavior tests.

  **Acceptance Criteria**:
  - [ ] New vault tools exposed and documented in prompt instructions.
  - [ ] Backlink/tag behavior covered by tests.

  **QA Scenarios**:
  - Scenario: Backlink query returns linked notes
    Tool: Bash
    Steps: Run targeted plugin/service tests
    Expected: Known linked files listed
    Evidence: `.sisyphus/evidence/task-7-happy.txt`
  - Scenario: Missing note/tag returns empty-safe result
    Tool: Bash
    Steps: Run negative tests for nonexistent entities
    Expected: no crash, empty response message
    Evidence: `.sisyphus/evidence/task-7-negative.txt`

- [ ] 8. Build governed skill promotion flow from vault source

  **What to do**:
  - Add a workflow: draft skill in vault source folder -> validate -> promote/copy to `.agents/skills`.
  - Reuse existing safety checks (name sanitization, SKILL.md presence).
  - Keep install-from-GitHub behavior intact.

  **Must NOT do**:
  - Do not auto-load all vault skill files into runtime.

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: `pocketbrain-dev-setup`, `pocketbrain-security-ops`

  **Parallelization**:
  - Can Run In Parallel: YES (Wave 2)
  - Blocks: 11, 13
  - Blocked By: 4

  **References**:
  - `src/adapters/plugins/install-skill.plugin.ts:71` - Current install workflow and validations.
  - `src/adapters/plugins/install-skill.plugin.ts:121` - `SKILL.md` existence requirement.
  - `src/core/prompt-builder.ts:48` - Agent skills rules text.

  **Acceptance Criteria**:
  - [ ] Promotion requires valid skill folder and explicit action.
  - [ ] Existing GitHub install flow remains working.

  **QA Scenarios**:
  - Scenario: Valid vault skill promoted to runtime path
    Tool: Bash
    Steps: Run targeted plugin tests for promotion path
    Expected: skill copied/available in `.agents/skills/<name>`
    Evidence: `.sisyphus/evidence/task-8-happy.txt`
  - Scenario: Invalid/unreviewed skill rejected
    Tool: Bash
    Steps: Run negative tests (missing `SKILL.md`, bad name)
    Expected: deterministic error and no runtime install
    Evidence: `.sisyphus/evidence/task-8-negative.txt`

- [ ] 9. Update prompt instructions for new vault and skills behavior

  **What to do**:
  - Update prompt builder vault instructions to mention link/tag tools.
  - Update skills rules to reflect governed vault-source promotion flow.
  - Keep plain-text/no-markdown response constraints unchanged.

  **Must NOT do**:
  - Do not loosen response format restrictions.

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: `pocketbrain-dev-setup`

  **Parallelization**:
  - Can Run In Parallel: YES (Wave 2)
  - Blocks: 12, 13
  - Blocked By: 4,5

  **References**:
  - `src/core/prompt-builder.ts:23` - Main system prompt assembly.
  - `src/core/prompt-builder.ts:98` - Vault instruction block.
  - `src/core/prompt-builder.ts:48` - Existing skill rules and path wording.
  - `tests/core/prompt-builder.test.ts:5` - Prompt text regression tests.

  **Acceptance Criteria**:
  - [ ] Prompt includes new tools/rules text.
  - [ ] Prompt tests pass.

  **QA Scenarios**:
  - Scenario: Vault tool instructions present
    Tool: Bash
    Steps: Run `bun test tests/core/prompt-builder.test.ts`
    Expected: PASS with updated assertions
    Evidence: `.sisyphus/evidence/task-9-happy.txt`
  - Scenario: Vault-disabled prompt still omits vault section
    Tool: Bash
    Steps: Run disabled-path test
    Expected: no VAULT ACCESS block
    Evidence: `.sisyphus/evidence/task-9-negative.txt`

- [ ] 10. Add integration tests for PKM discovery workflows

  **What to do**:
  - Add integration tests covering content search + links + tags end-to-end at service/plugin layer.
  - Verify backward compatibility for existing behaviors.

  **Must NOT do**:
  - Do not create brittle tests tied to timestamps/random ordering.

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: `pocketbrain-dev-setup`

  **Parallelization**:
  - Can Run In Parallel: YES (Wave 2)
  - Blocks: 13
  - Blocked By: 1,2,3,6,7

  **References**:
  - `tests/vault/vault-service.test.ts:192` - Existing vault search patterns.
  - `tests/adapters/persistence/sqlite-repositories.test.ts:20` - Integration-like repository style.
  - `tests/core/assistant.test.ts:23` - Mocked core integration strategy.

  **Acceptance Criteria**:
  - [ ] New integration scenarios pass reliably.
  - [ ] Existing test suite remains green.

  **QA Scenarios**:
  - Scenario: Discovery flow works (content -> backlinks -> tags)
    Tool: Bash
    Steps: Run new integration test file(s)
    Expected: all assertions pass
    Evidence: `.sisyphus/evidence/task-10-happy.txt`
  - Scenario: Legacy vault search still passes
    Tool: Bash
    Steps: Run legacy vault tests
    Expected: no regression failures
    Evidence: `.sisyphus/evidence/task-10-negative.txt`

- [ ] 11. Harden skill and vault path security boundaries

  **What to do**:
  - Re-check path traversal and unsafe input controls for new flows.
  - Add security-focused tests for promotion and vault access boundaries.
  - Ensure sync-conflict style filenames do not bypass validation.

  **Must NOT do**:
  - Do not weaken existing path safety checks.

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: `pocketbrain-security-ops`

  **Parallelization**:
  - Can Run In Parallel: YES (Wave 3)
  - Blocks: 13
  - Blocked By: 4,8

  **References**:
  - `src/vault/vault-service.ts:264` - Vault path confinement logic.
  - `src/adapters/plugins/install-skill.plugin.ts:58` - Skill name sanitization.
  - `src/adapters/plugins/install-skill.plugin.ts:96` - Invalid-name rejection.

  **Acceptance Criteria**:
  - [ ] Security tests added and passing.
  - [ ] Invalid input rejected with explicit errors.

  **QA Scenarios**:
  - Scenario: Valid skill promotion survives security checks
    Tool: Bash
    Steps: Run security-focused promotion tests
    Expected: PASS for valid path/names
    Evidence: `.sisyphus/evidence/task-11-happy.txt`
  - Scenario: Traversal/conflict filename blocked
    Tool: Bash
    Steps: Run negative security tests
    Expected: rejection + no write outside allowed paths
    Evidence: `.sisyphus/evidence/task-11-negative.txt`

- [ ] 12. Update docs and runbooks for Life-OS workflow

  **What to do**:
  - Document Obsidian/VSCode usage patterns for vault files.
  - Document skills lifecycle (draft in vault, promote to runtime, verify).
  - Document scope decision: no runtime model policy changes in this implementation.

  **Must NOT do**:
  - Do not describe unsupported multi-user flows.

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: `pocketbrain-dev-setup`

  **Parallelization**:
  - Can Run In Parallel: YES (Wave 3)
  - Blocks: 13
  - Blocked By: 5,9

  **References**:
  - `README.md` - Primary entrypoint docs.
  - `docs/setup/developer-onboarding.md` - Setup docs location.
  - `docs/architecture/repository-structure.md` - Architecture documentation baseline.

  **Acceptance Criteria**:
  - [ ] Docs include search/link/tag/skill workflows.
  - [ ] Docs include scope note that runtime model policy is unchanged.

  **QA Scenarios**:
  - Scenario: Commands in docs are executable
    Tool: Bash
    Steps: Execute documented test/typecheck commands
    Expected: commands run as documented
    Evidence: `.sisyphus/evidence/task-12-happy.txt`
  - Scenario: Model policy docs match config behavior
    Tool: Bash
    Steps: Run config tests after docs update
    Expected: docs and tests aligned
    Evidence: `.sisyphus/evidence/task-12-negative.txt`

- [ ] 13. Final implementation verification sweep

  **What to do**:
  - Run full typecheck and test suite.
  - Validate key user journeys: capture note, search content, find backlinks/tags, promote skill.
  - Produce consolidated evidence index.

  **Must NOT do**:
  - Do not ship with failing tests or undocumented behavior changes.

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: `pocketbrain-dev-setup`

  **Parallelization**:
  - Can Run In Parallel: NO (Wave 3 final gate)
  - Blocks: Final Verification Wave
  - Blocked By: 6,7,8,9,10,11,12

  **References**:
  - `package.json:10` - Typecheck command.
  - `package.json:11` - Test command.
  - `tests/` - full suite location.

  **Acceptance Criteria**:
  - [ ] `bun run typecheck` passes.
  - [ ] `bun test` passes.
  - [ ] Evidence files captured for all tasks.

  **QA Scenarios**:
  - Scenario: Full suite green
    Tool: Bash
    Steps: Run `bun run typecheck && bun test`
    Expected: all pass
    Evidence: `.sisyphus/evidence/task-13-happy.txt`
  - Scenario: Regression check catches accidental break
    Tool: Bash
    Steps: Run targeted legacy tests if failures appear
    Expected: failure localized and fixed before completion
    Evidence: `.sisyphus/evidence/task-13-negative.txt`

---

## Final Verification Wave

- [ ] F1. **Plan Compliance Audit** — `oracle`
  Verify each must-have and must-not-have against actual changes and evidence.

- [ ] F2. **Code Quality Review** — `unspecified-high`
  Run `bun run typecheck`, `bun test`, and review diffs for anti-patterns.

- [ ] F3. **Real QA Replay** — `unspecified-high`
  Replay task scenarios and confirm evidence files under `.sisyphus/evidence/final-qa/`.

- [ ] F4. **Scope Fidelity Check** — `deep`
  Confirm no multi-user or non-markdown scope creep.

---

## Commit Strategy

- **1**: `feat(vault): add content-aware search, links, and tags`
- **2**: `feat(skills): add vault source promotion workflow`
- **3**: `docs(pkm): document life-os workflows and skill governance`

---

## Success Criteria

### Verification Commands
```bash
bun run typecheck
bun test
```

### Final Checklist
- [ ] All Must Have items present
- [ ] All Must NOT Have items absent
- [ ] New discovery features work on markdown files
- [ ] Skills promotion works with explicit review/promotion step
- [ ] Runtime model behavior remains unchanged from baseline
