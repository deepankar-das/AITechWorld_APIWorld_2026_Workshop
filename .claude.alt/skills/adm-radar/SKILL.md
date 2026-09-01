---
name: adm-radar
description: >
  AgenticDevelopmentModel RADAR test runner operations and failure analysis. Use when
  running RADAR, analyzing test failures, debugging E2E regressions,
  or when the user says "RADAR", "test run", "full run", "smoke",
  "e2e-all", "AUTH_FAIL", "pool exhaustion", "SPA never rendered",
  or "test report". Covers RADAR commands, cell-mode vs shard-mode,
  full-run restart gates, root-cause-first triage, cross-agent
  verification, and counter-example discipline.
---

# AgenticDevelopmentModel RADAR Operations

## RADAR Command Reference

Always use RADAR — never run test scripts directly:

| Command | What it does | When to use |
|---------|-------------|-------------|
| `<radar script> --quick` | tsc + vitest self-tests + build (skip Playwright) | Fast gate check |
| `<radar script> --smoke` | tsc + build + all Vitest + ~126 smoke E2E | Pre-commit validation (~5 min) |
| `<radar script>` | RADAR-selected tests based on git diff | During iteration (1-3 min) |
| `<radar script> --all` | Full Vitest + Playwright suite | Milestone / RC |
| `<radar script> --exit-on-fail` | Bail on first failure | Developer iteration / bisection |

- Local topology default is **cell-mode**; use `--shard-mode` only for fallback/bisection.
- **Never run `npx vitest run` or `npx playwright test` directly** — always through RADAR for telemetry.
- A server must be running before RADAR. Use `--force` to skip Playwright E2E when no server is available.

## Role of the Full RADAR Run

Full RADAR runs are retained for **composition-risk validation only**:
- Pool exhaustion under real load (budgets pinned by INV4, but realistic pressure only emerges under concurrent runtime)
- Cross-module timing (ordering pinned by INV1, but 4-cell x 27-worker latency distribution is only runtime-observable)
- UI + backend composition (`lazyRetry` pinned by INV6, but post-deploy stale-SPA recovery needs Playwright)
- Tenant provisioning races on cold-start (parity pinned by INV2, but the "ensure all tenants exist" timing needs a real run)

**Cadence: weekly milestone + release candidates. Not per commit.**

## Critical Failure Recurrence Guardrail

### 1) Root-cause-first, not spec-count-first
- Do not triage only by "top failing files."
- First classify by causal bucket: auth/session, SPA hydration, pool pressure, schema/data contract, spec drift.
- Fix the largest causal bucket first.

### 2) Full-run restart gate (hard block)
Before requesting another `--e2e-all` / `--all` run, report these deltas vs previous run:
- `AUTH_FAIL` volume trend (must be clearly down)
- `SPA never rendered` count (target zero)
- Top page-mount sentinel failures (must be down)
- Pool-exhaustion margin (must not be near threshold)

If these are not improved, do not request another full run.

### 3) No report-first behavior
A run report without corresponding fix tasks + owners + exit criteria is incomplete.

### 4) No "fixed" claim without metric proof
Require before/after evidence on the same metric in the next run artifact comparison.

### 5) Full-distribution sampling before pattern claims
Run full pattern aggregation: `grep <pattern> | ... | sort | uniq -c | sort -rn`. Reading the first 20 lines is not a sample — it is a biased glimpse.

### 6) Smoke is not sufficient for cascade-class closure
For any signal that scales with load, closure must be proven against a matched-load artifact (`--e2e-all`), not smoke.

### 7) Cross-agent verification when fixes span tracks
The agent that did NOT commit the larger half must independently re-grep the underlying metric before any status flips to Done.

### 8) Counter-example discipline
Before flipping any row to Done, write down ONE counter-example that would refute closure. Run that check. Cite the result.

## Test Reporting

Reports are generated automatically on every RADAR run and stored in the `<test-reports directory>` (last 5 kept). See the `<testing operations doc>` Section 2.
