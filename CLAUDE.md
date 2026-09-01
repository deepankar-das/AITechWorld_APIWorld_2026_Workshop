> Author: Deepankar Das
# Enforcer -- Claude Code Instructions

> **Enforcer: A security and policy control plane for AI coding agents operating in developer environments.**

---

## Project Status: Phase 1 Complete + Go Port Complete

The prototype is **fully implemented** with 55 features across TypeScript (original) and Go (ported core). The Go port produces 4 statically compiled binaries with zero runtime dependencies.

- **Go core:** 45 source files, 197 tests, 4 binaries (daemon, hook handler, Management Hub, Sentinel agent)
- **TypeScript console:** 18 source files, 145 Vitest tests, 25 Playwright E2E tests
- **Policies:** 13 rules in default bundle, 8 canned policy packs, network allowlist + warning list
- **Scripts:** 11 shell scripts (prepare, build, deploy, test, demo, readiness, hooks, service, certs, deploy, package)

---

## Product Summary

Enforcer sits between AI coding agents (Claude Code, Cursor, Copilot agents, MCP-driven workflows) and the systems they can affect. It intercepts actions, evaluates them against organizational governance policies and developer-local guardrails, enforces allow/deny/approval decisions, and produces security-grade audit trails.

The product thesis: AI coding agents are gaining access to source code, shells, package managers, credentials, cloud resources, APIs, and MCP-driven tools, while most organizations still rely on controls built for human developers. Enforcer is the governance layer that makes agent adoption safe enough to approve.

**Primary buyer:** Security engineering lead or platform engineering lead at a mid-market or enterprise software organization.

---

## Design Documents (the actual codebase today)

| Document | Purpose |
|----------|---------|
| [Enforcer Prompt.md](docs/AA%20Firewall%20Prompt.md) | Original venture prompt and deliverable requirements |
| [Enforcer_PRD_Final.md](docs/Enforcer_PRD_Final.md) | Final product requirements document |
| [enforcer_prd_vscode_agents.md](docs/enforcer_prd_vscode_agents.md) | PRD with VS Code and agent integration focus |
| [Enforcer_TDD_Final_2.md](docs/Enforcer_TDD_Final_2.md) | Final technical design document (most complete) |
| [Enforcer_TDD_Final.md](docs/Enforcer_TDD_Final.md) | Technical design document (prior version) |
| [Enforcer_TDD_Addendum.md](docs/Enforcer_TDD_Addendum.md) | TDD addendum |
| [enforcer_market_study_mrd.md](docs/enforcer_market_study_mrd.md) | Market study and marketing requirements |

Earlier iterations (retained for reference): [enforcer_prd_1.md](docs/enforcer_prd_1.md), [enforcer_prd_2.md](docs/enforcer_prd_2.md), [enforcer_prd_3.md](docs/enforcer_prd_3.md), [Enforcer_TDD.md](docs/Enforcer_TDD.md).

---

## Prototype Requirements (from the venture prompt)

The prototype must:

1. **Intercept agent actions** across at least two of: file system reads/writes, shell command execution, network calls, package installs, credential/secret access.
2. **Enforce configurable policy** (allow / deny / require approval) with at least one non-trivial rule (e.g., block writes outside project directory, deny non-allowlisted network hosts, require approval before package install).
3. **Produce structured audit logs** a security reviewer can meaningfully read.
4. **Implement one depth area**: real-time human-in-the-loop approval UX, anomaly detection over agent action sequences, secrets/PII redaction in agent context, multi-agent policy isolation, or org-level policy distribution.
5. **Depth over breadth**: going deep on one area is stronger than touching all of them.

---

## Technology Stack

### Core (Go — compiled binaries, zero runtime dependencies)

