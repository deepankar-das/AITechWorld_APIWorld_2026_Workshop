# Invariant Test Catalogue (INV1–INV17)

When your change touches a category below and the corresponding INV test does not yet exist, add it in the same commit. Invariant tests live in the `<invariant-test suite>`.

| # | Category | Invariant test | Status |
|:-:|----------|----------------|:------:|
| INV1 | Middleware mount order (request-id → response-count logger → admission control → session → auth init → auth session → tenant-context resolver → tenant-inflight limiter) | `<middleware-order invariant test>` | **Done** |
| INV2 | Tenant-schema parity (`<shared schema module>` tenant tables ↔ `<tenant-DDL builder>`, bidirectional) | `<tenant-schema-parity invariant test>` | **Done** |
| INV3 | Platform-schema parity (`<platform schema namespace>` ↔ `<platform-schema bootstrap module>` + `<platform-schema verifier>`, 3-way bidirectional) | `<platform-schema-parity invariant test>` | **Done** |
| INV4 | Pool-budget sum (`<per-cell connection budget>` ≥ sum of pool sizes) | `<pool-budget invariant test>` | **Done** |
| INV5 | Auth-state fixture contract (auth serialize/deserialize, session store, cookie config, auth-setup post-write validation) | `<auth-state-fixture invariant test>` | **Done** |
| INV6 | `lazyRetry` wrapping on every `lazy(() => import(...))` + no other client files call `lazy(` | `<lazy-retry invariant test>` | **Done** |
| INV7 | Escape-hatch scanner (8 banned patterns) + parser attribution completeness gate | `<escape-hatch-scanner invariant test>` | **Done** |
| INV8 | Cell-executor `--exit-on-fail` wiring | `<cell-executor exit-on-fail test>` | **Done** |
| INV9 | No mock data in production code | `<no-mock-in-prod invariant test>` | **Done** |
| INV10 | Shell-script portability (no GNU-only flags / `rg` / `declare -A` / `script -c`) | `<shell-portability invariant test>` | **Done** |
| INV14 | Fixture-user quarantine policy | `<fixture-user-quarantine invariant test>` | **Done** |
| INV15 | Full-repo E2E typecheck gate | `<e2e-typecheck-gate invariant test>` | **Done** |
| INV16 | Scope-change approval receipt enforcement | `<scope-change-receipt invariant test>` | **Done** |
| INV17 | Closure-status consistency enforcement | `<closure-status-consistency invariant test>` | **Done** |

## G6 Mechanical Enforcement

The counter-example receipt rule is enforced mechanically:

1. **Allowlist:** the `<metric-threshold allowlist config>` lists file paths that trigger the receipt requirement (auth/pool/schema-parity/SPA-hydration/fixture-cascade classes).
2. **Local hook:** the `<local commit-msg hook>` blocks any local commit whose staged diff touches an allowlisted file unless the four-line stanza is present in the message body.
3. **CI gate:** the `<CI receipt-check workflow>` runs on every PR and rejects the merge if any non-merge commit in the PR range touches an allowlisted file without the stanza. `--no-verify` skips only the local hook; CI still rejects.
4. **Reviewer routing:** the `<code-owners file>` makes the cross-agent reviewer pair mandatory for every allowlisted file.
5. **Validator:** the `<G6 receipt validator script>` is the single shared implementation called from both the local hook and CI. Cross-platform (macOS bash 3.2 + Linux bash 4); python3 for JSON parsing; no `rg` / no `declare -A` / no GNU-only flags.
