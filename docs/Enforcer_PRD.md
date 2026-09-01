> Author: Deepankar Das

# Enforcer — Product Requirements Document

## Executive Summary: Product and Implementation Overview

### Product Vision

Enforcer is a mandatory governance and security control plane purpose-built for AI coding agents operating in developer environments. It sits between AI agents (Claude Code, Cursor, Copilot agents, MCP-driven workflows) and the systems those agents can affect -- intercepting every sensitive action, evaluating it against organizational policy, and enforcing a deterministic outcome (allow, deny, or require approval) before the action executes. The product is designed for security engineering leads and platform engineering leads at mid-market and enterprise software organizations who need to move AI coding agent adoption from constrained pilots to governed production use. Enforcer is the trust boundary that converts agent productivity from an unmanaged experiment into an approvable operating model.

### Market Context

AI coding agents have shifted from passive code completion to autonomous task execution across terminals, codebases, package managers, network APIs, credentials, databases, and MCP tool ecosystems. This expansion increases both productivity upside and governance risk. Enterprise security controls built for human developers -- endpoint agents, secrets managers, CI policy gates -- do not natively understand agent intent, tool invocation context, MCP traffic, or the action chains between a prompt, a tool call, a file mutation, and a network request. Organizations face a deadlock: engineering leaders want agent leverage, but security leaders cannot approve broad rollout without a trust boundary purpose-built for this operating model. The governance gap is now the primary blocker to enterprise-scale AI coding agent adoption.

### Architecture Summary

Enforcer uses a Hub + Sentinel architecture with 5 defense layers:

- **Layer 1 -- Runtime Hook / SDK Wrapper.** Intent-aware interception before action execution. The hook handler binary (`enforcer-hook`) reads JSON from stdin, evaluates policy via the local daemon, and returns allow/deny/approval decisions via exit codes (0 = allow, 2 = deny). The hook handler performs project root walk-up detection and logs to `~/.enforcer/hook.log`. Tool mappings cover 8 system-affecting tools (Read, Edit, Write, Bash, WebFetch, WebSearch, Glob, Grep) and 17 internal orchestration tools.
- **Layer 2 -- Managed Hooks.** Hooks are installed by the Sentinel agent running as root (via sudo or MDM) into Claude Code's settings, ensuring developers cannot remove or bypass enforcement.
- **Layer 3 -- Privileged Daemon.** The local daemon (`enforcer-daemon`) runs on localhost:9100, performs policy lookup, decision caching, audit buffering, RBAC (operator role), and serves the Sentinel Console. It persists all audit events to PostgreSQL (append-only, no UPDATE or DELETE) with decision normalization and unlimited query limit. The daemon provides `governed_user` from health endpoint for Sentinel data filtering.
- **Layer 4 -- OS Kernel Enforcer (planned).** eBPF (Linux) / ESF (macOS) intercepts at the syscall level for file.open, execve, and connect -- catching raw terminal bypass outside the agent runtime.
- **Layer 5 -- Management Hub.** The central server (`enforcer-central`) uses mTLS on ports 9200 (client) and 9201 (admin) for policy distribution, audit aggregation, signed bundles, Sentinel heartbeat monitoring, and the Hub Console for security admins. All Hub state (policies, clients, enforcement toggles) is persisted in PostgreSQL. The Hub Console supports RBAC with admin and reviewer roles.

The Sentinel agent (`enforcer-client`) handles registration with the Hub, policy sync with hash-based change detection, heartbeat reporting, and audit event forwarding -- all over mTLS.

### Key Capabilities

**Six enforcement surfaces.** The policy engine evaluates actions across file system reads/writes, shell command execution, network egress, package installations, credential/secret access, and MCP tool invocations. The command classifier recognizes 15 destructive patterns (rm, chmod, git push --force, etc.), 13 network tool patterns (curl, wget, ssh, etc.), and 9 package manager patterns (npm install, pip install, cargo install, etc.).

**Policy engine.** Versioned YAML policy bundles with hierarchical inheritance (org, team, project, local). The engine evaluates subject (agent type, user), action type, and resource conditions to return allow, deny, or require-approval decisions with machine-readable reason codes and human-readable explanations. Policies support path-based rules (writes outside project root), pattern-based rules (command and file patterns), host allowlists with separate warning lists, and wildcard matching. Policy bundles can be cryptographically signed (Ed25519) with monotonic version enforcement. The system ships with 13 default policy rules and 8 canned policy packs covering source code protection, supply chain security, secrets hardening, infrastructure safety, network egress control, compliance/audit, developer best practices, and MCP governance.

**Approval workflow.** Non-blocking approval flow where the hook handler exits immediately and the developer retries after approval. Three scope types: single-use (consumed after one action), time-bounded (expires after a configurable window), and session-scoped (persists for the session). Approvals support pattern matching against action types and resource values. The system includes break-glass emergency overrides that bypass normal flow but record full audit context. Approval metrics track created, approved, denied, and expired counts.

**Dual console architecture.** Two separate console builds serve different audiences:

- **Hub Console** (port 9201, admin/reviewer RBAC): Dashboard, Sessions, Approvals, Search, Export, Policies, Analytics. The Analytics page shows Enterprise Analytics with org-wide metrics.
- **Sentinel Console** (port 9100, operator role): Dashboard, Sessions, Search, Export, My Activity. The My Activity page shows personal analytics for the governed developer.

Both consoles are built with Next.js 15 (App Router), React, shadcn/ui, and Tailwind CSS, compiled to separate static HTML/JS/CSS builds (`out-hub` and `out-sentinel`) and embedded in the respective Go binaries via `go:embed`. Developer groups are named professionally (Compliant Developer, High-Friction Developer, etc.). All drill-downs navigate Analytics to Search to Session detail.

**Audit trail.** PostgreSQL-backed, append-only audit store with decision normalization and unlimited query limit. Every intercepted action produces a structured event recording actor identity, session, agent type, action type, resource, policy evaluated, decision returned, reason code, and timestamp. Events are linked by session ID for chain-of-action reconstruction. The Hub aggregates audit events from all connected Sentinels into a central PostgreSQL instance.

**Secret detection and redaction.** The secret detector scans file paths against 20 sensitive file patterns (SSH keys, AWS credentials, Kubernetes configs, PEM keys, .env files, etc.) and commands against 12 sensitive command patterns (printenv, aws configure, vault access, macOS Keychain, etc.). The redaction engine uses 18 compiled regex patterns covering AWS keys, GitHub tokens, Slack tokens, Stripe keys, JWTs, private keys, database URLs, SSNs, credit cards, bearer tokens, generic API keys, and passwords. Redaction modes include mask, tokenize (reversible via token map), and summarize.

**Analytics and anomaly detection.** The analytics engine computes stack-ranked blocked operations, approval bottlenecks, per-developer enforcement impact, and developer group classifications (Compliant Developer, High-Friction Developer, and others named professionally). The Hub shows Enterprise Analytics (org-wide), while the Sentinel shows personal analytics for the individual developer. The recommender generates data-driven policy recommendations: auto-approve patterns for frequently approved actions, allowlist additions for commonly blocked hosts, evasion alerts for boundary testers, and onboarding assistance for new joiners. The anomaly detector maintains per-session event windows and evaluates sequence-based patterns for suspicious behavior.

### Current Implementation Status

The Go port is complete and produces 5 statically compiled binaries with zero runtime dependencies (`CGO_ENABLED=0`):

| Binary | Role |
|--------|------|
| `enforcer-daemon` | Local enforcement daemon with Sentinel Console (localhost:9100) |
| `enforcer-hook` | Hook handler for Claude Code PreToolUse/PostToolUse with project root walk-up |
| `enforcer-central` | Management Hub with mTLS (ports 9200/9201) and Hub Console |
| `enforcer-client` | Sentinel agent (registration, policy sync, heartbeat, audit forwarding) |
| `enforcer-authseed` | Authentication seed utility for initial credential provisioning |

The console uses dual builds: `out-hub` (embedded in `enforcer-central` for port 9201) and `out-sentinel` (embedded in `enforcer-daemon` for port 9100), each with separate navigation items tailored to their respective audiences.

### Deployment Model

The Hub (`enforcer-central`) runs on the security team's server with PostgreSQL for state persistence and audit aggregation. It exposes two mTLS ports: 9200 for Sentinel client communication and 9201 for admin access and the Hub Console (with admin/reviewer RBAC). The Sentinel (`enforcer-client`) runs on each developer machine, managed via sudo or MDM, and handles local enforcement through the daemon and hook handler. The Sentinel registers with the Hub, syncs policies (with hash-based change detection to minimize bandwidth), forwards audit events, and sends heartbeats. The Hub tracks Sentinel status (online, stale, offline) and can push policy updates and enforcement state changes to all connected Sentinels. Certificates for mTLS are generated via the `generate-certs.sh` script. Deployment scripts support single-machine (Hub + Sentinel collocated), separate Hub deployment, and separate Sentinel deployment.

### Differentiation

Enforcer differs from existing security tools in several concrete ways. Unlike AI gateways and LLM firewalls that inspect prompts at the model layer, Enforcer governs the actual machine actions agents execute -- file writes, shell commands, network calls, package installs. Unlike endpoint security tools that monitor process behavior, Enforcer understands agent intent and maps tool invocations to policy decisions before execution, not after. Unlike secrets managers that govern credential lifecycle, Enforcer governs the full action chain from prompt to tool call to system effect. The system enforces mandatory mediation (not passive logging), preserves policy rationale for every decision, supports non-blocking approval workflows that maintain developer flow, and produces forensic-grade audit trails linking actor, intent, policy, and outcome. The combination of runtime interception, protocol-aware governance (MCP tool mappings), hierarchical policy with signed bundles, approval orchestration with scoped permissions, and replayable audit context in a single coordinated control plane is the product's structural advantage.

---

## 0. MVP Prototype Requirements Traceability

This section maps every requirement from the [Enforcer venture prompt](Enforcer_Prompt.md) to the MVP prototype, showing how each item is addressed and where it integrates into the product architecture.

### 0.1 Deliverable 1 — Working Prototype

The venture prompt states: *"Build a functional prototype that intercepts, inspects, and governs the actions of an AI coding agent operating on a developer environment."*

| Req No. | Requirement | Understanding | Details | Integration |
|---|---|---|---|---|
| **MVP-1** | **Intercept agent actions across at least two of: file system reads/writes, shell command execution, network calls, package installs, credential or secret access** | The requirement asks Enforcer to prove it can sit in the critical path of agent execution — not just observe after the fact, but intercept before the action reaches the operating system or network. "At least two" sets a floor; the real test is whether interception is pre-execution (can block) rather than post-execution (can only log). The five listed surfaces represent the highest-risk action classes for enterprise security teams: file mutation can corrupt codebases, shell execution can destroy infrastructure, network egress can exfiltrate IP, package installs can introduce supply-chain attacks, and credential access can escalate privilege. Covering more surfaces with real enforcement is strictly better, but each surface requires a different interception mechanism, so the requirement wisely limits the minimum to avoid shallow coverage across all five. | The MVP intercepts three surfaces (exceeding the minimum of two): (1) **file system reads/writes** via a filesystem guard that enforces project-path boundaries and logs all file operations with path, content hash, and policy decision; (2) **shell command execution** via a shell proxy that captures commands before execution, evaluates them against command-pattern policies, and blocks or gates destructive/exfiltrative commands; (3) **network egress** via a network proxy that enforces host allowlists, logs destination metadata, and blocks calls to non-sanctioned hosts. Package installs and credential access are governed as P1 extensions. | Filesystem guard, shell proxy, and network proxy all report to the local daemon, which evaluates policy and emits normalized audit events. Primary target agent is Claude Code (VS Code + CLI) with full pre-execution interception; secondary is Cursor via MCP gateway. See Section 8.4 (MVP Integration Targets), Section 11 (Architecture), FR-1 (Action Interception), and Section 14 (Threat Surface Matrix). |
| **MVP-2** | **Enforce a configurable policy (allow / deny / require approval) with at least one non-trivial rule** | The three decision types — allow, deny, require approval — represent a minimal but complete governance vocabulary. "Allow" permits safe work to proceed without friction. "Deny" hard-blocks actions that violate policy unconditionally. "Require approval" is the most interesting: it introduces human judgment into the loop, enabling controlled risk acceptance rather than binary allow/deny. "Configurable" means policies must not be hardcoded — enterprises need to tune rules to their own risk tolerance, environment, and workflows. "Non-trivial" means the rule must depend on context (path, host, command pattern, etc.), not just action type. A rule like "deny all shell commands" is trivial and useless; "deny shell commands matching exfiltration patterns unless the user holds an elevated approval" is non-trivial and demonstrates real governance value. The deeper implication: the policy engine must be expressive enough to encode real enterprise security policies, not just demo rules. | The MVP ships a hierarchical policy engine supporting three decision types — allow, deny, and require approval — evaluated against context including actor, agent type, session, file path, command pattern, destination host, and environment. Non-trivial rules shipped by default: (a) deny writes outside the project root directory; (b) deny outbound network calls to hosts not on the organization allowlist; (c) require human approval before executing shell commands matching high-risk patterns (rm -rf, git push --force, curl to unknown hosts). Policies are configurable per organization, team, and developer-local scope with hierarchical inheritance where lower levels can tighten but never weaken upper-level baselines. | Central policy engine with hierarchical inheritance (org → team → repo → local). See Section 12 (Policy and Governance Model), FR-3 (Policy Engine), and Section 15.1 (MVP Feature Priorities — P0). |
| **MVP-3** | **Produce a structured audit log of agent actions that a security reviewer could meaningfully read** | This requirement has two parts that are easy to conflate but are distinct. "Structured" means machine-parseable, schema-consistent, filterable, and queryable — not free-text log lines or unstructured JSON blobs. "Meaningfully read" means the log must tell a story a security reviewer can follow: who did what, to which resource, under which policy, with what outcome, and with what approval chain. Most existing tools fail the "meaningfully read" test because they emit raw OS events (process IDs, syscall numbers, file descriptors) that require deep system knowledge to interpret. Enforcer's audit must bridge the semantic gap between agent intent ("I want to install lodash") and system effect ("npm install lodash was executed, package.json was modified, network call to registry.npmjs.org was made"). The audit trail is also the product's compliance asset — it is the evidence that enterprises present during security reviews, incident investigations, and regulatory audits to prove governance was active. If the audit is noisy, incomplete, or hard to navigate, the product fails its core enterprise value proposition. | Every intercepted action generates a structured audit event containing: timestamp, agent identity, user identity, session ID, workspace/repo context, action type, resource target, payload summary (redacted as appropriate), policy evaluated, decision returned, approver identity and rationale (when applicable), and observed execution result. Events are append-only (no UPDATE or DELETE). The security review console provides session-level replay, search/filter across all dimensions, and exportable evidence packages. Audit events are linked by session and delegation chain — not flat isolated logs, but a causal narrative of the agent's behavior. | Audit event pipeline → append-only audit store → security review console. See FR-5 (Audit Trail and Session Replay), FR-10 (Security Console), and Section 22 (Success Metrics — audit completeness rate). |
| **MVP-4** | **Implement one depth area: real-time human-in-the-loop approval UX, anomaly detection over agent action sequences, secrets/PII redaction in agent context, multi-agent policy isolation, or org-level policy distribution** | The five listed depth areas each represent a different dimension of enterprise agent governance. The requirement explicitly asks for one — going deep rather than spreading thin. **Approval UX** is about converting hard blocks into supervised permission, which is the buyer's core ask: "let agents work, but let me control the risky parts." **Anomaly detection** is about identifying suspicious behavior patterns (e.g., "read .env then curl to unknown host") — powerful but requires ML infrastructure and stable event models. **Secrets/PII redaction** prevents sensitive data from leaking into model context or logs — critical for compliance but narrower in scope. **Multi-agent isolation** prevents cross-session data leakage — important as agent architectures grow more complex. **Org-level policy distribution** enables central governance at scale — essential for enterprise but less dramatic in a prototype. The choice of depth area should maximize both demo impact and pipeline validation. Approval UX is the strongest choice because: it is visible to both developers and security reviewers; it exercises the full interception → policy → human decision → audit pipeline; it directly demonstrates the product's core value proposition of converting "blocked" to "approved with oversight"; and it does not require ML infrastructure or complex distributed systems. | The MVP implements **non-blocking human-in-the-loop approval UX** as the primary depth area. When a policy evaluates to "require approval," the hook handler exits immediately with a deny exit code, creates an approval request, and the developer retries after approval is granted. Approvals are managed via the Hub Console (admin/reviewer RBAC on port 9201) and the Sentinel Console (operator role on port 9100). Approvals support three scope types (single-use, time-bounded, session-scoped), pattern matching against action types and resource values, and break-glass emergency access with explicit exception tracking. Every approval decision — including approver identity, rationale, scope, and expiry — is recorded in the audit trail. Approval metrics track created, approved, denied, and expired counts. | Approval service integrated with policy engine and audit pipeline. Hub Console and Sentinel Console surface approvals. See FR-4 (Approval Workflows), FR-10 (Security Console), FR-11 (Developer Transparency), and Section 15.2 (Recommended Depth Area rationale). |
| **MVP-5** | **Depth over breadth: going deep on one area is stronger than touching all of them** | This is a meta-requirement about product strategy, not a feature. It says: do not build a shallow prototype that can technically claim all five surfaces and all five depth areas but does none of them convincingly. The evaluation will favor a product that can demonstrate real, working, mandatory enforcement on a narrow scope over a product that logs events across many surfaces without actually blocking anything. The implication for engineering is: every surface we claim to cover must have real enforcement (pre-execution blocking, not post-execution logging), real policy evaluation (context-aware decisions, not hardcoded rules), and real audit evidence (structured, queryable, reviewer-friendly events). It is better to ship three surfaces with real enforcement and one excellent depth area than to ship five surfaces with shallow logging and no depth. The secondary implication: defer capabilities that cannot be made excellent in the MVP timeframe. Anomaly detection, multi-agent isolation, and advanced secrets redaction are all valuable but each requires significant infrastructure investment. Ship them when the foundation is proven, not before. | The MVP prioritizes depth in two areas: (1) **approval UX** as the primary depth area — fully realized with in-IDE delivery, time-bounded scopes, reusable windows, break-glass access, and full audit integration; (2) **MCP-aware governance** as a secondary differentiator — protocol-level tool governance that no legacy security product can replicate. All other capabilities (anomaly detection, multi-agent isolation, advanced secrets redaction) are explicitly scoped to P2 and deferred until the core engine is proven. The three interception surfaces (file, shell, network) are each implemented with real enforcement — not shallow logging — including mandatory policy evaluation, blocking capability, and structured audit events for every action. | See Section 15 (MVP Definition), Section 16 (Phased Roadmap — Phase 1), and Section 24 (Sequencing Rationale). |

