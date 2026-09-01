> **AgenticDevelopmentModel**
> Copyright © 2026 AgenticDevelopmentModel Inc. Proprietary and Confidential.
>
> Author: Deepankar Das
# E2E and Backend Checklist

## E2E expectations

- Validate real values, not only visibility.
- Use API-then-UI verification for data pages.
- Cover success and failure paths.
- Check 375px, 768px, and 1280px layouts.
- Run at least one axe accessibility scan.
- Reuse shared fixtures instead of local helper copies.

## Backend expectations

- Every backend change gets Tier 1 and Tier 2 coverage.
- Tier 2 tests call `requireIntegration()` as the first executable line.
- Verify side effects, not just response status.

## Escape hatches to reject

- `test.skip`, `test.todo`, conditional pass blocks, permissive status lists, swallowed assertions, or exists-only checks.