| Layer | Technology |
|---|---|
| Language | Go 1.26+ (compiled, statically linked, `CGO_ENABLED=0`) |
| Daemon, policy engine, enforcement | Go `net/http`, `regexp`, `crypto/tls` (stdlib) |
| Hook handler | Go binary (`enforcer-hook`) — reads stdin JSON, exits 0/2 |
| Management Hub | Go with mTLS (`crypto/tls`) on ports 9200/9201 |
| Sentinel agent | Go binary (`enforcer-client`) — registration, policy sync, heartbeat |
| YAML parsing | `gopkg.in/yaml.v3` |
| UUID generation | `github.com/google/uuid` |
| PostgreSQL (audit store) | `github.com/jackc/pgx/v5` (pure Go) — TLS-encrypted, append-only central audit store |
| Go testing | `go test` (197 tests) |

### Hub Console (TypeScript — embedded in Go binary)

| Layer | Technology |
|---|---|
| Hub Console | Next.js 15 (App Router) + React + shadcn/ui + Tailwind CSS |
| Console deployment | Built to static HTML/JS/CSS, embedded in Go daemon via `go:embed` |
| VS Code extension | TypeScript (VS Code Extension API) — Phase 2 |

### Legacy (TypeScript — retained for development/reference)

| Layer | Technology |
|---|---|
| TypeScript daemon | Node.js (original prototype, `src/` directory) |
| TypeScript tests | Vitest (145 tests) + Playwright (25 E2E tests) |
| Validation | Zod (action schemas, policy schemas, audit events) |

### Infrastructure

| Layer | Technology |
|---|---|
| Policy format | Versioned YAML bundles |
| Container mode | Docker (rootless, hardened profile) |
| IPC | Local HTTP (localhost:9100) |

---

## Planned Architecture (from TDD)

Enforcer is a hybrid enforcement system combining 5 defense layers:

- **Runtime hook / SDK wrapper** -- intent-aware interception before action execution (Layer 1)
- **Managed hooks** -- developer cannot remove enforcement hooks (Layer 2)
- **Privileged daemon** -- policy lookup, decision caching, audit buffering, RBAC, runs as root/LaunchDaemon (Layer 3)
- **OS kernel enforcer** -- eBPF (Linux) / ESF (macOS) intercepts file.open, execve, connect syscalls; catches raw terminal bypass (Layer 4)
- **Management Hub** -- mTLS policy distribution, audit aggregation, signed bundles, heartbeat monitoring (Layer 5)

Application-level enforcement components:
- **Filesystem guard** -- project-path enforcement and write policy
- **Shell proxy / exec wrapper** -- command mediation and classification
- **Network proxy** -- egress allowlists and exfiltration controls
- **MCP gateway / wrapper** -- tool-call governance and payload inspection
- **Package guard** -- package install detection and approval
- **Secret detector + redaction** -- 20+ patterns, mask/tokenize/summarize
- **Secure container mode** -- controlled workspace isolation (higher assurance)

### Logical Layers

| Layer | Components |
|-------|-----------|
| **Experience** | IDE integrations, CLI integrations, approval UIs, Hub Console, Sentinel Console, admin dashboard, policy rationale surfaces |
| **Agent Runtime** | Primary orchestrator, sub-agents, runtime hooks, MCP clients, model-routing |
| **Enforcement (Application)** | Shell proxies, filesystem guards, network proxies, MCP gateways, package guard, secret detector, redaction engine, managed hooks |
| **Enforcement (Kernel)** | KernelEnforcer (eBPF/ESF), syscall gate (file.open, execve, connect), invocation audit logging |
| **Control Plane** | Local daemon, Management Hub policy engine, approval service, policy distribution, Hub Console |
| **Observability** | Event ingestion, graph-native replay, anomaly detection, policy simulation, alert routing |

### Implementation Phases (from TDD)

**Phase 1 -- Prototype / MVP:**
- Runtime hook for one coding-agent integration
- Local daemon with local policy bundle evaluation
- Shell proxy and network proxy
- Filesystem rule: deny writes outside project root
- Network rule: deny non-allowlisted hosts
- Approval rule: require approval before package install
- Structured JSON audit events
- Minimal admin and approval console

**Phase 2 -- Depth and hardening:**
- MCP gateway and tool/method policy
- Secure secret broker
- Secure-container execution mode
- Replay views and session graphs
- SIEM integration and policy simulation mode

