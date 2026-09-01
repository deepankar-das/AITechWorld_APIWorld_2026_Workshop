> Author: Deepankar Das

# Enforcer Product Requirements Document

## Document purpose
This PRD defines the product requirements for Enforcer, a runtime security and governance layer for AI coding agents, with explicit support for Visual Studio Code environments and adjacent agent workflows including Claude Code, OpenAI Codex, Cursor, and MCP-based tools.[cite:1][cite:29][cite:35][cite:37][cite:43]

## Product summary
Enforcer is a security and policy layer that sits between AI coding agents and the systems they can access, including file systems, shell commands, package managers, network calls, API keys, cloud credentials, and production-adjacent environments. The core product goal is to let engineering organizations adopt AI coding agents with enforceable policy controls, approval workflows, and audit trails that satisfy security and platform teams.[cite:1]

The product must be designed for real developer workflows rather than abstract model safety. That means Enforcer needs first-class integration into the IDE and CLI paths where agents actually operate, especially in VS Code and VS Code-compatible tooling.[cite:35][cite:37][cite:43]

## Problem statement
AI coding agents increasingly act with permission to read and write files, execute shell commands, install packages, call external services, and access secrets, yet most organizations still lack a dedicated policy and audit layer for those machine-speed actions. The attached brief identifies this governance gap as the core blocker to enterprise rollout of coding agents inside mid-market and enterprise development teams.[cite:1]

Existing controls are fragmented. Cursor exposes MCP and permission configuration, Claude Code provides file diff approval workflows inside VS Code, and Codex now offers a VS Code extension and support for VS Code forks, but none of these products is primarily built as an independent cross-agent security control plane for enterprise governance.[cite:29][cite:30][cite:35][cite:37][cite:41][cite:43]

## Product vision
Enforcer should become the default runtime control plane for enterprise AI coding agents. It should give security teams confidence that any supported coding agent operating inside local development, IDE, terminal, CI, or MCP-mediated workflows can be observed, constrained, and investigated without forcing developers to abandon the tools they already use.[cite:1][cite:29][cite:35][cite:37]

## Goals
- Enable safe rollout of AI coding agents across software teams by enforcing action-level policies at runtime.[cite:1]
- Integrate with existing developer environments, especially VS Code and VS Code-compatible tools, instead of requiring a separate proprietary editor.[cite:35][cite:37][cite:43]
- Produce clear, structured, reviewer-friendly audit trails for agent behavior and policy decisions.[cite:1]
- Support multiple leading coding agents through a common interception and policy architecture, including Claude Code, Cursor, Codex, and MCP-based workflows.[cite:29][cite:35][cite:37]
- Preserve developer velocity by using selective approvals and risk-based controls rather than blanket blocking.[cite:1][cite:37]

## Non-goals
- Enforcer is not a full-feature IDE.
- Enforcer is not a general-purpose LLM content moderation platform.
- Enforcer is not a replacement for endpoint protection, SIEM, or secrets management systems.
- Enforcer should not attempt to train or fine-tune foundation models in the MVP.

## Target users
### Primary buyer
The primary buyer is the security engineering lead or platform engineering lead at a mid-market or enterprise software organization. The attached brief explicitly points to security engineering, platform engineering, and CISO-type buyers because those are the functions responsible for approving or blocking broader use of coding agents.[cite:1]

### Primary user personas
- Security engineer reviewing agent behavior, policies, exceptions, and incidents.[cite:1]
- Platform engineer enabling approved agent workflows across teams, machines, repos, and environments.[cite:1]
- Engineering manager or developer experience lead rolling out coding agents safely across the organization.[cite:1]
- Developer using Claude Code, Cursor, Codex, or adjacent agent tooling from VS Code, terminal, or compatible IDE workflows.[cite:35][cite:37][cite:41][cite:43]

## User stories
- As a security engineer, there needs to be a readable audit trail showing which agent accessed which file, ran which command, contacted which host, and whether policy allowed, denied, or escalated the action.[cite:1]
- As a platform engineer, there needs to be a central policy layer that can govern multiple agent tools without custom one-off rules for each editor or assistant.[cite:1][cite:29][cite:35]
- As a developer, there needs to be low-friction approval handling inside the normal coding workflow so security controls do not feel like a separate external product.[cite:37][cite:43]
- As an engineering leader, there needs to be evidence that high-risk behaviors are constrained and that adoption can expand beyond small pilot groups safely.[cite:1]

## Product principles
- Agent-native: model actions matter more than prompt content.
- IDE-compatible: integrate where developers already work.
- Cross-vendor: support multiple coding agents through a common control layer.
- Enforceable: policies must block or require approval, not just log events.
- Legible: logs and approvals must be understandable to both security teams and developers.

## Functional requirements

