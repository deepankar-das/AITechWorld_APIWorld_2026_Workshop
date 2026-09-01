> Author: Deepankar Das

# AA_Firewall PRD

## Executive summary

AA_Firewall is a security and policy enforcement layer for AI coding agents operating in developer environments. It sits between an agent and the systems that agent can touch—such as files, shells, networks, packages, credentials, and MCP-connected tools—to make agent actions governable, reviewable, and enforceable rather than merely observable.[file:1][web:7][web:13]

The market window is timely because AI coding is moving from assistant workflows to autonomous and long-running agent workflows, while enterprise oversight still lags behind adoption. Anthropic’s 2026 report argues that coding agents are moving into production systems, human oversight remains necessary for high-stakes work, and security-first architecture is becoming a priority as agentic coding expands across teams and tasks.[page:1] AA_Firewall’s wedge is therefore not “better code generation,” but enabling safe organizational rollout of coding agents by making actions policy-compliant, auditable, and bounded by mandatory guardrails.[file:1][page:1]

## Problem statement

AI coding agents now read and write files, execute shell commands, install packages, access external services, and interact with sensitive repositories and credentials. The attached product brief frames the core gap clearly: most teams lack visibility into what these agents actually do, lack an enforceable policy layer, and lack audit trails that security teams can use for real review.[file:1]

Existing enterprise controls are fragmented for this new workflow. IDE vendors and coding agents increasingly expose approvals, admin controls, privacy settings, and allow/deny lists, but those controls are vendor-specific and generally do not provide a unified, cross-agent, organization-wide control plane for agent actions across the full developer environment.[web:7][page:0] Traditional endpoint, IAM, and developer security tools were built around human users and app workloads; they do not model the sequence-level behavior of semi-autonomous agents acting at machine speed across multiple tools and permissions.[web:9][page:1]

### Stakeholder pain

| Stakeholder | Pain today | Why existing tools fall short |
|---|---|---|
| Security engineering lead | Cannot answer what agents did, what they touched, or whether actions complied with policy.[file:1][web:16] | Logs are scattered across IDEs, shells, CI, and cloud tools; there is no unified policy and evidence layer for agent action sequences.[file:1][web:13] |
| Platform engineering lead | Wants to enable agents safely without breaking developer workflows or creating support burden.[file:1] | Current controls are either too weak, too vendor-specific, or too disruptive to developer environments.[web:7][web:13] |
| Engineering leadership | Sees productivity upside but worries about unsafe autonomy, policy drift, and compliance exposure.[page:1][web:9] | Existing products focus on productivity or generic security posture rather than enforceable runtime governance for agent actions.[file:1][web:3] |
| Developers | Want fast autonomy for routine tasks, but not arbitrary blocking, repetitive approvals, or opaque failures.[web:7][page:1] | Existing approval models are usually coarse and local to one tool, not adaptive to team policy and context.[web:7][web:16] |

## Users and buyers

The primary economic buyer for the initial product should be the security engineering lead or platform engineering lead in a mid-market or enterprise software organization with active AI coding rollout plans. The attached brief explicitly points to this buyer set, including security engineering leaders, platform leads, and CISOs at mid-market developer organizations.[file:1]

The primary daily operator will usually be a platform/security administrator who defines policies, approves exceptions, and reviews evidence. The primary end user is the developer using AI coding agents in tools such as terminal-based agents, IDE agents, and MCP-connected workflows, while secondary stakeholders include compliance, infrastructure, developer productivity, and engineering management teams.[web:7][web:13][page:1]

### Initial ICP

The best initial ICP is a 300–5,000 engineer organization in a regulated or governance-conscious sector, or any software company with platform maturity, centralized identity, internal repositories, and visible pressure to deploy coding agents broadly. Cursor highlights enterprise controls such as SSO, SCIM, repo/model/MCP allowlists, analytics, and global agent settings, which indicates that larger organizations are already demanding centralized governance features around coding agents.[page:0]

