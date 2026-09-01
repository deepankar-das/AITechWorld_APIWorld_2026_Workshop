---
name: adm-test-writing
description: >
  AgenticDevelopmentModel test writing standards for Vitest, Playwright, and E2E.
  Use when writing tests, fixing test failures, adding test coverage,
  or when the user says "write tests", "test", "E2E", "Playwright",
  "vitest", "coverage", "test gap", "escape hatch", or "tier 1/tier 2".
  Covers two-tier architecture, escape hatch rules, seed-aware assertions,
  API-then-UI verification, responsive/a11y testing, and per-element
  minimum test depth.
---

# AgenticDevelopmentModel Test Writing Standards

## Before Writing Tests

1. **Read the `<testing standards doc>`** — the comprehensive testing standards document:
   - **Section 2.1** — 15 E2E/UI gaps AI assistants default to.
   - **Section 2.2** — 15 backend gaps AI assistants default to.
   - **Section 5** — Per-page-type checklist (dashboard, table, form, wizard, settings, analytics).
   - **Section 6** — Backend API contract testing.
2. **Write both tiers** — Every backend change needs Tier 1 (mock) + Tier 2 (integration) tests with the same test names.
3. **Write E2E tests** — Every UI change needs Playwright tests covering: data accuracy, error paths, responsive (375px), accessibility (axe-core), and API-then-UI verification.

## Two-Tier Testing Architecture (MANDATORY)

Every external service integration and every DB-dependent route MUST have tests at both tiers. Both tiers always run; neither uses `test.skip`.

- **Tier 1 (`*.test.ts`)** — unit/logic tests with `vi.mock` for external clients. Always pass, no credentials needed.
- **Tier 2 (`*.integration.test.ts`)** — real sandbox HTTP calls, real signatures. Calls `requireIntegration("Service", "ENV_VAR_1", ...)` on the first line; fails visibly with `[INTEGRATION TEST — Service]` label when env vars missing. Helper at the `<integration-guard helper>`.

**Pairing rule:** every Tier 1 scenario has a Tier 2 counterpart with the **exact same test name**. Create both files at the same time.

## No Escape Hatches in Tests (MANDATORY)

**A test must fail when the feature it covers is broken.** These patterns are BANNED:

1. **`expect(a || b).toBe(true)`** — OR-ing assertions. Write separate assertions for each condition.
2. **`if (await el.isVisible()) { assertions } else { fallback }`** — conditional blocks. Make the visibility check itself an assertion.
3. **`expect([400, 422, 500]).toContain(status)`** — permissive status lists including 500. List only exact valid codes.
4. **`if (body.field) { assert length }`** — conditional field assertions. Assert field exists first, then shape.
5. **`.catch(() => {})` on `expect()` calls** — swallowing assertion failures.
6. **`toBeGreaterThan(0)` / `toBeTruthy()` as sole assertion** — "not empty" is not a test.
7. **`test.skip` / `test.skipIf` / `describe.skip` / `test.todo`** — skipped tests provide zero coverage.
8. **Silent early returns** (`if (!visible) return;`).

**The test you wrote must produce RED when the feature is broken.**

## Non-Negotiable E2E Rules

- **Data validation** — assert exact values from seed/API, never just "visible" or "> 0".
- **Seed-aware** — import from the `<seed-constants module>` and assert the exact per-entity seed counts, never `> 0`.
- **API-then-UI** — call API first, then verify UI shows those exact values.
- **Error paths** — every form/API must have a failure-path test.
- **Responsive** — 375px / 768px / 1280px; assert `scrollWidth <= clientWidth`.
- **Accessibility** — every page has an axe-core scan: `expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([])`.
- **Cross-page consistency** — same data on multiple pages must match.
- **Playwright locators** — never `waitForSelector()` (deprecated), never `locator('text=...')` (legacy), never `.catch(() => {})` on assertions. Use `getByText`, `getByRole`, `getByTestId`.
- **No local helper copies** — import from the `<shared e2e navigation helpers>` and `<shared e2e wait helpers>`.

## Per-Test Quality Checklist

Every test MUST pass this checklist:
- [ ] Asserts **exact values** (not just "visible" or "length > 0")?
- [ ] Uses **seed constants** where applicable?
- [ ] Follows **API-then-UI pattern** for data pages?
- [ ] Has a **descriptive assertion message** on every `expect()`?
- [ ] **Avoids silent early returns**?
- [ ] Covers the **error path**?
- [ ] For backend: Has a **Tier 2 pair with the same test name**?
- [ ] For backend: Verifies **side effects** (DB row, audit log), not just return value?

## No Mock Data in Production Code

- Never implement mock/stub data in production code.
- `vi.mock` is acceptable ONLY in Tier 1 test files — never in Tier 2.
- Never use `test.skip`, `test.skipIf`, `describe.skip`, or `test.todo` without a tracking ticket.

## Test Output Must Be Diagnostic

- Descriptive test names ("I5: Stop button opens confirmation dialog" not "test button").
- `console.log` with `[DEBUG]` prefix for key state before assertions.
- Multi-step flows: log step number + expected vs actual at each checkpoint.

For the full 24-row AI test defect table, see `references/ai-test-defects.md`.
For minimum test depth per element type, see `references/element-rubric.md`.
For non-functional test requirements and integration credentials, see `references/e2e-rules.md`.
