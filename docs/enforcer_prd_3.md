> Author: Deepankar Das

# Enforcer PRD

## Executive Summary

Enforcer is a mandatory security and policy control plane for AI coding agents operating in developer environments. It sits between the agent and the systems the agent can affect, intercepting actions, evaluating them against organizational governance policy and developer-local guardrails, enforcing allow, deny, and approval decisions, and producing security-grade audit trails that make agent activity reviewable and governable.[cite:1]

The product thesis is that AI coding agents are gaining access to source code, shells, package managers, network paths, secrets, APIs, databases, MCP endpoints, and model context, while most organizations still rely on security controls designed for human developers rather than autonomous systems acting at machine speed.[cite:1] That gap creates a category need for an agent-specific enforcement layer that can make AI-assisted development safe enough for broad enterprise rollout.[cite:1]

Containers are a critical part of this story, but not the whole story. Properly configured containers provide strong isolation through namespaces, resource controls through cgroups, reduced privilege through capability dropping, and additional hardening options such as user namespaces, AppArmor, and SELinux; however, Docker’s own guidance also makes clear that container security depends heavily on configuration profile, daemon exposure, and the surrounding system architecture.[cite:18][cite:19][cite:2] gVisor goes further by intercepting system calls through a user-space kernel to reduce host kernel exposure, while also warning that a sandbox is not a substitute for a secure architecture.[cite:3]

## Product Vision

Enforcer’s vision is to become the default trust boundary for agentic software development: the layer every enterprise inserts between AI coding agents and the environments, tools, protocols, and data those agents touch.[cite:1] In the same way identity providers became mandatory for SaaS access and endpoint agents became mandatory for device governance, Enforcer aims to become mandatory infrastructure for safe software agents.[cite:1]

The long-term ambition is a unified control plane spanning local developer machines, remote workspaces, secure containers, MCP ecosystems, agent-to-agent workflows, network egress, secrets access, database operations, and model-context data handling.[cite:1] The product should make agent actions not just visible, but policy-evaluable, replayable, attributable, and controllable in real time.[cite:1]

## Problem Statement

AI coding agents can already read and write files, execute shell commands, install packages, call APIs, access credentials, and communicate with external tools at speeds and scales that make human review impractical.[cite:1] Existing developer-security tools only partially address this because they assume a human initiates actions, understands consequences, and can be trained or warned before proceeding.[cite:1]

For organizations, the problem is much broader than file access. A coding agent can exfiltrate source code over the network, leak secrets in prompts or MCP payloads, modify dependencies, write unsafe changes to disk, query or mutate databases, communicate with other agents, and pass sensitive data across MCP clients, MCP servers, remote APIs, and LLMs.[cite:1] Without an explicit enforcement layer, security teams lack deterministic control over these flows and lack an audit trail that reflects the full action chain rather than isolated OS events.[cite:1]

The result is deployment friction. Security and platform teams often slow or block enterprise rollout of coding agents because they cannot answer basic governance questions such as what the agent can touch, which actions require approval, which hosts it can contact, whether it can access production credentials, whether it can leak customer data into model context, and what evidence exists after an incident.[cite:1]

## Why Now

AI coding agents have moved from autocomplete toward autonomous or semi-autonomous workflows that execute tasks across local machines, terminals, codebases, package ecosystems, and external tools.[cite:1] This creates a step-function increase in operational risk compared with earlier copilots that mainly generated text in-editor.[cite:1]

Enterprise interest in agentic development is also rising because buyers expect meaningful productivity gains in code generation, debugging, refactoring, testing, and integration work.[cite:1] That appetite is real, but scaled deployment expands only when security and platform leaders can impose guardrails with enough confidence and evidence.[cite:1]

The timing is favorable because the tool surface is consolidating into identifiable control points. MCP-based tool invocation, agent orchestration layers, secure development environments, and containerized workspaces create practical enforcement seams where Enforcer can insert policy without requiring a complete rebuild of the developer stack.[cite:1]

## Users and Buyers

### Primary ICP

The primary ideal customer profile is a mid-market or enterprise software organization that is actively piloting or expanding AI coding agents across engineering teams, has a meaningful security or compliance posture, and needs a way to move from limited experimentation to governed production use.[cite:1]