## Integration requirements
Enforcer must support two integration surfaces from day one: local interception and IDE-visible workflow integration. This is essential because Claude Code, Cursor, and Codex already operate through a mix of extensions, CLI agents, MCP tools, and VS Code side panels rather than one universal runtime model.[cite:29][cite:35][cite:37][cite:41][cite:43]

### FR-1: VS Code integration
Enforcer must provide a VS Code extension or companion integration that can:
- display agent action approvals,
- show policy decisions and blocked actions,
- link to audit records for the current session,
- surface high-risk prompts or actions requiring confirmation,
- and map events to current workspace, repository, and branch context.[cite:37][cite:43]

The VS Code integration should feel native to the editor rather than like a browser popup. Claude Code’s current VS Code experience demonstrates that developers expect in-IDE diff review, permission prompts, and context-aware interaction, so Enforcer must integrate into a similarly natural workflow layer.[cite:37][cite:43]

### FR-2: Claude Code support
Enforcer must support Claude Code in both extension and CLI-connected modes. Claude Code documentation states that the recommended workflow is a native VS Code extension, while the CLI can also connect into VS Code and provide diff viewing and diagnostic sharing.[cite:37][cite:43]

Required Claude Code capabilities:
- capture proposed file edits before acceptance where feasible,
- capture shell command requests and outcomes,
- apply path, command, network, and package policies,
- and preserve user approval context in the audit trail.[cite:1][cite:37]

### FR-3: Cursor support
Enforcer must integrate with Cursor’s MCP and permission model. Cursor documentation shows that Cursor supports MCP servers and configurable permissions, making MCP a practical insertion point for governance, inspection, and policy-aware tool mediation.[cite:29][cite:30]

Required Cursor capabilities:
- inspect and govern MCP tool usage,
- map agent activity to server/tool identity,
- enforce allow/deny/approval rules on file and network actions,
- and record enabled server status and permission context as part of the audit trail when available.[cite:29][cite:30][cite:33]

### FR-4: Codex support
Enforcer must support OpenAI Codex through the Codex IDE extension path for VS Code and VS Code forks. OpenAI documents that the Codex IDE extension works with VS Code forks such as Cursor, and that Codex can read, edit, and run code either side-by-side in the IDE or through delegated cloud tasks.[cite:35][cite:41]

Required Codex capabilities:
- capture local edit and run actions initiated through the IDE extension where exposed,
- tag locally initiated actions separately from delegated cloud tasks,
- and warn when actions cannot be fully observed because execution occurred outside the local governed runtime.[cite:35][cite:41]

### FR-5: Claude, Codex, Cursor, Claude Code, and Codex IDE compatibility matrix
The product must maintain an internal compatibility matrix by tool, platform, and mode. “Supported” must be defined precisely per surface, since some products operate via extension, some via CLI, and some via cloud delegation.[cite:29][cite:35][cite:37][cite:41]

Minimum matrix columns:
- Vendor/tool name
- Integration mode, for example extension, CLI, MCP, delegated task
- Observable actions
- Enforceable actions
- Approval support
- Audit coverage quality
- Known limitations

## Runtime policy requirements
### FR-6: Action interception
Enforcer must intercept and evaluate at least these action classes:
- file reads and writes,
- shell command execution,
- outbound network calls,
- package installation,
- and credential or secret access attempts.[cite:1]

### FR-7: Policy engine
The product must support configurable allow, deny, and require-approval actions with rules based on:
- path scope,
- command pattern,
- host allowlist or denylist,
- package source,
- credential class,
- environment,
- repository,
- user,
- and agent identity.[cite:1]

### FR-8: Human approval
Enforcer must support human-in-the-loop approvals for high-risk actions. At minimum, this applies to writes outside project scope, privileged shell commands, installs from untrusted sources, access to secrets, and outbound network calls to unapproved hosts.[cite:1]

Approvals must be available both in-IDE, when supported, and through a fallback browser or desktop review flow. The PRD should not assume all agents expose identical UI hooks, so Enforcer needs a normalized approval path across products.[cite:29][cite:35][cite:37]

### FR-9: Audit logging
The product must create a structured audit trail containing:
- timestamp,
- user identity,
- agent identity,
- IDE or runtime source,
- workspace or repository context,
- action type,
- resource targeted,
- policy matched,
- decision,
- approval actor if any,
- result,
- and correlation identifiers for session tracing.[cite:1]

### FR-10: Session replay and traceability
Security reviewers must be able to reconstruct an agent session timeline, including prompts or prompt metadata where policy permits, actions attempted, decisions made, and final outcomes. The emphasis is not full video replay but a structured incident-review narrative suitable for risk and compliance workflows.[cite:1]

## Platform requirements
### FR-11: MCP compatibility
Enforcer must support Model Context Protocol-oriented environments because Cursor and other agent frameworks rely on MCP servers to expose tools, data, and actions. The product should be able to govern MCP tool invocations directly or through an MCP wrapper/gateway pattern.[cite:29][cite:31][cite:33]