AA_Firewall is strongest where the customer has all three conditions: meaningful coding-agent usage, real security review requirements, and a willingness to standardize developer environment controls. Companies with only light ad hoc usage may not yet feel enough pain, while very small teams may tolerate manual approvals and trust-based workflows for longer.[file:1][page:1]

## Market appetite

Market appetite is emerging and increasingly urgent rather than merely theoretical. Anthropic’s 2026 trends report describes coding agents as moving from experimentation to production, predicts longer-running and multi-agent workflows, and emphasizes that organizations need to scale human oversight without creating bottlenecks.[page:1] Independent enterprise security commentary similarly argues that AI deployment is outpacing governance and that organizations are adopting AI systems without equivalent control maturity.[web:2][web:3]

This creates a category-level control problem. The shift is not just “developers use autocomplete more often”; it is that agents can now take actions on systems, and the security model must change when humans are no longer the only direct actors. Forbes notes that agentic AI challenges the assumption that humans are the primary decision-makers in enterprise systems, and specifically highlights the need to constrain authority and require consent for sensitive actions.[web:9]

### Why now

- Coding agents are taking broader action across the SDLC, including testing, debugging, implementation, and documentation, often in long-running workflows.[page:1]
- Enterprises are already buying governance features from coding-tool vendors, such as model controls, repo controls, MCP controls, and org-wide settings.[page:0]
- Current controls remain fragmented by vendor and workflow, leaving a gap for an independent enforcement and evidence layer.[web:7][web:13]
- The strongest blocker to broader rollout is not interest; it is trust, policy, and auditability.[file:1][web:2]

## Market sizing

The market should be defined as “enterprise software for governing and enforcing AI coding agent actions in developer environments,” not the entire AI developer tools market. Because this category is nascent, bottom-up assumptions are more credible than top-down claims.[file:1][page:1]

### Assumptions

- Pricing model: $20,000–$75,000 annual platform fee plus $15–$40 per governed developer seat per month, depending on deployment model, controls, and compliance requirements.
- Initial target customers: companies with 200+ developers and active rollout of AI coding agents.
- Reachable value is driven less by all developers and more by high-governance teams where enforcement matters: platform, backend, infra, security-adjacent, and regulated teams.
- The broader trend base is expanding quickly; Cursor claims 50,000+ enterprises use its platform and that 64% of Fortune 500 companies are using Cursor.[page:0]

### Market ranges

| Metric | Definition | Logic | Range |
|---|---|---|---|
| TAM | Global spend potential for AI coding-agent governance and enforcement across enterprise and mid-market software orgs | Assume 25,000–60,000 targetable organizations globally with meaningful AI coding adoption, each worth roughly $40,000–$250,000 ARR depending on seat count and governance depth. | Approximately $1.0B–$8.0B |
| SAM | Near-term serviceable market: North America and Europe, software-forward mid-market and enterprise dev orgs with centralized governance and immediate agent rollout | Assume 5,000–12,000 organizations in the next 3–5 years with enough AI coding usage and governance maturity, at $50,000–$180,000 ARR. | Approximately $250M–$2.2B |
| SOM | Realistic 3-year attainable market for a startup wedge product | Assume 75–250 customers in 3 years, average ARR $60,000–$150,000, depending on expansion. | Approximately $4.5M–$37.5M |

These ranges are intentionally broad because the category is forming in real time. The strategic point is that AA_Firewall can become a meaningful venture-scale product if it becomes the control plane for agent actions, not just a point feature added to one coding assistant.[page:0][page:1]

## Competitive landscape

Competition will come from adjacent categories before it comes from obvious direct clones. The product has overlap with coding-assistant vendors, sandboxing providers, AI gateways, PAM/identity tools, and developer security platforms, but no single category fully solves cross-agent runtime governance in developer environments.[web:7][web:11][web:13]

### Competitive map