### 0.2 Deliverable 2 — PRD

The venture prompt states: *"Cover user, pain point, market wedge, MVP scope, and sequencing rationale. Specify the primary buyer and why."*

| Req No. | Requirement | Understanding | Details | Integration |
|---|---|---|---|---|
| **PRD-1** | **User** | The requirement asks us to define who will actually use the product day-to-day — not just who buys it. Enterprise security products fail when they only consider the buyer persona and ignore the daily operator and the end user who must tolerate the controls. Enforcer touches at least three distinct interaction surfaces: the security team that reviews and investigates, the platform team that configures and deploys, and the developer whose agent workflow is being governed. Each persona has different tolerance for friction, different information needs, and different definitions of "value." A product that nails the buyer persona but alienates developers will be bypassed; a product that delights developers but fails security review will never be purchased. | Five user personas defined: Security Engineer (reviewer), Platform Engineer (administrator), Developer (end user), Engineering Manager (approver), Executive Buyer (CISO/VP). Each includes role description, daily interaction pattern, and core needs. Jobs-to-be-done statements articulate what each persona is trying to accomplish in their own language. | Section 5.3 (User Personas), Section 5.4 (Jobs to Be Done). |
| **PRD-2** | **Pain point** | The pain is not "AI is risky" in the abstract — the specific, concrete pain is that security and platform teams cannot answer basic governance questions about what agents do, and this inability blocks adoption that the organization wants. The pain is felt as deployment friction: pilots stall, rollout proposals fail security review, and engineering leaders cannot get budget approval for tools their teams want to use. The pain is acute because it sits at the intersection of strong demand (productivity gains from agents) and strong fear (uncontrolled access to sensitive systems). Every week the governance gap persists, the organization either accepts unmitigated risk or forgoes measurable productivity gains. | The core pain point is the governance gap: AI coding agents take actions on real systems at machine speed, while security teams cannot answer what the agent did, what it tried to access, whether policy was enforced, and what evidence exists for review. This blocks enterprise-wide adoption. Nine specific threat surfaces are detailed with concrete risk examples spanning file mutation, shell execution, network exfiltration, supply-chain attacks, credential theft, MCP abuse, database extraction, context leakage, and unbounded delegation. | Section 3 (Problem Statement), Section 3.3 (Full Threat Surface), Section 3.4 (The Adoption Blocker). |
| **PRD-3** | **Market wedge** | The wedge is not "AI security" broadly — that category is crowded and vague. The wedge is the specific, narrow, high-urgency blocker: the security review that prevents enterprise-wide agent rollout. This is a blocking problem, not an optimization problem. Buyers can justify spend if Enforcer converts "not yet approved" into "approved with controls." The wedge must be defensible against three adjacencies: LLM guardrails vendors (who govern prompts, not machine actions), endpoint security vendors (who were built for humans, not agents), and IDE vendors (who have vendor-specific controls, not neutral enforcement). The sharpest positioning is "runtime governance for AI coding agents" — specific enough to own, broad enough to expand from. | The wedge is the security review that blocks broad agent adoption in mid-market and enterprise engineering organizations. Enforcer removes the blocker by providing mandatory control, not just observability. Positioned as "runtime governance for AI coding agents" — distinct from LLM guardrails, endpoint security, and broad AI security platforms. GTM positioning table maps against five adjacent categories with specific differentiation messages. | Section 17 (Market Appetite), Section 17.2 (Positioning Statement), Section 17.4 (GTM Positioning Against Adjacent Categories). |
| **PRD-4** | **MVP scope** | MVP scope must be ruthlessly bounded. The prompt explicitly rewards depth over breadth, so the MVP should not attempt to cover every agent, every surface, and every enterprise need. The scope should be narrow enough to be excellent and broad enough to be credible. "Credible" means a security reviewer can watch the demo and believe the product could pass their organization's security review. "Excellent" means the surfaces we cover have real enforcement (not logging), real policy (not hardcoded rules), and real audit (not raw events). The out-of-scope list is as important as the in-scope list — it shows discipline and signals that the team understands what matters first. | MVP covers three interception surfaces (file, shell, network), policy engine with allow/deny/approval and hierarchical inheritance, structured audit logs with session-level replay, security review console with search/filter/export, approval UX as primary depth area with in-IDE delivery, VS Code integration, opinionated default policies, secure-container deployment mode, and support for Claude Code, Cursor, and Codex. Explicit out-of-scope: CNAPP, broad endpoint monitoring, universal agent support, LLM content moderation, ML-based anomaly detection. | Section 8 (Product Scope), Section 15 (MVP Definition). |
| **PRD-5** | **Sequencing rationale** | The sequencing must answer why this order and not another. The logic is: (1) mandatory enforcement on the highest-risk surfaces must come first because it establishes credibility — without it, the product is just another dashboard; (2) protocol-aware expansion (MCP, secrets) comes second because it creates differentiation that legacy tools cannot replicate; (3) intelligence layers (anomaly detection, graph replay) come last because they require a stable event model and replay substrate to build on, and shipping them prematurely creates noisy, unreliable systems. Each phase must be independently valuable — Phase 1 alone must be enough to pass a security review and win a pilot customer. | Depth over breadth: win one high-trust workflow deeply before broadening. Phase 1 proves mandatory enforcement on file/shell/network with excellent approval UX and credible audit. Phase 2 adds protocol-aware expansion (MCP, secrets, packages, SIEM integration). Phase 3 builds enterprise control plane (multi-agent governance, anomaly detection, graph replay, policy simulation). Intelligence layers wait until the event model and replay substrate are stable. Each phase is independently deployable and valuable. | Section 16 (Phased Roadmap), Section 24 (Sequencing Rationale). |
| **PRD-6** | **Primary buyer and why** | The prompt asks us to name the specific role that signs the purchase order, not just "enterprise security." The buyer must be the person whose problem the product directly solves and who has budget authority. The security engineering lead is the right choice because: (a) they own the security review process that is the literal blocker — when they say "not approved," agent rollout stops; (b) they have direct budget for developer-security infrastructure; (c) they are the person who will be called at 2 AM if an agent exfiltrates source code; (d) they are motivated by both risk reduction and by enabling the engineering organization to use tools safely. The platform engineering lead is the technical champion who validates the product works in their stack. The CISO is the economic sponsor for larger deals. Getting the buyer wrong means selling to someone who cannot actually unblock the purchase. | Primary buyer is the **security engineering lead** at a mid-market or enterprise software organization (300+ engineers). Why: this role is directly accountable for approving or blocking coding-agent rollout, owns the security review process that is the adoption bottleneck, and has budget authority for developer-security infrastructure. Secondary buyers: platform engineering lead (technical champion who validates deployment), CISO/VP Security (economic sponsor for enterprise deals). | Section 5.1 (ICP), Section 5.2 (Buyer Map). |

### 0.3 Deliverable 3 — TDD

The venture prompt states: *"Cover architecture decisions, where the interception layer sits, policy model, audit log schema, performance trade-offs, and what you would change with more time."*

| Req No. | Requirement | Understanding | Details | Integration |
|---|---|---|---|---|
| **TDD-1** | **Architecture decisions** | The requirement asks for explicit, justified choices about how the system is built — not just what it does. The key architectural question for Enforcer is: can a single control point govern all agent action surfaces? The answer is no. A runtime hook sees intent but cannot enforce if the agent bypasses the SDK. A container constrains execution but cannot govern MCP traffic outside its boundary. A proxy controls network but cannot see local file operations. This means the architecture must be hybrid by necessity, not by preference. The decision has direct implications for complexity, deployment friction, and enforcement guarantees. The TDD must show the evaluator that this trade-off is understood and that the hybrid approach is the minimum viable architecture for credible mandatory enforcement. | Hybrid enforcement architecture: no single control point is sufficient. The product combines agent runtime hooks (intent capture), local daemon (policy evaluation and coordination), shell proxy (command mediation), filesystem guard (path enforcement), network proxy (egress control), and MCP gateway (protocol-aware tool governance). Three deployment topologies supported: host-based pilot (lowest friction), secure-container (high assurance for local dev), and remote-workspace (strongest tamper resistance for enterprise). Five logical layers: experience, agent runtime, enforcement, control plane, and observability/intelligence. | Section 11 (Architecture Overview), specifically 11.1 (Why Hybrid), 11.2 (Enforcement Points), 11.3 (Recommended Architecture). |
| **TDD-2** | **Where the interception layer sits** | This is the most tactically important TDD question because it determines what the product can actually enforce versus what it can only observe. Each interception position has a different combination of semantic visibility (does it understand what the agent is trying to do?), enforcement strength (can it actually block the action?), and bypass resistance (can the agent or developer circumvent it?). No single position scores high on all three, which is why the architecture must layer multiple positions. The evaluator wants to see that the team understands these trade-offs and has made deliberate choices about which positions to implement first and which to defer. | Nine enforcement points analyzed with strengths, weaknesses, and best use: agent runtime hook (high context, low enforcement), local daemon (coordination, tamper resistance), shell proxy (strong command mediation, misses non-shell actions), filesystem guard (path enforcement, limited to file ops), network proxy (egress control, limited semantic visibility), MCP gateway (protocol-aware, only covers MCP traffic), secure container (strong isolation, not sufficient alone), IDE extension (good UX, weak enforcement), VDI/remote workspace (strong centralization, high friction). The interception layer is distributed — not a single insertion point — with each position compensating for the weaknesses of the others. | Section 11.2 (Enforcement Points table), Section 11.4 (Deployment Topologies), Section 11.5 (Secure Container Analysis). |
| **TDD-3** | **Policy model** | The policy model is what makes the product configurable rather than hardcoded, and what makes it enterprise-ready rather than demo-only. "Configurable" is not enough — the model must support hierarchical inheritance because enterprises do not have flat permission structures. An organization-wide baseline must be non-negotiable; team-level and developer-local rules must be able to add restrictions but never weaken the baseline. The policy object must be expressive enough to encode real rules (path patterns, host allowlists, command classifications, data sensitivity labels, approval routing) while remaining understandable to administrators who are not policy-language experts. Shipping opinionated defaults is critical for time-to-value: a customer should see enforcement working within minutes of deployment, not after weeks of policy authoring. | Hierarchical inheritance (org → team → repo → local), where lower levels can tighten but never weaken baselines. Policy object schema: Subject (agent/session/user/team/environment), Action (read/write/exec/install/connect/query/invoke/prompt-submit), Resource (file path/host/secret/database/MCP server/tool/model endpoint), Conditions (data classification/environment/destination/package source/path scope/approval context), Effect (allow/deny/require approval/redact/quarantine/simulate/alert), Logging mode, Approval mode. Six decision types. Opinionated defaults shipped out of the box covering the most common enterprise security concerns. | Section 12 (Policy and Governance Model), FR-3 (Policy Engine). |
| **TDD-4** | **Audit log schema** | The audit log schema defines the evidentiary foundation of the entire product. If the schema is too sparse, security reviewers cannot reconstruct what happened. If it is too verbose, logs become noisy and expensive to store. If it lacks correlation IDs, investigators cannot follow action chains across sessions. If it captures raw sensitive payloads, the audit system itself becomes a liability. The schema must balance four tensions: completeness (capture everything needed for forensics), privacy (redact everything that should not be stored), performance (emit events without blocking the action path), and usability (structure events so a reviewer can navigate them without deep system knowledge). The "meaningfully read" requirement from MVP-3 flows directly into this schema — every field must earn its place by contributing to reviewer comprehension. | Normalized event schema: timestamp (ISO 8601 UTC), actor identity (user ID + agent type + agent instance + session ID), environment context (workspace, repo, branch, environment tier, deployment mode), action type, resource target, payload summary (redacted as appropriate per policy), policy evaluated and version, decision returned, approval state (not required/pending/approved/denied/expired), approver identity + rationale + scope + expiry (when applicable), observed execution result, content hashes for integrity verification, correlation IDs for session and delegation tracing. Events are append-only (no UPDATE or DELETE), immutable, and privacy-aware with configurable redaction applied before storage. | FR-2 (Action Normalization — full schema), FR-5 (Audit Trail and Session Replay). |
| **TDD-5** | **Performance trade-offs** | This is where the TDD must be honest about engineering constraints. Mandatory enforcement means the product sits in the critical path of every governed agent action. If policy evaluation adds 500ms to every file write, developers will revolt. If it adds 5ms, nobody notices. The performance budget is not uniform across all operations: approval-free safe actions (the vast majority) must be near-instantaneous; approval-required actions use a non-blocking model where the hook exits immediately with a deny code and the developer retries after approval, avoiding agent runtime stalls. The deepest trade-off is between inspection depth and latency: full content analysis of every file write or network payload is expensive; metadata + pattern matching is fast. The MVP should start with the fast approach and add deeper inspection as an opt-in capability for high-sensitivity environments. Fail-mode configuration is a trust decision: fail-closed is safer but risks blocking all work if the daemon goes down; fail-open is more available but creates enforcement gaps. Enterprises in regulated industries will demand fail-closed; others may prefer fail-open with alerting. | Policy checks for approval-free safe actions target <50ms for local evaluation. Latency budget is spent on policy lookup and decision, not on logging (async pipeline). Approval-required actions use a non-blocking model: the hook handler exits immediately, the approval request is created, and the developer retries after approval — no agent runtime blocking. Fail mode is configurable per environment: fail-closed (deny all actions if daemon is unavailable) for high-risk enterprise environments, fail-open (allow with alert) only where explicitly configured by administrators. Key trade-off: deeper payload inspection (content scanning, regex matching on file bodies) increases latency significantly; MVP starts with metadata + pattern matching (paths, hosts, command prefixes) and defers full content analysis to P2 as an opt-in capability for high-sensitivity environments. | NFR-1 (Mandatory Enforcement), NFR-2 (Performance), NFR-3 (Reliability). |
| **TDD-6** | **What to change with more time** | This question tests whether the team understands the gap between "good enough for prototype" and "enterprise-grade platform." The MVP makes deliberate simplifications: flat event logs instead of graph-native replay, pattern-based policy instead of content-aware classification, ambient credential access instead of scoped issuance, single-agent governance instead of multi-agent isolation. Each simplification is acceptable for a prototype but would be a liability in production. The answer should show a clear line from current simplifications to future improvements, demonstrating that the architecture was designed to evolve — not that it will require a rewrite. The six evolution paths listed are ordered by dependency: protocol mediation enables context-sensitive redaction; graph replay enables anomaly detection; both enable the unified control plane. | With more time, the design should evolve toward six specific improvements: (a) **protocol-aware mediation** — semantic governance of MCP, model API, DB, and secret channels through a common abstraction, not just OS-level event capture; (b) **graph-native session replay** — causal action graphs replacing flat event logs, with timeline/impact/exfiltration views and exportable incident bundles; (c) **anomaly detection** — sequence and graph models with baselines segmented by environment tier and agent role, not one-size-fits-all alerts; (d) **scoped credential issuance** — action-scoped, time-bounded, destination-bound credentials replacing ambient secret access; (e) **context-sensitive redaction** — route-aware policies where internal models receive more context than external providers, with classification by source type and sensitivity; (f) **unified multi-agent governance** — one control plane unifying policy, mediation, replay, and intelligence with per-agent trust envelopes and delegation lineage. These are captured in Phase 2 and Phase 3 of the roadmap and ordered by dependency. | Section 16 (Phased Roadmap — Phases 2 and 3), Section 23.2 (Open Questions). |