### Primary buyer

The primary buyer is typically one of the following:

- Security engineering lead responsible for developer and internal platform controls.[cite:1]
- Platform engineering lead responsible for developer environment standards and rollout of AI tooling.[cite:1]
- CISO or VP Security in organizations where AI adoption requires executive approval and evidentiary controls.[cite:1]

### Secondary buyers and champions

| Role | Motivation | Concerns | Relevance |
|---|---|---|---|
| Platform engineering lead | Enable broad agent rollout safely | Friction, latency, deployment burden | Technical champion [cite:1] |
| Security engineering lead | Enforce policy and reduce risk | Coverage gaps, bypass risk, auditability | Primary operational buyer [cite:1] |
| CISO / VP Security | Governance, compliance, board comfort | Mandatory control, incident response, liability | Economic sponsor [cite:1] |
| Engineering leadership | Developer productivity with guardrails | Workflow disruption, false positives | Internal stakeholder [cite:1] |
| Developer experience / AI tooling owner | Standardize internal usage | Poor UX, low adoption, fragmented policy | Power user / champion [cite:1] |

## Core Use Cases

Enforcer should support high-value use cases that directly unlock enterprise agent adoption:

- Prevent writes outside approved project boundaries, especially on local machines or shared workspaces.[cite:1]
- Deny or require approval for shell commands with destructive, persistence, or exfiltration potential.[cite:1]
- Restrict network calls to allowlisted domains, approved SaaS endpoints, and sanctioned package registries.[cite:1]
- Gate package installation, dependency upgrades, and build-script execution.[cite:1]
- Prevent access to sensitive secrets, production credentials, and customer data unless explicitly allowed.[cite:1]
- Govern MCP communication, including which tools, servers, methods, and payload types an agent may invoke.[cite:1]
- Detect and stop sensitive data flowing from codebase or database into LLM context or external tools.[cite:1]
- Produce a replayable audit trail of what the agent attempted, what policy applied, which actor approved exceptions, and what final system effects occurred.[cite:1]

## Product Scope

The first version is a mandatory interception and enforcement layer for AI coding agents in developer environments, not a generic AI observability product.[cite:1] The scope must explicitly include real-time mediation of agent actions, configurable policy decisions, structured audit logs, and at least one depth area that makes the product materially stronger than a shallow monitor-only solution.[cite:1]

### In scope for MVP

- Interception of at least two high-risk action classes: file system, shell commands, network calls, package installs, or credential and secret access.[cite:1]
- Policy actions: allow, deny, require approval.[cite:1]
- Centralized policy engine with org policy plus developer-local overlays.[cite:1]
- Structured audit logging with agent identity, action, resource, payload summary, policy result, approver, and timestamp.[cite:1]
- One differentiated depth area: approval UX, anomaly detection, redaction, multi-agent isolation, or org-level policy distribution.[cite:1]

### Out of scope for MVP

- Full CNAPP-style cloud posture management.[cite:1]
- Broad employee endpoint monitoring unrelated to AI coding agents.[cite:1]
- Universal support for every agent framework, IDE, and model provider on day one.[cite:1]

## Functional Requirements

### Action interception

The system must intercept or mediate actions across the following surfaces over time:

- File reads and writes.[cite:1]
- Shell and terminal execution.[cite:1]
- Network connections and HTTP requests.[cite:1]
- Package manager operations and dependency mutations.[cite:1]
- Credential, token, API key, and secret access.[cite:1]
- MCP client-to-server calls and payload exchange.[cite:1]
- Agent-to-agent communication and delegated tool calls.[cite:1]
- Database reads and writes where the agent can issue queries directly or through tools.[cite:1]
- Model-context submission pathways that may expose sensitive tokens or data to an LLM.[cite:1]

### Policy engine

The policy engine must:

- Support hierarchical policies: organization baseline, team policy, workspace policy, developer-local guardrails.[cite:1]
- Evaluate context including actor, agent type, session, project, file path, command, destination host, data classification, and approval state.[cite:1]
- Return decisions such as allow, deny, require approval, redact, isolate, or log-only in simulation mode.[cite:1]
- Generate human-readable rationale and machine-readable logs for each decision.[cite:1]

