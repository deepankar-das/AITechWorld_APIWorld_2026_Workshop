> Copyright © {{YEAR}} {{AUTHOR}}. Proprietary and Confidential.
> Author: {{AUTHOR}}

# {{PROJECT_NAME}} -- Claude Code Instructions

> **{{PROJECT_NAME}}: {{ONE_LINE_DESCRIPTION}}**

---

## Project Status: {{STATUS}}

<!--
  Replace {{STATUS}} with one of:
  - Pre-Implementation (Planning Phase)
  - Active Development (Phase N)
  - Production
-->

{{STATUS_DESCRIPTION}}

---

## Product Summary

{{PRODUCT_SUMMARY}}

**Primary buyer:** {{PRIMARY_BUYER}}

---

## Design Documents

<!-- List all design documents that exist in the repository. Remove any that don't exist. -->

| Document | Purpose |
|----------|---------|
| {{DOC_LINK}} | {{DOC_PURPOSE}} |

---

## Technology Stack

<!-- Define the canonical tech stack. Agents must not introduce technologies outside this list without an architecture decision. -->

| Layer | Technology |
|---|---|
| Language | {{LANGUAGE}} |
| Framework | {{FRAMEWORK}} |
| Database | {{DATABASE}} |
| Testing | {{TEST_FRAMEWORK}} |
| Validation | {{VALIDATION}} |

---

## Architecture Decision Gate (MANDATORY)

Before implementing any change that affects system topology, enforcement points, data model, auth flow, trust boundaries, deployment topology, or public interfaces: present the decision, alternatives, tradeoffs, and a recommendation for developer approval. Do not implement first.

### Past Architecture Decisions

<!-- Record decisions here as they are made. Format: -->
<!-- - **Decision name** (date): Description. Reference doc if applicable. -->

---

## Quality Standards

### Per-Commit Gates (commit-blocking)

Every commit must clear these gates before landing. A change is high-quality when its failure modes are provable by fast, per-change artifacts: code review + tests.

| Gate | Signal | Command | Blocks on |
|------|--------|---------|-----------|
| **G1 -- Type check** | Zero type errors | `{{TYPECHECK_CMD}}` | Any new error |
| **G2 -- Unit tests** | All passing | `{{TEST_CMD}}` | Any failure |
| **G3 -- Lint** | No errors | `{{LINT_CMD}}` | Errors |
| **G4 -- Build** | Production build passes | `{{BUILD_CMD}}` | Failure |
| **G5 -- Reviewer checklist** | See below | Manual walk | Any unchecked item |

### Reviewer Checklist (G5)

<!-- Customize these checklists for your project's architecture. Each category should cover the files and patterns that matter for your codebase. -->

**API / endpoint changes:**
- [ ] Is the endpoint authenticated?
- [ ] Are error responses sanitized (no sensitive data in error bodies)?
- [ ] Is the endpoint covered by at least one test?

**Data model / schema changes:**
- [ ] Does the schema match the type definitions?
- [ ] Are new fields included in seed data or migration?

**Frontend / UI changes:**
- [ ] Is auth checked and unauthenticated users redirected?
- [ ] Are form submissions validated before API calls?

**Test changes:**
- [ ] Every assertion fails when the feature it covers is broken (no escape hatches)?
- [ ] No `test.skip`, `test.todo`, `describe.skip`, or conditional assertions?

<!-- Add project-specific checklists below. Examples: -->
<!-- **Enforcement component changes** (for security products): -->
<!-- - [ ] Does the component enforce policy decisions, not just log? -->
<!-- - [ ] Are audit events emitted for every intercepted action? -->

### No Dropped Commitments (MANDATORY)

Every item the user asks for must ship. No silent deferrals, no "follow-up" buckets that never return.

- **Never drop, defer, or remove an item** without explicit permission.
- **Treat every user ask as a durable commitment** until the user releases it.
- **When narrowing scope, ask first.**
- **Keep every unfinished commitment visible** in the todo list.
- **If you discover an item was dropped, own it immediately**: acknowledge, build, audit.

### Banned Patterns

- **Bundled commits** joining multiple scopes. Split before landing.
- **`expect([400, 422, 500]).toContain(status)`** -- 500 is never acceptable.
- **Conditional assertions** (`if (visible) { assert } else { fallback }`) -- masks failures.
- **`test.skip` / `test.todo`** -- skipped tests provide zero coverage.
- **Silent early returns** in tests (`if (!visible) return;`).
- **Mock data in production code** -- `vi.mock` / `jest.mock` only in test files.