### 0.4 Peer Review Reference

A detailed comparison of this PRD (Claude-authored) against the Codex-authored Peer PRD (`Enforcer_PRD_Peer.md`) is provided in **Appendix B** at the end of this document. It includes requirement-by-requirement scoring, unique additions from each PRD, a summary table, Codex's review comments on the comparison, and a recommendation disposition tracker.

---

## 1. Executive Summary

Enforcer is a mandatory security and policy control plane for AI coding agents operating in developer environments. It sits between the agent and the systems the agent can affect — file systems, shells, package managers, network connections, credentials, databases, MCP-connected tools, and model-context pathways — intercepting actions, evaluating them against organizational governance policy and developer-local guardrails, enforcing allow, deny, and approval decisions, and producing security-grade audit trails that make agent activity reviewable, reproducible, and governable.

The product thesis is that AI coding agents such as Claude Code, Cursor, GitHub Copilot agents, OpenAI Codex, and MCP-driven workflows are gaining unprecedented access to source code, terminals, package ecosystems, API keys, cloud accounts, internal databases, and production-adjacent infrastructure, while most organizations still rely on security controls designed for human developers rather than autonomous systems acting at machine speed. The resulting gap is not simply observability; it is a lack of deterministic governance over what the agent can do, where it can send data, which resources it can access, and how incidents can be reconstructed after the fact.

Enforcer's wedge is therefore the security and governance blocker that prevents broad rollout of coding agents inside mid-market and enterprise engineering organizations. The first job of the product is to make agent adoption safe enough to approve — not merely visible enough to monitor.

---

## 2. Product Vision

Enforcer's vision is to become the default trust boundary for agentic software development: the layer every enterprise inserts between AI coding agents and the environments, tools, protocols, and data those agents touch. In the same way identity providers became mandatory for SaaS access and endpoint agents became mandatory for device governance, Enforcer aims to become mandatory infrastructure for safe software agents.

The long-term product promise is that every sensitive action an AI coding agent attempts should be:

- **Attributable** — tied to a specific agent, session, user, project, and environment.
- **Policy-evaluable** — checked against organizational baselines, team rules, and developer-local guardrails before execution.
- **Replayable** — reconstructable as a causal chain of intent, action, decision, and observed effect for forensic review.
- **Blockable in real time** — stoppable before damage occurs when policy demands it.

Over time, Enforcer should evolve from a point solution for coding-agent safety into the unified control plane for agentic execution across local developer machines, secure containers, remote workspaces, CI/CD runners, MCP ecosystems, network egress, secrets stores, databases, model-context pathways, and agent-to-agent delegation chains.

---

## 3. Problem Statement

### 3.1 The Agent Access Problem

AI coding agents can already read and write files, execute arbitrary shell commands, install and modify packages, call APIs, use secrets and credentials, query databases, communicate with external tools through MCP, and coordinate with other agents — all at speeds and scales that make real-time human review impractical. This represents a fundamentally different risk model from ordinary human developer activity, where actions are slower, more deliberate, and bounded by human judgment.

### 3.2 Why Existing Controls Fail

Existing security products were built for humans. Endpoint agents can log process activity and monitor filesystems, but they do not understand agent intent, tool invocation context, MCP traffic, or the causal chain between a prompt, a tool call, a file mutation, and a network request. Secrets managers govern access to credential stores but do not govern the full action chain an agent executes once it obtains a credential. CNAPP platforms protect cloud posture but are not embedded in the local coding-agent execution path. IDE vendors expose per-tool permission settings that are vendor-specific, not cross-agent, and are designed to optimize adoption rather than provide neutral enforcement.

The result: organizations lack both the preventive controls and the evidentiary logs required to approve enterprise-scale use of AI coding agents.

### 3.3 The Full Threat Surface

The risk surface extends far beyond file access:

| Threat Surface | Example Risk |
|---|---|
| **File system reads and writes** | Agent writes outside the project directory, reads sensitive configuration or credentials from host paths, modifies files the developer did not intend to change |
| **Shell command execution** | Agent runs destructive commands (rm -rf, git push --force), installs persistence mechanisms, exfiltrates data via curl/wget |
| **Package installation** | Agent installs malicious or vulnerable dependencies, modifies lockfiles, executes post-install scripts |
| **Network egress** | Agent sends source code, secrets, or customer data to non-allowlisted hosts, exfiltrates IP through HTTP requests |
| **Credential and secret access** | Agent reads API keys, cloud credentials, database passwords, or session tokens from environment variables, config files, or credential stores |
| **MCP client-server communication** | Agent invokes unsafe tools through MCP servers, passes sensitive payloads to unvetted MCP endpoints, or accesses data through MCP tools that bypass other controls |
| **Database reads and writes** | Agent extracts sensitive records, mutates production-adjacent data, or runs destructive queries through database tools |
| **LLM context submission** | Agent includes proprietary source code, customer PII, credentials, or trade secrets in prompts sent to external model providers |
| **Agent-to-agent delegation** | Agent delegates tasks to sub-agents with broader permissions, creating hidden action chains and unbounded delegation depth |

### 3.4 The Adoption Blocker

The practical consequence is deployment friction. Security and platform teams slow or block enterprise rollout of coding agents because they cannot answer fundamental governance questions:

- What exactly can the agent touch?
- Which actions require human approval?
- Which network hosts can it contact?
- Can it access production credentials or customer data?
- Can it leak proprietary code into model context?
- Who approved an exception, and why?
- What evidence exists for post-incident investigation?

These unanswered questions are the single largest blocker to enterprise-wide coding-agent adoption. Enforcer exists to answer them.

---

## 4. Why Now

### 4.1 Agents Have Become Actors

AI coding agents have shifted from passive autocomplete assistance toward autonomous or semi-autonomous task execution across terminals, codebases, package ecosystems, network APIs, databases, and external tools. This creates a step-function increase in operational risk compared with earlier copilots that mainly generated text in-editor. Agents now take actions on real systems, and the security model must change when humans are no longer the only direct actors in the development environment.

### 4.2 Enterprise Demand Is Real but Blocked

Enterprise interest in agentic development is rising because buyers expect meaningful productivity gains in code generation, debugging, refactoring, testing, documentation, and integration work. Adoption is no longer blocked by lack of interest; it is blocked by lack of trusted security controls and acceptable governance evidence. Engineering leaders see material productivity potential but cannot get past security review without answers to the governance questions above.

### 4.3 Control Points Are Consolidating

The timing is favorable because the tool surface is consolidating into identifiable enforcement seams. MCP-based tool invocation provides a protocol-layer insertion point. Agent orchestration layers create delegation-tracking opportunities. Containerized workspaces and remote development environments create isolation boundaries. Enterprise AI platform governance is emerging as a recognized category. These architectural trends create concrete integration points where Enforcer can insert policy without requiring a complete rebuild of the developer stack.

### 4.4 Market Is Fragmented

The market for agent runtime governance is fragmented rather than settled into a single category. No incumbent vendor owns the cross-agent enforcement layer for coding-agent workflows. LLM guardrail vendors focus on prompt/response filtering, not machine-level actions. Endpoint security vendors were built for human users. IDE vendors provide vendor-specific controls that are not neutral or cross-agent. This fragmentation creates a clear category-creation opportunity for a purpose-built product.

---

## 5. Users and Buyers

### 5.1 Ideal Customer Profile

The primary ideal customer profile is a mid-market or enterprise software organization (300+ engineers) that is actively piloting or scaling AI coding agents across engineering teams, has a meaningful security or compliance posture, and needs a way to move from limited experimentation to governed production use. Best-fit accounts have regulated workflows, sensitive codebases, platform teams, or formal security review processes that can block agent rollout if controls are weak.

Secondary segment: fast-growing cloud-native companies with high developer velocity and emerging internal AI agent usage, especially where MCP servers, package installs, and shell automation are entering production-adjacent workflows.

### 5.2 Buyer Map

| Role | Motivation | Core Concern | Buying Role |
|---|---|---|---|
| **Security engineering lead** | Reduce risk and enforce guardrails for autonomous agent activity | Mandatory control, bypass risk, auditability, incident response | Primary buyer |
| **Platform engineering lead** | Standardize safe rollout of AI tooling without creating support burden | Deployment friction, policy manageability, latency, UX | Technical champion |
| **CISO / VP Security** | Governance, accountability, and board comfort | Compliance, enterprise exposure, liability, mandatory control | Economic sponsor |
| **Engineering leader / VP Eng** | Preserve developer productivity while reducing risk | False positives, workflow disruption, developer adoption | Key stakeholder |
| **AI tooling owner / DX lead** | Operationalize agent usage across the organization | Integration sprawl, UX consistency, policy fragmentation | Internal operator / champion |

### 5.3 User Personas

**Security Engineer (Reviewer)**
Reviews agent behavior, inspects policy decisions, investigates blocked actions, manages exception workflows, exports evidence for compliance and incident response. Needs structured, searchable audit trails and session-level replay — not raw event exhaust. Primary daily user of the security console.

**Platform Engineer (Administrator)**
Defines and distributes organizational policies, configures agent environment controls, manages approval routing, monitors deployment health, and onboards new teams. Needs policy authoring tools, environment-specific baselines, and deployment validation. Primary author of governance rules.

**Developer (End User)**
Uses Claude Code, Cursor, Codex, or other coding agents in VS Code, terminal, or compatible IDE workflows. Needs low-friction approval handling that feels native to the coding workflow, clear explanations when actions are blocked or escalated, and minimal disruption to routine safe work. Does not want to leave the IDE to interact with security controls.

**Engineering Manager / Approver**
Receives approval requests for high-risk actions, reviews context, approves or denies with rationale. Needs enough context to decide quickly and confidently — not raw command output, but structured summaries of what the agent is attempting, why it was flagged, and what the policy says.

**Executive Buyer (CISO / VP)**
Needs dashboard-level evidence that governance is active, policies are enforced, exceptions are tracked, and the organization can pass audit. Does not interact daily with the product but needs quarterly evidence that the investment is working.

### 5.4 Jobs to Be Done

**Security Engineering Lead:**
"When AI coding agents act on developer machines and repositories, there must be a provable policy layer that shows what they did, blocks disallowed actions, and generates evidence suitable for review and compliance."

**Platform Engineering Lead:**
"Enable coding agents for developers without turning the platform team into a bottleneck, and without introducing uncontrolled shell, package, network, or secret exposure."

**Developer:**
"Let the agent move quickly on routine tasks, but keep guardrails predictable, explain policy decisions clearly, and minimize pointless approvals."

**Approver:**
"Surface only the actions that genuinely need attention, with enough context to approve or deny quickly and confidently."

**Executive Buyer:**
"Roll out coding agents broadly with confidence that autonomy is bounded, risk is auditable, and governance is not left to ad hoc trust."

---

## 6. Design Principles

1. **Mandatory where promised.** Never market observability as enforcement. If the product claims to block an action, it must block it — not log it and hope someone reads the log.

2. **Action chains, not isolated events.** Govern the full causal chain from agent intent through tool invocation through system effect. An isolated log of "file written" is not governance; the chain of "agent received task → planned action → evaluated policy → executed or blocked → observed result" is governance.

3. **Protocol-aware by default.** Especially for MCP and model-context flows. Deep protocol mediation is how Enforcer differentiates from generic endpoint tools that see only OS-level events without semantic understanding of what the agent is doing.

4. **Enterprise baselines plus developer-local guardrails.** Organization-level policies define non-negotiable constraints. Teams and developers can add stricter rules but never weaken organizational baselines. This mirrors how enterprises actually operate: central mandate plus local customization.

5. **Explainable decisions.** Every policy outcome must be understandable to both developers (who need to know why they were blocked) and security reviewers (who need to know why something was allowed). Silent failures erode trust; opaque decisions erode adoption.

6. **Containers as substrate, not as the sole answer.** Secure containers provide strong isolation for filesystem, process, and network enforcement. But containers alone do not govern MCP traffic outside the runtime, secrets accessed through external identity systems, data leaked into model context, agent-to-agent delegation, or database activity through remote tools. Enforcer should use containers as a foundational enforcement layer within a broader defense-in-depth architecture.

7. **Low-friction developer experience.** Approvals and blocks should be exception-based and context-aware so that safe work stays fast. If the product feels like a heavy security overlay that blocks normal development, it will be bypassed or abandoned.

8. **Agent-native architecture.** The product must model the development environment as inherently multi-agent. A single "coding agent" often delegates to tool agents, MCP servers, retrieval agents, CI agents, database tools, and remote model providers. Enforcer must govern not only direct actions but delegated actions and inter-agent communication.

---

## 7. Core Use Cases

### UC-1: Block Writes Outside Project Boundaries
Prevent the agent from writing files outside the approved project directory or workspace root, especially on local machines or shared workspaces where the host filesystem is fully accessible. This is the simplest and most universally demanded control.

### UC-2: Govern Shell Command Execution
Deny or require approval for shell commands with destructive, persistence, or exfiltration potential. Examples: rm -rf outside project scope, curl/wget to non-allowlisted hosts, git push --force to protected branches, chmod on sensitive files, background process spawning.

### UC-3: Restrict Network Egress
Restrict network calls to allowlisted domains, approved SaaS endpoints, and sanctioned package registries. Block exfiltration of source code, credentials, or customer data to unknown hosts. Log all network destinations for audit.

### UC-4: Gate Package Installation
Require approval for package installation, dependency upgrades, lockfile modifications, and build-script execution. Prevent installation from untrusted registries. Log all package operations including post-install scripts.

### UC-5: Protect Secrets and Credentials
Prevent access to high-risk secrets, production credentials, environment variables, API keys, and customer data unless explicitly allowed by policy. Redact secret values from audit logs, error messages, and model context. Detect when agents attempt to access credential stores, cloud CLIs, or browser sessions.

### UC-6: Govern MCP Communication
Control which MCP tools, servers, methods, and payload types the agent may invoke. Inspect MCP request and response payloads for policy compliance. Block unsafe tool invocations and log all MCP traffic for audit. This is a key differentiator because MCP is how many modern agent workflows access external capabilities.

### UC-7: Prevent Data Leakage into Model Context
Detect and stop sensitive data — proprietary source code, customer PII, credentials, trade secrets — from flowing from the codebase, database, or credential stores into LLM prompts or external tool payloads. Apply route-aware redaction: internal models may receive more context than external model providers.

### UC-8: Produce Replayable Audit Trails
Generate a structured, graph-linked audit trail of what the agent attempted, what policy applied, which actor approved exceptions, and what final system effects occurred. The audit trail must be meaningful to security reviewers — not raw event exhaust — and must support incident investigation, compliance evidence, and forensic replay of agent sessions.

### UC-9: Human-in-the-Loop Approval
For high-risk actions that should not be auto-denied but require human judgment, implement a non-blocking approval workflow. The hook handler exits immediately when approval is required, creating an approval request. The developer retries the action after approval is granted via the Hub Console or Sentinel Console. Approvals support three scope types (single-use, time-bounded, session-scoped), pattern matching, and break-glass emergency overrides.

### UC-10: Multi-Agent Governance
As agent workflows grow more complex, govern the full delegation chain. Prevent one agent session from reading another session's context, artifacts, or credentials. Track parent/child agent relationships and ensure delegated sub-agents operate within their own trust envelopes.

---

## 8. Product Scope

### 8.1 In Scope for MVP

- **Interception** of at least two high-risk action classes: file system reads/writes, shell command execution, network calls, package installs, or credential/secret access.
- **Policy engine** with configurable allow, deny, and require-approval decisions. Organization-level policies plus developer-local guardrails that can tighten but not weaken baselines.
- **Structured audit logging** with agent identity, action type, resource targeted, payload summary, policy evaluated, decision returned, approver (if applicable), and timestamp.
- **One differentiated depth area** where the product goes materially deeper than a shallow monitor-only solution. Recommended options: human-in-the-loop approval UX, MCP-aware governance, or secrets/PII redaction in agent context.
- **Security review console** for security and platform teams to review sessions, search audit logs, inspect blocked actions, manage policies, and export evidence.
- **Opinionated default policies** that demonstrate enterprise value immediately: block writes outside project root, require approval for package installs, deny network calls to non-allowlisted hosts, prevent production secrets from entering model context.

### 8.2 In Scope for MVP (Agent Integrations)

- **VS Code extension or companion integration** that displays agent action approvals, shows policy decisions and blocked actions, links to audit records, surfaces high-risk actions requiring confirmation, and maps events to workspace/repository/branch context.
- **Claude Code support** in both extension and CLI-connected modes: capture proposed file edits, shell command requests, apply path/command/network/package policies, preserve user approval context in audit trail.
- **Cursor support** through MCP and permission model integration: inspect and govern MCP tool usage, map agent activity to server/tool identity, enforce allow/deny/approval rules.
- **Codex IDE support** for VS Code and VS Code forks: capture local edit and run actions, tag locally-initiated actions separately from delegated cloud tasks, warn when actions cannot be fully observed because execution occurred outside the governed runtime.

### 8.3 Out of Scope for MVP