| Category | Example signals | Overlap | Why insufficient alone | What AA_Firewall must do to win |
|---|---|---|---|---|
| Coding-agent vendors | Cursor enterprise controls, Claude Code approval modes and admin restrictions.[web:7][page:0][web:16] | Native controls, approvals, usage analytics | Controls are tool-specific, not a neutral cross-agent policy layer; customers may use multiple agents and MCP tools simultaneously.[web:7][page:0] | Become the independent enforcement and evidence layer across agents, IDEs, shells, and runtimes. |
| Secure sandbox / execution vendors | E2B, Daytona, microVM and container sandboxes.[web:11][web:14] | Isolation substrate, execution control, environment mediation | Sandboxes help constrain execution but do not, by themselves, define enterprise policy, approval logic, audit semantics, or organization-wide governance.[web:11][web:14] | Pair substrate-level isolation with policy, identity, approvals, and evidence. |
| IAM / PAM / secrets tools | Existing identity, secret vault, privileged access vendors | Control credentials and access boundaries | They govern identities and secrets, not the semantic sequence of agent actions in dev workflows. | Use them as enforcement inputs and integration points, not replacements. |
| AI gateways / observability / policy tools | LLM gateways, prompt logging, policy scanners | Prompt/tool observability and model governance | Often focus on model calls and content policy, not file writes, shell actions, package installs, or repo-bound enforcement in coding environments. | Own the action layer, not just the model-call layer. |
| DevSecOps / code security vendors | SAST, code review, CI security, repository scanning | Post-action detection and prevention in software lifecycle | Most controls trigger after code is created or committed, not at the moment an autonomous agent acts. | Shift left to real-time interception, approvals, and blast-radius control. |

### Competitive conclusion

The largest medium-term threat is bundling by incumbent coding-assistant vendors that already offer some approvals, analytics, and admin controls. However, that same trend validates demand and may increase the need for an agent-neutral policy layer, especially in enterprises that mix vendors, require independent controls, or want stronger enforcement than an IDE-native settings panel can provide.[web:7][page:0][web:16]

## Product vision

AA_Firewall’s long-term vision is to become the control plane for AI agent operations in software development environments. In the wedge phase, it governs coding agents. In the expansion phase, it governs multi-agent developer workflows across CI, infrastructure, and internal tools. In the platform phase, it becomes the system of record for agent identity, policy, approvals, evidence, and bounded autonomy across engineering organizations.[file:1][page:1]

### Horizon roadmap

| Horizon | Goal | Outcome |
|---|---|---|
| Horizon 1: Wedge | Govern coding-agent actions in local/remote developer environments | Safe rollout of coding agents with real policy enforcement and auditability |
| Horizon 2: Expansion | Extend to multi-agent workflows, CI/CD, issue trackers, MCP tools, and environment classes | Unified governance across the broader software delivery workflow |
| Horizon 3: Platform | Become enterprise control plane for agent identity, policy distribution, risk scoring, and evidence | Standard system-of-record for agent operations and compliance |

## Product principles

- Enforcement before analytics: observability matters, but the wedge is preventing unsafe or non-compliant actions, not merely recording them.[file:1]
- Defense in depth: no single control layer is sufficient for enterprise trust, especially as agents operate across tools and execution surfaces.[web:11][web:14]
- Low-friction developer experience: approvals and blocks should be exception-based and context-aware so that safe work stays fast.[web:7][page:1]
- Policy should map to real org structure: rules should be scoped by org, team, repository, environment, agent, and user role.[file:1][page:0]
- Auditability must be reviewer-friendly: logs should explain intent, action, target, policy decision, identity, and evidence chain in plain language.[file:1]
- Integrate where developers already work: terminal agents, IDE agents, MCP servers, containers, and remote workspaces must all be first-class.
- Trust must be independently defensible: enterprise buyers will prefer controls that are not solely dependent on vendor-native goodwill or local client-side settings.[web:9][page:0]

## Jobs to be done

### Security engineering lead

“When AI coding agents act on developer machines and repositories, there must be a provable policy layer that shows what they did, blocks disallowed actions, and generates evidence suitable for review and compliance.”[file:1][web:16]