### Approval workflows

The system should provide:

- Real-time approval prompts for risky actions.[cite:1]
- Configurable approvers by team, repository, sensitivity level, or environment.[cite:1]
- Time-bounded approvals and reusable approval scopes where appropriate.[cite:1]
- Policy controls for break-glass access and explicit exception tracking.[cite:1]

### Audit and replay

The system must produce an audit trail meaningful to a security reviewer, not just raw system-event exhaust.[cite:1] It should preserve agent intent, requested action, transformed action, policy context, decision, approver, and observed execution result to support review, forensics, and compliance evidence.[cite:1]

## Non-Functional Requirements

- Mandatory enforcement for covered surfaces; avoid monitor-only modes masquerading as protection.[cite:1]
- Low latency so developer workflow remains usable in active coding sessions.[cite:1]
- Reliability under offline, degraded, and fail-open or fail-closed scenarios with explicit admin choice.[cite:1]
- Strong tamper resistance, with detection of bypass attempts and unsupported paths.[cite:1]
- Privacy-aware logging, including minimization and redaction of unnecessary sensitive content.[cite:1]
- Clear user messaging that explains why an action was denied or escalated.[cite:1]
- Secure deployment defaults, especially around container privileges, network policy, and daemon exposure.[cite:2][cite:4][cite:5]

## Policy and Governance Model

Enforcer’s policy model should combine enterprise governance with developer usability. Organizational policies define non-negotiable constraints such as forbidden hosts, protected credential classes, blocked filesystem regions, data handling rules, and escalation requirements.[cite:1] Developer-local guardrails allow teams or individuals to add stricter rules for a repository, branch, or workflow without overriding organization baselines.[cite:1]

A practical policy object should include:

- Subject: agent, session, user, team, environment.[cite:1]
- Action: read, write, exec, install, connect, query, invoke, prompt-submit.[cite:1]
- Resource: file path, host, secret store, database, MCP server, tool, model endpoint.[cite:1]
- Conditions: data classification, environment, destination, package source, path scope, approval context.[cite:1]
- Effect: allow, deny, require approval, redact, quarantine, simulate, alert.[cite:1]

The product should ship with opinionated default policies that reflect common enterprise concerns, such as blocking writes outside repo roots, requiring approval for package installs, denying network calls to non-allowlisted hosts, and preventing production secrets from entering model context unless explicitly approved.[cite:1]

## Architecture Options and Enforcement Model

Enforcer can be implemented through multiple interception positions. No single control point is sufficient for all risk surfaces, so the product should adopt a layered architecture from the outset.[cite:1]

| Enforcement point | Strengths | Weaknesses | Best use |
|---|---|---|---|
| Local runtime hook | High context from agent SDK/runtime; can see intent before execution | Limited if agents bypass SDK or use unmanaged tools | Fast initial integrations [cite:1] |
| Shell proxy | Strong mediation for terminal commands | Misses non-shell actions and direct syscalls | Command governance [cite:1] |
| Network proxy | Centralizes egress policy and logging | Limited semantic visibility into local file and in-memory data creation | Host allowlisting and exfil control [cite:1] |
| MCP wrapper/gateway | Protocol-aware governance for MCP tools and payloads | Only covers MCP-routed actions | Strong differentiation layer [cite:1] |
| Secure container / sandbox | Good isolation for files, processes, mounted workspace, and egress control | Not enough alone for all external systems, identity paths, and remote control planes | Controlled workspace execution [cite:1][cite:2][cite:3] |
| IDE plugin | Good UX and user feedback | Weak for mandatory enforcement by itself | Messaging and approvals [cite:1] |
| Sidecar / daemon | Persistent policy point on endpoint or workspace | Requires secure deployment and tamper resistance | Local control plane [cite:1] |
| VDI / remote workspace | Strong centralization and enterprise governance | Higher deployment friction | Regulated environments [cite:1] |
| Hybrid architecture | Broadest coverage and defense-in-depth | More complex product and deployment | Recommended long-term [cite:1] |

