---
name: adm-commit-gates
description: >
  AgenticDevelopmentModel per-commit quality gates G1-G6 and reviewer checklist.
  Use when preparing a commit, running pre-commit checks, verifying
  a change is ready to land, or when the user says "commit", "verify",
  "gates", "checklist", "ready to ship", or "pre-commit". Covers
  TypeScript check, impact-selected tests, invariant tests, reviewer
  checklist by change category, build verification, and G6 counter-
  example receipts for metric-threshold files.
---

# AgenticDevelopmentModel Per-Commit Quality Gates

## Per-Commit Gates (commit-blocking, 3 min total)

| Gate | Signal | Runs in | Blocks on |
|------|--------|:------:|-----------|
| **G1 — TypeScript** | `npx tsc --noEmit` | ~30s | Any new error |
| **G2 — Unit tests affected by diff** | impact-selected `npx vitest run <subset>` | ~10–60s | Any failure |
| **G3 — Invariant tests for touched modules** | `npx vitest run <invariant-test suite>` (filtered) | ~5–20s | Any failure |
| **G4 — Reviewer checklist** (below) | Manual per-category walk | ~2–5 min | Any unchecked item |
| **G5 — Build** | `npx vite build` | ~45s | Failure |
| **G6 — Counter-example + cross-agent metric receipt** | one explicit counter-example check + pre/post metric grep + independent cross-agent re-grep citation | ~2–5 min | Missing receipt OR missing independent re-grep OR mismatch |

## G4 Reviewer Checklist — answer every applicable question before commit

**Request-handling changes** (routes / middleware / session / auth / tenant context):
- [ ] Does this change alter middleware mount order in the `<server entrypoint>` or `<route registration module>`? If yes: which invariant test pins the new order? If none exists, add one in this commit.
- [ ] Does this change read `req.session`, `req.user`, `req.tenantContext`, or the tenant id? Are all four guaranteed to be populated at the point of read (downstream of the middleware that sets them)?
- [ ] Does this change respond with a non-2xx? Is the error path unit-tested?

**Database / schema changes** (`<shared schema module>`, `<platform-schema bootstrap module>`, `<tenant-schema module>`):
- [ ] Does the `<shared schema module>` match the `<tenant-DDL builder>` for every tenant table?
- [ ] Does the `<platform-schema verifier>` list every new platform column?
- [ ] Are all `CREATE TABLE` / `ALTER TABLE` statements `IF NOT EXISTS` / `IF EXISTS`?

**Pool / concurrency changes** (`<db pool module>`, `<pool-pressure middleware>`, `<tenant-inflight-limit middleware>`, `<cluster module>`):
- [ ] Does this change allocate connections? Is the sum still within the `<per-cell connection budget>`?
- [ ] Does this change add thresholds / timeouts / rate-limit values? Does a test pin the boundary?

**Front-end changes** (`<client source tree>`):
- [ ] New lazy route wrapped in `lazyRetry()`?
- [ ] Per-type checklist from the `<testing standards doc>` §5 followed?

**Test changes** (any `*.test.ts`, `*.spec.ts`):
- [ ] Every new test passes the No-Escape-Hatches rule?
- [ ] For every assertion, is there a product change that would break the test? If you cannot name one, the test has an escape hatch.

## Pre-Task Checklist — Before Implementation

1. **Read the relevant implementation plan** (if one exists in the `<implementation-plan directory>`) — understand what's already built, what's in progress, and what depends on what.
2. **If changing schema:** Read the `adm-schema-change` skill — DDL in the `<platform-schema bootstrap module>` or `<tenant-schema module>` is mandatory.
3. **If changing frontend:** Read the `adm-frontend` skill — wrap lazy imports with `lazyRetry()`.
4. **If changing deploy/build scripts:** Read the `adm-deploy` skill. Ensure cross-platform compatibility (macOS + Linux).

## Pre-Task Checklist — Before Marking Complete

9. **Clear the per-commit gates G1–G6** from the table above.
10. **Update implementation plans** — Mark items as **Done**, add new items if scope expanded. Update progress summary tables.
11. **Update seed/simulation data** — If your change impacts schema, API fields, or pipeline stages, update seed scripts and the `<seed-constants module>`.
12. **Add document headers** to any new files (see CLAUDE.md §Document Header Requirements).
13. **Never run git commit/push** — suggest the commit message only. Commit must be single-purpose — no bundled scope.

## Banned Commit Patterns

- **Bundled commits.** A commit title joining two scopes with `+` is BANNED. Split before landing.
- **"All outstanding issues fixed" without receipts.** That claim now means "no INV* test is red AND impact-selected unit tests green."
- **Forward-fixing a regressed bundle.** When a bundle regresses, revert the whole bundle first, then re-land components one at a time.
- **Feature-flag bypass.** Any speculative middleware / session / auth change that cannot be proven safe by unit + invariant tests ships flagged OFF.

## G6 Counter-Example Receipt Format

Required receipt format for metric-threshold rows (files listed in the `<metric-threshold allowlist config>`):

```
Counter-example checked: <one sentence>
Pre-fix grep: <command + result>
Post-fix grep: <command + result>
Cross-agent verifier: independently re-greped at <commit/run-id> with same result
```

NEVER put agent names (Claude, Codex, AI) in commit messages. Use "self-verified — not independently checked" when no cross-check is available.

For full invariant catalogue and G6 mechanical enforcement details, see `references/invariant-catalogue.md`.