### Platform engineering lead

“Enable coding agents for developers without turning the platform team into a bottleneck, and without introducing uncontrolled shell, package, network, or secret exposure.”[file:1][web:9]

### Developer

“Let the agent move quickly on routine tasks, but keep guardrails predictable, explain policy decisions clearly, and minimize pointless approvals.”[web:7][page:1]

### Approver or reviewer

“Surface only the actions that genuinely need attention, with enough context to approve or deny quickly and confidently.”[file:1][page:1]

### Executive buyer

“Roll out coding agents broadly with confidence that autonomy is bounded, risk is auditable, and governance is not left to ad hoc trust.”[file:1][web:2]

## Product requirements

### Functional requirements

1. **Action interception**: intercept at least file reads/writes, shell command execution, network calls, package installation, and secret/credential access where technically possible, with MVP requiring strong coverage on at least two of these surfaces.[file:1]
2. **Action normalization**: normalize raw events into a common event schema with actor, agent, user, repo, environment, tool, resource target, action type, timestamp, policy context, and decision.[file:1]
3. **Policy engine**: support allow, deny, and require-approval outcomes, plus rule scoping by user, team, repo, environment, agent, path, host, command class, secret class, and time.[file:1]
4. **Approval workflow**: real-time human-in-the-loop approvals with clear rationale, expiry, fallback behavior, and policy-based routing to the right approver.[file:1][page:1]
5. **Audit log and replay**: generate structured, reviewer-friendly logs and timeline replay of action sequences, not just raw command traces.[file:1][web:16]
6. **Secrets handling**: detect and govern secret access paths, integrate with vault or broker patterns where possible, and redact sensitive values from prompts, logs, or review surfaces when required.[file:1]
7. **Policy management**: support global policies, org-level bundles, repo-level overrides, and dry-run/simulation mode before hard enforcement.[file:1]
8. **Agent and session identity**: tie actions to a specific agent session, model, user sponsor, workspace, and environment class for accountability.[page:0][web:16]
9. **Integration layer**: support terminal agents, IDE agents, MCP-mediated actions, and remote/ephemeral development environments.[web:7][page:0]
10. **Admin console**: provide policy authoring, approval queues, audit review, risk reporting, and deployment configuration.
11. **Developer UX**: explain decisions locally and quickly, with low-latency prompts for approvals and transparent reason codes.
12. **Evidence and reporting**: export evidence for incidents, governance reviews, and compliance workflows.

### Non-functional requirements

- **Latency**: enforcement should add minimal overhead to local/remote developer workflows; approval-free safe actions should feel near real time.
- **Reliability**: policy enforcement and logging must fail safely, with clear degraded-mode behavior.
- **Tamper resistance**: controls must be difficult for developers or agents to disable without authorization.
- **Low false positives**: excessive blocking will kill adoption; policy simulation and graduated rollout are essential.[page:1]
- **Privacy**: logs and review interfaces must avoid unnecessary capture of sensitive code or secrets.
- **Enterprise readiness**: SSO, SCIM, RBAC, deployment options, APIs, and exportability will be expected by serious buyers.[page:0]
- **Scalability**: support many sessions, agents, and events across distributed engineering organizations.
- **Portability**: the architecture should not depend on one IDE or one model vendor.

## Architecture and enforcement model

The core product question is where the interception layer should sit. The attached brief specifically calls out options such as sandbox, proxy, runtime hook, and MCP wrapper, and asks whether a secure container can help enforce guardrails in a mandatory manner.[file:1]

### Architecture options

