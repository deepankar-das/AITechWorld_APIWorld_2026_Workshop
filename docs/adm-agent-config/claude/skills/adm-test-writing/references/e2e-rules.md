# Non-Functional Tests and Integration Rules

## Non-Functional Tests for Infrastructure Changes

Changes to the `<db pool module>`, `<cluster module>`, `<server entrypoint>`, rate-limit middleware, or pool code must include:
- Pool exhaustion -> 503 breaker
- Concurrent request handling (100+ parallel)
- Statement timeout
- Large payload (413)
- Malformed JSON (400 not 500)
- Rate limiter (429 + Retry-After)

See the `<backend test-gap doc>` Section 8.

## Test Reporting

Reports are generated automatically on every RADAR run and stored in the `<test-reports directory>` (last 5 kept). Pipeline internals, report sections, and the full testing-ops reference live in the `<testing operations doc>` Section 2.

## Integration Credential Map

Every Tier 2 integration test guards its required credentials on the first line
via `requireIntegration("<Service>", "<ENV_VAR>", ...)` and fails visibly with a
`[INTEGRATION TEST — <Service>]` label when they are missing. The credential
names are environment-specific and are documented in the `<testing operations doc>`
Section 1 — treat that as the single source of truth rather than hardcoding a
list here.

Typical shape:

| Tier 2 file | Env vars required |
|-------------|-------------------|
| `<service-a>.integration.test.ts` | `<SERVICE_A_TEST_CREDENTIAL>` |
| `<service-b>.integration.test.ts` | `<SERVICE_B_TEST_CREDENTIAL_1>`, `<SERVICE_B_TEST_CREDENTIAL_2>` |
| `<db-backed-route>.integration.test.ts` | `<database URL>` |

Full credential map and pipeline detail: the `<testing operations doc>` Section 1.