The recommended MVP architecture is a hybrid of agent integration, local enforcement daemon, shell and network mediation, and MCP-aware policy wrapper, with optional containerized execution for higher-assurance customers.[cite:1] This provides enough coverage to enforce meaningful controls while preserving agent context necessary for good decisioning and user experience.[cite:1]

## Secure Container Analysis

Containers are a powerful security primitive for AI agents because they create isolated, ephemeral, and tightly controlled environments that can constrain what the agent can read, write, execute, and reach over the network.[cite:2] Docker documents that containers rely on namespaces for isolation and cgroups for resource accounting and limiting, while also supporting reduced Linux capability sets, user namespaces, and integration with host hardening systems such as AppArmor and SELinux.[cite:2] These characteristics make containers a strong foundation for Enforcer’s enforcement model, especially in secure workspaces and high-assurance enterprise deployments.[cite:2]

### How containers help secure AI coding agents

#### Isolated environments

Containers provide filesystem isolation by exposing only explicitly mounted directories, which makes it possible to give an agent access to the project workspace without exposing the broader host filesystem.[cite:2] Docker also uses kernel namespaces so container processes cannot normally see or affect host processes or the network stack of other containers in the same way they could on an unrestricted host.[cite:2][cite:18]

Ephemeral containers are especially useful for AI coding agents because they reduce persistence. When the task is complete, the container can be destroyed, clearing any malicious scripts, implants, or unauthorized state the agent may have created during execution.[cite:2]

#### Least privilege and resource control

Cgroups help limit CPU, memory, and I/O consumption, which is valuable for preventing runaway or buggy agents from consuming host resources and creating denial-of-service conditions.[cite:2][cite:19] Docker also supports reduced capability profiles and recommends removing all capabilities except those explicitly required, which can sharply reduce the impact of unexpected commands even if a process becomes root inside the container.[cite:2]

Network restriction is another major advantage. Docker notes that container networking can be restricted, and gVisor explicitly recommends applying network policy controls at the container level to enforce appropriate network behavior.[cite:2][cite:3] For Enforcer, this makes containerized egress mediation a practical way to block exfiltration to unknown hosts and constrain package-install traffic to approved registries.[cite:2][cite:3]

#### Additional hardening

Read-only filesystems, reduced mounts, and non-root operation improve resilience by limiting persistence and tampering pathways inside the container.[cite:2] Docker also supports user namespaces so root inside a container can be mapped to a non-uid-0 user outside the container, which helps mitigate breakout risk.[cite:2]

For higher-assurance isolation, hardened runtimes matter. gVisor reduces host-kernel exposure by intercepting application interactions with the system API through its Sentry rather than passing system calls directly through to the host kernel.[cite:3] Kata Containers and microVM-style approaches offer stronger isolation than standard shared-kernel containers by placing workloads behind additional virtualization boundaries.[cite:26][cite:32]

### What a secure container can reliably intercept or constrain

A secure container can materially help Enforcer govern:

- Filesystem reads and writes within explicitly mounted paths.[cite:2]
- Process creation and shell execution inside the container namespace boundary.[cite:2]
- Package installation and dependency changes executed within the controlled runtime.[cite:2]
- Network traffic originating from the container, especially when routed through controlled egress policy.[cite:2][cite:3]
- Resource consumption through cgroup limits, reducing the blast radius of runaway agents.[cite:2][cite:19]
- Persistence risk when using ephemeral lifecycle and read-only or narrowly writable filesystems.[cite:2]

### What a secure container cannot fully solve alone

A container is not enough by itself to guarantee enterprise-grade mandatory enforcement across the full Enforcer threat model. Docker’s security model explicitly highlights the attack surface of the Docker daemon, loopholes in configuration profiles, and the importance of surrounding hardening controls.[cite:2] gVisor is even more direct that a sandbox is not a substitute for a secure architecture.[cite:3]

A secure container alone does not fully solve:

- MCP traffic outside the controlled runtime unless all MCP clients and servers are forced through a governed gateway.[cite:1]
- Secrets accessed through host identity systems, browser sessions, external credential brokers, or SaaS control planes outside the container boundary.[cite:1][cite:2]
- Data leakage into LLM context when prompt assembly, chat history, or tool payload construction occurs outside the container or in remote SaaS services.[cite:1]
- Agent-to-agent communication happening through external orchestrators or remote agent platforms.[cite:1]
- Database activity that occurs via remote tools or services unless those paths are separately proxied or instrumented.[cite:1]
- Host compromise risk if the deployment uses dangerous settings such as `--privileged` or mounts the Docker socket into the agent container.[cite:4][cite:5]