| Option | What it can see | What it can block | Bypass risk | Friction | Assessment |
|---|---|---|---|---|---|
| Secure container / dev container | Files in mounted workspace, process execution in container, package installs, outbound network from container, environment variables injected into container.[web:11][web:14] | Strong control over filesystem scope, shell/process execution, package installation, and egress from inside the container.[web:11][web:14] | Medium if developers or agents can escape the container, use host-side tools, or access unmanaged paths; lower with managed remote workspaces or microVMs.[web:11] | Moderate | Strong MVP substrate for mandatory controls if the workflow is forced through the environment. |
| MicroVM sandbox | Similar to container, but with stronger isolation due to dedicated kernel per execution.[web:11] | Stronger isolation and blast-radius control than containers.[web:11] | Lower than containers for kernel-level isolation, but still limited if workflows depend on external unmanaged tools or identities.[web:11] | Moderate to high | Best for high-assurance execution, but may increase cost and integration complexity. |
| Proxy / egress proxy | Network traffic, package registry access, API calls through the proxy | Network destinations, package download policy, some content scanning | High for local actions outside proxy path | Low to moderate | Useful layer, but insufficient alone because it cannot govern local filesystem or shell behavior. |
| Runtime hook / syscall or process instrumentation | Process execution, file/network syscalls depending on depth of instrumentation | Potentially strong, especially for local shell/file controls | Medium to low if deeply integrated; complexity is high | High | Powerful but harder to ship cross-platform and maintain. |
| MCP wrapper / protocol mediation | Tool calls routed through MCP, tool parameters, tool outputs | Strong for tools that the agent accesses through MCP | High if agent can bypass MCP and act directly through shell/editor/local tools | Low to moderate | Good abstraction layer, but not enough for mandatory enterprise enforcement by itself. |
| IDE plugin / extension | Agent actions inside the editor, some file and command requests, UX prompts | Partial controls inside one IDE | High outside that IDE or when terminal/remote tools are used | Low | Useful as user experience and telemetry surface, not primary enforcement root. |
| Agent SDK integration | High semantic understanding of planned actions | Can stop or require approval before SDK tool use | High if customers use multiple agents or unintegrated tools | Low | Valuable for semantics and quick integrations, but weak as sole trust anchor. |

### Secure container analysis

A secure container can meaningfully help intercept and enforce mandatory guardrails for a large share of coding-agent actions, especially filesystem scope, package installation, process execution, environment injection, and outbound network restrictions inside the governed environment.[web:11][web:14] This makes it a strong enforcement substrate for AA_Firewall, particularly when the customer workflow routes coding-agent activity through managed dev containers, remote workspaces, or ephemeral execution environments.[web:11]

However, a secure container is not sufficient by itself for enterprise-grade mandatory guardrails. Containers typically share the host kernel, can leave gaps if developers still have unmanaged host access, and do not automatically solve identity binding, approval semantics, cross-tool audit normalization, or policy reasoning across MCP tools, IDEs, shells, and secrets systems.[web:11][file:1] For higher-assurance cases, microVM-style isolation offers stronger boundaries than standard containers, but even microVMs still need surrounding policy, identity, and evidence layers.[web:11][web:14]

### Recommended architecture thesis

The best MVP architecture is a layered model:

1. **Primary enforcement substrate**: managed secure container or remote dev environment that the coding agent must run inside.
2. **Policy and event layer**: AA_Firewall policy engine and normalized event schema governing filesystem, process, package, and network actions.
3. **Protocol mediation**: MCP wrapper and agent integrations to add semantic context, pre-execution approvals, and tool-aware reasoning.
4. **Identity and secret controls**: bind actions to user, agent, session, and ephemeral credentials; avoid long-lived secret exposure.
5. **Evidence plane**: structured audit logs, sequence replay, and exportable compliance artifacts.

This layered approach balances mandatory enforcement with practical deployment. Containers or microVMs provide the hard boundary; MCP wrappers and SDK integrations provide semantic insight; policy and audit layers provide enterprise value; identity and secret controls close the governance loop.[file:1][web:11][web:14]

## Differentiated feature set

### Table stakes

- Cross-surface action interception for file, shell, network, package, and secret-related events.[file:1]
- Policy decisions of allow, deny, and require approval.[file:1]
- Structured audit logs with actor, action, resource, rationale, and outcome.[file:1]
- Integrations with major coding-agent environments and enterprise identity systems.[page:0][web:16]

### Wedge features