<!-- Add project-specific banned patterns below. Examples: -->
<!-- - **Hard DELETE** on any table containing {{SENSITIVE_DATA_TYPE}}. -->
<!-- - **Direct `npm install` / `brew install`** -- use `./scripts/prepare.sh --install`. -->

---

## Security Standards

<!-- Customize for your product's security posture. -->

| Requirement | Rule |
|---|---|
| **{{TRUST_BOUNDARY}}** | {{TRUST_RULE}} |
| **Audit trail immutability** | No UPDATE or DELETE on audit events. Append-only. |
| **Secrets never in logs** | Redact credentials, tokens, and PII from audit events, error messages, and context. |
| **Error messages never expose sensitive data** | Sanitize all error responses. No internal data in error output. |

---

## Git Commit Rules (MANDATORY)

- **Never** run `git commit`, `git push`, or any git write commands. The developer handles all commits.
- **Never** add `Co-Authored-By` trailers or AI attribution to commit messages.
- When asked about commits, only suggest the commit message -- do not execute it.

---

## No Shortcuts Policy (MANDATORY)

- **Never take shortcuts.** Every feature must follow: implementation, tests, build verification.
- **Never skip testing.** {{TESTING_RATIONALE}}
- **Never use workarounds** instead of proper fixes.
- **Never merge partial implementations.** All mandatory phases complete with tests before marking done.
- **Do not stop working to report status unless asked.** Keep working until done.

---

## Data Integrity Rules (MANDATORY)

- **Never fabricate data or features.** Only claim something exists if verified in the codebase.
- **Never change validated data in documents** without explicit instruction.
- **Source all claims.** Traceable to: (a) codebase evidence, (b) web research, or (c) developer-provided info.

---

## Evidence-First Decision Making (MANDATORY)

- **Never assert something as fact without evidence.** Read the file, grep the function, run the test.
- **Never guess at file contents or function signatures.**
- **Never assume a bug is fixed** without running tests.
- **Classify every claim:** Verified (evidence), Inferred (reasoning), Unknown (needs investigation).
- **Admit uncertainty.** Say "I don't know" and investigate rather than guessing.
- **Provide counterarguments** -- the strongest case AGAINST a proposed approach before agreeing.

---

## Implementation Plans

This project uses two implementation plan templates. Match the template to the task type.

- **Template A -- Feature / Code**: Building features, fixing bugs, refactoring code, adding endpoints, making schema changes.
- **Template B -- Test Writing**: E2E tests, test gaps, test coverage matrices.

### Template A: Feature / Code Implementation Plans

**Use when:** Building features, fixing bugs, refactoring code, adding API endpoints, making schema changes, implementing pipeline stages.

**Document structure:**

1. Header: copyright, version, parent doc, related docs, status
2. Table of Contents
3. Section 1 -- Overview: brief description + "Existing Infrastructure (Already Built)" table (`| Component | File(s) | Status |`)
4. Section 2 -- Current Infrastructure: "Key Files" table (`| File | What It Contains | Lines of Interest |`) + "Database Tables (Existing)" table
5. Section 3 -- Implementation Plan: phased tables (see format below)
6. Progress Summary table
7. Priority Legend and Tests Column Legend

**Phase table format:**

```markdown
### Phase X: Phase Name -- Layer/Scope

One-sentence description of what this phase does.

| # | Item | File(s) | Description | Pri | Deps | Status | Tests | Owner |
|:-:|------|---------|-------------|:---:|:----:|:------:|:-----:|:-----:|
| X1 | Item name | `file/path.ts` | Detailed description of what to build, function signatures, specific behavior. | High | None | Not Started | vitest + E2E | {{OWNER}} |
```

**Column semantics:**

- **#**: Phase letter + number (e.g., A1, B2, C3)
- **Item**: Short name (2-5 words)
- **File(s)**: Exact file paths with backticks
- **Description**: Detailed -- include function names, API endpoints, field names, behavior
- **Pri**: High / Medium / Low
- **Deps**: Other item IDs this depends on (e.g., "A1, A2") or "None"
- **Status**: "Not Started" initially, then "**Done**" when complete
- **Tests**: Test type required for this item
- **Owner**: Who builds this (when multiple agents or developers are involved)

### Template B: Test Writing Implementation Plans

**Use when:** Writing new E2E tests, fixing test gaps, improving test depth, implementing test coverage matrices.

**Document structure:**