### Dangerous container configurations to explicitly forbid

Enforcer should treat the following as policy violations or unsupported deployments:

- Running the agent container with `--privileged`, because privileged containers can effectively disable important isolation boundaries and expose full host-root style power.[cite:4]
- Mounting `/var/run/docker.sock` into the container, because control of the Docker socket can enable creation of new containers and host-level access pathways even with seemingly limited permissions.[cite:5]
- Broad host filesystem mounts that expose `/`, sensitive home directories, or credential stores.[cite:2]
- Running as root with unnecessary capabilities instead of a reduced capability profile and non-root user.[cite:2]

### Product recommendation

The right product position is that containers are a foundational enforcement substrate for Enforcer, not the complete solution.[cite:2][cite:3] The product should support a secure-container mode that uses ephemeral workspaces, limited volume mounts, read-only root filesystems where possible, dropped capabilities, user namespaces, controlled egress, and optional hardened runtimes such as gVisor or Kata for higher-assurance isolation.[cite:2][cite:3][cite:26][cite:32]

At the same time, Enforcer must extend beyond the container boundary through protocol-aware controls, network governance, MCP mediation, secret brokerage, and model-context protection if it wants to credibly claim mandatory enforcement across the agent lifecycle.[cite:1][cite:3]

## Data Flows and Threat Surfaces

| Asset / flow | Example risk | Needed control | Preferred enforcement point |
|---|---|---|---|
| Repo files | Write outside project, read sensitive files | Path policy, content inspection, approval | Container + local daemon [cite:1][cite:2] |
| Shell | Destructive or exfiltrative commands | Command policy, allow/deny/approval | Shell proxy + daemon [cite:1] |
| Package registry | Malicious dependency install | Registry allowlist, approval, logging | Network proxy + package hook [cite:1] |
| Secrets | Token exfiltration or misuse | Secret mediation, masking, approval | Secret broker + policy engine [cite:1] |
| Network egress | Source code or PII exfiltration | Host allowlist, payload controls, traceability | Network proxy + container policy [cite:1][cite:3] |
| MCP client/server | Unsafe tool invocation or payload leakage | Protocol-aware policy and method controls | MCP gateway/wrapper [cite:1] |
| Agent-to-agent traffic | Unbounded delegation and hidden actions | Identity, isolation, chain-of-custody logging | Orchestration layer + policy engine [cite:1] |
| Database | Data extraction or mutation | Query class policy, approval, masking | DB proxy/tool wrapper [cite:1] |
| LLM context | Sensitive token or code leakage | Context DLP/redaction and route policy | Agent middleware + model gateway [cite:1] |
| Container runtime | Escape, daemon abuse, dangerous mounts | Hardened runtime, no privileged mode, no docker.sock | Runtime policy + deployment validation [cite:2][cite:4][cite:5] |

This matrix makes clear that file enforcement alone is not enough.[cite:1] The strongest product narrative is that Enforcer governs the full action chain across execution, communication, and data movement, while using containers as a containment and mediation layer for the subset of actions they are best at constraining.[cite:1][cite:2]

## Auditability and Compliance

Enforcer should position evidentiary logging as a core feature, not an afterthought. Enterprises adopting coding agents need to prove not only that policy exists, but that high-risk actions were mediated, exceptions were approved, dangerous container settings were prevented, and post-incident investigations can reconstruct what the agent attempted and what actually happened.[cite:1][cite:2]

The audit schema should include:

- Agent identity, user identity, session ID, project, environment.[cite:1]
- Requested action and translated system action.[cite:1]
- Resource touched: file, host, tool, MCP method, secret, DB target, model endpoint.[cite:1]
- Container context: image, runtime, privilege mode, mounts, capability set, network policy, namespace mode.[cite:2]
- Data classification markers and redaction flags.[cite:1]
- Policy evaluated, effect returned, approver, reason, and timestamp.[cite:1]
- Observed result, hashes, destination metadata, and replay linkage where possible.[cite:1]

