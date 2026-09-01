---
name: adm-commit-gates
description: >
  AgenticDevelopmentModel per-commit quality gates G1-G6, commit-command preparation,
  reviewer checklist routing, and receipt requirements. Use when preparing a
  commit, checking whether a change is ready to land, generating a commit
  command, or verifying typecheck, build, invariant, impacted-test, G6, or
  scope-receipt requirements.
---
> **AgenticDevelopmentModel**
> Copyright © 2026 AgenticDevelopmentModel Inc. Proprietary and Confidential.
>
> Author: Deepankar Das

# AgenticDevelopmentModel Commit Gates

## When to use

Use this skill when the task involves commit readiness, commit commands, staged-diff inspection, or pre-commit verification.

## Workflow

1. Classify the touched files by risk surface: request/auth, schema, frontend, tests, metric-threshold, or scope-sensitive docs.
2. Run `G1` through `G5` for the impacted scope.
3. If staged files hit metric-threshold or scope-sensitive receipts, prepare the commit body before suggesting any commit command.
4. Keep the commit single-purpose. If the diff spans unrelated subsystems, split it before landing.

## Required checks

- `G1`: `npx tsc --noEmit`
- `G2`: impact-selected `npx vitest run <subset>`
- `G3`: impacted invariant tests
- `G4`: reviewer checklist from `references/reviewer-checklist.md`
- `G5`: `npx vite build`
- `G6`: receipt rules from `references/receipt-rules.md`

## References

- Read `references/reviewer-checklist.md` when the diff changes auth, schema, pool pressure, frontend routing, or tests.
- Read `references/receipt-rules.md` when suggesting a commit command for staged diffs.