- Full CNAPP-style cloud posture management.
- Broad employee endpoint monitoring unrelated to AI coding agents.
- Universal support for every agent framework, IDE, model provider, and operating system on day one.
- Generic LLM content moderation or prompt safety unrelated to action governance.
- Training or fine-tuning foundation models.
- Complex ML-based anomaly detection if deterministic policy controls are not yet excellent.

### 8.4 MVP Integration Targets

This subsection specifies which coding agents, platforms, and modes the MVP prototype targets and why. Technology stack, hook mechanisms, container runtime choices, and implementation details belong in the TDD — this section covers the product-level rationale for target selection.

#### 8.4.1 Primary Target: Claude Code (VS Code Extension + CLI)

**Why Claude Code is the strongest MVP target:**

- **Local execution model.** Claude Code runs entirely on the developer's machine. Every file write, shell command, and network call originates locally, which means Enforcer's enforcement layer can intercept actions before they reach the operating system or network. There is no cloud-delegated execution to create visibility gaps.
- **Hooks system.** Claude Code exposes a configurable hooks API (via `settings.json`) that fires before and after tool calls — file edits, shell commands, MCP invocations, and notifications. This provides a natural, supported insertion point for pre-execution policy evaluation without requiring reverse engineering or unsupported patching.
- **Approval surface.** Claude Code already has a permission model with approval prompts for file edits, shell commands, and MCP tool invocations inside VS Code. Enforcer can augment this existing approval surface with policy-driven decisions and structured audit logging, making the integration feel native rather than bolted on.
- **CLI mode.** Claude Code operates both as a VS Code extension and as a standalone CLI (`claude`). Supporting both modes means Enforcer can govern agent activity inside the IDE and in terminal-only workflows, covering the two most common enterprise developer environments.
- **Enterprise adoption.** Claude Code has significant enterprise traction. Targeting it first means the prototype demonstrates value against the agent environment most likely to be under security review at prospective customers.

**What Enforcer intercepts on Claude Code:**

| Action Surface | Claude Code Behavior | Enforcer Interception |
|---|---|---|
| File writes | Agent proposes edits via Edit/Write tools; user sees diff in VS Code | Filesystem guard evaluates path policy before write lands; blocks writes outside project root; logs path + content hash + decision |
| File reads | Agent reads files via Read tool | Filesystem guard evaluates read policy; blocks reads of sensitive host paths (e.g., ~/.ssh, ~/.aws); logs access |
| Shell commands | Agent executes commands via Bash tool | Shell proxy captures command before execution; evaluates against command-pattern policy; blocks destructive/exfiltrative patterns; surfaces approval for high-risk commands |
| Network egress | Agent's shell commands or tool calls make HTTP requests | Network proxy enforces host allowlist; blocks non-sanctioned destinations; logs destination + method + response status |
| MCP tool calls | Agent invokes MCP servers for external tool access | MCP gateway evaluates server + method + payload against policy; blocks unauthorized tool invocations |

#### 8.4.2 Secondary Target: Cursor (via MCP Gateway)

**Why Cursor as the secondary target:**

- **MCP-native architecture.** Cursor relies heavily on MCP servers to expose tools, data, and actions to its agent. This makes MCP the natural insertion point — Enforcer's MCP gateway sits between Cursor's agent and the MCP servers it calls, providing protocol-aware governance at the tool invocation layer.
- **Demonstrates cross-agent value.** Showing Enforcer governing both Claude Code (via hooks + proxies) and Cursor (via MCP gateway) in a single demo proves the product is agent-neutral — the same policy engine and audit trail govern different agents through different integration mechanisms.
- **VS Code fork.** Cursor is built on VS Code, so the VS Code extension and IDE-level integration work developed for Claude Code can be partially reused.
- **Enterprise presence.** Cursor claims 50,000+ enterprises on its platform. It is one of the most widely deployed coding agents alongside Claude Code.

**What Enforcer intercepts on Cursor:**

| Action Surface | Cursor Behavior | Enforcer Interception |
|---|---|---|
| MCP tool invocations | Agent calls MCP servers for file, shell, search, database, and API tools | MCP gateway evaluates server identity + tool + method + payload against policy; blocks unauthorized invocations; logs full request/response metadata |
| File writes | Agent writes files through editor or MCP file tools | Filesystem guard (same as Claude Code) evaluates path policy |
| Shell commands | Agent executes commands through terminal or MCP shell tools | Shell proxy (same as Claude Code) captures and evaluates commands |
| Network egress | Agent or MCP servers make outbound HTTP requests | Network proxy (same as Claude Code) enforces host allowlist |

#### 8.4.3 Tertiary Target: OpenAI Codex (VS Code Extension — Visibility Mode)

**Why Codex is tertiary, not primary:**

- **Cloud-delegated execution.** Codex can delegate tasks to cloud-based execution environments outside the developer's local machine. Actions that execute remotely cannot be intercepted by a local enforcement layer. Enforcer must be honest about this visibility gap rather than claiming false completeness.
- **VS Code extension mode is local.** When Codex operates through its VS Code extension for local edit and run actions, those actions are interceptable using the same filesystem guard, shell proxy, and network proxy used for Claude Code.
- **MVP approach:** Support Codex in "visibility mode" — intercept and govern locally-initiated actions; tag remotely-delegated actions as partially observable; surface explicit warnings in the audit trail when actions cannot be fully governed because execution occurred outside the local runtime.

#### 8.4.4 Platform and Mode Matrix

| Agent | Mode | Integration Mechanism | Enforcement Strength | MVP Priority |
|---|---|---|---|---|
| **Claude Code** | VS Code extension | Hooks API + filesystem guard + shell proxy + network proxy | Full (all actions local, pre-execution interception) | **P0** |
| **Claude Code** | CLI (`claude`) | Process wrapper + filesystem guard + shell proxy + network proxy | Full (all actions local) | **P0** |
| **Cursor** | VS Code (agent mode) | MCP gateway + filesystem guard + shell proxy + network proxy | Strong for MCP-routed actions; standard for direct OS actions | **P1** |
| **Codex** | VS Code extension (local) | Filesystem guard + shell proxy + network proxy | Full for local actions | **P1** |
| **Codex** | Cloud-delegated tasks | Audit-only (no local interception possible) | Visibility only — log delegation event, flag as partially observable | **P2** |
| **Claude Code** | Desktop app | Same as VS Code extension if hooks API is available; TBD pending integration surface analysis | TBD | **P2** |
| **Copilot agents** | VS Code / GitHub | TBD pending integration surface analysis | TBD | **P2** |

#### 8.4.5 MVP Demo Scenario

The prototype demonstration should show Enforcer governing a real coding agent performing a real task in real time. The recommended demo flow:

**Setup:** Developer has a project open in VS Code with Claude Code active. Enforcer is running with default enterprise policies (deny writes outside project root, deny non-allowlisted network hosts, require approval for high-risk shell commands).

**Demo sequence:**

1. **Safe work proceeds without friction.** Developer asks Claude Code to refactor a function. Agent reads files inside the project, writes modified files inside the project. Enforcer evaluates each action, allows it, and logs it silently. The developer sees no interruption — safe work is fast.

2. **File write outside project root — blocked.** Agent attempts to write a configuration file to `~/.config/`. Enforcer's filesystem guard evaluates the path, matches the "deny writes outside project root" policy, and blocks the write. The developer sees a clear explanation in VS Code: "Blocked: write to ~/.config/ denied by policy 'project-boundary'. The agent may only write to files within /Users/dev/project/."

3. **Destructive shell command — approval required (non-blocking).** Agent attempts to run `rm -rf node_modules && npm install`. Enforcer's hook handler matches the `rm -rf` pattern via the policy engine and creates an approval request. The hook exits immediately with a deny code — the agent receives a message explaining approval is required. A reviewer approves via the Hub Console or Sentinel Console. The developer retries the command — this time the hook finds the active approval and exits 0, allowing execution. Approval decision is logged with approver identity and rationale.

4. **Network call to unknown host — blocked.** Agent attempts to `curl https://unknown-pastebin.io/upload` to share a code snippet. Enforcer's network proxy checks the host against the allowlist, finds no match, and blocks the request. Agent receives an error with explanation. Event is logged.

5. **Security reviewer opens the Hub Console.** A security engineer opens the Hub Console (port 9201), navigates to Sessions, searches for the session, and sees the full timeline: every action the agent attempted, every policy decision, the approval workflow with approver identity, the blocked actions with rationale, and the final outcomes. The reviewer can drill down from Analytics to Search to Session detail. The reviewer can also navigate to the Enterprise Analytics page for org-wide enforcement metrics, blocked operation rankings, and developer group classifications. The session is exportable as an evidence package via the Export page.

6. **(Optional — secondary demo) Cursor MCP governance.** Switch to Cursor. Agent invokes an MCP tool to query a database. Enforcer's MCP gateway intercepts the tool call, evaluates the server + method + payload, and blocks a query that would return customer PII. The same audit console shows both the Claude Code session and the Cursor session under unified governance.

#### 8.4.6 What Belongs in the TDD (Not Here)

The following implementation-level decisions are documented in the Technical Design Document (`Enforcer_TDD_Final_2.md`). Key decisions made:

- **Technology stack**: Go (statically compiled, `CGO_ENABLED=0`) for all 5 binaries (daemon, hook handler, central, client, authseed). Next.js 15 + React + shadcn/ui + Tailwind CSS for dual console builds.
- **Hook mechanism**: Claude Code hooks API integration via `enforcer-hook` binary reading JSON from stdin with project root walk-up detection, logging to `~/.enforcer/hook.log`.
- **Policy language**: Versioned YAML policy bundles with Ed25519 signing and monotonic version enforcement.
- **Audit store**: PostgreSQL (append-only, no UPDATE or DELETE) with decision normalization and unlimited query limit.
- **Console stack**: Dual builds (`out-hub` for port 9201, `out-sentinel` for port 9100) embedded via `go:embed`. Hub Console with admin/reviewer RBAC, Sentinel Console with operator role.
- **Container technology**: Docker (rootless, hardened profile) for secure-container deployment mode.
- **Network security**: mTLS on ports 9200 (client) and 9201 (admin) for Hub communication.

---

## 9. Functional Requirements

### FR-1: Action Interception

The system must intercept or mediate agent actions across the following surfaces, with MVP requiring strong coverage on at least two:

| Surface | What to Intercept | MVP Priority |
|---|---|---|
| File system | Reads and writes, path scope enforcement, content inspection | P0 |
| Shell / terminal | Command execution, argument parsing, output capture | P0 |
| Network / HTTP | Outbound connections, destination hosts, payload metadata | P0 |
| Package manager | Install, upgrade, lockfile modification, post-install scripts | P1 |
| Credentials / secrets | Environment variable reads, credential store access, token retrieval | P1 |
| MCP communication | Tool invocations, server connections, request/response payloads | P1 |
| Agent-to-agent | Delegation requests, sub-agent spawning, cross-session communication | P2 |
| Database | Direct or tool-mediated queries, data extraction, mutation | P2 |
| LLM context | Prompt assembly, context window content, model provider routing | P2 |

### FR-2: Action Normalization

All intercepted events must be normalized into a common event schema containing:

- Timestamp (ISO 8601, UTC)
- Actor identity (user, agent type, agent instance, session ID)
- Environment context (workspace, repository, branch, environment tier, deployment mode)
- Action type (read, write, exec, install, connect, query, invoke, prompt-submit)
- Resource target (file path, host, command, package, secret, MCP server/tool/method, database, model endpoint)
- Payload summary (redacted as appropriate)
- Policy evaluated and decision returned
- Approval state (not required, pending, approved, denied, expired)
- Approver identity and rationale (when applicable)
- Observed execution result
- Correlation identifiers for session and delegation tracing

### FR-3: Policy Engine

The policy engine must support:

- **Hierarchical policy inheritance**: organization baseline → team policy → workspace/repository policy → developer-local guardrails. Lower levels can tighten but never weaken higher-level constraints.
- **Rich evaluation context**: actor, agent type, session, project, file path, command pattern, destination host, data classification, package source, credential class, environment tier, deployment mode, and approval state.
- **Policy decisions**: allow, deny, require approval, redact, quarantine, isolate, simulate (log-only / dry-run), alert.
- **Human-readable rationale and machine-readable structured logs** for every decision.
- **Opinionated default policies** shipped out of the box that cover common enterprise concerns.

A policy object should include:

```
Subject:    agent | session | user | team | environment
Action:     read | write | exec | install | connect | query | invoke | prompt-submit
Resource:   file path | host | secret store | database | MCP server | tool | model endpoint
Conditions: data classification | environment | destination | package source | path scope | approval context
Effect:     allow | deny | require approval | redact | quarantine | simulate | alert
```

### FR-4: Approval Workflows

The system implements a non-blocking approval workflow where the hook handler exits immediately when approval is required, and the developer retries the action after approval is granted. This preserves developer flow and avoids blocking the agent runtime.

The system must support:

- Non-blocking approval flow: hook exits immediately with deny exit code, creates approval request, developer retries after approval.
- Three approval scope types: single-use (consumed after one action), time-bounded (expires after configurable window), session-scoped (persists for session).
- Pattern matching against action types and resource values for approval rules.
- Break-glass emergency overrides that bypass normal flow but record full audit context.
- Approval management via Hub Console (admin/reviewer RBAC) and Sentinel Console (operator role).
- Approval metrics tracking created, approved, denied, and expired counts.
- Structured approval records in the audit trail including approver identity, rationale, scope, and expiry.

### FR-5: Audit Trail and Session Replay

The system must produce an audit trail meaningful to a security reviewer — not raw system event exhaust. The audit trail must support:

- **Session-level replay**: reconstruct what happened during an agent session as a structured narrative, not just a list of events. Show the sequence of actions, the policy decisions applied, the approvals granted, and the final outcomes.
- **Graph-linked causality**: link parent and child actions, delegation chains, and cross-resource relationships so investigators can follow the full action chain from agent intent to system effect.
- **Search and filter**: by agent, user, session, action type, resource, policy decision, time range, environment, and risk level.
- **Export**: exportable evidence packages for incident response, compliance review, and legal proceedings.
- **Immutability**: audit events are append-only. No UPDATE or DELETE.
- **Privacy-aware retention**: configurable policies for what content is captured vs. metadata-only vs. redacted, since some organizations permit action metadata logging but restrict full prompt or payload retention.

### FR-6: MCP Governance

The system must support protocol-aware governance for MCP-based agent workflows:

- Server allowlists and denylists.
- Method-level allow/deny/approval policies.
- Request and response payload inspection and metadata capture.
- Payload transformation (masking, truncation, schema-based filtering) for sensitive content.
- Tool capability classification and risk tagging.
- All MCP traffic logged with full audit context.

### FR-7: Secrets-Aware Controls

The system must detect and govern access to:

- Local credentials and environment variables.
- Credential stores and secret managers.
- Cloud CLI profiles and authentication tokens.
- API keys and database passwords.

At minimum, the audit trail must record secret-access attempts and policy outcomes even when the actual secret value is redacted. Advanced controls should include scoped, time-bounded, destination-bound credential issuance where supported.

### FR-8: VS Code Integration

Enforcer must provide a VS Code extension or companion integration that can:

- Display agent action approvals natively within the editor.
- Show policy decisions and blocked actions with explanations.
- Link to audit records for the current session.
- Surface high-risk prompts or actions requiring confirmation.
- Map events to current workspace, repository, and branch context.
- Feel native to the editor — not like a browser popup or external console.

### FR-9: Multi-Agent Support

The system must maintain a compatibility matrix by tool, platform, and integration mode:

| Dimension | Values |
|---|---|
| Vendor/Tool | Claude Code, Cursor, Codex, Copilot agents, MCP-based workflows |
| Integration Mode | VS Code extension, CLI, MCP gateway, delegated cloud task |
| Observable Actions | Which action types can be captured per integration mode |
| Enforceable Actions | Which action types can be blocked or gated per integration mode |
| Approval Support | Whether in-IDE approvals are possible per integration mode |
| Audit Coverage Quality | Full, partial, metadata-only per action type |
| Known Limitations | Explicit documentation of visibility gaps per integration mode |

Where an action happens outside the local or governed runtime, the product must surface visibility limitations explicitly rather than imply false completeness.

### FR-10: Security Console (Dual Console Architecture)

Enforcer implements two separate web-based consoles serving different audiences:

**Hub Console** (port 9201, served by `enforcer-central`, admin/reviewer RBAC):
- Dashboard with org-wide enforcement metrics.
- Sessions list with cross-Sentinel session inspection.
- Approvals management for reviewing and acting on pending approvals.
- Search and filter audit logs across all connected Sentinels.
- Export evidence packages for investigation, compliance, or incident response.
- Policies management for authoring, viewing, and deploying policy bundles.
- Enterprise Analytics with org-wide blocked operations, approval bottlenecks, per-developer enforcement impact, and developer group classifications (Compliant Developer, High-Friction Developer, etc.).