## MVP Definition

The MVP should optimize for depth over breadth, consistent with the attached product brief.[cite:1] The recommended MVP is:

1. Intercept file writes and reads, shell commands, and network egress in a controlled developer environment.[cite:1]
2. Enforce allow, deny, and require-approval policies with org policy plus local developer guardrails.[cite:1]
3. Offer a secure-container execution mode with opinionated hardening defaults: ephemeral workspace, limited mounts, reduced capabilities, no privileged mode, and no Docker socket exposure.[cite:2][cite:4][cite:5]
4. Provide one differentiated depth capability: MCP-aware governance or human-in-the-loop approval UX.[cite:1]
5. Produce structured audit logs that security reviewers can inspect and filter by agent, action, resource, decision, and runtime posture.[cite:1][cite:2]

### Recommended MVP feature priority

| Priority | Feature | Why it matters |
|---|---|---|
| P0 | File, shell, and network interception | Covers the clearest enterprise risk surface [cite:1] |
| P0 | Policy engine with allow/deny/approval | Establishes mandatory control [cite:1] |
| P0 | Secure-container hardening profile | Makes containment concrete and deployment-safe [cite:2][cite:4] |
| P0 | Audit trail and reviewer console | Makes the product credible to security teams [cite:1] |
| P1 | MCP governance layer | Differentiates beyond legacy endpoint tools [cite:1] |
| P1 | Approval UX | Supports safe rollout without blocking all progress [cite:1] |
| P2 | Secrets/PII redaction in context | Important but can follow core control plane [cite:1] |
| P2 | Multi-agent isolation / anomaly detection | High upside, but secondary for initial wedge [cite:1] |

## Phased Roadmap

### Phase 1: Controlled enforcement wedge

- Support one or two major coding-agent environments.[cite:1]
- Deliver mandatory control over file, shell, and network actions.[cite:1]
- Ship secure-container execution with hardened defaults and deployment validation.[cite:2][cite:4][cite:5]
- Ship audit console and approval workflow.[cite:1]
- Prove value in pilot deployments with platform and security teams.[cite:1]

### Phase 2: Protocol-aware expansion

- Add MCP gateway features and method-level policy.[cite:1]
- Add secrets mediation and prompt/context redaction.[cite:1]
- Expand policy distribution and repository or team scoping.[cite:1]
- Integrate with SIEM/SOAR and ticketing systems.[cite:1]
- Support hardened runtimes such as gVisor and Kata for higher-assurance customers.[cite:3][cite:26][cite:32]

### Phase 3: Enterprise control plane

- Support agent-to-agent governance and delegation chains.[cite:1]
- Add database-aware controls and model-routing policy.[cite:1]
- Expand to remote workspaces, CI agents, and regulated environments.[cite:1]
- Build policy simulation, replay, and incident response workflows.[cite:1]
- Add posture checks for container runtime security, daemon exposure, image trust, and environment drift.[cite:2]

## Market Appetite

Market appetite is driven by a clear tension: enterprises want the productivity gains of AI coding agents, but security and compliance teams need stronger guarantees before allowing those agents to operate broadly across source code, credentials, infrastructure, and internal systems.[cite:1] The demand is strongest in organizations already experimenting with coding agents and encountering internal friction from security review, platform standardization, or compliance risk.[cite:1]

The product’s wedge is attractive because it solves a blocking problem rather than an optimization problem.[cite:1] Buyers can justify spend if Enforcer allows them to move from a constrained pilot to a sanctioned enterprise rollout of AI-assisted development, especially where one blocked security review can delay adoption across hundreds or thousands of developers.[cite:1]

### Recommended positioning statement

Enforcer is the mandatory enforcement and governance layer for AI coding agents, giving enterprises real-time control over what agents can read, write, execute, call, and disclose across developer environments, secure containers, MCP tools, networks, and model workflows.[cite:1][cite:2]

### Recommended wedge GTM motion