### FR-12: Multi-runtime architecture
The product must support local developer machines first, then extend to CI runners, remote dev boxes, and production-adjacent automation hosts. The attached brief explicitly frames the threat surface as broader than a single IDE process.[cite:1]

### FR-13: Central policy management
Admins must be able to define organization-level policies and distribute them across teams and supported agent environments. The attached brief lists org-level policy distribution as one promising depth area, and it aligns with enterprise rollout needs.[cite:1]

### FR-14: Secrets-aware controls
Enforcer must detect and govern access to local credentials, environment variables, and configured secret stores where technically feasible. At minimum, the audit trail must record secret-access attempts and policy outcomes even when the actual secret value is redacted.[cite:1]

## UX requirements
### FR-15: Native-feeling approvals
Approval UX must minimize developer disruption. If an agent proposes a file edit inside VS Code, the product should piggyback on or augment that review moment rather than forcing users into a separate console for every safe operation.[cite:37][cite:43]

### FR-16: Security console
Enforcer must provide a web console for security and platform teams to:
- review sessions,
- search audit logs,
- inspect blocked actions,
- manage policies,
- configure approval rules,
- and export evidence for investigation or compliance review.[cite:1]

### FR-17: Developer transparency
When Enforcer blocks or gates an action, it must provide a clear reason, the rule matched, and the next available path such as request approval, retry in allowed scope, or open policy exception workflow. Silent failures will erode trust and reduce adoption.

## Non-functional requirements
### NFR-1: Performance
Policy checks for common file and shell actions should add minimal interactive latency and should not noticeably degrade the coding workflow. The MVP target is that most checks complete fast enough to feel immediate in local development use.

### NFR-2: Reliability
If the Enforcer UI is unavailable, the policy engine must fail according to configurable policy mode: fail closed for high-risk enterprise environments and fail open only where explicitly configured.

### NFR-3: Security
Audit records must be tamper-evident or tamper-resistant, and sensitive values must be redacted or hashed where appropriate.

### NFR-4: Privacy
Prompt capture and content capture should be configurable by policy because some organizations will permit action metadata logging but restrict full prompt retention.

### NFR-5: Extensibility
The interception architecture must be designed so additional agent environments can be added without re-architecting the entire policy model.

## MVP scope
The MVP should include:
- VS Code extension or companion integration,
- Claude Code support,
- Cursor MCP and permission-aware support,
- Codex IDE support for VS Code and VS Code forks,
- centralized policy engine with allow, deny, and approval actions,
- audit logging for file, shell, and network events,
- approval workflow for high-risk actions,
- and a security review console.[cite:1][cite:29][cite:30][cite:35][cite:37][cite:41][cite:43]

The MVP should not promise universal deep enforcement for every cloud-executed agent task. Where an action happens outside the local or governed runtime, the product should surface visibility limitations explicitly rather than imply false completeness.[cite:35][cite:41]

## Success metrics
- Number of pilot customers able to approve at least one coding-agent deployment after installing Enforcer.
- Percentage of governed sessions with complete audit traces for file, shell, and network actions.
- Median time to approve or deny a gated action.
- Reduction in security-review cycle time for agent rollout.
- Number of supported IDE and agent combinations achieving production-ready status in the compatibility matrix.

## Risks and dependencies
- Agent vendors expose different integration points and may change them frequently, which creates maintenance risk across extension, CLI, MCP, and cloud task surfaces.[cite:29][cite:35][cite:37]
- Some delegated or cloud-executed actions may be partially observable rather than fully enforceable, especially when execution leaves the governed local environment.[cite:35][cite:41]
- Approval UX must balance security rigor with developer experience, especially in IDE-driven workflows where repeated prompts can quickly cause fatigue.[cite:37][cite:43]

## Open questions
- Which integration surface should be built first for strongest enterprise traction: VS Code extension, CLI shim, MCP gateway, or all three in parallel?
- How should Enforcer represent partially observable cloud tasks in audit reports?
- What is the minimum viable secrets-governance feature set for launch?
- Which SIEM, ticketing, and IAM integrations are most important for the first ten enterprise customers?

## Recommended sequencing
Phase 1 should focus on local runtime interception, VS Code integration, Claude Code support, Cursor MCP support, and Codex IDE support. These surfaces match current developer behavior and provide the clearest path to enterprise design-partner pilots.[cite:29][cite:35][cite:37][cite:41][cite:43]

Phase 2 should expand into organization-wide policy distribution, CI and remote runtime support, anomaly detection, and integrations with enterprise security systems. This sequencing follows the attached brief’s wedge: unblock agent approval first, then broaden into a fuller runtime governance platform.[cite:1]