1. Mandatory Rules reference table (R1-RN mapped to testing rules in this file)
2. Per-Component UI Element Inventory: `| Category | Elements | Count |` for every component/page
3. Per-Component Test Matrix: Rules x Elements -- each cell = one test to write
4. Cross-Component Journey Tests: Multi-step workflows across components
5. Backend Test Matrix: Unit + Integration pairing
6. Execution and Verification Plan
7. Progress Summary

**Key difference from Template A:** Template B inventories every interactive element on every component/page, then cross-references against the mandatory testing rules to produce a test matrix. Each intersection (rule x element) is a test to write. This ensures complete coverage.

**Per-component test matrix format:**

```markdown
### Component: /route-or-module -- Type: Component Type

| Element | R1 Data | R2 Action | R3 Overflow | R5 Placement | R6 Seed | R8 API-UI | R10 Error | R12 A11y |
|---------|:--:|:--:|:--:|:--:|:--:|:--:|:--:|:--:|
| Element name | [N] | [N] | -- | [N] | [N] | [N] | [N] | [N] |
```

**Cell semantics:**

- **[N]** = Not started
- **[D]** = Done
- **[--]** = Not applicable

Every `[N]` is a test to write. Count of `[N]` cells = total tests needed for that component.

### Progress Summary Format

Use this table at the bottom of every implementation plan:

```markdown
## Progress Summary

| Phase | Total | Done | In Progress | Not Started |
|:-----:|:-----:|:----:|:-----------:|:-----------:|
| A -- Phase Name | 9 | 0 | 0 | 9 |
| B -- Phase Name | 0 | 0 | 0 | 0 |
| **Total** | **9** | **0** | **0** | **9** |
```

### Legends

**Priority Legend:**

| Priority | Description |
|:--------:|-------------|
| High | Critical path -- blocks other work or core functionality |
| Medium | Important but not blocking -- can be done in parallel |
| Low | Nice to have -- can be deferred without impacting delivery |

**Tests Column Legend:**

| Value | Description |
|:-----:|-------------|
| Covered | No automated tests written for this item |
| Not Passed | Tests written but fail when run |
| Done | Tests written, executed, and all pass |
| N/A | Not applicable -- infrastructure, documentation, or one-time manual action |

**Status Legend:**

| Value | Description |
|:-----:|-------------|
| Not Started | Work has not begun |
| In Progress | Actively being worked on |
| **Done** | Complete and verified |
| Blocked | Cannot proceed -- dependency or decision needed |
| Deferred | Moved to a later phase |

---

## Testing Requirements (MANDATORY)

### Test maintenance rule

When ANY code change modifies enforcement, API, UI, or behavior:
1. **Fix ALL existing tests** that break.
2. **Write at least one regression test** for every fix.
3. **Verify ALL tests pass** before marking complete.

### No Escape Hatches in Tests

**A test must fail when the feature it covers is broken.** Banned patterns:

1. `expect(a || b).toBe(true)` -- OR-ing assertions
2. `if (visible) { assertions } else { fallback }` -- conditional blocks
3. `expect([400, 422, 500]).toContain(status)` -- 500 in accepted list
4. `if (body.field) { assert }` -- field-existence guard skipping assertion
5. `.catch(() => {})` on assertions
6. `toBeGreaterThan(0)` / `toBeTruthy()` as sole assertion
7. `test.skip` / `test.todo` / `describe.skip`
8. Silent early returns (`if (!visible) return;`)

### Running tests

<!-- Customize these commands for your project -->

- `{{BUILD_CMD}}` -- typecheck + tests + build
- `{{TEST_CMD}}` -- unit tests only
- `{{E2E_CMD}}` -- E2E tests only

---

## Ownership

<!-- Define who builds what when multiple agents or developers are involved. Remove if single-developer project. -->

| Owner | Scope |
|---|---|
| **{{OWNER_1}}** | {{SCOPE_1}} |
| **{{OWNER_2}}** | {{SCOPE_2}} |

---

## Self-Check Before Reply

Before you respond or ship a change, confirm:

1. **Evidence cited** -- every factual claim references verified sources. No guessing.
2. **Scope respected** -- change touches only what was requested. No bundled commits.
3. **Gates cleared** -- typecheck clean, tests green, build passes (once code exists).
4. **Uncertainty surfaced** -- anything not verified is marked Unverified / Inferred / Unknown.
5. **No emoji, no AI attribution, no auto-commit.**

If any item is not met, fix before replying or flag the gap to the user.
