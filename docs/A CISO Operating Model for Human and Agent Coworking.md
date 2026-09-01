# **A CISO Operating Model for Human and Agent Coworking**

### Left Shift Security for the Agentic Era

[Deepankar Das](https://substack.com/@deepankardas)  
Jul 26, 2026

## **Executive Summary**

The age of agentic AI requires the CISO to shift left again, but the meaning of left shift has changed. It is no longer enough to move security controls earlier into software development. The CISO now has to shape how humans and AI agents work together across the full lifecycle: strategy, design, architecture, implementation, testing, security testing, penetration testing, deployment, post-deployment security operations, and the continuous feedback loop that converts incidents and near misses into stronger future security and governance controls.

The central problem is not simply that AI can generate insecure code faster. The larger problem is that systems involving humans and AI agents working together can observe, decide, and act through tools, connectors, APIs, memory, and downstream systems at a speed and scale that can amplify both attacker capability and defender mistakes. OWASP’s agentic guidance highlights risks such as agent goal hijack, tool misuse, identity and privilege abuse, memory and context poisoning, insecure inter-agent communication, and cascading failures, while OWASP’s LLM guidance still applies to prompt injection, insecure output handling, training-data and supply-chain risk, excessive agency, and system prompt leakage. The gap is that these frameworks identify the risk landscape, but they do not by themselves define the operating model needed to govern human-agent coworking across authority design, secure development, testing, pen testing, runtime operations, and the closed-loop learning system that feeds production lessons back into the next design cycle.

Thanks for reading\! Subscribe for free to receive new posts and support my work.

## **What is the CISO Operating Model**

The **CISO operating model** is the full system by which the CISO’s responsibilities are organized and executed: the roles, decision rights, structures, processes, tools, metrics, closed-loop learning, and governance mechanisms that together define how the security program runs day to day and evolves over time.

It is the practical blueprint that translates security strategy for human-agent coworking into concrete actions across product design, development, deployment, governance, architecture, risk management, operations, oversight, and accountability.

Human-agent coworking adds a new operating layer that must be deliberately designed, governed, and integrated into the CISO’s existing remit.

The CISO should be treated not as a downstream reviewer and enforcer, but as a Steward of the human-agent operating system end to end, with a formal role in decision rights, architecture gates, testing rules, deployment approval, runtime controls, and closed-loop learning from incidents, testing, and operations.

This far left shift is necessary because the CISO’s organization can no longer govern AI coding agents and AI tools only through late-stage review or restrictive policy. It must define how those tools are used securely and effectively in practice, which forces the CISO operating model to move security earlier into product design, development workflows, testing, deployment, and operational governance.

A strong CISO operating model does not merely compensate for role overload; it strengthens the individual CISO role by clarifying and making decision rights transparent, establishing durable governance mechanisms, and making accountability visible through operational KPIs. By making decision rights transparent, the operating model ensures that the CISO and the broader executive team share a common understanding of who decides, who is accountable, and which KPIs define success in securing human-agent coworking.

## **1\. The New Meaning of Left Shift**

In the DevSecOps era, left shift meant moving security earlier in the software delivery lifecycle so that defects could be prevented before release. In the agentic era, left shift must expand from earlier scanning and review to earlier control over delegation, authority, autonomy, tool exposure, runtime behavior, and the governance around building applications before agents are ever allowed to act. That governance must include auditability, guardrails, hallucination-aware controls, secure development rules, and machine-verifiable evidence around code authored by both humans and agents, because the operating challenge is no longer only how to scan software after it is written, but how to shape the system, workflow, and tooling so vulnerabilities are less likely to be introduced in the first place.

That change is necessary because the object being secured has changed. Traditional software primarily executes logic predetermined by human developers. Agentic systems work differently: they combine models, prompts, memory, retrieval, tools, external APIs, and sometimes multiple cooperating agents, which means the system can take actions whose exact path was not manually scripted line by line in advance.

This expands the attack surface in two directions at once. Attackers now have better tools for reconnaissance, prompt manipulation, credential abuse, and social engineering, while defenders are deploying systems with broader tool access, greater runtime discretion, more external connectivity, and larger blast radius when authorization or guardrails fail.

The CISO must therefore left shift across the entire control loop, not merely the coding phase. The shift begins at business intent and authority design, continues through implementation and validation, and extends all the way into post-deployment monitoring and operational feedback, where each observed failure becomes a design input for the next cycle.

## **2\. Why the CISO’s Mandate Has Expanded**

The CISO’s traditional mandate centered on protecting systems, identities, networks, endpoints, data, and software. In a human-agent enterprise, that mandate must expand to include systems in which AI agents increasingly co-author code, generate tests, operate tooling, investigate issues, perform support tasks, and in some contexts take production actions that were previously reserved for human operators.

This makes security less like a review function and more like an operating-model function. If the organization has not defined who may delegate what to which agent, what an agent is allowed to touch, what approvals are required for irreversible actions, and what evidence is required for trust, then the organization has already created a security problem before a single line of code is scanned.

The CISO operating model provides a useful structure for this expanded mandate. It separates human roles such as Executive, Architect, Domain Owner, Reviewer, Operator, and Steward, and it gives the Steward unilateral block authority over compliance, security, and privacy matters, ownership of the audit evidence trail, and control over exceptions and data-handling changes.

For CISOs, that role separation is significant. It formalizes security as a non-delegable authority in a system where much of the work may be delegated to agents, and it places security governance inside the operating model rather than outside it.

## **3\. The Agentic Threat Model**

The first mistake organizations make is treating agentic AI as a narrow model-risk problem. Model behavior matters, but agentic security is broader because the real risk lies at the intersection of model output, tool access, identity, memory, external systems, and delegated action.

OWASP’s Top 10 for Agentic Applications identifies categories such as agent goal hijack, tool misuse and exploitation, agent identity and privilege abuse, agentic supply chain risk, memory poisoning, insecure inter-agent communication, and cascading failure conditions.

OWASP’s LLM guidance remains relevant as well, especially for prompt injection, insecure output handling, training-data poisoning, model denial of service, system prompt leakage, misinformation, excessive agency, and vector or embedding weaknesses in AI-enabled applications.

For the CISO, these risks can be organized into six operational families:

> > * **Authority risk:** the agent has access, but no principal with standing has authorized the action.  
> > * Identity risk: the system cannot reliably distinguish user intent, agent identity, service-account power, and delegated authority boundaries.  
> > * Tool risk: an agent can call a tool or connector in ways that are technically permitted but operationally unsafe.  
> > * Context risk: injected, poisoned, stale, or untrusted content changes the agent’s behavior.  
> > * Runtime risk: multi-step or multi-agent execution creates cascading failures or hidden side effects in downstream systems.  
> > * Learning risk: incidents are treated as isolated bugs rather than durable lessons that must become rules, tests, and control updates.

A practical threat model for agentic systems must therefore examine the complete agent-task-tool-data-authority chain. Reviewing the application in isolation is no longer sufficient.

## **4\. Access Is Not Authorization**

One of the most important shifts for CISOs in the agentic era is understanding that access is not the same as authorization. Agents often need broad technical access to be useful. They may need to read repositories, inspect logs, query ticketing systems, retrieve customer records, or access APIs and connectors. But broad access alone does not answer whether a specific action is authorized for a specific principal, in a specific context, for a specific purpose, at a specific moment.

A commission-based authority model provides a strong architectural answer to this gap. It defines a commission as a signed, scoped, machine-readable grant of authority from a principal to an agent, including the principal, grantee, acts authorized, resources in scope, constraints, expiry, and revocation channel.

This matters because many of the most serious agentic failures occur without any classical permission violation. The agent has enough system access to act, but no real authorization to take the action on behalf of the user or the organization. Commission-bound enforcement changes the question from “can the agent call this tool?” to “has a principal with standing granted authority for this action right now?”

The CISO should not treat this as a narrow architecture detail. It is foundational because it changes how security is designed, reviewed, tested, deployed, and monitored. In the agentic era, left shift begins with authority design, not only code review.

## **5\. The CISO and the Executive Team**

A practical constraint must be acknowledged: the CISO role is already under significant strain. Industry commentary and executive discussion increasingly describe sustained stress, expanding scope, and debate over whether the modern security remit should be split across multiple leaders or specialized deputies, especially as organizations add AI governance, resilience, privacy, and trust responsibilities.

Human-agent coworking makes that pressure more visible, not less, because it adds a new layer of authority design, agent governance, runtime oversight, evidence management, and policy translation into technical controls.

The need for a CISO operating model becomes more important as AI governance responsibilities spread across the executive team. In some organizations, these responsibilities may sit primarily with one CISO; in others, they may be shared across security, trust, technology, risk, privacy, or compliance leaders. Regardless of reporting structure or title, the organization still needs a clear, measurable operating model for governing human-agent coworking.

A practical CISO operating model should align the CISO with the CTO or CIO on engineering and platform controls, product leadership on autonomy boundaries and customer-risk tradeoffs, operations on runtime workflows and incident containment, legal and compliance on regulated actions and evidence, and business owners on accountability for high-risk workflows.

## **6\. Fatigue, Bottlenecks, and Tractability**

One of the most important objections to any expanded CISO remit is that the role already feels unmanageably broad. That objection is valid and should be addressed directly rather than implying that the answer is simply to add more reviews, more approvals, and more dashboards.

Agentic systems can create review fatigue, alert fatigue, approval fatigue, and exception fatigue because machine-speed activity produces more code, more workflow changes, more signals, more edge cases, and more escalation opportunities than human-paced systems.

A sound CISO operating model makes the role tractable by replacing ad hoc decision making with structured decision rights, evidence-first gates, automation for low-risk paths, permanent human-only boundaries for the highest-risk actions, and route-by-risk escalation so the CISO is involved where executive security judgment is necessary rather than everywhere by default.

For that reason, The value of the CISO operating model is not only that it expands security coverage. It also helps preserve executive effectiveness by making the playing field visible, reducing political ambiguity, providing KPIs, demanding automation artifacts, and preventing the CISO from becoming the informal bottleneck for every human-agent decision. Those same no-rerun logs, convergence records, and automation receipts are not only delivery artifacts; they also function as audit evidence and root-cause material when issues later appear in production.

## **7\. A CISO Operating Model for Human and Agent Coworking**

A viable CISO operating model for this era needs five structural elements.

### **7.1 Role clarity and decision rights**

Human-agent coworking fails when organizations delegate work without defining non-delegable decisions. A clear decision-rights matrix is useful here because it explicitly keeps accountability for customer outcome definition, architecture decisions, merge authority, deploy approval, rollback, and compliance or security veto with humans, while allowing agents to be accountable for bounded code and test authorship tasks.

The CISO should either occupy or formally control the Steward function for security, privacy, and compliance matters. That means explicit veto authority over changes that violate security policy, ownership of evidence and auditability requirements, and time-bounded approval of exceptions.

### **7.2 Agent inventory and classification**

Every agent should be inventoried by capability, autonomy level, tool surface, data exposure, and operating environment. Capability, autonomy, and collaboration dimensions provide a practical way to classify how an agent is used, while NIST-style governance expects organizations to understand the systems, actors, environments, and potential impacts involved.

A coding assistant that only suggests code inside an IDE is not equivalent to a deployment agent with shell access or a support agent that can read sensitive records and trigger downstream actions. The CISO needs a living inventory of these distinctions. That inventory should also extend to the artifact trail produced by AI coding assistants, including no-rerun logs, convergence records, prompts, tool-call receipts, and related automation evidence, because these artifacts should be examined continuously for signals of insecure patterns, missing controls, policy violations, or latent vulnerabilities introduced during development.

### **7.3 Tool and connector governance**

Every tool, API, external system, MCP server, data destination, and action channel should be registered, classified, and wrapped with enforcement points. OWASP’s agentic guidance repeatedly highlights tool misuse and privilege abuse, and a commission-based authority model shows why wrappers, JIT tokens, egress gates, and out-of-commission refusal are necessary at the system boundary.

### **7.4 Evidence-first trust**

Agent-authored work is trustworthy only when claims are independently verifiable. An evidence-first trust contract requires evidence-cited review, per-commit gates, machine-parseable receipts, versioned run folders, and explicit audit trails that can be re-verified after the fact.

This is highly relevant for security operations because unverifiable AI-generated output is not just a quality problem. It is a security problem, especially when code, tests, infrastructure changes, or runtime actions are machine-authored and then trusted without proof.

### **7.5 Incident-to-rule learning**

The CISO operating model must include a formal rule-creation loop. A mature learning loop converts every production incident into an instruction-file update, invariant test, skill update, or architecture-decision modification, producing an incident-to-rule traceability matrix.

For the CISO, this is the closed loop that makes left shift continuous rather than episodic. Security incidents, blocked actions, red-team findings, false positives, false negatives, and control bypass attempts must all become upstream changes to requirements, design, tests, and runtime guardrails.

## **8\. Human-Agent Security Lifecycle Control Loop**

The Human-Agent Security Lifecycle Control Loop is the lifecycle expression of the CISO operating model. It connects governance, design, build, testing, deployment, runtime operations, incident response, and learning into one closed security system. Rather than treating security as a set of checkpoints that end at release, it treats each lifecycle phase as both a control point and a feedback source, so that evidence, incidents, drift findings, and operational lessons from later phases are fed back upstream into earlier decisions, policies, tests, and architectures.

This Human-Agent Security Lifecycle Control Loop maps directly to the lifecycle of a product in a software engineering organization and to the operational lifecycle of an IT organization. In product engineering, Govern maps to portfolio and policy decisions, Design to product and architecture definition, Build to implementation, Test and Security Test to QA and adversarial validation, Deploy to release and rollout, Monitor and Respond to production operations and incident handling, and Learn to retrospectives and upstream backlog, policy, and control changes. In IT operations, the same loop overlays service design, change management, release, monitoring, incident response, and continual improvement, adding authority-aware controls, runtime evidence, and closed-loop learning to the operating model.

The most operational way to present the CISO operating model to the organization and the leadership is by explicitly and transparently defining the focus and key outputs for each stage in the Human-Agent Security Lifecycle Control Loop.

This Human-Agent Security Lifecycle Control Loop moves the idea of left shift from a linear development metaphor to a circular operating model for human-agent systems, where each phase produces controls and evidence for the next and every later-phase failure becomes an earlier-phase design input.

Continuous compliance sits inside the Human-Agent Security Lifecycle Control Loop as an ongoing discipline: monitoring controls, behavior, and evidence so the organization remains aligned with its obligations between audits. Compliance-drift monitoring is the specific mechanism that detects when actual human-agent behavior deviates from documented, tested, or authorized patterns and feeds those findings back into the Govern phase of the Human-Agent Security Lifecycle Control Loop, where policies, controls, and KPIs are updated for the next cycle.

## **9\. Security in Design and Architecture**

Security for agentic systems begins before implementation. The design phase must define the business objective, acceptable autonomy level, human approval boundaries, authority model, tool surface, data sensitivity classes, downstream actions, failure modes, and rollback conditions.

The CISO should require that any feature or workflow involving agents answer a minimum set of design questions:

> > * Who is the principal for each meaningful action?  
> > * Which actions are delegated to agents, and which remain human-only?  
> > * What tools and systems may the agent call?  
> > * What sensitive data classes can be accessed, transformed, or transmitted?  
> > * What approvals are required for irreversible or regulated actions?  
> > * How is revocation, shutdown, and rollback handled when something goes wrong?

Architecture reviews should therefore move earlier and become more explicit. Formal architecture decision gates provide a strong mechanism because system-wide changes, including configuration changes with system-wide effects, require a written proposal, rationale, review, and approval before implementation begins.

For CISOs, this is where security left shift becomes practical. Security is not merely “consulted” at release time. It is embedded at the moment the organization decides how authority, interfaces, connectors, and invariants will work. Design should also define the **Agentic AI Guardrails** that sit between foundation-model coding assistants, open-weight models, agent frameworks, MCP-connected tools, and the systems they can affect. Those guardrails should control which files, repositories, shells, package managers, credentials, network destinations, external APIs, and MCP servers are in scope, which actions are denied outright, which require approval, and which must always produce tamper-evident audit logs and receipts for later review.

## **10\. Secure Development in Human-Agent Teams**

Secure development changes when humans increasingly specify and review while agents increasingly author and modify code. The bottleneck moves from code production to code verification, authority validation, and policy enforcement.

Several first principles are directly useful for the CISO here: verification shifts to the point of authorship, evidence is first-class, no work item has shared ownership, single-purpose commits reduce rollback complexity, architecture is a gate, memory is external and durable, and explainability becomes a required output rather than a nice-to-have. These principles matter because secure application delivery in the agentic era depends on governance around how code is produced, reviewed, constrained, logged, tested, and approved, not only on whether a scanner later finds a defect.

A CISO operating model should require at least the following controls in agent-assisted or agent-authored development:

> > * Per-commit static and build checks before a commit is accepted.  
> > * Impact-selected tests and invariant tests tied to the changed surface.  
> > * Sensitive-data and connector-diff scans whenever code changes touch data, channels, or tool surfaces.  
> > * Cross-agent or independent review for non-trivial changes, with human merge authority above a defined risk threshold.  
> > * Explicit ownership records so every lane, file set, or change area has one accountable owner.  
> > * Machine-verifiable proof that the required gates actually ran and passed.

These controls do not eliminate human review; they make human review possible at agent velocity. Without them, the volume of machine-authored change outpaces any meaningful security verification process, and without surrounding governance such as audit logs, guardrails, approval boundaries, and reproducible evidence, organizations cannot reliably distinguish acceptable automation from latent vulnerability creation.

## **11\. Testing Beyond Functionality**

Testing in the agentic era must expand beyond “does the feature work?” to “does the system behave safely within its intended authority, context, and operational boundaries?”

Functional testing remains necessary, but it is no longer sufficient. Teams must test fallback behaviors, approval paths, denied actions, out-of-scope data requests, malformed or adversarial inputs, context shifts, and multi-agent coordination boundaries.

A mature program should include at least five test classes:

> > * Standard correctness testing: unit, integration, end-to-end, and regression coverage.  
> > * Boundary testing: ensure agents cannot exceed allowed resource, tenant, data, or action scope.  
> > * Failure-path testing: prove that denied, ambiguous, or revoked actions halt safely and log correctly.  
> > * Approval-path testing: verify high-risk actions require the intended human intervention.  
> > * Drift testing: detect behavior changes when models, prompts, tools, policies, or memory inputs are updated.

This is also where blast-radius-aware testing becomes important. Commission-bound action definitions and risk-aware test selection make it possible to choose the tests that matter for a given change without pretending that the full suite can run on every change forever.

## **12\. Security Testing and Red Teaming**

Security testing for agentic systems must be threat-led and agent-aware. Traditional dynamic testing and SAST or DAST remain useful, but they do not cover the full surface of prompt-driven, tool-using, memory-enabled systems.

At a minimum, the CISO should require dedicated test scenarios for:

> > * Direct prompt injection.  
> > * Indirect prompt injection through documents, web content, tickets, logs, PDFs, and retrieved content.  
> > * Tool misuse and action escalation.  
> > * Identity and privilege abuse across principals, tenants, or team boundaries.  
> > * Memory poisoning and stale-context misuse.  
> > * Insecure output handling where generated content drives privileged execution.  
> > * Inter-agent communication abuse and failure cascades.  
> > * Excessive agency and hidden side effects across external systems.

These tests should be mapped to both the OWASP Agentic Top 10 and the OWASP LLM Top 10, because enterprise systems increasingly contain both classic LLM weaknesses and newer multi-step agentic execution risks.

Security testing also needs to be durable. The useful output is not just a point-in-time report. It is a growing library of attack patterns, regression cases, and rule updates that the organization can replay against future versions of the system. That testing should explicitly include the Agentic AI Guardrails themselves: red teams and security testers should attempt to bypass hook enforcement, approval flows, policy wrappers, audit logging, and any OS- or runtime-level controls that are supposed to constrain agent behavior. A guardrail that exists only on paper, or that can be disabled by local workflow changes, should not be treated as a meaningful control.

## **13\. Penetration Testing for Agentic Systems**

Penetration testing changes significantly when the target is not just a web app or API but a system of humans, agents, tools, tokens, memory, workflows, and downstream actions. The pen tester must think less like a pure exploit hunter and more like an attacker steering a semi-autonomous operator.

A modern agentic pen test should attempt to answer questions such as:

> > * Can an attacker steer the agent into actions outside a legitimate principal’s authority?  
> > * Can user-controlled or retrieved content alter tool use or data movement?  
> > * Can cross-tenant, cross-team, or cross-principal boundaries be breached by chaining individually permitted actions?  
> > * Can tokens, credentials, or trust assumptions be abused through tool wrappers, connectors, or orchestration layers?  
> > * Can the agent cause state changes in downstream systems that are hard to detect or reverse?

Penetration testing should also move earlier in the delivery path. A practical pattern is to run an impact-selected subset of pen-test cases on every check-in so high-risk regressions are surfaced while engineers are still working, then run the full pen-test suite nightly when no one is blocked on check-in latency. The pen-test report should not be treated as the end of the process. Each finding should become either a rule update, a new approval requirement, a new boundary test, a revised commission pattern, or a runtime control addition. In other words, pen testing is one feeder into the same closed-loop learning system, not a standalone audit ritual.

## **14\. Deployment and Runtime Controls**

If development and testing are the left side of the loop, deployment and runtime operations are where the organization proves its design assumptions in the real world. A secure deployment model for agentic systems should activate controls, not merely release features.

Runtime controls should include, at minimum:

> > * Short-lived commission-bound or task-scoped tokens.  
> > * Tool wrappers and egress gates around all high-consequence actions.  
> > * Human approval workflows for irreversible, regulated, or high-blast-radius actions.  
> > * Real-time policy evaluation for destination scope, tenant boundary, and act class.  
> > * Append-only, commission-keyed audit logs with enough evidence for investigation and compliance review.  
> > * Canary or staged rollouts with invariant monitoring during the first observation window.  
> > * Immediate revocation, pause, or kill-switch mechanisms for misbehaving agents or compromised workflows.

The critical point is that runtime security must be designed as part of the system, not bolted on afterward. An organization that waits until deployment to think about runtime policy has already waited too long.

## **15\. Security Operations After Deployment**

Once agentic systems are in production, the SOC and broader security operations function need new telemetry, new detections, and new runbooks. Traditional observability data still matters, but it must be extended with policy, authority, action-level evidence, continuous compliance signals, and the operational traces that show how both human-authored and agent-authored changes were proposed, constrained, approved, and executed.

The CISO should require monitoring for at least the following classes of signals:

> > * Out-of-commission attempts or refusals.  
> > * Unusual tool usage volume, sequence, or destination patterns.  
> > * Prompt-injection indicators and suspicious retrieved-content influence.  
> > * Cross-tenant or cross-principal access attempts.  
> > * Novel data exfiltration paths or abnormal egress destinations.  
> > * Repeated human override or approval-denial patterns that suggest system drift.  
> > * Cascading agent failures or anomalous multi-agent behavior.  
> > * Compliance drift, where actual agentic behavior departs from documented, tested, or authorized patterns.

Security operations also needs new runbooks. A meaningful agentic incident response playbook should include commission revocation, token invalidation, connector disablement, memory quarantine, task replay analysis, rollback of reversible actions, and evidence preservation for both security and compliance review.

Separating Operator and Steward responsibilities is useful here because it divides urgent runtime accountability from compliance, privacy, and audit accountability while keeping both inside the same operating loop.

## **16\. The Continuous Learning Loop**

The single most important difference between a conventional security program and a mature agentic security program is whether learning is operationalized. Agentic systems change quickly, and their risks evolve through new prompts, new tools, new connectors, new models, new contexts, and new adversarial strategies. A static control framework will degrade.

ADM’s Phase 8 provides a concrete answer: production telemetry, incident reports, and customer feedback flow into post-incident root-cause analysis with evidence citation, and every material incident is converted into an instruction-file update, invariant test, skill update, or architecture-decision change.

For the CISO, this loop should be expanded to include all security learning inputs:

> > * Security incidents.  
> > * Near misses and blocked out-of-commission actions.  
> > * Security test failures.  
> > * Red-team and pen-test findings.  
> > * False positives and false negatives from policy engines or detectors.  
> > * Customer complaints and operator exceptions that reveal friction or blind spots.

Each of these should produce one or more durable outputs:

> > * Updated threat models.  
> > * Revised commissions and authority patterns.  
> > * New or amended architecture decisions.  
> > * New tests or mandatory floors in the validation pipeline.  
> > * New runbook steps and incident triggers.  
> > * New instruction-file memories, skill constraints, or banned-pattern rules for future agent sessions.

This turns security into a compounding system. The more the enterprise learns, the more its agents, controls, tests, human operators, and development tooling become aligned around what safe behavior actually means and how to prevent hallucination-led or workflow-led vulnerabilities before they spread.

## **17\. Operational Playbook for CISOs**

A CISO who wants to operationalize this model can begin with a practical first-wave checklist.

### **In the first 30 days**

> > * Create an inventory of all production and near-production AI agents, including capability, autonomy, tools, and sensitive data reach.  
> > * Define a named owner and a named security approver for every high-risk agent workflow.  
> > * Register all tools, external systems, connectors, and high-risk action channels available to agents.  
> > * Define initial autonomy tiers and identify actions that are permanently human-only.

### **In the first 60 days**

> > * Add architecture review gates for any agent feature that touches regulated data, external channels, or irreversible actions.  
> > * Establish baseline security test cases mapped to OWASP Agentic and OWASP LLM risks.  
> > * Require append-only action logs and evidence retention for all high-consequence agent workflows.  
> > * Implement runtime revocation and emergency-disable procedures.

### **In the first 90 days**

> > * Introduce commission-bound or equivalent authority checks for the highest-risk actions first.  
> > * Add policy-aware monitoring to the SOC for out-of-commission attempts, egress anomalies, and cross-boundary access behavior.  
> > * Build an incident-to-rule workflow so findings from operations, testing, and pen testing are translated into durable upstream changes.  
> > * Publish a CISO dashboard with metrics on agent inventory coverage, approval coverage, control coverage, blocked actions, and incident-learning closure rate.

## **18\. Agentic AI Guardrails Across Development and Runtime**

Agentic AI Guardrails are the security and policy control plane for agentic workflows across the full lifecycle, not only a runtime firewall or IDE plugin. They define what foundation-model coding assistants, open-weight models, agent frameworks, and MCP-connected tools may do, what they may access, and when they must stop for human approval or refuse an unsafe action altogether. They also ensure that every significant agentic action produces security-grade evidence so CISOs and security teams can reconstruct, review, and learn from what happened. That same automation and evidence trail also makes it easier to support SOC 2 and other regulatory or contractual compliance obligations, because organizations can produce consistent audit records, approval histories, policy-enforcement evidence, and tamper-evident logs without relying on ad hoc manual reconstruction after the fact.

### **18.1 Development-time guardrails for AI coding agents**

Development-time Agentic AI Guardrails sit between AI coding agents and the systems they can affect during Build, Test, Security Test, and Pen Test. They constrain which repositories and files can be read or written, which shell commands and package operations are allowed, which network destinations are in scope, and how credentials and secrets are handled. They apply equally to cloud-hosted assistants, local open-weight models, IDE-integrated agents, and MCP-driven coding workflows.

Effective development guardrails provide at least three properties:

> > * **Deterministic enforcement during authoring.** Agentic hooks and enforcement points intercept file, shell, network, package, credential, and MCP actions and apply explicit allow, deny, or require-approval decisions before the action executes.  
> > * **Governance that cannot be disabled locally.** Managed hooks, privileged daemons, and OS-level enforcement make sure developers cannot simply turn off guardrails in the IDE, bypass them via raw terminals, or silently change local policy without security approval.  
> > * **Security-grade artifact trails.** Hook logs, no-rerun logs, prompts, tool-call receipts, append-only audit events, anomaly-detection signals, and secret-redaction traces are treated as part of the security evidence set, not just engineering telemetry. They are designed to support both convergence in development and root-cause analysis when issues appear in the field.

In the CISO operating model, these development-time guardrails belong inside the Human-Agent Security Lifecycle Control Loop’s Design, Build, and Test phases. Design defines which surfaces and actions the guardrails must cover. Build implements hooks, enforcement layers, and audit pipelines around all AI coding assistants and agent tools. Test verifies not only that features work, but that guardrails are active, boundaries are enforced, logs are produced, and unsafe or hallucinatory behavior is blocked or escalated.

### **18.2 Deployment and runtime guardrails for agentic systems**

Deployment and runtime Agentic AI Guardrails surround agentic applications, copilots, multi-agent workflows, MCP workflows, and production systems once they are live. They provide admission control before agentic actions begin and execution gating while they run, so agents cannot quietly expand their authority, exfiltrate data, or chain together high-risk operations beyond what the organization has authorized.

Runtime guardrails should include:

> > * **Policy-aware access control.** Short-lived, task-scoped credentials and policy engines that enforce destination, tenant, data-class, and action-type constraints at runtime.  
> > * **Runtime monitoring and circuit breakers.** Detectors for prompt injection, tool misuse, data exfiltration, drift, runaway autonomy, and cross-boundary access, coupled with kill paths and circuit breakers that can pause or terminate misbehaving agents and workflows.  
> > * **Persistent, tamper-evident audit.** Append-only, hash-chained audit logs for agentic actions, including who or what acted, what was requested, what tools and systems were touched, what decisions were made (allow, deny, approval), and what evidence supports those decisions.

In the CISO operating model, these deployment and runtime guardrails integrate directly with the Human-Agent Security Lifecycle Control Loop’s Deploy, Monitor, Respond, and Learn phases. Deploy activates guardrails alongside features, not afterward. Monitor uses guardrail signals as part of SOC telemetry. Respond relies on guardrail controls for containment, revocation, and rollback. Learn converts guardrail violations, blocked actions, and approval patterns into upstream changes to policies, architecture decisions, tests, and development-time guardrails, so the system becomes safer over time.

## **19\. Metrics That Matter**

The wrong metrics will produce false confidence. In the agentic era, counting AI adoption or token consumption is not enough. The CISO needs metrics that show whether the organization is controlling autonomy, authority, and learning effectively.

These KPIs operationalize the CISO operating model for human-agent coworking by making governance, authority, verification, runtime control, and learning visible in a form that can be reviewed by the CISO and the broader executive team.

Organizations should start with a focused core set and expand over time. The purpose of the table is not to create a giant scorecard, but to make the CISO operating model measurable in ways that support executive accountability and practical improvement.

## **20\. Strategic Implications**

We define a CISO operating model for the agentic era. Its central claim is that left shift must be redefined as full-loop security for human and agent coworking systems: security that begins with governance and authority design, persists through development and testing, continues through runtime operations, and closes the loop by converting every important lesson into upstream improvement.

A mature enterprise architecture for authority and delegated action should provide a structural answer to the access-versus-authorization gap that agentic systems create.

That operating model also requires software-delivery and verification discipline strong enough to build and ship securely at machine speed, including per-commit gating, trust receipts, blast-radius-aware testing, convergence loops, and review protocols in human-agent teams.

Taken together, these elements form a coherent approach: a strategic CISO operating model, an authority and enforcement pattern, and an engineering execution model for secure human-agent coworking.

## **21\. The Application Security Gap the CISO Operating Model Addresses**

OWASP’s agentic guidance and OWASP’s LLM guidance are both important, but they do not by themselves answer the CISO’s core operating question. They identify major risk categories and mitigations for agentic and LLM-based systems, yet they remain primarily risk taxonomies and security-control guidance rather than a full executive operating model for how security should govern human-agent coworking across the entire lifecycle.

That gap matters because a CISO does not only need a list of what can go wrong. The CISO needs a practical operating model for who owns which decisions, where security intervenes, how authority is bounded before deployment, how testing and pen testing are integrated into delivery, how runtime operations detect policy drift, and how incidents become durable changes to specifications, architectures, controls, and runbooks.

This is the specific gap that must be addressed.

### **21.1 Gap one: from taxonomy to operating model**

OWASP names the risks, but it does not by itself define the CISO operating system for managing them. It does not fully specify the human roles, decision-rights matrix, veto points, escalation paths, and accountability model needed when agents co-design, co-build, co-test, and sometimes co-operate production systems alongside humans.

We fill that gap by defining left shift as a governance and operating-model problem, not only a control-selection problem. It places the CISO or Steward inside the lifecycle with explicit authority over architecture gates, approval boundaries, audit evidence, runtime controls, and the closed-loop learning of the CISO operating model.

### **21.2 Gap two: from vulnerabilities to authority design**

OWASP correctly identifies prompt injection, tool misuse, identity and privilege abuse, and excessive agency. But a recurring structural cause underneath many of these issues is the authorization gap in agentic systems: the agent has technical access and tool power, but no machine-checkable proof that a principal with standing authorized this specific action, for this purpose, at this moment.

We name that gap explicitly and make it central to the CISO agenda. The point is not only to harden prompts or test tools, but to require an authority model in which access is separated from authorization and high-risk actions are bounded, reviewable, revocable, and auditable.

### **21.3 Gap three: from build-time controls to full-loop security**

OWASP guidance is highly valuable for secure development and security testing, but CISOs also need a lifecycle model that joins design, development, testing, security testing, pen testing, deployment, runtime security operations, and post-incident learning into one continuous loop.

We fill that gap by framing left shift as full-loop security. It extends the security conversation beyond pre-release controls into deployment approval, runtime telemetry, out-of-commission detection, operational response, and incident-to-rule feedback.

### **21.4 Gap four: from point mitigations to institutional learning**

OWASP guidance describes risks and mitigations, but enterprises also need a durable way to retain lessons in systems where agents themselves may not preserve session memory. Without an explicit institutional memory mechanism, every incident risks being rediscovered rather than learned from.

We fill that gap by making closed-loop learning a first-class security function. Production incidents, blocked actions, pen-test findings, red-team discoveries, and false positives are treated as raw material for new rules, new tests, updated architecture decisions, and updated agent instruction layers.

### **21.5 Gap five: from security guidance to CISO practice**

OWASP tells organizations what classes of agentic and LLM risk matter. We focus on telling CISOs how to run security in response: how to define roles, when to intervene, what to test, what to monitor, what to approve, what to block, and how to make the whole system improve over time.

In that sense, this model is not a replacement for OWASP. It is the missing management and operating layer above OWASP’s risk catalog, designed specifically for CISOs managing human-agent coworking systems in production.

## **22\. Conclusion**

The CISO of the agentic era is not simply the guardian of an AI application. The CISO becomes the steward of a human-agent operating system in which security depends on how authority is delegated, how tools are exposed, how actions are bounded, how trust is evidenced, how runtime is controlled, and how operational learning compounds over time.

That is why left shift must be redefined. In the age of agentic AI, left shift does not mean only earlier scanning. It means designing the full control loop earlier, running it continuously, and ensuring that every failure, refusal, and incident makes the next cycle safer than the last.

## **References**

> > 1. Das, Deepankar. *The Post-Agile Era: Building with Co-Working Humans and Agents*. Substack. Included as a supporting reference for terminology and framing around post-Agile, human-agent coworking, and AI-assisted software development practices. [https://deepankardas.substack.com/p/the-post-agile-era-building-with](https://deepankardas.substack.com/p/the-post-agile-era-building-with)  
> > 2. NIST. *AI Risk Management Framework (AI RMF)*. National Institute of Standards and Technology. Framework for managing AI risks across design, development, deployment, and evaluation, with emphasis on trustworthy and responsible AI lifecycle governance. [https://www.nist.gov/itl/ai-risk-management-framework](https://www.nist.gov/itl/ai-risk-management-framework)  
> > 3. NIST. *Artificial Intelligence Risk Management Framework: Generative Artificial Intelligence Profile*. NIST AI 600-1. Companion guidance for identifying and managing risks specific to generative AI systems. [https://www.nist.gov/itl/ai-risk-management-framework](https://www.nist.gov/itl/ai-risk-management-framework)  
> > 4. OWASP Gen AI Security Project. *State of Agentic AI Security and Governance 2.01*. Practical guidance on securing and governing autonomous and agentic AI systems. [https://genai.owasp.org/resource/state-of-agentic-ai-security-and-governance/](https://genai.owasp.org/resource/state-of-agentic-ai-security-and-governance/)  
> > 5. OWASP Gen AI Security Project. *LLM Top 10 2025*. Risks and mitigations for large language model applications, including prompt injection, improper output handling, excessive agency, and system prompt leakage. [https://genai.owasp.org/llm-top-10/](https://genai.owasp.org/llm-top-10/)  
> > 6. OWASP. *OWASP Top 10 for LLM Applications 2025*. PDF reference edition for secure design and testing of LLM-enabled applications. [https://owasp.org/www-project-top-10-for-large-language-model-applications/assets/PDF/OWASP-Top-10-for-LLMs-v2025.pdf](https://owasp.org/www-project-top-10-for-large-language-model-applications/assets/PDF/OWASP-Top-10-for-LLMs-v2025.pdf)  
> > 7. Cisco, *State of AI Security Report 2026* — industry report on AI threat intelligence, policy, standards, and enterprise AI security posture.  
> > 8. NIST, *AI Risk Management Framework* — foundational framework for managing AI risk across governance, mapping, measurement, and management functions.  
> > 9. OWASP, *Secure by Design Framework* — practical guidance for embedding security into architecture and software design from the start.  
> > 10. Cloud Security Alliance, *The State of AI Security and Governance* — survey-based view of how governance maturity correlates with AI readiness.  
> > 11. Help Net Security, *How to use NIST and ISO frameworks to govern AI agents* — practical translation of AI governance standards into agent ownership, scope, lifecycle, and auditability controls.  
> > 12. Obsidian Security, *What the OWASP AI Security Guidance Means for Enterprise* — discussion of enterprise implications of OWASP AI security guidance for governance and controls.  
> > 13. Swimlane, *CISO Guide: AI’s Security Impact* — summary of SANS 2025 findings on governance gaps, security-team involvement, and CISO priorities.  
> > 14. CSO Online, *NIST’s AI guidance pushes cybersecurity boundaries* — overview of NIST’s increasingly operational approach to AI security, identity, privacy, and agent systems.  
> > 15. TrustCloud, *The 2025 CISOs’ Guide to AI Governance* — CISO-oriented framing for AI risk, compliance, and internal control programs.  
> > 16. ISO, *ISO 42001 explained* — explanation of ISO/IEC 42001 as the first AI management system standard.  
> > 17. NIST CSRC, *SP 800-218A, Secure Software Development Practices for Generative AI and Dual-Use Foundation Models* — AI-specific secure software development profile aligned with SSDF.  
> > 18. Microsoft Learn, *ISO/IEC 42001:2023 Artificial Intelligence Management System* — accessible summary of ISO 42001 requirements and organizational implications.  
> > 19. NIST CSRC, *NIST Publishes SP 800-218A* — announcement and summary of the AI-focused SSDF profile.  
> > 20. MITRE ATLAS overview, as discussed in enterprise governance commentary — structured threat framework for adversarial AI activity and mitigations.  
> > 21. Zenity, *MITRE ATLAS AI Security and Agentic Threats 2026 Update* — note on posture documentation and risk modeling for agentic AI threats.  
> > 22. Pivot Point Security, *ISO 42001 AI Management System Elements* — description of lifecycle-oriented AI governance and continuous improvement.  
> > 23. KPMG, *ISO/IEC 42001:2023 – A new standard for AI governance* — governance and regulatory-alignment perspective for enterprise AI programs.  
> > 24. NIST, *NIST SP 800-218A PDF* — full primary source for secure software development practices specific to generative AI and dual-use foundation models.  
> > 25. OWASP AI Exchange, *AI Security Overview* — core framework for AI security fundamentals, threats, controls, and related practices.  
> > 26. Palo Alto Networks, *MITRE’s Sensible Regulatory Framework for AI Security* — explanation of how regulatory and threat-mapping approaches complement one another.