**Sentinel Console** (port 9100, served by `enforcer-daemon`, operator role):
- Dashboard with local enforcement metrics for the governed developer.
- Sessions list filtered to the governed developer's sessions.
- Search and filter the developer's own audit logs.
- Export evidence for the developer's own sessions.
- My Activity page showing personal analytics and enforcement impact.

Both consoles use `governed_user` from the daemon health endpoint for data filtering. All drill-downs navigate from Analytics to Search to Session detail. Developer groups are named professionally (Compliant Developer, High-Friction Developer, etc.).

### FR-11: Developer Transparency

When Enforcer blocks or gates an action, it must provide:

- A clear reason for the block.
- The specific rule or policy that matched.
- The next available path: request approval, retry within allowed scope, or open a policy exception workflow.

Silent failures will erode trust and reduce adoption.

---

## 10. Non-Functional Requirements

### NFR-1: Mandatory Enforcement
Policy enforcement must be mandatory for covered surfaces. Monitor-only modes must not masquerade as protection. If the product claims to govern an action class, governance must be enforced — not optional.

### NFR-2: Performance
Policy checks for common file and shell actions must add minimal interactive latency. Approval-free safe actions should feel near-instantaneous in local development use. Target: common policy decisions complete in under 50ms for local evaluation.

### NFR-3: Reliability
If the policy engine or daemon is unavailable, behavior must follow configurable fail mode: fail-closed (deny all) for high-risk enterprise environments, fail-open only where explicitly configured by administrators. Degraded-mode behavior must be documented and testable.

### NFR-4: Tamper Resistance
Controls must be difficult for developers or agents to disable, bypass, or circumvent without authorization. The system must detect and log bypass attempts and unsupported execution paths. Tamper resistance is stronger in containerized or remote-workspace deployments.

### NFR-5: Privacy-Aware Logging
Logs and review interfaces must avoid unnecessary capture of sensitive code, secrets, or customer data. Prompt capture and content capture should be configurable by policy. Redaction must be applied before storage, not as a post-hoc filter.

### NFR-6: Security
Audit records must be tamper-evident or tamper-resistant. Sensitive values must be redacted or hashed. The product itself must not introduce new attack surfaces: no secrets in logs, no sensitive data in error messages, no unauthenticated API endpoints.

### NFR-7: Extensibility
The interception architecture must support additional agent environments, enforcement surfaces, and policy dimensions without re-architecting the core policy model.

### NFR-8: Enterprise Readiness
SSO, SCIM, RBAC, deployment options (on-premise, hosted, hybrid), API access, exportability, and multi-tenancy support will be expected by serious enterprise buyers.

### NFR-9: Cross-Platform Support
Initial support for macOS and Linux development environments. Windows support as a fast follow. The architecture must not depend on one operating system, one IDE, or one model vendor.

---

## 11. Architecture Overview

### 11.1 Why Hybrid

No single enforcement point fully governs all relevant action surfaces. A runtime hook alone sees intent but cannot guarantee enforcement if the agent bypasses the SDK or uses unmanaged tools. A container alone constrains execution but may not understand agent-level context, approval semantics, or MCP-specific payloads. A proxy alone controls network traffic but cannot govern local filesystem or in-memory operations.

Enforcer must therefore adopt a hybrid enforcement architecture that combines multiple complementary control points.

### 11.2 Enforcement Points

| Enforcement Point | Strengths | Weaknesses | Best Use |
|---|---|---|---|
| **Agent runtime hook / SDK wrapper** | High semantic context; sees agent intent before execution | Limited if agents bypass SDK or use unmanaged tools | Fast initial integrations, intent capture |
| **Local daemon / sidecar** | Persistent policy point on endpoint or workspace; coordinates all local enforcement | Requires secure deployment and tamper resistance | Local control plane, policy evaluation |
| **Shell proxy / exec wrapper** | Strong mediation for terminal commands | Misses non-shell actions and direct syscalls | Command governance |
| **Filesystem guard** | Project-path enforcement, content inspection | Limited to filesystem operations | Write/read policy |
| **Network proxy** | Centralizes egress policy and logging | Limited semantic visibility into local operations | Host allowlisting, exfiltration control |
| **MCP gateway / wrapper** | Protocol-aware governance for MCP tools and payloads | Only covers MCP-routed actions | Tool governance, payload inspection |
| **Secure container / sandbox** | Strong isolation for files, processes, network, tool execution | Not sufficient alone for all external systems and identity paths | Controlled workspace execution |
| **IDE extension** | Good UX, user feedback, approval surfaces | Weak for mandatory enforcement by itself | Developer messaging, approvals |
| **VDI / remote workspace** | Strong centralization, enterprise governance | Higher deployment friction | Regulated environments |

### 11.3 Recommended Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     Experience Layer                             │
│  IDE Extensions  │  CLI Integrations  │  Approval UIs  │  Console│
└────────┬────────────────┬─────────────────┬──────────────┬──────┘
         │                │                 │              │
┌────────▼────────────────▼─────────────────▼──────────────▼──────┐
│                    Agent Runtime Layer                           │
│  Runtime Hooks  │  SDK Wrappers  │  MCP Clients  │  Model Routes│
└────────┬────────────────┬─────────────────┬──────────────┬──────┘
         │                │                 │              │
┌────────▼────────────────▼─────────────────▼──────────────▼──────┐
│                    Enforcement Layer                             │
│ Shell Proxy│FS Guard│Net Proxy│MCP Gateway│DB Proxy│Secret Broker│
│                  │ Context Protection Gateway │                  │
└────────┬────────────────┬─────────────────┬──────────────┬──────┘
         │                │                 │              │
┌────────▼────────────────▼─────────────────▼──────────────▼──────┐
│                    Control Plane Layer                           │
│ Local Daemon │ Central Policy Engine │ Approval Service │        │
│ Policy Distribution │ Admin Console │ Posture Validation │       │
└────────┬────────────────┬─────────────────┬──────────────┬──────┘
         │                │                 │              │
┌────────▼────────────────▼─────────────────▼──────────────▼──────┐
│              Observability & Intelligence Layer                  │
│ Event Ingestion │ Graph-Native Replay │ Anomaly Detection │      │
│ Policy Simulation │ Alert Routing │ Export Pipelines │           │
└─────────────────────────────────────────────────────────────────┘
```

### 11.4 Deployment Topologies

**Topology A: Host-Based Pilot**
For initial pilots and lower-friction adoption. Runtime hooks, local daemon, and local proxies provide most mediation. Suitable for organizations starting with lighter enforcement and expanding over time.

**Topology B: Secure-Container Deployment**
For controlled developer environments where the workspace, process execution, and egress can be constrained in an ephemeral sandbox. Recommended high-assurance local-development topology. Uses namespaces for isolation, cgroups for resource control, capability dropping for least privilege, and controlled egress for network governance.

**Topology C: Remote-Workspace Deployment**
For enterprise-managed workspaces, regulated customers, and stronger tamper resistance. Preferred standardized enterprise topology because execution, storage, proxying, and identity can all be centrally controlled.

### 11.5 Secure Container Analysis

Containers are a powerful security primitive for AI agents because they create isolated, ephemeral, and tightly controlled environments. They provide:

- Filesystem isolation through explicit volume mounts only.
- Process isolation through namespaces.
- Resource control through cgroups.
- Network restriction through controlled egress.
- Reduced privilege through capability dropping.
- Reduced persistence risk through ephemeral lifecycle.

**What containers cannot fully solve alone:**

- MCP traffic outside the container boundary.
- Secrets accessed through external identity systems, browser sessions, or cloud CLIs.
- Data leakage into model context when prompt assembly occurs outside the container.
- Agent-to-agent communication through remote orchestrators.
- Database activity through external tools or services.
- Host compromise risk from dangerous configurations (--privileged, Docker socket mounting).

**Dangerous configurations Enforcer must forbid or flag:**

- Running agent containers with `--privileged`.
- Mounting `/var/run/docker.sock` into the container.
- Broad host filesystem mounts exposing `/`, home directories, or credential stores.
- Running as root with unnecessary capabilities.

**Product position:** Containers are a foundational enforcement substrate for Enforcer, not the complete solution. The product must extend beyond the container boundary through protocol-aware controls, network governance, MCP mediation, secret brokerage, and model-context protection.

---

## 12. Policy and Governance Model

### 12.1 Hierarchical Policy

```
Organization baseline (non-negotiable)
  └─ Team policy (can tighten, never weaken)
       └─ Repository / workspace policy (can tighten, never weaken)
            └─ Developer-local guardrails (can tighten, never weaken)
```

Organization-level policies define non-negotiable constraints: forbidden hosts, protected credential classes, blocked filesystem regions, required approvals, data handling rules, and escalation requirements. Lower-level policies may only add stricter rules — never weaken organizational baselines.

### 12.2 Policy Object

A practical policy object includes:

| Field | Purpose | Examples |
|---|---|---|
| **Subject** | Who or what the policy applies to | Agent type, session, user, team, environment tier |
| **Action** | What kind of action is being governed | read, write, exec, install, connect, query, invoke, prompt-submit |
| **Resource** | What the action targets | File path pattern, host/domain, secret store, database, MCP server:tool:method, model endpoint |
| **Conditions** | Additional context that affects the decision | Data classification, environment tier, destination, package source, path scope, time window, approval state |
| **Effect** | What happens when the policy matches | allow, deny, require approval, redact, quarantine, simulate, alert |
| **Logging mode** | How the event is recorded | full payload, metadata-only, redacted |
| **Approval mode** | How approval routing works | specific approver, team lead, on-call, break-glass |

### 12.3 Default Policies

The product ships with 13 default policy rules organized into 8 canned policy packs:

| Pack | Category | Rules |
|------|----------|-------|
| **Source Code Protection** | security | Block writes outside project root, block sensitive path reads, require approval for config file writes |
| **Supply Chain Security** | security | Require approval for package installs, require approval for lockfile changes |
| **Secrets & Credentials Hardening** | security | Deny credential access, block secret file reads |
| **Infrastructure Safety** | safety | Require approval for destructive commands, deny privilege escalation |
| **Network Egress Control** | network | Block non-allowlisted hosts |
| **Compliance & Audit** | compliance | Require approval for PII data access |
| **Developer Best Practices** | practices | Require approval for force push, deny direct production writes |
| **MCP Tool Governance** | agent | Require approval for untrusted MCP tool invocations |

Additionally, the network proxy enforces host allowlists with separate warning lists for hosts that should trigger alerts but not blocks.

---

## 13. Approval and Audit Flow

```
Agent attempts action (write / exec / network / MCP / secret access)
       │
       ▼
Hook handler intercepts and submits action context to daemon policy engine
       │
       ▼
Policy engine evaluates organizational + team + local guardrails
       │
       ├── ALLOWED → Hook exits 0 → Agent executes action
       │             → Log action + policy result to audit store
       │
       ├── DENIED → Hook exits 2 → Agent receives block with rationale
       │            → Log denial + rationale + agent identity to audit store
       │
       └── APPROVAL REQUIRED → Hook exits 2 (non-blocking)
                │               → Agent receives "approval required" message
                │               → Approval request created in audit store
                │
                ├── Reviewer approves via Hub Console or Sentinel Console
                │   → Developer retries → Hook exits 0 → Action executes
                │   → Log request + approver + result
                │
                └── Reviewer denies or approval expires
                    → Developer retries → Hook exits 2 → Action blocked
                    → Log request + denial + rationale