- Deterministic enforcement of non-trivial rules such as blocking writes outside project directories, denying non-allowlisted egress, and requiring approval for package installs.[file:1]
- Real-time, low-latency human approval workflow that gives enough context for quick decisions.[file:1][page:1]
- Replayable action timelines that show not only what happened, but the sequence of actions and the policy reasoning behind each step.[file:1]
- Dry-run policy simulation for rollout without breaking developer velocity.

### Differentiators

- **Sequence-aware policying**: evaluate action chains, not just single events, to detect suspicious combinations such as “read secrets → exfiltrate network call → cleanup.”
- **Org/repo/environment policy bundles**: apply policy by org, team, repository sensitivity, and environment class rather than one monolithic ruleset.[file:1]
- **Agent identity and trust scoring**: distinguish users from agents and bind each run to an attributable session, toolchain, and model.
- **Ephemeral credential brokering**: issue bounded, short-lived credentials into governed environments instead of exposing long-lived secrets.
- **Prompt/tool-context redaction**: reduce accidental leakage of secrets or PII into logs, approvals, or upstream tools.[file:1]
- **Multi-agent isolation**: prevent one agent session from reading another session’s context, artifacts, or credentials.[file:1]
- **Signed attestations and evidence exports**: create durable artifacts for compliance, incidents, and internal audit.

### Future platform features

- Anomaly detection over agent action sequences.[file:1]
- Cross-agent governance dashboard spanning IDE, terminal, CI, and infra automation.
- Policy recommendations learned from observed team behavior.
- Agent risk scoring and adaptive approval thresholds.
- Expansion beyond coding agents into broader engineering and operations agents.

## MVP scope

The MVP should be ruthless and opinionated. It should not attempt to solve every agent workflow or every enterprise surface at once.

### Include in MVP

- Managed secure execution environment, ideally container-based first, with strong enforcement around project directory writes, shell command controls, package install controls, and outbound network allowlisting.[file:1][web:11]
- Support for at least two high-value interception surfaces in depth, with shell/process execution and filesystem writes as the most important first pair.[file:1]
- Policy outcomes of allow, deny, and require approval.[file:1]
- One excellent approval UX for sensitive actions.[file:1]
- Structured audit logs and action-sequence replay.[file:1]
- One depth differentiator, preferably org-level policy distribution or sequence-aware anomaly detection, depending on technical feasibility and buyer feedback.[file:1]

### Exclude from MVP

- Full cross-platform endpoint instrumentation on day one.
- Broad support for every IDE and every coding agent.
- Compliance automation across every standard.
- Complex ML-based anomaly detection if deterministic policy controls are not already excellent.
- Generic AI observability features that are not directly tied to action governance.

### Sequencing rationale

The prototype brief explicitly says depth over breadth is stronger than touching everything lightly.[file:1] The same logic should drive the product: win first on one high-trust workflow where enforcement is real, buyer pain is acute, and implementation can prove that coding-agent autonomy can be bounded without wrecking developer experience.[file:1]

## Go-to-market wedge

Security and governance should be the initial GTM wedge because they are the main blockers to broad rollout, and because they create a high-value problem with an accountable buyer. The attached brief states that the security review is the wedge that blocks broad agent adoption in mid-market and enterprise dev organizations.[file:1]

### GTM motion

- **Land** with platform/security leaders who are already being asked to approve coding-agent rollout but lack policy enforcement and evidence.
- **Start** in one governed workflow, such as terminal-based coding agents in remote dev environments for backend or platform teams.
- **Expand** by adding more repos, more agent types, more environments, and eventually CI/infrastructure workflows.
- **Sell** top-down for budget and trust, but support bottom-up enthusiasm through developer-friendly UX and quick pilot wins.

### Proof points needed

- Demonstrated interception and blocking on real developer tasks.[file:1]
- Evidence that approvals are fast and sparse rather than constant.[page:1][web:7]
- Audit outputs that security reviewers find useful, not noisy.[file:1]
- Clear statement of what the product can enforce mandatorily versus what it only observes.

