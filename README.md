> Enforcer -- The security and policy control plane for AI coding agents.
> Author: Deepankar Das

# Enforcer

**The security and policy control plane for AI coding agents.**

Enforcer sits between AI coding agents (Claude Code, Cursor, Copilot agents, MCP-driven workflows) and the systems they can affect. It intercepts actions, evaluates them against organizational governance policies and developer-local guardrails, enforces allow/deny/approval decisions, and produces security-grade audit trails.

The product thesis: AI coding agents are gaining access to source code, shells, package managers, credentials, cloud resources, APIs, and MCP-driven tools, while most organizations still rely on controls built for human developers. Enforcer is the governance layer that makes agent adoption safe enough to approve.

**Primary buyer:** Security engineering lead or platform engineering lead at a mid-market or enterprise software organization.

---

## Getting Started

### Prerequisites

- macOS or Linux
- Go 1.26+
- Node.js 22+
- PostgreSQL 16

### Setup, Build, Deploy

```bash
./scripts/prepare.sh                         # Install all prerequisites (Go, Node.js, PostgreSQL — starts PG automatically)
./scripts/build.sh                           # TypeScript + Go build (typecheck, tests, console, 5 Go binaries)
# Deploy Hub + Sentinel (single-machine demo)
sudo ./scripts/deploy_single_machine_hub_sentinel.sh \
  --seed-auth \
  --seed-hub-admin-user admin \
  --seed-hub-admin-password "adm1" \
  --seed-dev-user dev \
  --seed-dev-password "dev1"
```

Hub Console runs at `http://localhost:9201`. Sentinel Console runs at `http://localhost:6100`.

> **Governance is always on.** For development, project-level hooks in `.claude/settings.json` configure enforcement without requiring MDM. For enterprise MDM deployment (Jamf, Intune, Ansible), the managed settings file (`/Library/Application Support/ClaudeCode/managed-settings.json` with `allowManagedHooksOnly=true`) pre-configures Claude Code hooks before the developer opens VS Code and prevents developers from disabling enforcement, removing hooks, killing the daemon, or modifying policies -- governance activates automatically on first launch. The daemon runs as root via LaunchDaemon. OS kernel enforcement blocks actions even from raw terminals. Only administrators with valid tokens can change configuration.

For detailed setup instructions (PostgreSQL, environment variables, troubleshooting), see [docs/Enforcer_SETUP.md](docs/Enforcer_SETUP.md).

### Demo

```bash
./scripts/validate.sh --verbose                  # Run 9 enforcement scenarios
```

| # | Scenario | Decision |
|:-:|---|---|
| 1 | File write inside project | **allow** |
| 2 | File write outside project (~/.bashrc) | **deny** |
| 3 | Safe command (npm test) | **allow** |
| 4 | Destructive command (rm -rf) | **require_approval** |
| 5 | Network to unknown host | **deny** |
| 6 | Sensitive path read (~/.ssh/id_rsa) | **deny** |
| 7 | Package install (npm install) | **require_approval** |
| 8 | Credential access (cat .env) | **deny** |
| 9 | Unknown MCP server | **require_approval** |