- Land with platform engineering and security on a pilot tied to one coding-agent rollout.[cite:1]
- Prove mandatory controls on a narrow but painful risk surface, such as shell, network, package, and workspace governance.[cite:1]
- Use secure-container deployment as the fastest path to high-assurance pilots where local endpoint trust is limited.[cite:2]
- Expand from one team to organization-wide policy distribution and audit reporting.[cite:1]
- Upsell into broader MCP governance, secrets protection, and multi-agent control.[cite:1]

## TAM / SAM / SOM

The following market model uses a bottom-up methodology based on paid developer seats in mid-market and enterprise organizations likely to deploy coding agents, adjusted by security-governance attach rate.

### Assumptions

- Target monetization unit: protected developer or agent seat under governance.[cite:1]
- Plausible annual price range: $300 to $1,200 per governed seat depending on enforcement depth, compliance features, and deployment model.[cite:1]
- Initial adoption is strongest in organizations with at least 100 developers and active AI coding-agent experimentation.[cite:1]

### TAM

A practical total addressable market assumes a global pool of enterprise and mid-market developer seats that will eventually use coding agents and require governance. If 10 million developer seats globally become plausible long-term governed agent seats and the average annual contract value per seat is $500, TAM is approximately $5.0 billion annually.[cite:1]

### SAM

A more realistic serviceable available market focuses on organizations in North America and Europe with meaningful security programs and active coding-agent deployment. If 1.5 million seats fall into that reachable segment and attach at $500 per seat annually, SAM is approximately $750 million annually.[cite:1]

### SOM

A plausible near-term serviceable obtainable market for the first several years could be 40,000 to 80,000 seats across early-adopter mid-market and enterprise customers. At $500 per seat annually, SOM ranges from $20 million to $40 million in annual recurring revenue potential.[cite:1]

## Competitive Landscape

Enforcer competes across direct, adjacent, and future-platform categories rather than against one narrowly defined incumbent set.[cite:1]

| Category | What they do well | Likely weakness vs. Enforcer | Threat type |
|---|---|---|---|
| AI gateway / LLM firewall vendors | Model routing, prompt inspection, policy at API boundary | Weak machine-action enforcement and limited endpoint/MCP context | Adjacent [cite:1] |
| Endpoint / developer security tools | Strong device visibility, process and filesystem monitoring | Built for humans, not agent intent or MCP workflows | Substitute / adjacent [cite:1] |
| Secrets management vendors | Secret lifecycle and access control | Do not govern full agent action chain | Adjacent [cite:1] |
| CNAPP / DSPM / DLP vendors | Data and cloud governance | Often too broad and not embedded in coding-agent execution path | Adjacent [cite:1] |
| IDE / agent platform vendors | Native UX and deep product context | Incentive to optimize adoption, not neutral enforcement; limited cross-platform control | Future platform threat [cite:1] |
| Sandbox / remote workspace vendors | Strong environment isolation | Usually not full policy/governance products for agent actions | Adjacent / partner [cite:1] |
| Open-source policy tooling | Flexible and low-cost primitives | Fragmented, high integration burden, weaker enterprise UX | Substitute [cite:1] |

### Competitive conclusion

The most defensible position is not “another AI security dashboard,” but “the control plane where agent intent meets mandatory enterprise policy.”[cite:1] Enforcer wins when it provides deeper enforcement, better protocol awareness, stronger auditability, and lower operational friction than stitching together endpoint tools, network controls, model gateways, container runtimes, and custom policy scripts.[cite:1][cite:2]

## Differentiation and Best-in-Class Strategy

### Table-stakes capabilities

- File, shell, and network interception.[cite:1]
- Policy engine with allow/deny/approval.[cite:1]
- Structured audit logs.[cite:1]
- Secure-container deployment profile with safe defaults.[cite:2][cite:4]
- Basic admin console and reviewer workflows.[cite:1]

### Differentiated capabilities

- MCP-native governance and payload-aware controls.[cite:1]
- Unified policy over agent actions, data flows, and model-context exposure.[cite:1]
- Blending enterprise baselines with developer-local guardrails.[cite:1]
- Full-chain replay from agent intent to system effect.[cite:1]
- Secure-container posture validation that forbids dangerous runtime patterns like `--privileged` and Docker socket mounting.[cite:4][cite:5]
- Optional hardened runtime support using gVisor or Kata for higher-assurance workloads.[cite:3][cite:26][cite:32]

