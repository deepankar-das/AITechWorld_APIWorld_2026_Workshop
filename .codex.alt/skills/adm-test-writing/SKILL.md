---
name: adm-test-writing
description: >
  AgenticDevelopmentModel Vitest, integration, and Playwright test-writing rules. Use when
  adding coverage, fixing tests, validating UI behavior, or planning Tier 1,
  Tier 2, E2E, accessibility, responsive, or API-then-UI checks.
---
> **AgenticDevelopmentModel**
> Copyright © 2026 AgenticDevelopmentModel Inc. Proprietary and Confidential.
>
> Author: Deepankar Das

# AgenticDevelopmentModel Test Writing Standards

## When to use

Use this skill for any new or modified test, or when a product change requires regression coverage.

## Workflow

1. Read the product testing instructions before writing tests.
2. Pair backend logic with Tier 1 and Tier 2 scenarios.
3. For UI changes, assert data correctness, error paths, responsive behavior, and accessibility.
4. Reject every escape hatch that would let the test stay green when the product is broken.

## References

- the `<testing standards doc>`
- `references/e2e-backend-checklist.md`