```

The approval workflow is non-blocking: the hook handler exits immediately with a deny exit code when approval is required, allowing the developer to continue other work. The developer retries the action after approval is granted. This preserves developer flow and avoids blocking the agent runtime while waiting for human decisions.

This flow preserves both agent intent and final system effect. Security teams care not just that a process ran, but that an agent attempted a specific action, a policy was evaluated, and a specific human or rule permitted or denied it.

---

## 14. Data Flows and Threat Surface Matrix

| Asset / Flow | Example Risk | Needed Control | Preferred Enforcement Point |
|---|---|---|---|
| **Repo files** | Writes outside project, reads from sensitive host paths | Path policy, content inspection, approval | Container + FS guard |
| **Shell** | Destructive or exfiltrative commands | Command policy, allow/deny/approval | Shell proxy + daemon |
| **Network egress** | Source code or PII exfiltration | Host allowlist, request logging, approval | Network proxy + container |
| **Package registry** | Malicious install or dependency change | Registry policy, approval, logging | Package hook + network proxy |
| **Secrets** | Token theft or misuse | Secret mediation, masking, approvals | Secret broker + policy engine |
| **MCP traffic** | Unsafe tool invocation or payload leakage | Tool and method controls, payload policy | MCP gateway |
| **Database** | Extraction or mutation of sensitive data | Query-class policy, approval, masking | DB proxy / tool wrapper |
| **LLM context** | Sensitive code or token exposure | Redaction, route policy, context DLP | Agent middleware + model gateway |
| **Agent-to-agent** | Hidden delegation chain, unbounded autonomy | Identity, isolation, lineage logging | Orchestrator + policy layer |
| **Container runtime** | Escape, daemon abuse, dangerous mounts | Hardened runtime, no privileged mode, no docker.sock | Runtime policy + deployment validation |

---

## 15. MVP Definition

The MVP should optimize for **depth over breadth**, consistent with the venture brief. Going deep on one or two areas is stronger than touching all of them.

### 15.1 MVP Feature Priorities

| Priority | Feature | Why It Matters |
|---|---|---|
| **P0** | File, shell, and network interception | Covers the clearest and most universally demanded enterprise risk surface |
| **P0** | Policy engine with allow/deny/require-approval | Establishes mandatory control — the core product promise |
| **P0** | Structured audit trail and security review console | Makes the product credible to security teams and compliance reviewers |
| **P0** | Opinionated default guardrails | Demonstrates enterprise value immediately without requiring policy authoring |
| **P1** | Human-in-the-loop approval UX (recommended depth area) | Enables controlled deployment without hard-blocking everything — the path from "denied" to "approved with oversight" |
| **P1** | VS Code integration | Meets developers where they work; native-feeling approvals instead of external console |
| **P1** | Secure-container deployment mode | Improves containment and deterministic enforcement for high-assurance pilots |
| **P1** | MCP governance layer | Creates protocol-aware differentiation that no endpoint tool can replicate |
| **P1** | Claude Code, Cursor, and Codex IDE support | Covers the most common enterprise coding-agent environments |
| **P2** | Secrets / PII redaction in agent context | Important for trust and compliance, but can follow core control plane |
| **P2** | Multi-agent isolation / delegation governance | High upside but secondary for initial wedge |
| **P2** | Anomaly detection over action sequences | Valuable but requires stable event model and replay substrate first |
| **P2** | Package installation governance | Clear value but narrower scope than file/shell/network |

### 15.2 Recommended Depth Area

The venture brief specifies choosing one depth area and going deep. **Human-in-the-loop approval UX** is the recommended primary depth area because:

1. It directly demonstrates the product's ability to convert "blocked" into "approved with oversight" — the core buyer value proposition.
2. It is visible to both developers and security teams, creating dual-persona validation.
3. It exercises the full pipeline: interception → policy evaluation → routing → human decision → execution or denial → audit logging.
4. It does not require ML infrastructure or complex anomaly models.
5. It is the feature most likely to turn a pilot into a production deployment.

**MCP-aware governance** is the recommended secondary depth area for differentiation, because it is the capability most likely to create sustainable competitive advantage by governing a protocol layer that no legacy security tool understands.

---

## 16. Phased Roadmap

### Phase 1: Controlled Enforcement Wedge (Complete)

**Objective:** Prove mandatory control over the highest-risk agent action surfaces in a controlled developer environment.

**Delivered:**

- 5 statically compiled Go binaries (daemon, hook handler, central, client, authseed) with zero runtime dependencies.
- Claude Code integration via hooks API with project root walk-up detection and logging to `~/.enforcer/hook.log`.
- Mandatory control over file system, shell execution, and network egress.
- Hierarchical policy engine with 13 default rules organized into 8 canned policy packs.
- Non-blocking approval workflow: hook exits immediately, developer retries after approval. Three scope types (single-use, time-bounded, session-scoped).
- Dual console architecture: Hub Console (port 9201, admin/reviewer RBAC) with Dashboard, Sessions, Approvals, Search, Export, Policies, Analytics; Sentinel Console (port 9100, operator role) with Dashboard, Sessions, Search, Export, My Activity.
- PostgreSQL append-only audit with decision normalization and unlimited query limit.
- Enterprise Analytics (Hub, org-wide) and personal analytics (Sentinel, individual developer).
- Developer group classifications named professionally (Compliant Developer, High-Friction Developer, etc.).
- mTLS Hub communication (ports 9200/9201) with certificate generation scripts.
- Sentinel agent with registration, policy sync (hash-based change detection), heartbeat, and audit forwarding.
- All drill-downs navigate from Analytics to Search to Session detail.
- `governed_user` from daemon health endpoint for Sentinel data filtering.
- Secret detection (20 file patterns, 12 command patterns) and redaction engine (18 regex patterns).

### Phase 2: Protocol-Aware Expansion

**Objective:** Expand enforcement surfaces and deepen protocol-aware governance.

- Add MCP gateway with method-level governance and payload-aware controls.
- Add package installation governance with registry policy and approval.
- Add secrets mediation and credential-access governance.
- Add prompt/context redaction and model-routing policy.
- Expand policy distribution to support team, repository, and environment scoping.
- Ship SIEM/SOAR integration for enterprise log pipelines.
- Support hardened runtimes (gVisor, Kata) for higher-assurance customers.
- Add policy simulation / dry-run mode for rollout without breaking developer velocity.

### Phase 3: Enterprise Control Plane

**Objective:** Become the unified governance platform for enterprise agentic development.

- Support agent-to-agent governance, delegation chains, and multi-agent isolation.
- Add database-aware controls for query-class policy and data masking.
- Expand to remote workspaces, CI/CD runners, and regulated environments.
- Build graph-native session replay with causal linking, impact-diff views, and exportable incident bundles.
- Build anomaly detection over action sequences with environment-tier and agent-role baselines.
- Add policy simulation, preflight planning, and agent risk scoring.
- Ship enterprise governance packs for regulated industries.
- Expand platform support and high-availability central services.

---

## 17. Market Appetite

### 17.1 Category Position

Enforcer is best positioned as a **runtime security and governance layer for AI coding agents** — not a generic "AI security" vendor, not an LLM guardrails tool, and not an endpoint monitoring product. The sharper message is "runtime governance for AI coding agents," with initial focus on engineering organizations where security approval is the gating factor for deployment at scale.

The market is fragmented rather than settled into a single category. The space is splitting across deterministic access control, continuous observability, behavioral tracking, intent-based authorization, and escalation controls — which creates room for a focused product aimed specifically at software-development agents.

### 17.2 Positioning Statement

Enforcer is the mandatory enforcement and governance layer for AI coding agents, giving enterprises real-time control over what agents can read, write, execute, call, and disclose across developer environments, secure containers, MCP tools, networks, and model workflows.

### 17.3 Wedge Go-to-Market Motion

1. **Land** with platform engineering and security on a pilot tied to one coding-agent rollout.
2. **Prove** mandatory controls on a narrow but painful risk surface: shell, network, package, and workspace governance.
3. **Expand** from one team to organization-wide policy distribution and audit reporting.
4. **Upsell** into broader MCP governance, secrets protection, remote workspace enforcement, and multi-agent control.

Use secure-container deployment as the fastest path to high-assurance pilots where local endpoint trust is limited.

### 17.4 GTM Positioning Against Adjacent Categories

| Adjacent Category | Their Limitation | Enforcer Message |
|---|---|---|
| **LLM guardrails** | Focus on prompt/response filtering, not machine-level actions | Enforcer governs real actions on real systems — files, shells, networks, tools |
| **Traditional endpoint / developer security** | Built for human users, not autonomous agents at machine speed | Enforcer is agent-aware and protocol-driven for coding tools |
| **Broad AI security platforms** | Span model scanning, red teaming, runtime defense across many AI use cases | Enforcer is purpose-built for the coding-agent rollout decision |
| **IDE / agent platform vendors** | Native controls are vendor-specific, not neutral enforcement | Enforcer is the independent cross-agent governance layer |
| **Sandbox / workspace vendors** | Strong isolation but not full policy/governance products | Enforcer adds policy, identity, approvals, and evidence on top of isolation |

---

## 18. TAM / SAM / SOM

The market should be modeled bottom-up around governed developer or agent seats in organizations adopting coding agents, adjusted by security-governance attach rate.

### Assumptions

- Target monetization unit: protected developer or agent seat under governance.
- Plausible annual price range: $300 to $1,200 per governed seat depending on enforcement depth, compliance features, and deployment model.
- Initial adoption strongest in organizations with 100+ developers and active AI coding-agent experimentation.

### Market Model

| Metric | Definition | Methodology | Estimate |
|---|---|---|---|
| **TAM** | Global spend potential for AI coding-agent governance across enterprise and mid-market software organizations | 10M long-term governed seats globally at $500/seat/year | ~$5.0B annually |
| **SAM** | Reachable seats in security-conscious North American and European mid-market and enterprise organizations | 1.5M reachable seats at $500/seat/year | ~$750M annually |
| **SOM** | Realistic 3-5 year attainable market for a focused wedge product | 40,000-80,000 seats at $500/seat/year | ~$20M-$40M ARR |

These are scenario-planning figures, not precise forecasts. The strategic point: even modest penetration of enterprise coding-agent deployments can create a meaningful security infrastructure company, especially if the product expands from per-seat controls into platform-wide governance and compliance modules.

---

## 19. Competitive Landscape

### 19.1 Competitive Map

| Category | Examples | What They Do Well | Weakness vs. Enforcer | Threat Type |
|---|---|---|---|---|
| **AI gateway / LLM firewall vendors** | Various prompt/model gateway products | Prompt inspection, model routing, API-boundary policy | Weak machine-action enforcement; limited endpoint and MCP context | Adjacent |
| **Endpoint / developer security tools** | EDR, SAST, code review, CI security | Strong device visibility, process/filesystem monitoring | Built for humans, not agent intent or MCP workflows | Substitute / adjacent |
| **Secrets management vendors** | HashiCorp Vault, cloud-native secret stores | Secret lifecycle, access control, rotation | Do not govern the full agent action chain | Adjacent |
| **CNAPP / DSPM / DLP vendors** | Broad cloud and data governance platforms | Data and infrastructure posture management | Too broad, not embedded in coding-agent execution path | Adjacent |
| **IDE / agent platform vendors** | Cursor, Claude Code, Codex native controls | Deep product context, native UX, embedded approvals | Vendor-specific, not neutral enforcement; incentive to optimize adoption, not mandate governance | Future platform threat |
| **Sandbox / remote workspace vendors** | E2B, Daytona, microVM providers | Strong environment isolation and blast-radius control | Usually not full policy/governance products; no policy, identity, or evidence layer | Adjacent / partner |
| **Open-source policy tooling** | OPA, custom policy scripts | Flexible primitives, low cost | Fragmented, high integration burden, weak enterprise UX | Substitute |

### 19.2 Competitive Conclusion

The most defensible position is not "another AI security dashboard," but **"the control plane where agent intent meets mandatory enterprise policy."** Enforcer wins when it provides deeper enforcement, better protocol awareness, stronger auditability, and lower operational friction than stitching together endpoint tools, network controls, model gateways, container runtimes, and custom policy scripts.

The largest medium-term threat is bundling by incumbent coding-assistant vendors that already offer some approvals, analytics, and admin controls. However, that same trend validates demand and may increase the need for an agent-neutral policy layer, especially in enterprises that mix vendors, require independent controls, or want stronger enforcement than an IDE-native settings panel can provide.

---

## 20. Differentiation Strategy

### 20.1 Table Stakes (Must Have)

- File, shell, and network interception.
- Policy engine with allow/deny/approval outcomes.
- Security-grade audit logs.
- Secure-container deployment profile with safe defaults.
- Basic admin console and reviewer workflows.

### 20.2 Differentiators (Why Enforcer Wins)

- **MCP-native governance** and payload-aware controls — no legacy endpoint tool understands MCP semantics.
- **Unified policy** over agent actions, data flows, and model-context exposure — one engine governing all surfaces.
- **Enterprise baselines plus developer-local guardrails** — hierarchical policy that mirrors how organizations actually work.
- **Full-chain replay** from agent intent to system effect — not just flat event logs but causal graphs of what happened and why.
- **Hybrid enforcement architecture** spanning runtime, container, proxy, and protocol layers — defense in depth, not single-point control.
- **Container posture validation** that forbids dangerous patterns (--privileged, docker.sock mounting) and supports hardened runtimes (gVisor, Kata).

### 20.3 Durable Moats

- Deep integrations across agent runtimes, MCP ecosystems, and secure development environments.
- Proprietary policy corpus and action-risk ontology tuned for agent workflows.
- High-quality audit data and forensics workflows that become embedded in enterprise security and compliance processes.
- Trust brand around enforceability, low-friction deployment, and strong secure-runtime support.

---

## 21. Pricing and Packaging Hypotheses

### 21.1 Model

Seat-based pricing with platform minimums. Value maps to protected developers, sanctioned agent sessions, and policy-governed rollout.

### 21.2 Candidate Tiers

| Tier | Capabilities | Target Buyer |
|---|---|---|
| **Team** | Core local enforcement, basic policy engine, audit logs, VS Code integration | Small-team pilots, developer-led adoption |
| **Business** | Centralized policy distribution, approval workflows, SIEM integration, containerized workspace enforcement, admin console, team/repo scoping | Platform engineering and security-led rollout |
| **Enterprise** | MCP governance, secrets/context protection, advanced forensics and replay, remote workspace support, hardened runtimes, compliance mapping, SSO/SCIM, premium support | Organization-wide deployment, regulated industries |

### 21.3 Pricing Range

$300-$1,200 per governed seat annually, depending on tier and deployment model. Platform minimums for business and enterprise tiers.

---

## 22. Success Metrics

### Product Metrics

| Metric | What It Measures |
|---|---|
| Percentage of covered agent actions mediated by policy | Enforcement completeness |
| Mean policy decision latency | Developer experience impact |
| Number of blocked, approved, and logged-only actions by class | Policy effectiveness |
| Coverage of MCP tools, hosts, and command classes under governance | Surface breadth |
| Percentage of deployments passing secure-container posture validation | Deployment security |

### Business Metrics

| Metric | What It Measures |
|---|---|
| Pilot-to-production conversion rate | Product-market fit |
| Number of protected developer seats per customer | Land-and-expand success |
| Expansion from one team to org-wide rollout | Growth motion |
| Reduction in security-review cycle time for coding-agent deployment | Core value proposition delivery |
| Average ARR per customer | Commercial viability |

### Trust Metrics

| Metric | What It Measures |
|---|---|
| Audit completeness rate for mediated actions | Evidentiary reliability |
| False-positive and false-block rates | Developer friction and policy quality |
| Number of bypass attempts detected | Tamper resistance |
| Time to investigate a policy incident using logs and replay | Forensic utility |
| Number of blocked dangerous runtime configurations | Deployment safety |

---

## 23. Key Risks and Open Questions

### 23.1 Risks

| Risk | Severity | Mitigation |
|---|---|---|
| **Bypass risk**: Developers or agents act outside the governed environment | High | Make the governed path the default; containerized deployment reduces bypass surface; detect and log bypass attempts |
| **Developer friction**: Too many approvals or false blocks kill adoption | High | Approval UX testing, policy simulation, graduated rollout, reusable approval scopes, smart defaults |
| **Coverage fragmentation**: Different agents, IDEs, terminals, and MCP servers create integration complexity | Medium | Compatibility matrix, common event schema, layered architecture |
| **Bundling risk**: IDE/agent vendors embed native controls that compress the category | Medium | Focus on cross-agent independence, mandatory enforcement, and audit quality that vendor-native settings cannot match |
| **Container limitations**: Container isolation is strong but imperfect, especially on shared kernels | Medium | Position containers as one layer in defense-in-depth; support hardened runtimes for higher assurance |
| **Evidence burden**: Storing useful audit trails without over-collecting sensitive data | Medium | Configurable retention policies, redaction-before-storage, privacy-aware defaults |
| **Category confusion**: Buyers conflate observability, content safety, and runtime enforcement | Medium | Crisp positioning as mandatory enforcement, not monitoring |

### 23.2 Open Questions

1. Can mandatory coverage remain strong on unmanaged laptops, or is the product strongest in controlled workspaces and containerized environments?
2. Which integration surface should be built first for strongest enterprise traction: VS Code extension, CLI shim, MCP gateway, or all three in parallel?
3. Which initial depth area creates the strongest market pull: approval UX, MCP governance, or secrets/context protection?
4. How quickly will major IDE or agent vendors embed native controls that compress this category?
5. How much of the market wants policy simulation before hard enforcement?
6. How should Enforcer represent partially observable cloud tasks (e.g., Codex delegated tasks) in audit reports?
7. What is the minimum viable secrets-governance feature set for launch?
8. Which SIEM, ticketing, and IAM integrations are most important for the first ten enterprise customers?

---

## 24. Sequencing Rationale

The venture brief explicitly states that **depth over breadth** is stronger than touching everything. The same logic drives the phased roadmap:

1. **Win first on one high-trust workflow** where enforcement is real, buyer pain is acute, and implementation can prove that coding-agent autonomy can be bounded without wrecking developer experience.
2. **Prove mandatory guardrails** on a narrow but painful risk surface (file, shell, network) with excellent approval UX and credible audit trails.
3. **Expand enforcement surfaces** (MCP, secrets, packages, databases) once the core engine is proven.
4. **Expand deployment models** (remote workspaces, CI, regulated environments) once the architecture supports it.
5. **Build intelligence** (anomaly detection, graph replay, policy simulation) once the event model, protocol normalization, and replay substrate are stable.

This sequencing ensures the product is credible at each stage — not a shallow prototype that claims broad coverage, but a deep product that proves mandatory enforcement in a real enterprise workflow and expands from proven trust.

---

## 25. Appendix A: Assumptions

1. AI coding-agent adoption will continue rising in mid-market and enterprise engineering organizations, driven by productivity gains in code generation, debugging, testing, refactoring, and integration work.
2. Security and governance blockers are one of the primary constraints on broader rollout, not technical capability or developer interest.
3. A hybrid architecture is necessary because no single proxy, container, plugin, or hook governs all relevant action and data flows.
4. Mid-market and enterprise buyers will pay a premium for mandatory control over a high-risk but high-value productivity layer.
5. The strongest initial wedge is enabling safe rollout of coding agents by satisfying security review requirements with mandatory control and reviewable evidence — not selling generic productivity or observability benefits.
6. Containers provide a strong safety foundation through namespaces, cgroups, capability restriction, and workspace isolation, but their security value depends on safe runtime configuration and surrounding controls.
7. Enterprises will accept governed remote or containerized agent environments for at least a meaningful subset of coding-agent workflows.
8. The product can obtain enough integration depth with popular agent frameworks and IDE workflows to capture semantic context without depending on any single vendor.
9. The strongest open question — how much enforcement can be centralized without imposing too much developer friction — must be validated through prototype pilots with policy simulation, approval UX testing, and measured impact on developer throughput.

---

## 26. Appendix B: Peer Review — Claude PRD vs. Codex PRD

> Compared against: `Enforcer_PRD_Peer.md` (Codex Peer Draft v1.0, April 26, 2026)
> Comparison date: April 26, 2026

This appendix compares the requirements coverage between the Claude-authored PRD (`Enforcer_PRD.md`) and the Codex-authored Peer PRD (`Enforcer_PRD_Peer.md`) against the venture prompt requirements. The goal is to identify which treatment is stronger per requirement and synthesize a best-of-both "New Requirement" for the final product.

### B.1 Venture Prompt Requirements

| Req No. | Claude Req | Codex Req | Verdict | New Requirement |
|---|---|---|---|---|
| **MVP-1: Intercept at least two surfaces** | Commits to three surfaces (file, shell, network) as P0. Detailed per-agent interception tables for Claude Code, Cursor, and Codex showing exactly what is intercepted per surface per agent. Exceeds the minimum requirement. | Commits to two surfaces (file writes, shell commands) as baseline with network egress as "optional third surface for stronger security posture." Higher-level surface descriptions. Adds the concept of tracking `attempted_action` vs `observed_effect` to measure interception quality. | **Claude is stronger** on scope and specificity (three firm surfaces, per-agent detail). **Codex adds** a valuable operational concept: measuring interception quality by tracking attempted vs observed effects. | Commit to three surfaces (file, shell, network) as P0 with per-agent interception tables. Add Codex's `attempted_action` / `observed_effect` tracking to every audit event so interception quality is measurable — including for blocked attempts that never produce an observed effect. |
| **MVP-2: Configurable policy with allow/deny/require approval** | Hierarchical policy engine (org → team → repo → local) with a seven-field policy object schema (Subject, Action, Resource, Conditions, Effect, Logging mode, Approval mode). Six decision types. Opinionated defaults. Very detailed model. | Supports allow/deny/require_approval with rules over actor, action, resource, path/host, and environment. Adds two operational details not in Claude: (a) machine-readable reason codes for every decision, and (b) policy version stamping for reproducibility and debugging. | **Claude is stronger** on policy model richness (hierarchy, schema depth, more decision types). **Codex adds** two critical operational features: reason codes and version stamping. Both are needed for production debugging and compliance reproducibility. | Use Claude's hierarchical model (org → team → repo → local) with full policy object schema and six decision types. Add Codex's machine-readable reason codes (attached to every decision) and policy version stamping (policy version recorded with every audit event for exact reproducibility). |
| **MVP-3: Structured audit logs for security reviewers** | 15+ field normalized event schema, session-level replay, graph-linked causality, search/filter/export, append-only immutability, privacy-aware retention with configurable redaction. Very comprehensive. | Structured and machine-parsable with causal linkage between attempt/decision/approval/effect. Append-only. Adds: mandatory minimum event schema enforcement (`who`, `what`, `when`, `policy`, `decision`, `result`) as a validation gate, plus session correlation IDs and export-ready formats. | **Claude is stronger** on schema richness, replay, and causality linking. **Codex adds** the concept of a mandatory minimum schema as a validation gate — ensuring no event can be emitted without the core fields, which prevents schema drift. | Use Claude's rich 15+ field schema with graph-linked causality and session replay. Add Codex's mandatory minimum schema validation: every event must pass a gate confirming `who`, `what`, `when`, `policy`, `decision`, `result` are present before it is accepted into the audit store. |
| **MVP-4: One depth area (approval UX)** | Rich feature set: in-IDE delivery via VS Code extension, browser fallback, time-bounded scopes, reusable approval windows ("approve all npm installs from this registry for this session"), break-glass emergency access, exception tracking, full audit integration. Strong rationale for choosing approval UX. | Approval UX with: pending action pause, reviewer decisioning, timeout behavior, full audit coverage. Adds three operational details not in Claude: (a) approval SLA targets (measurable response time expectations), (b) explicit timeout/fallback behavior (what happens when nobody responds), (c) reviewer context bundles (structured risk rationale presented to the approver). | **Claude is stronger** on feature richness (reusable windows, break-glass, multiple delivery channels). **Codex is stronger** on operational completeness: SLA targets, timeout behavior, and structured context bundles are essential for production but missing from Claude. | Use Claude's full feature set (in-IDE + fallback, reusable windows, break-glass). Add Codex's operational requirements: (a) approval SLA target (e.g., median decision under 60 seconds), (b) configurable timeout behavior (deny-on-timeout for high-risk actions, allow-on-timeout only where explicitly configured), (c) reviewer context bundles (structured summary: action, resource, risk rationale, policy rule, agent identity, session history — not raw command output). |
| **MVP-5: Depth over breadth** | Explicit P0/P1/P2 priority table with 14 items. Two depth areas: approval UX (primary) and MCP governance (secondary). All others deferred with explicit P2 scope. | Prioritizes strong enforcement quality on fewer surfaces before scaling scope. Adds: objective readiness gates (coverage percentage, false-positive rate, approval latency thresholds) that must be met before adding new interception surfaces. | **Equal** on the philosophy. Claude has more structured prioritization. **Codex adds** readiness gates — a disciplined product management practice that prevents premature expansion. | Keep Claude's P0/P1/P2 priority table with explicit deferral. Add Codex's readiness gates: define measurable thresholds (e.g., >95% interception coverage on governed surfaces, <5% false-positive block rate, <60s median approval latency) that must be met before a new surface is promoted from P1 to P0. |

### B.2 PRD Deliverable Requirements

| Req No. | Claude Req | Codex Req | Verdict | New Requirement |
|---|---|---|---|---|
| **PRD-1: User** | Five detailed personas (Security Engineer, Platform Engineer, Developer, Engineering Manager, Executive Buyer) with role descriptions, daily interaction patterns, core needs, and JTBD statements. | Four roles (Primary Buyer, Technical Champion, Economic Sponsor, End Users). Per-persona pain mapping in a dedicated section (Section 5) with specific pain points per role. | **Claude is stronger** on persona depth and JTBD articulation. **Codex adds** a dedicated per-persona pain mapping section that ties each persona directly to specific pain points — a useful format for sales and product. | Keep Claude's five personas with JTBD. Add Codex's per-persona pain mapping format as a companion section: for each persona, list 3-4 specific, concrete pain points that the product directly addresses. |
| **PRD-2: Pain point** | Detailed: 9-surface threat matrix with examples, "why existing controls fail" analysis, "adoption blocker" section listing specific unanswered governance questions. | Concise: lists 5 recurrent organizational failures. No threat matrix. More focused on the organizational consequence (can't answer, can't enforce, can't scale, can't reconstruct, inconsistent controls). | **Claude is stronger** on analytical depth (threat matrix, control gap analysis). **Codex is stronger** on executive clarity — the 5-failure framing is immediately compelling in a buyer conversation. | Keep Claude's detailed threat matrix and control gap analysis. Add Codex's 5-failure executive framing as a summary at the top of the problem statement for quick buyer communication. |
| **PRD-3: Market wedge** | Full treatment: 7-category competitive landscape, TAM/SAM/SOM, positioning statement, wedge GTM motion, adjacent-category positioning table with specific differentiation messages. | Concise: wedge described as "security review blocker," three wedge outcomes, brief competitive differentiation (3 points), GTM starting point. | **Claude is significantly stronger** on market analysis, competitive mapping, and sizing. Codex is leaner but lacks the depth needed for investor or board-level discussions. | Keep Claude's full market treatment. Adopt Codex's three concise wedge outcomes as a quick-reference summary alongside the detailed analysis. |
| **PRD-4: MVP scope** | Three interception surfaces, policy engine, audit logs, review console, approval UX depth area, VS Code integration, default policies, secure-container mode, three agent integrations. Detailed in/out scope. Section 8.4 with per-agent integration targets, platform matrix, and 6-step demo scenario. | Two surfaces (optional third), policy engine, approval service, audit pipeline, reviewer-facing minimal UI or CLI. Section 9.3.1 with MVP target environment table covering VS Code, CLI, desktop, MCP, and container paths. | **Claude is stronger** on scope ambition, agent-specific detail, and demo scenario. **Codex is stronger** on target environment breadth — explicitly addresses desktop apps, CLI, and container as demo paths in a single table. | Keep Claude's detailed scope with per-agent analysis and demo scenario. Integrate Codex's target environment table format to ensure desktop apps and CLI are not second-class demo paths. |
| **PRD-5: Sequencing rationale** | 3-phase roadmap (controlled enforcement → protocol-aware expansion → enterprise control plane) with detailed content per phase. Explicit sequencing rationale section explaining the logic. | 4-phase approach (Phase 0: foundations → Phase 1: MVP → Phase 2: hardening → Phase 3: expansion) with explicit exit criteria per phase. Phase 0 defines canonical schema before implementation begins. | **Equal** on strategic logic. **Codex is stronger** on two operational additions: (a) Phase 0 (foundations) — defining the canonical event schema and policy interface before building surfaces, which prevents schema rework; (b) explicit exit criteria per phase, which makes phase transitions objective rather than subjective. | Keep Claude's 3-phase strategic roadmap. Add Codex's Phase 0 (foundations: canonical schema, policy interface, audit contract) as a prerequisite before Phase 1. Add explicit exit criteria per phase from Codex so phase transitions are measurable. |
| **PRD-6: Primary buyer** | Security engineering lead at 300+ engineer organization. Detailed rationale: owns the security review process, has budget authority, is the person who blocks or approves rollout. Full buyer map with 5 roles. | Security Engineering Lead. Rationale: owns enforcement standards, owns risk acceptance, needs auditability, has organizational authority. Briefer but covers the same logic. | **Equal** on buyer identification and rationale. Claude has more supporting detail (buyer map, ICP definition). | Keep Claude's treatment with full buyer map. No changes needed — both PRDs agree on the primary buyer and the reasoning. |

### B.3 Unique Additions (Present in One PRD Only)

| Req No. | Claude Req | Codex Req | Verdict | New Requirement |
|---|---|---|---|---|
| **NEW-1: User stories with acceptance criteria** | Not present. Claude has personas and JTBD but no formal acceptance criteria. | Section 13: Four user stories (Developer, Security Reviewer, Incident Responder, Platform Operator) each with explicit, testable acceptance criteria. | **Codex is stronger.** Acceptance criteria make requirements verifiable. Without them, "structured audit logs" is subjective; with them, "events can be filtered by session, actor, time, and decision" is testable. | Add Codex's four user stories with acceptance criteria to the PRD. These serve as the definition of done for MVP features and as the basis for demo validation. |
| **NEW-2: Phase 0 — Foundations** | Not present. Claude jumps from scope definition to Phase 1 implementation. | Section 15.1: Phase 0 defines canonical action schema, policy decision interface, and audit event schema before any surface implementation begins. Exit criteria: a simulated action can be evaluated and logged end-to-end. | **Codex is stronger.** Defining the core contracts (event schema, policy interface, audit contract) before building surfaces prevents costly schema rework and ensures all surfaces emit consistent events from day one. | Add Phase 0 (Foundations) to the roadmap: define canonical action schema, policy decision interface, and audit storage contract. Exit criteria: a simulated action flows through interception → policy evaluation → audit logging end-to-end before any real surface is built. |
| **NEW-3: Readiness gates for phase expansion** | Not present. Claude's phases describe what to build but not when to advance. | Section 9.4/15: Objective readiness gates (coverage, false-positive rate, approval latency) must be met before adding new surfaces. | **Codex is stronger.** Readiness gates prevent premature expansion and ensure each phase is solid before the next begins. | Define measurable readiness gates per phase. Example for Phase 1 → Phase 2: >95% interception coverage on governed surfaces, <5% false-positive block rate, <60s median approval latency, >99% audit completeness. |
| **NEW-4: Least privilege by default** | Not present as an explicit design principle. Claude's principles cover mandatory enforcement, action chains, and explainability but not default-deny posture. | Section 7: "Least privilege by default: narrow what agents can do unless explicitly allowed." | **Codex is stronger.** Least privilege is a foundational security principle that should be explicit, not implied. It shapes default policy: the product should ship with restrictive defaults that are progressively relaxed, not permissive defaults that are progressively tightened. | Add "Least privilege by default" as a design principle: agents should have minimal permissions unless explicitly granted by policy. Default policies should restrict, not permit. |
| **NEW-5: Progressive rollout** | Not present as an explicit principle. Claude's roadmap implies progressive expansion but doesn't frame it as a principle. | Section 7: "Progressive rollout: support high-control mode and low-friction mode." | **Codex is stronger.** Progressive rollout as a principle means the product should support both strict enforcement (for security-conscious teams) and lighter monitoring (for teams still building confidence). This enables gradual adoption within an organization. | Add "Progressive rollout" as a design principle: support both high-control enforcement mode and low-friction simulation/monitoring mode so teams can adopt governance gradually and tighten controls as confidence grows. |
| **NEW-6: Platform operator story (policy change safety)** | Not present. Claude covers policy authoring but not the operational concern of safe policy updates. | Section 13.4: "As a platform operator, I want to change policy safely and predictably without redeploying every client workflow." Acceptance criteria: policies updatable via config change, policy version recorded with each decision, rollback path exists. | **Codex is stronger.** Policy change safety is a real operational concern — a bad policy update can either block all work (too strict) or create a security gap (too permissive). Rollback and versioning are essential. | Add the platform operator story: policies must be updatable without redeployment, every decision records the policy version that produced it, and a rollback path must exist for incorrect policy updates. |
| **NEW-7: Bypass detection** | Claude mentions tamper resistance in NFRs but does not detail bypass detection as a specific strategy. | Section 16: "Instrument bypass detection, expand coverage by phase" as a risk mitigation. | **Codex is stronger** on making bypass detection an explicit mitigation rather than an implicit NFR. | Add explicit bypass detection: the system should detect when agents or developers execute actions outside the governed path (e.g., direct shell access bypassing the proxy) and log these as ungoverned execution events with alerts. |
| **NEW-8: Competitive landscape depth** | Full 7-category competitive map with strengths, weaknesses, threat types. TAM/SAM/SOM. Pricing and packaging hypotheses (3 tiers). | Brief: 3-point differentiation summary against "patched together" alternatives. No market sizing or pricing. | **Claude is significantly stronger.** The competitive landscape, market sizing, and pricing hypotheses are essential for investor conversations and strategic planning. | Keep Claude's full competitive treatment. No changes needed. |
| **NEW-9: Secure container deep analysis** | Full analysis: what containers can/cannot solve, dangerous configurations to forbid, three deployment topologies, hardened runtime options (gVisor, Kata). | Not present in detail. MVP target table lists "containerized execution mode" as secondary. | **Claude is significantly stronger.** Container analysis is critical for the product's enforcement credibility story. | Keep Claude's container analysis. No changes needed. |
| **NEW-10: MCP governance as explicit FR** | FR-6: dedicated functional requirement for MCP governance with server allowlists, method-level policy, payload inspection, response transformation. | MCP listed as secondary MVP target but not elaborated as a functional requirement. | **Claude is stronger.** MCP governance is the primary technical differentiator — it deserves explicit FR treatment. | Keep Claude's FR-6 for MCP governance. No changes needed. |

### B.4 Summary

| Dimension | Claude PRD | Codex Peer PRD | Winner |
|---|---|---|---|
| **Scope and ambition** | More surfaces, more agents, more detail | More conservative, tighter MVP boundary | Claude |
| **Analytical depth** | Threat matrix, competitive landscape, market sizing, container analysis | Leaner, focused on core | Claude |
| **Operational rigor** | Rich feature descriptions but fewer operational guardrails | Reason codes, version stamping, SLA targets, timeout behavior, readiness gates, exit criteria | Codex |
| **Testability** | Personas and JTBD but no formal acceptance criteria | Four user stories with testable acceptance criteria | Codex |
| **Implementation readiness** | Demo scenario and per-agent integration analysis | Phase 0 foundations, target environment matrix, exit criteria per phase | Codex |
| **Design principles** | 8 principles (strong on enforcement and explainability) | 6 principles (adds least privilege and progressive rollout) | Equal — different strengths |
| **Market and competitive** | Comprehensive (7-category map, TAM/SAM/SOM, pricing) | Minimal | Claude |
| **Policy model** | Hierarchical with rich schema | Simpler but adds reason codes and version stamping | Claude (with Codex additions) |

**Overall:** Claude PRD is the stronger document for product strategy, market positioning, and technical comprehensiveness. Codex Peer PRD is stronger on operational rigor, testability, and implementation discipline. The best final PRD takes Claude's analytical depth and scope, then integrates Codex's operational additions: reason codes, version stamping, SLA targets, timeout behavior, readiness gates, exit criteria, Phase 0 foundations, user stories with acceptance criteria, least privilege principle, and bypass detection.

### B.5 Codex Review Comments on the Comparison

The following review comments were contributed by the Codex agent after reviewing the Claude-authored comparison above.

| Comment No. | Review Focus | Codex Comment | Impact | Recommendation |
|---|---|---|---|---|
| C-1 | Comparison quality | The comparison is comprehensive and requirement-traceable. It clearly maps prompt requirements to both documents and synthesizes merged requirements. | High positive: strong decision-support artifact for PRD finalization. | Keep this section as the canonical merge reference for PRD scope decisions. |
| C-2 | Decision framing | The "Winner" column is useful, but binary winner language can hide "use both" outcomes that are already implied in the `New Requirement` column. | Medium: could lead to over-prioritizing one source instead of integrating both. | Add a `Disposition` marker in future revisions (`Adopt Claude`, `Adopt Codex`, `Merge`, `Defer`) to make implementation intent explicit. |
| C-3 | Operationalization gap | The comparison identifies operational additions (SLA targets, readiness gates, version stamping), but it does not explicitly track whether each item is now incorporated into the final PRD body. | High: risk of good recommendations not being executed. | Add a short post-comparison checklist table with `Recommendation`, `Section Added`, `Status` (`Done/Partial/Todo`). |
| C-4 | Document maintainability | Section 0.4 is dense and long, which is useful for authoring but heavy for readers consuming the PRD as a product spec. | Medium: readability cost for stakeholders who only need final requirements. | Keep 0.4 for traceability but move it to an appendix in the final published PRD (or keep a separate `PRD_comparison_notes.md`). |
| C-5 | Version alignment | The comparison references Codex peer content snapshots; the peer doc has evolved with additional MVP targeting details and matrix updates. | Medium: stale cross-document references can create confusion during implementation. | Stamp comparison sections with "Compared against version/date" and rerun diff checks before locking PRD v1. |
| C-6 | MVP demo clarity | The comparison correctly argues for strong MVP depth but final implementation planning still needs one explicitly chosen primary coding-agent integration path for demo execution. | High for execution: ambiguity can delay build decisions. | Lock one primary demo path (`VS Code + Claude Code` or `VS Code + Codex`) and mark others as secondary/compatibility for MVP. |

### B.6 Recommendation Disposition Tracker

| Recommendation | Source | Disposition | Status | Notes |
|---|---|---|---|---|
| Add `attempted_action` / `observed_effect` tracking to audit events | Codex (MVP-1) | Merge | Todo | Incorporate into FR-2 (Action Normalization) schema |
| Add machine-readable reason codes to every policy decision | Codex (MVP-2) | Merge | Todo | Incorporate into FR-3 (Policy Engine) |
| Add policy version stamping to every audit event | Codex (MVP-2) | Merge | Todo | Incorporate into FR-2 and FR-5 |
| Add mandatory minimum schema validation gate for audit events | Codex (MVP-3) | Merge | Todo | Incorporate into FR-5 (Audit Trail) |
| Add approval SLA targets | Codex (MVP-4) | Merge | Todo | Incorporate into FR-4 (Approval Workflows) and Section 22 (Metrics) |
| Add configurable timeout/fallback behavior for approvals | Codex (MVP-4) | Merge | Todo | Incorporate into FR-4 |
| Add reviewer context bundles for approval requests | Codex (MVP-4) | Merge | Todo | Incorporate into FR-4 |
| Add readiness gates before phase expansion | Codex (MVP-5) | Merge | Todo | Incorporate into Section 16 (Roadmap) |
| Add per-persona pain mapping section | Codex (PRD-1) | Merge | Todo | Add companion to Section 5.3 |
| Add 5-failure executive framing to problem statement | Codex (PRD-2) | Merge | Todo | Add summary at top of Section 3 |
| Add Phase 0 (Foundations) to roadmap | Codex (NEW-2) | Adopt Codex | Todo | Add before Phase 1 in Section 16 |
| Add exit criteria per roadmap phase | Codex (NEW-2) | Adopt Codex | Todo | Add to each phase in Section 16 |
| Add user stories with acceptance criteria | Codex (NEW-1) | Adopt Codex | Todo | Add new section (Section 26 or inline after FRs) |
| Add "Least privilege by default" design principle | Codex (NEW-4) | Adopt Codex | Todo | Add to Section 6 |
| Add "Progressive rollout" design principle | Codex (NEW-5) | Adopt Codex | Todo | Add to Section 6 |
| Add platform operator story (policy change safety) | Codex (NEW-6) | Adopt Codex | Todo | Include with user stories |
| Add explicit bypass detection | Codex (NEW-7) | Adopt Codex | Todo | Add to NFRs or risk mitigations |
| Keep Claude's competitive landscape, container analysis, MCP governance | Claude (NEW-8/9/10) | Adopt Claude | Done | Already in Sections 19, 11.5, FR-6 |
| Lock primary MVP demo integration path | Codex C-6 | Pending | Todo | Decision: VS Code + Claude Code recommended in Section 8.4.1 |
| Add comparison version/date stamping | Codex C-5 | Adopt Codex | Done | Added to Appendix B header |

---

## 27. Appendix C: Final Consolidated Requirements and Deliverables (Ratified)

> Status: **Final (ratified requirements; implementation docs pending).** All scope decisions resolved as of April 26, 2026.
> Decisions made: Cursor is Phase 2 (not Phase 1). Claude Code VS Code extension + CLI is the sole Phase 1 integration. TDD to be authored as `docs/Enforcer_TDD.md` based on these requirements.

This appendix presents the authoritative, merged requirements for Enforcer. Each item integrates the strongest treatment from the Claude PRD and the Codex Peer PRD, as determined by the peer review in Appendix B. This is the definitive reference for the development team.

### C.1 Prototype Deliverables (from Venture Prompt)

| Del. No. | Deliverable | Acceptance Standard |
|---|---|---|
| **D-1** | Working prototype | Functional system that intercepts, inspects, and governs the actions of an AI coding agent in a developer environment. Must pass all requirements in C.2 and all acceptance criteria in C.5. |
| **D-2** | PRD | This document. Covers user, pain point, market wedge, MVP scope, sequencing rationale, and primary buyer. |
| **D-3** | TDD | Separate document (`docs/Enforcer_TDD.md`, to be authored based on these requirements). Existing design references: `Enforcer_TDD_Final_1.md`, `_2.md`, `_3.md`, `_Peer.md`. Covers architecture decisions, interception layer placement, policy model, audit log schema, performance trade-offs, and evolution path. |

### C.2 Final MVP Requirements

| Req No. | Requirement | Final Specification | Source |
|---|---|---|---|
| **R-1** | **Intercept agent actions across at least three surfaces** | Intercept file system reads/writes, shell command execution, and network egress with pre-execution enforcement (block capability, not just logging) on all three. Every audit event must include both `attempted_action` and `observed_effect` fields so interception quality is measurable — including for blocked attempts that produce no observed effect. Package installs and credential access are P1 extensions. | Claude scope + Codex interception quality tracking |
| **R-2** | **Enforce configurable hierarchical policy** | Policy engine with hierarchical inheritance (org → team → repo → developer-local) where lower levels can tighten but never weaken upper-level baselines. Policy object schema: Subject, Action, Resource, Conditions, Effect, Logging mode, Approval mode. Decision types: allow, deny, require approval, redact, quarantine, simulate. Every decision must include a machine-readable reason code. Every audit event must record the policy version that produced the decision for exact reproducibility. Ship opinionated default policies: deny writes outside project root, deny non-allowlisted network hosts, require approval for high-risk shell commands. | Claude hierarchical model + Codex reason codes and version stamping |
| **R-3** | **Produce structured, reviewer-grade audit logs** | Normalized event schema (15+ fields): timestamp, actor identity (user + agent type + instance + session ID), environment context (workspace, repo, branch, tier, deployment mode), action type, resource target, attempted action, observed effect, payload summary (redacted per policy), policy evaluated + version, decision + reason code, approval state, approver identity + rationale + scope + expiry, correlation IDs for session and delegation tracing. Mandatory minimum schema validation gate: every event must pass a check confirming `who`, `what`, `when`, `policy`, `decision`, `result` are present before acceptance into the audit store. Events are append-only (no UPDATE or DELETE). Session-level replay with graph-linked causality. Search, filter, and export via security review console. | Claude rich schema + Codex minimum schema validation gate |
| **R-4** | **Implement non-blocking human-in-the-loop approval UX as the primary depth area** | When policy evaluates to "require approval," the hook handler exits immediately with a deny exit code and the system creates an approval request. The developer can continue other work and retries the action after approval is granted. Approvals are managed via the Hub Console (admin/reviewer RBAC) and the Sentinel Console (operator role). Features: three scope types (single-use, time-bounded, session-scoped), pattern matching against action types and resource values, break-glass emergency access with exception tracking. Reviewer context bundles: structured summary of action, resource, risk rationale, policy rule, agent identity, and session history — not raw command output. System latency target: approval request created within 2 seconds of policy evaluation; decision enforced on next retry within 1 second. Operational benchmark: median end-to-end approval decision under 60 seconds (dependent on organizational staffing, not solely product design). Approval metrics track created, approved, denied, and expired counts. Every approval decision recorded in audit trail with approver identity, rationale, scope, and expiry. | Claude feature set + Codex SLA targets, timeout behavior, context bundles + non-blocking implementation |
| **R-5** | **Depth over breadth with readiness gates** | P0/P1/P2 priority framework. Primary depth area: approval UX. Secondary differentiator: MCP-aware governance. All other capabilities (anomaly detection, multi-agent isolation, advanced secrets redaction) deferred to P2. Measurable readiness gates before phase expansion: >95% interception coverage on governed surfaces, <5% false-positive block rate, <60s median approval latency, >99% audit event completeness. No new surface is promoted from P1 to P0 until gates are met. | Claude priority framework + Codex readiness gates |

### C.3 Final Design Principles

| No. | Principle | Description |
|---|---|---|
| **P-1** | Mandatory where promised | Never market observability as enforcement. If the product claims to block an action, it must block it. |
| **P-2** | Action chains, not isolated events | Govern the full causal chain from agent intent through tool invocation through system effect. |
| **P-3** | Protocol-aware by default | Deep protocol mediation for MCP and model-context flows — how Enforcer differentiates from generic endpoint tools. |
| **P-4** | Enterprise baselines plus local guardrails | Organization-level policies are non-negotiable. Teams and developers can add stricter rules but never weaken baselines. |
| **P-5** | Explainable decisions | Every policy outcome must be understandable to both developers and security reviewers. |
| **P-6** | Containers as substrate, not sole answer | Secure containers provide strong isolation but do not govern MCP traffic, external secrets, model context, or agent-to-agent delegation outside the boundary. |
| **P-7** | Low-friction developer experience | Approvals and blocks should be exception-based and context-aware so safe work stays fast. |
| **P-8** | Agent-native architecture | Model the environment as inherently multi-agent. Govern direct actions and delegated actions. |
| **P-9** | Least privilege by default | Agents have minimal permissions unless explicitly granted by policy. Default policies restrict, not permit. |
| **P-10** | Progressive rollout | Support both high-control enforcement mode and low-friction simulation/monitoring mode so teams can adopt governance gradually and tighten controls as confidence grows. |

### C.4 Final Phased Roadmap

| Phase | Name | Objective | Key Deliverables | Exit Criteria |
|---|---|---|---|---|
| **Phase 0** | Foundations | Define core contracts before building any surface | Canonical action schema, policy decision interface, audit event storage contract, minimum schema validation gate | A simulated action flows through interception → policy evaluation → audit logging end-to-end. All schema contracts are versioned and reviewed. |
| **Phase 1** | Controlled Enforcement Wedge (Complete) | Prove mandatory enforcement on highest-risk surfaces with one agent | 5 Go binaries (daemon, hook handler, central, client, authseed); file, shell, network interception with real enforcement; hierarchical policy engine with 13 default rules and 8 canned packs; non-blocking approval workflow (hook exits immediately, developer retries); dual console builds (Hub Console on port 9201 with admin/reviewer RBAC, Sentinel Console on port 9100 with operator role); Hub nav: Dashboard, Sessions, Approvals, Search, Export, Policies, Analytics; Sentinel nav: Dashboard, Sessions, Search, Export, My Activity; PostgreSQL append-only audit with decision normalization; Claude Code integration via hooks API with project root walk-up; mTLS Hub communication (ports 9200/9201); Enterprise Analytics (Hub) and personal analytics (Sentinel). | Demonstrable allow/deny/approval enforcement across three surfaces on Claude Code. Reviewer can inspect and approve actions via Hub Console. Audit output reconstructs session-level action chains with drill-downs from Analytics to Search to Session detail. |
| **Phase 2** | Protocol-Aware Expansion | Expand enforcement surfaces and deepen protocol governance | Cursor integration via MCP gateway; MCP gateway with method-level policy; Codex VS Code extension support; package install governance; secrets mediation; prompt/context redaction; SIEM integration; policy simulation/dry-run mode; hardened runtimes (gVisor, Kata) | Stable pilot operation in controlled enterprise environments. MCP traffic is policy-evaluable at server and method level. Secrets access is governed and audited. Phase 1 readiness gates remain met. |
| **Phase 3** | Enterprise Control Plane | Become the unified governance platform | Multi-agent governance and delegation chains; database-aware controls; graph-native session replay; anomaly detection with environment/role baselines; remote workspace support; CI/CD runner governance; policy simulation and preflight planning; enterprise governance packs | Multi-team deployment readiness. Policy, replay, and anomaly systems operate on shared graph objects. Delegated agents have distinct policy envelopes. Phase 2 readiness gates remain met. |

### C.5 Final User Stories and Acceptance Criteria

#### US-1: Developer — Controlled Autonomy

**As a developer,** I want my agent to run normal tasks without constant blocking, while clearly stopping on high-risk operations.

**Acceptance criteria:**
- Low-risk actions (file reads/writes within project root, safe shell commands) execute automatically when policy allows, with no visible interruption (hook exits 0).
- Blocked actions return a clear reason (the specific policy rule that matched) and the next available path (request approval, retry in allowed scope, or open exception workflow). Hook exits 2.
- Approval-required actions: hook exits immediately with deny code (non-blocking), creates approval request. Developer retries after approval is granted via Hub Console or Sentinel Console.

#### US-2: Security Reviewer — Action Approval

**As a security reviewer,** I want to approve or deny sensitive actions with enough context to make a fast, defensible decision.

**Acceptance criteria:**
- Reviewer sees a structured context bundle in the Hub Console or Sentinel Console: actor identity, resource targeted, command/path/host, policy rule matched, risk rationale, and session history.
- Reviewer can approve or deny with one action and an optional rationale field.
- Decision is enforced on the developer's next retry (non-blocking workflow: hook exits immediately, developer retries after approval) and fully audited with approver identity, rationale, scope, and timestamp.
- System latency target: approval request created within 2 seconds of policy evaluation; decision enforced on next retry within 1 second.
- Operational benchmark: median end-to-end time from approval request to reviewer decision under 60 seconds (dependent on organizational staffing and on-call process, not solely product design).

#### US-3: Incident Responder — Forensic Trace

**As an incident responder,** I want to reconstruct what happened in an agent session from attempted action to final system effect.

**Acceptance criteria:**
- Events can be filtered by session, actor, action type, resource, policy decision, time range, and risk level.
- Event chain includes attempted action, policy evaluated (with version), decision returned (with reason code), approval state, and observed execution result.
- Blocked attempts are recorded with the same schema as executed actions (with `observed_effect` marked as "blocked").
- Logs are exportable in structured format (JSON) as evidence packages for investigation or compliance review.

#### US-4: Platform Operator — Policy Control

**As a platform operator,** I want to change policy safely and predictably without redeploying every client workflow.

**Acceptance criteria:**
- Policies can be updated through configuration change (YAML/JSON bundle) without daemon restart or client redeployment.
- Every policy decision records the policy version that produced it, enabling exact reproducibility.
- A rollback path exists for incorrect policy updates (revert to previous version with one operation).
- Policy changes are themselves audited (who changed what, when, with what justification).

#### US-5: Executive Buyer — Governance Evidence

**As a CISO or VP Security,** I need evidence that governance is active and that the organization can pass audit.

**Acceptance criteria:**
- Dashboard-level summary: number of governed sessions, actions mediated, actions blocked, approvals granted/denied, audit completeness rate.
- Evidence that policy enforcement is mandatory for governed surfaces (not optional or bypassable).
- Exportable compliance evidence package covering a configurable time window.

### C.6 Final Integration Targets

**Phase 1 targets (P0):**

| Target | Mode | Integration Mechanism | Enforcement |
|---|---|---|---|
| **Claude Code** | VS Code extension | Hooks API + filesystem guard + shell proxy + network proxy | Full — all actions local, pre-execution interception |
| **Claude Code** | CLI (`claude`) | Process wrapper + filesystem guard + shell proxy + network proxy | Full — all actions local |

**Phase 2 targets (P1):**

| Target | Mode | Integration Mechanism | Enforcement |
|---|---|---|---|
| **Cursor** | VS Code (agent mode) | MCP gateway + filesystem guard + shell proxy + network proxy | Strong for MCP-routed actions; standard for direct OS actions |
| **Codex** | VS Code extension (local) | Filesystem guard + shell proxy + network proxy | Full for local actions |

**Deferred targets (P2 — pending integration surface analysis):**

| Target | Mode | Notes |
|---|---|---|
| **Codex** | Cloud-delegated tasks | No local interception possible. Visibility only — log delegation event, flag as partially observable. |
| **Claude Code** | Desktop app | Likely same mechanism as VS Code extension if hooks API is available. Requires integration surface analysis before committing. |
| **Copilot agents** | VS Code / GitHub | Requires integration surface analysis. Not committed until Phase 2 scope is finalized. |

### C.7 Final Success Metrics

| Category | Metric | Definition | Target | Measurement Window |
|---|---|---|---|---|
| **Enforcement** | Policy mediation rate | (Actions on governed surfaces that received a policy decision) / (Total actions on governed surfaces). Denominator: all file, shell, and network actions intercepted by the daemon. | >95% | Rolling 7-day per deployment |
| **Enforcement** | Enforcement fidelity | (Denied actions where execution was successfully blocked) / (Total denied actions). Measures whether "deny" actually stops the action. | >99% | Rolling 7-day per deployment |
| **Enforcement** | False-positive block rate | (Denied or approval-required actions later marked unnecessary by reviewer or policy author) / (Total denied + approval-required actions). Requires a feedback mechanism (e.g., "mark as false positive" in review console). | <5% | Rolling 30-day per deployment |
| **Performance** | Policy decision latency (system) | Time from daemon receiving an action request to returning a policy decision. Measured at p50 and p95. Excludes human approval time. | p50 <10ms, p95 <50ms | Continuous, reported per session |
| **Performance** | Approval delivery latency (system) | Time from policy decision "require approval" to approval request visible to reviewer. | <2 seconds | Continuous |
| **Performance** | Approval enforcement latency (system) | Time from reviewer clicking approve/deny to action proceeding or being blocked. | <1 second | Continuous |
| **Approval** | End-to-end approval time (operational) | Time from approval request created to reviewer decision recorded. Includes human response time. | Median <60 seconds (operational benchmark, not product SLA) | Rolling 7-day per deployment |
| **Audit** | Audit completeness | (Governed actions with a full event chain: attempted action + policy decision + result) / (Total governed actions). | >99% | Rolling 7-day per deployment |
| **Audit** | Schema validation pass rate | (Events passing minimum schema validation gate) / (Total events emitted). Should be 100% — any failure indicates a bug. | 100% | Continuous, alert on any failure |
| **Adoption** | Pilot-to-production conversion rate | (Pilot customers that move to production deployment) / (Total pilot customers). | Track from first deployment | Per cohort |
| **Adoption** | Protected seats per customer | Number of developer seats under active governance per customer account. | Track expansion | Monthly |
| **Business** | Security approval cycle time | Time from agent rollout request to security team sign-off, measured before and after Enforcer deployment. | Measure reduction vs. baseline | Per approval event |
| **Trust** | Bypass attempts detected | Count of actions detected outside the governed path (e.g., direct shell bypassing proxy). | Track and alert on any occurrence | Rolling 7-day per deployment |
| **Trust** | Incident investigation time | Time from incident opened to root cause identified using Enforcer audit trail and replay. | Measure and optimize | Per incident |