**Phase 3 -- Enterprise expansion:**
- Multi-agent lineage and isolation
- Database-aware controls
- Advanced anomaly detection over action sequences
- Expanded platform support and HA central services

---

## Architecture Decision Gate (MANDATORY)

Before implementing any change that affects system topology, enforcement points, policy model, trust boundaries, deployment topology, or public interfaces: present the decision, alternatives, tradeoffs, and a recommendation for developer approval. Do not implement first.

---

## Quality Standards

### Per-Commit Gates (once code exists)

| Gate | Signal | Command | Blocks on |
|------|--------|---------|-----------|
| **G1 -- Type check** | Zero type errors | `npx tsc --noEmit` | Any new error |
| **G2 -- Unit tests** | All passing | `npx vitest run` | Any failure |
| **G3 -- Lint** | No errors | `npx eslint .` (if configured) | Errors |
| **G4 -- Build** | Production build passes | `npm run build` | Failure |
| **G5 -- Reviewer checklist** | See below | Manual walk | Any unchecked item |

### Reviewer Checklist (G5)

**Enforcement component changes** (daemon, proxies, guards, gateways):
- [ ] Does the component enforce policy decisions (allow/deny/approval), not just log?
- [ ] Are audit events emitted for every intercepted action?
- [ ] Is the trust boundary respected (agent runtime is untrusted by default)?

**Policy engine changes:**
- [ ] Is the policy model expressive enough for the target surfaces (files, commands, network, MCP, secrets)?
- [ ] Are policy decisions deterministic and reproducible?
- [ ] Can the policy be simulated before enforcement?

**Audit / replay changes:**
- [ ] Are audit events append-only (no UPDATE or DELETE)?
- [ ] Is the event schema structured for security reviewer consumption?
- [ ] Are action chains linked (not isolated flat events)?

**API / console changes:**
- [ ] Is the endpoint authenticated?
- [ ] Are error responses sanitized (no sensitive data in error bodies)?
- [ ] Is the endpoint covered by at least one test?

### No Dropped Commitments (MANDATORY)

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
- **Mock data in production code** -- `vi.mock` only in test files.

---

## Security Standards

Enforcer is itself a security product. Its own implementation must meet high standards:

| Requirement | Rule |
|---|---|
| **Agent runtime is untrusted** | Even locally-initiated agents are untrusted by default. Enforcement must not depend on agent cooperation. |
| **Audit trail immutability** | No UPDATE or DELETE on audit events. Append-only. Must support forensic investigation. |
| **Secrets never in logs or context** | Redact credentials, tokens, and PII from audit events, error messages, and model context. |
| **Error messages never expose sensitive data** | Sanitize all error responses. No file contents, credentials, or policy internals in error output. |
| **Policy decisions are mandatory** | Passive logging is not enforcement. Every governed action must receive a policy decision before execution. |

---

## Git Commit Rules (MANDATORY)

- **Never** run `git commit`, `git push`, or any git write commands. The developer handles all commits.
- **Never** add `Co-Authored-By` trailers or AI attribution to commit messages.
- When asked about commits, only suggest the commit message -- do not execute it.

---

## No Shortcuts Policy (MANDATORY)

- **Never take shortcuts.** Every feature must follow: implementation, tests, build verification.
- **Never skip testing.** A security product with untested enforcement is worse than no product.
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

## Testing Requirements (MANDATORY, once code exists)

### Test maintenance rule

When ANY code change modifies enforcement, policy, API, or UI behavior:
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

---

## Self-Check Before Reply

Before you respond or ship a change, confirm:

1. **Evidence cited** -- every factual claim references verified sources. No guessing.
2. **Scope respected** -- change touches only what was requested. No bundled commits.
3. **Gates cleared** -- tsc clean, vitest green, build passes (once code exists).
4. **Uncertainty surfaced** -- anything not verified is marked Unverified / Inferred / Unknown.
5. **No emoji, no AI attribution, no auto-commit.**

If any item is not met, fix before replying or flag the gap to the user.