## Risks and open questions

- **Bypass risk**: developers or agents may act outside the governed environment unless deployment makes the governed path the default or mandatory.[web:11]
- **Developer friction**: too many approval requests or false blocks will destroy usage.[web:7][page:1]
- **Coverage fragmentation**: different agents, IDEs, terminals, and MCP servers create integration complexity.[file:1][web:13]
- **Bundling risk**: native controls from coding-assistant vendors may improve quickly.[page:0][web:16]
- **Category confusion**: buyers may conflate observability, content safety, and runtime enforcement; positioning must stay crisp.
- **Container limits**: container isolation is strong but imperfect, especially on shared kernels and mixed host/container workflows.[web:11]
- **Evidence burden**: storing useful audit trails without over-collecting sensitive code or secrets requires disciplined design.
- **ROI proof**: the company must show that governance unlocks adoption, not merely that it creates another security dashboard.

## Success metrics

### Product metrics

- Number of governed agent sessions per week.
- Number of protected repositories and workspaces.
- Percentage of agent actions evaluated by policy.
- Policy deployment time from draft to enforcement.

### Security effectiveness metrics

- Number and severity of blocked disallowed actions.
- Percentage of sensitive actions routed to approval.
- Mean time to approve or deny exception requests.
- Reduction in unmanaged agent workflows.

### Policy quality metrics

- False positive block rate.
- Approval fatigue rate, such as approvals per 100 agent actions.
- Percentage of actions covered by explicit policy.
- Drift rate between simulated and enforced policies.

### Business metrics

- Design partner conversion rate.
- Expansion from pilot team to multi-team deployment.
- Net revenue retention.
- Average ARR and payback period.

## Recommended MVP thesis

- Start with a governed execution environment for coding agents, not a passive observability layer.[file:1]
- Enforce a few high-value controls extremely well: filesystem boundaries, shell/process restrictions, package installation approvals, and egress policy.[file:1]
- Make approvals sparse, fast, and context-rich so developers do not revolt.[web:7][page:1]
- Generate audit evidence that security teams can actually use to approve wider rollout.[file:1]
- Win one workflow deeply before broadening across all agents and environments.[file:1]

## Recommended architecture thesis

- Use a secure container or remote governed workspace as the primary enforcement substrate for mandatory controls.[web:11][web:14]
- Add semantic mediation through MCP and agent integrations to understand intent before execution.[file:1]
- Bind every action to agent, user, session, repo, and environment identity for accountability.[web:16][page:0]
- Treat policy, identity, execution isolation, and evidence as separate but coordinated layers.[file:1][web:11]
- Consider microVM-based isolation for higher-assurance environments where container risk is unacceptable.[web:11]

## Recommended GTM thesis

- Sell the product as the control plane for safe AI coding-agent adoption, not as a generic AI security tool.[file:1]
- Target platform/security leaders in organizations already under pressure to deploy coding agents broadly.[file:1][page:0]
- Land with one tightly governed team and expand through policy templates and audit-backed trust.
- Position against fragmented native controls by emphasizing cross-agent independence and mandatory enforcement.[web:7][page:0]
- Use design partners to validate the minimum set of controls that turns “not yet” into “approved for rollout.”[file:1]

## Assumptions and unresolved decisions

- The PRD assumes enterprises will accept governed remote or containerized agent environments for at least a meaningful subset of coding-agent workflows. This must be validated in customer discovery.[web:11][web:14]
- The market sizing assumes security/governance buyers will budget separately for agent control rather than rely entirely on bundled features from coding-assistant vendors.[page:0][web:16]
- The architecture recommendation assumes the product can obtain enough integration depth with popular agent frameworks and IDE workflows to gather semantic context without depending on any single vendor.[web:7][web:13]
- The strongest open question is how much enforcement can be centralized without imposing too much developer friction. This should be validated through prototype pilots with policy simulation, approval UX testing, and measured impact on developer throughput.[file:1][page:1]