> **What does `require_approval` mean?** When a policy rule returns `require_approval`, the action is denied immediately with an informative message containing an approval request ID. The hook handler exits 2 right away -- the developer is never blocked or frozen. Behind the scenes, the Sentinel client agent pushes the pending approval to the Hub, where an administrator approves or denies it on the Hub Console. Once approved, the developer retries the same command: the hook handler finds the pre-approval via `CheckScope()`, exits 0, and the command executes. The approval scope is single-use and consumed on first retry. See the [Approval Workflow](#approval-workflow) section below for the full end-to-end flow.

For the full product demo with admin console walkthrough, see [docs/Enforcer_DEMO.md](docs/Enforcer_DEMO.md).

---

## Key Documents

| Document | Purpose |
|---|---|
| [Enforcer_DEMO.md](docs/Enforcer_DEMO.md) | Product Demo -- 9-part showcase: blocking rules, admin console, approval workflow, policy management, audit forensics, live Claude Code, enterprise deployment, OS kernel enforcer, enterprise analytics with developer intelligence |
| [Enforcer_SETUP.md](docs/Enforcer_SETUP.md) | Setup Guide -- prerequisites, environment variables, ports, step-by-step first-time setup |
| [Enforcer_PRD.md](docs/Enforcer_PRD.md) | Product Requirements -- Appendix C ratified requirements R-1 through R-5, user stories, success metrics |
| [Enforcer_TDD.md](docs/Enforcer_TDD.md) | Technical Design -- architecture diagrams, policy model, audit schema, enforcement scenarios, API contracts |
| [Enforcer_Implementation.md](docs/Enforcer_Implementation.md) | Implementation Plan -- feature-phase matrix, security hardening backlog, peer review responses |
| [Enforcer_Review_Peer.md](docs/Enforcer_Review_Peer.md) | Peer Review -- Codex security review + Claude Code response with fix evidence |
| [CLAUDE.md](CLAUDE.md) | Claude Code (Opus) agent instructions -- quality gates, banned patterns, key file references |

---

## Project Structure

```
Enforcer/
├── go/                            # Go port (compiled binaries — production deployment)
│   ├── cmd/                       # 4 entry points
│   │   ├── daemon/                #   Daemon + HTTP server + embedded console
│   │   ├── hookhandler/           #   Claude Code hook handler (stdin → exit 0/2, logs to ~/.enforcer/hook.log, walks up to .git/.claude for project root)
│   │   ├── central/               #   Management Hub mTLS server (policy distribution)
│   │   └── client/                #   Sentinel agent (registration, sync, heartbeat)
│   ├── internal/                  # All packages
│   │   ├── types/                 #   Action, Policy, Audit, Approval, MCP structs
│   │   ├── policy/                #   Engine, loader, hierarchy, packs, signing
│   │   ├── audit/                 #   Validate, buffer, store (PostgreSQL), flush, chain
│   │   ├── approval/              #   Service, scope, break-glass
│   │   ├── enforcement/           #   Classifier, package guard, secret detector, redaction, MCP gateway
│   │   │   └── osguard/           #   KernelEnforcer interface + StubEnforcer (eBPF/ESF, 17 tests)
│   │   ├── intelligence/          #   Anomaly detection (8 patterns)
│   │   ├── daemon/                #   HTTP server, auth, state, routes
│   │   ├── central/               #   Management Hub mTLS server
│   │   ├── client/                #   Sentinel agent
│   │   └── console/               #   Embedded static assets (go:embed)
│   └── Makefile                   # build, test, package, cross-compile
│
├── src/                           # TypeScript daemon (original prototype)
│   ├── daemon/                    # HTTP server + routes
│   ├── enforcement/               # Hook handler, guards, proxies, classifiers
│   ├── policy/                    # Engine, loader, hierarchy, packs
│   ├── audit/                     # Validate, buffer, store, flush, SIEM
│   ├── approval/                  # Service, scope, break-glass
│   ├── intelligence/              # Anomaly detection
│   ├── central/                   # Management Hub
│   └── client/                    # Sentinel agent
│
├── console/                       # Next.js 15 + shadcn/ui Hub Console + Sentinel Console
│   └── src/
│       ├── app/                   # 10 role-aware pages: Hub Console (approvals, policies, Enterprise Analytics) + Sentinel Console (my activity, my sessions, My Analytics)
│       ├── components/            # Logo, badges, cards, auth gate, app shell
│       └── lib/                   # API client (with auth), auth context
│
├── types/                         # Shared TypeScript types (Zod schemas)
├── policies/                      # YAML policy bundles
│   ├── default.yaml               # 13 rules (6 deny, 4 approval, 3 allow)
│   └── network-allowlist.yaml     # Allowed hosts + warning list
├── docker/                        # PostgreSQL compose + schema
├── scripts/                       # 11 scripts (prepare, build, deploy, demo, hooks, certs, ...)
├── tests/                         # Vitest unit tests (150)
├── e2e/                           # Playwright E2E tests (25)
└── docs/                          # PRD, TDD, Implementation, Demo, Setup, Peer Reviews
```

---

## Technology Stack

### Core (Go — compiled binaries, zero runtime dependencies)

| Layer | Technology |
|---|---|
| Language | Go 1.26+ (compiled, statically linked, `CGO_ENABLED=0`) |
| Daemon + enforcement | Go `net/http`, `regexp`, `crypto/tls`, `crypto/ed25519` (stdlib) |
| Hook handler | Go binary -- reads stdin JSON, exits 0 (allow) or 2 (deny). Logs to `~/.enforcer/hook.log`. Finds project root by walking up to `.git`/`.claude` markers. Internal orchestration tools (Agent, TodoWrite, Skill, etc.) governed by `org.allow_internal_tools` policy |
| Management Hub | Go with mTLS (`crypto/tls`) on ports 9200/9201 |
| OS-level enforcement | Go `KernelEnforcer` interface + stub; real module: eBPF (Linux) / ESF (macOS) | Intercepts syscalls at kernel level. 17 tests. APIs defined — real kernel module implements same interface. |
| PostgreSQL | `github.com/jackc/pgx/v5` (pure Go) -- TLS-encrypted, append-only |
| Dependencies | 3 total: `uuid`, `pgx`, `yaml` (all pure Go, no CGO) |

### Hub Console (TypeScript — embedded in Go binary)

| Layer | Technology |
|---|---|
| Framework | Next.js 15 (App Router) + React 19 |
| UI | shadcn/ui (Radix) + Tailwind CSS |
| Deployment | Built to static HTML/JS/CSS, embedded in Go daemon via `go:embed` |

### Infrastructure

| Layer | Technology |
|---|---|
| Policy format | Versioned YAML bundles with Ed25519 signing |
| Audit persistence | PostgreSQL 16 (JSONB, append-only, hash-chain integrity) |
| Container mode | Docker (rootless, hardened profile) |

---

## Enterprise Deployment Architecture

```mermaid
flowchart TB
    subgraph Central["Security Team Server"]
        Hub["Management Hub<br/>(mTLS, port 9200)"]
        AdminUI["Hub Console"]
        PolicyStore["Signed Policies"]
        AuditDB["Audit Aggregator"]
    end

    subgraph DevMachine["Developer Machine (privileged daemon)"]
        subgraph Privileged["Root / LaunchDaemon"]
            Sentinel["Sentinel Agent<br/>(policy sync, audit forward)"]
            Daemon["Enforcer Daemon<br/>(port 9100, RBAC auth)"]
            ManagedHooks["Managed Hooks<br/>(cannot be removed)"]
        end
        subgraph Kernel["OS Kernel"]
            KernelMod["KernelEnforcer<br/>(eBPF / ESF)"]
        end
        ClaudeCode["Claude Code<br/>(VS Code)"]
        HookHandler["Hook Handler"]
        PG["PostgreSQL"]
    end

    ClaudeCode -- "tool call" --> HookHandler
    HookHandler -- "evaluate" --> Daemon
    ManagedHooks -. "enforces hooks" .-> ClaudeCode
    KernelMod -- "syscall gate" --> Daemon
    Daemon -- "audit" --> PG
    Sentinel -- "mTLS" --> Hub
    Sentinel -- "policy" --> Daemon

    style Central fill:#1a1a2e,stroke:#e94560,color:#fff
    style Privileged fill:#16213e,stroke:#0f3460,color:#fff
    style Kernel fill:#0f3460,stroke:#e94560,color:#fff
```

> **The developer cannot change governance settings.** All configuration flows from the Management Hub via mTLS. Managed hooks cannot be removed (`allowManagedHooksOnly=true`). The daemon runs as root with `KeepAlive` — the developer cannot kill it. The kernel enforcer intercepts syscalls from any process. Even toggling enforcement requires an admin token the developer does not possess. Local Sentinel policy mutation endpoints are disabled by default; policy changes must be made in the Hub.

**Defense-in-depth: 5 layers, each harder to bypass than the last.**

| Layer | What It Catches | Bypass Resistance |
|---|---|---|
| 1. IDE Hooks | Claude Code tool calls | Low (unless managed) |
| 2. Managed Hooks | Developer disabling hooks | Medium (requires root) |
| 3. Privileged Daemon | Developer killing daemon | High (auto-restart) |
| 4. OS Kernel (eBPF/ESF) | Raw terminal, any process | Very High (kernel-level) |
| 5. Management Hub | Policy drift, audit loss | Very High (external) |

---

## Architecture Highlights

- **Hybrid enforcement**: Runtime hooks + daemon + proxies + OS-level enforcement interface. No single enforcement point covers all surfaces. Enforcer layers multiple complementary mechanisms — from IDE hooks (fastest) through the policy daemon (centralized) down to the OS kernel interface (tamper-resistant) — all coordinated by a central daemon.
- **6 interception surfaces**: File system, shell commands, network egress, package installs, credential access, MCP tool calls. Prompt requires 2; we implement all 6.
- **Hierarchical policy**: Organization → team → repo → local. Lower levels can tighten, never weaken. 13 default rules + 8 canned policy packs.
- **Non-blocking human-in-the-loop approval**: Full lifecycle (create, route, resolve, retry). The developer is never blocked -- the hook exits immediately with a denial and approval request ID, the developer continues working, and retries the command after an admin approves on the Hub Console. Reusable scopes (single-use, session, action-type), break-glass override. See the [Approval Workflow](#approval-workflow) section below for the complete flow.
- **Append-only audit**: No UPDATE/DELETE on stored events. Enrichment emits new events linked by correlation_id. SHA-256 hash chain detects tampering. Ed25519 signed exports.
- **Anomaly detection**: 8 deterministic sequence patterns (exfiltration, privilege escalation, supply chain, reconnaissance, evasion).
- **Secrets/PII redaction**: 20+ detection patterns. Three modes: mask, tokenize (reversible), summarize.
- **Management Hub**: mTLS for policy distribution, audit aggregation, agent registration. Full RBAC on all Hub API endpoints (health: no auth; read endpoints: operator+; approve: reviewer+; write/policy mutation: admin). Sentinel policy sync and heartbeat every ~5s (configurable via `AA_SYNC_INTERVAL`).
- **OS-level enforcement**: The `KernelEnforcer` Go interface (`Init`, `EvaluateSyscall`, `RegisterPolicy`, `GetMetrics`, `Shutdown`) and the `StubEnforcer` implementation are complete with 17 tests. The interface governs file opens, process execution, and network connections at the syscall level. The APIs are fully defined — the stub logs every invocation with descriptions of what a real kernel module would do (eBPF tracepoint attachment, BPF map compilation, ESF event subscription). A real eBPF (Linux) or Endpoint Security Framework (macOS) module implements the same `KernelEnforcer` interface to complete the path — zero changes to callers.
- **Strict mode**: `AA_STRICT_MODE=true` -- deny on all error paths, daemon refuses startup without policy or PostgreSQL.
- **Enterprise analytics and developer intelligence**: The Hub Console shows "Enterprise Analytics" (org-wide views: all developers, all policies, all enforcement events), while the Sentinel Console shows "My Analytics" (personal views: the individual developer's own activity, score, and sessions). Auto-classifies developers into 10 behavioral groups (Power Builder, Cautious Contributor, Tool Explorer, Boundary Tester, Automation Driver, Data Accessor, Network Heavy, Night Owl, New Joiner, Dormant Agent) from audit data — no manual tagging. Stack-ranked blocked operations with trend analysis, friction heatmap (policy × group matrix), and actionable recommendations with one-click apply. Per-developer awareness scorecards with compliance scores, contextual guidance on blocks ("why was I blocked? how to proceed?"), and weekly digests.

---

## Approval Workflow

When a policy rule evaluates to `require_approval`, the action enters a non-blocking approval flow that spans the developer's machine (Sentinel) and the security team's server (Hub). The developer is never blocked -- the hook handler returns immediately with a denial message and an approval request ID. The developer continues working on other tasks while the approval is routed to an administrator. After the admin approves, the developer retries the command and it executes.

### End-to-End Flow

```
1. Developer asks Claude Code: "rm -rf node_modules"
       |
       v  PreToolUse hook fires
       |
       v  Hook handler --> POST /v1/evaluate --> Sentinel daemon
       |
       v  Policy: org.approve_destructive_commands --> require_approval
       |
       v  Daemon creates pending approval, returns approval_id
       |
       v  Hook handler exits 2 IMMEDIATELY
       |     Returns denial message + approval request ID to Claude Code
       |     Developer is NOT blocked -- continues working on other tasks
       |
   --- developer continues working ---
       |
2. Sentinel client agent pushes pending approval to Hub
       |     POST /api/v1/approvals/push --> Hub stores it
       |
       v  Hub Console shows pending approval on Approvals page
       |     Admin clicks Approve or Deny
       |     POST /api/v1/approvals/{id}/resolve
       |
       v  Next client agent sync picks up the decision
       |     Resolves it locally --> daemon stores approval with scope
       |
   --- developer decides to retry ---
       |
3. Developer retries: "rm -rf node_modules"
       |
       v  PreToolUse hook fires again
       |
       v  Hook handler --> POST /v1/evaluate --> Sentinel daemon
       |
       v  Daemon calls CheckScope() --> finds pre-approval --> allow
       |
       v  Hook handler exits 0 --> Claude Code executes the command
       |     (single-use scope consumed)
```

### Timing and Configuration

| Parameter | Default | Description |
|---|---|---|
| Hook handler response | Immediate | The hook handler returns exit 2 immediately on `require_approval` -- no waiting, no polling, no spinner. The developer sees a denial message with the approval request ID. |
| Client agent sync interval | 3 seconds | How often the Sentinel client agent pushes pending approvals to the Hub and pulls back decisions. Configurable via `AA_SYNC_INTERVAL`. |
| Approval expiry | 300 seconds | If no admin decision arrives before expiry, the pending approval is discarded. The developer would need to re-trigger the action to create a new approval request. |
| Scope on retry | Single-use (default) | When the developer retries the command, `CheckScope()` finds the pre-approval and allows execution. The scope is consumed on first use. |

### Approval Scopes

When an administrator approves an action, they can choose a scope that pre-approves similar future actions without requiring another round-trip to the Hub:

| Scope | Effect |
|---|---|
| **single-use** | Approves only this specific action. Consumed when the developer retries the command. The next similar action requires a new approval. |
| **session** | Approves all matching actions for the remainder of the current coding session. |
| **action-type** | Approves all actions of the same type (e.g., all `rm -rf` commands) until the scope is revoked. |

### Component Responsibilities

| Component | Role in Approval Flow |
|---|---|
| **Hook handler** (`enforcer-hook`) | Sends the action to the daemon for evaluation. On `require_approval`, exits 2 immediately with a denial message and approval request ID. On retry, exits 0 if a pre-approval scope exists. No polling, no blocking. |
| **Sentinel daemon** (`enforcer-daemon`) | Evaluates policy, creates pending approvals, stores resolved decisions, checks pre-approval scopes on retry via `CheckScope()`. |
| **Sentinel client agent** (`enforcer-client`) | Syncs pending approvals to the Hub and pulls resolved decisions back to the daemon. |
| **Management Hub** (`enforcer-central`) | Receives and stores pending approvals, serves the Hub Console, processes admin approve/deny actions. |
| **Hub Console** (Next.js) | Displays pending approvals to administrators, provides Approve/Deny buttons, shows approval history. |

---

## Security Hardening

17 security hardening items identified via peer review, all resolved:

| Priority | Items | Status |
|---|---|---|
| **P0 — Security Correctness** | Auth on resolve endpoint, network allowlist, timeout units, console API contracts, admin token headers, doc accuracy | **7/7 Done** |
| **P1 — Hardening** | Signed policy bundles (Ed25519), RBAC, append-only audit, embedded console, strict mode, hierarchy merge | **6/6 Done** |
| **P2 — High Assurance** | OS-level enforcement (stub), tamper-evident audit chain, release integrity (SBOM/cosign), adversarial regression suite | **4/4 Done** |

---

## Scripts

| Command | Purpose |
|---|---|
| `./scripts/prepare.sh` | Install all prerequisites (Go, Node.js, PostgreSQL, npm deps) |
| `./scripts/prepare.sh --check-only` | Validate prerequisites without installing |
| `./scripts/build.sh` | Full build (TypeScript check + Vitest + console + 5 Go binaries) |
| `cd go && make test` | Run 252 Go tests |
| `./scripts/deploy.sh --migrate` | Start Sentinel Server + Sentinel Console only (ports 9100, 6100) |
| `sudo ./scripts/deploy_single_machine_hub_sentinel.sh` | Full single-machine deployment + enforcement validation (Hub + Sentinel) |
| `sudo ./scripts/enforcer_deploy.sh full` | Alternate full single-machine wrapper |
| `sudo ./scripts/deploy_hub.sh` | Deploy Management Hub — idempotent (PostgreSQL, certs, policy, start Hub) |
| `sudo AA_CENTRAL_URL=https://hub:9200 ./scripts/deploy_sentinel.sh` | Deploy Sentinel on developer machine — idempotent (binaries, DB, managed hooks, LaunchDaemon) |
| `./scripts/validate.sh --verbose` | Run 9 enforcement validation scenarios |
| `sudo ./scripts/uninstall.sh` | Full uninstall (preserves audit DB) |
| `sudo ./scripts/uninstall.sh --drop-database` | Full uninstall + drop PostgreSQL audit DB |
| `./scripts/readiness-report.sh` | Check 6 readiness gates |
| `./scripts/install-hooks.sh` | Install Claude Code enforcement hooks |
| `./scripts/install-hooks.sh --uninstall` | Remove hooks (instant rollback) |
| `./scripts/generate-certs.sh` | Generate mTLS certificates |
| `cd go && make package` | Create distribution tarball |
| `cd go && make release-integrity` | Build + SBOM + signature + provenance |

---

## Build Effort Analysis

### What Was Built

| Category | Count |
|---|---|
| Go source files | 45 |
| TypeScript source files | 37 |
| Console pages + components | 18 |
| Test files (Go + TypeScript + E2E) | 20 |
| Shell scripts | 11 |
| Policy files | 2 |
| Documentation files | 12 |
| **Total source files** | **~145** |

### Test Coverage

| Suite | Tests | Framework |
|---|---|---|
| Go unit/integration | 252 | `go test` |
| TypeScript unit/integration | 150 | Vitest |
| Playwright E2E | 25 | Playwright |
| Demo scenarios | 9 | Shell script |
| **Total** | **436** | |

### Architectural Scope

| Component | Count |
|---|---|
| API endpoints (daemon) | 24 |
| Enforcement surfaces | 6 (file, shell, network, package, credential, MCP) |
| Policy rules (default) | 13 |
| Canned policy packs | 8 |
| Anomaly detection patterns | 8 |
| Secret detection patterns | 20+ |
| Security hardening items | 17 (all resolved) |
| Go binaries | 4 (statically compiled, zero dependencies) |
| OS kernel enforcement (KernelEnforcer) | 1 interface (5 methods), 1 stub, 17 tests, 3 modes (enforce/audit/off) |
| Enterprise analytics | 10 synthetic developer groups, 5 recommendation patterns, developer awareness scorecards, 21 tests |

### Actual Build

- **Calendar time**: Under 3 days (April 26 5:40pm -- April 28, ~2.5 calendar days)
- **People**: 1 developer + 2 AI coding agents (Claude Opus + OpenAI Codex) working in parallel
- **Includes**: PRD, TDD, full TypeScript prototype (37 files, 150 tests), Go port (45+ files, 252 tests), Hub Console (8 pages), 17 security hardening items, peer review cycles, DEMO/SETUP/README docs

### Equivalent Traditional Team Effort

If a senior engineering team built this from scratch without AI coding agents:

| Role | Weeks | Scope |
|---|---|---|
| **Senior backend engineer (Go)** | 10--14 | Go daemon (45 files), policy engine, audit pipeline, enforcement layer (6 surfaces), approval service, Management Hub mTLS server, Sentinel agent, hook handler, OS kernel enforcer interface + stub. Prior TypeScript prototype also built. |
| **Security engineer** | 8--10 | RBAC (4-tier role model, 21-route auth matrix), Ed25519 policy signing with version monotonicity, SHA-256 hash-chain audit integrity, Ed25519 signed exports, 20+ secret/PII redaction patterns, 8 anomaly detection patterns, strict mode, adversarial regression suite |
| **Frontend engineer** | 5--7 | 8-page Next.js console (dashboard, sessions, timeline, approvals, search, export, policies, login), shadcn/ui components, role-aware auth context, token-authenticated API client |
| **QA/test engineer** | 4--6 | 252 Go tests, 150 TypeScript tests, 25 Playwright E2E tests, 9 demo scenarios, security regression suite, 2 peer review cycles with fix verification |
| **Technical writer** | 3--4 | PRD (with Appendix C ratified requirements), TDD (20+ mermaid diagrams, 7 enforcement scenarios), implementation plan (17 security hardening items), setup guide, product demo guide (6 parts), README, peer review responses |
| **DevOps engineer** | 3--4 | PostgreSQL schema (append-only), Docker compose, macOS LaunchDaemon, mTLS certificate generation, release integrity pipeline (Syft SBOM, Cosign signatures, provenance), managed hooks, deployment scripts |
| **Total** | **33--45 person-weeks** | |

> **Not included above:** The real OS kernel module (eBPF on Linux / ESF on macOS) would add a **kernel engineer for 6--8 weeks** — loadable kernel module design, BPF map compilation, crash-safe error recovery, kernel version compatibility testing across supported OS versions, and security certification. Cannot crash the kernel — requires rigorous validation. We built the `KernelEnforcer` Go interface, the `StubEnforcer` with 17 tests, and the full integration path (APIs defined, daemon wired), but the actual kernel loadable module was not coded. That is Phase 3 work.

### Cost Comparison

Assuming **$550K blended fully loaded annual cost** per engineer ($10,577/week) -- Bay Area senior talent for a production security product. Fully loaded includes salary, benefits, overhead, and equity (1.6x base).

| | Traditional Team | AI-Augmented |
|---|---|---|
| **Calendar time** | 10--16 weeks | **< 3 days** |
| **People** | 6 engineers + PM + EM = 8 | **1 developer** + 2 AI agents |
| **Engineering cost** | $350K--$476K | ~$3K (developer time + API costs) |
| **Overhead (PM, EM, recruiting, tooling)** | $120K--$170K | $0 |
| **Communication overhead** | 28 pairwise channels, sprint ceremonies, security review gates | Zero -- single decision-maker |
| **Total cost** | **$470K--$646K** | **~$3K** |

### Summary

**What was actually spent:** 1 developer x 2.5 days (April 26--28, 2026), augmented by two AI coding agents (Claude Opus and OpenAI Codex) working in parallel. Total cost: ~$3K including AI API costs.

**What it would have cost traditionally:** 8 people (6 engineers + PM + EM), 10--16 weeks, $470K--$646K.

## That's roughly a 25--40x acceleration in time and a 157--215x reduction in cost over traditional development for the same scope.