### Durable moats

- Deep integrations across agent runtimes, MCP ecosystems, and secure dev environments.[cite:1]
- Proprietary policy corpus and action-risk ontology tuned for agent workflows.[cite:1]
- High-quality audit data and forensics workflows that become embedded in enterprise processes.[cite:1]
- Trust brand around enforceability, low-friction deployment, and strong secure-runtime support.[cite:1][cite:2]

### Design principles

- Mandatory where promised; never oversell observability as enforcement.[cite:1]
- Coverage of action chains, not isolated events.[cite:1]
- Protocol-aware by default, especially for MCP.[cite:1]
- Container-first for containment, but not container-only for governance.[cite:2][cite:3]
- Enterprise-governable, developer-tolerable.[cite:1]
- Explain every policy outcome clearly.[cite:1]

## Pricing and Packaging Hypotheses

The best initial pricing model is likely seat-based with platform minimums, because buyers will map value to protected developers and sanctioned agent rollout.[cite:1] Packaging can expand with higher tiers for compliance evidence, advanced approvals, MCP governance, secrets redaction, remote workspace enforcement, and hardened runtime support.[cite:1][cite:3]

### Candidate packaging

- Team: basic local enforcement, core policy engine, audit logs.[cite:1]
- Business: centralized policy distribution, approval workflows, SIEM integration, containerized workspace enforcement, better admin controls.[cite:1][cite:2]
- Enterprise: MCP governance, secrets/context protection, advanced forensics, remote workspace support, hardened runtimes, compliance mapping, premium support.[cite:1][cite:3]

## Key Risks and Open Questions

- Can coverage remain mandatory and tamper-resistant on unmanaged laptops, or is the product strongest in controlled workspaces and secure containers?[cite:1][cite:2]
- How much enforcement can be delivered without causing developers to bypass the system?[cite:1]
- Which integration path delivers fastest time-to-value: agent SDKs, shell/network controls, or secure-container deployment?[cite:1][cite:2]
- Will major IDE or agent vendors embed enough native controls to commoditize parts of the wedge?[cite:1]
- How much of the market wants policy simulation before hard enforcement?[cite:1]
- Which initial differentiated area creates the strongest pull: MCP governance, approval UX, or secure-container posture and secrets protection?[cite:1]

## Success Metrics

### Product metrics

- Percentage of covered agent actions mediated by policy.[cite:1]
- Mean policy decision latency.[cite:1]
- Number of blocked, approved, and logged-only events by class.[cite:1]
- Coverage of MCP tools, hosts, and command classes under governance.[cite:1]
- Percentage of deployments passing secure-container posture validation.[cite:2][cite:4][cite:5]

### Business metrics

- Pilot-to-production conversion rate.[cite:1]
- Number of protected developer seats per customer.[cite:1]
- Expansion from one team to org-wide rollout.[cite:1]
- Security-review cycle time reduced for coding-agent deployment.[cite:1]

### Trust metrics

- Audit completeness rate for mediated actions.[cite:1]
- False-positive and false-block rates.[cite:1]
- Number of bypass attempts detected.[cite:1]
- Time to investigate a policy incident using product logs and replay artifacts.[cite:1]
- Number of blocked dangerous runtime configurations such as privileged mode and Docker socket mounts.[cite:4][cite:5]

## Appendix: Assumptions

- The product category for AI coding-agent governance will expand materially as autonomous tooling adoption rises.[cite:1]
- Mid-market and enterprise buyers will pay a premium for mandatory control over a high-risk but high-value productivity layer.[cite:1]
- A hybrid architecture is necessary for meaningful enterprise coverage because no single container, proxy, or plugin can govern all relevant data and action flows.[cite:1][cite:3]
- Containers provide a strong safety foundation through namespaces, cgroups, capability restriction, and workspace isolation, but their security value depends heavily on safe runtime configuration and surrounding controls.[cite:2]
- The strongest initial wedge is enabling safe rollout of coding agents by satisfying security and compliance blockers rather than selling generic productivity or observability benefits.[cite:1]
