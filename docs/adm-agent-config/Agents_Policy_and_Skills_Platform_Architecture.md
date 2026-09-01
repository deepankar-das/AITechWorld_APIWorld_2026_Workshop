> **AgenticDevelopmentModel**
> Copyright © 2026 AgenticDevelopmentModel Inc. Proprietary and Confidential.
>
> Author: Deepankar Das

# Agents Policy and Skills Platform Architecture

**Version:** 1.0
**Date:** 2026-05-20
**Parent Document:** the platform architecture doc
**Related:** [Agents_Policy_and_Skills_Implementation.md](Agents_Policy_and_Skills_Implementation.md), `AGENTS.md`, the Claude skills implementation doc
**Status:** Proposed; Use Case 2 Discussion Only
**Scope:** Architecture for implementing `AGENTS.md`-style rules and Codex-style skills as a first-class subsystem inside an Agents-as-a-Service SaaS platform

---

## Table of Contents

1. [Overview](#1-overview)
2. [Problem Framing](#2-problem-framing)
3. [Architectural Thesis](#3-architectural-thesis)
4. [Platform Role of AGENTS.md](#4-platform-role-of-agentsmd)
5. [Platform Role of Skills](#5-platform-role-of-skills)
6. [Core Subsystems](#6-core-subsystems)
7. [Policy Architecture](#7-policy-architecture)
8. [Skill Architecture](#8-skill-architecture)
9. [Controller and Execution Architecture](#9-controller-and-execution-architecture)
10. [Data and Contract Model](#10-data-and-contract-model)
11. [Multi-Tenant Isolation Model](#11-multi-tenant-isolation-model)
12. [Security, Governance, and Audit](#12-security-governance-and-audit)
13. [Review, Verification, and Release Architecture](#13-review-verification-and-release-architecture)
14. [Observability and Operations](#14-observability-and-operations)
15. [Recommended Service Topology](#15-recommended-service-topology)
16. [Reference Request Lifecycle](#16-reference-request-lifecycle)
17. [Maturity Roadmap](#17-maturity-roadmap)
18. [Open Architecture Decisions](#18-open-architecture-decisions)

---

## 1. Overview

This document defines the proper full-featured architecture for turning:

1. `AGENTS.md`-style operating rules, and
2. Codex-style skills

into a production-grade subsystem inside a large-scale Agents-as-a-Service SaaS platform serving hundreds of thousands of organizations and millions of users.

The critical architectural point is:

1. `AGENTS.md` should not remain a repo-local prose file,
2. skills should not remain only local folders of instructions,
3. both must become governed, versioned, auditable platform primitives.

At scale:

1. `AGENTS.md` becomes the **policy system**,
2. skills become the **capability registry**,
3. a controller becomes the **decision and routing engine**,
4. typed packets become the **execution protocol**,
5. evidence and audit become the **trust substrate**.

### 1.1 Document Position

This document is the subsystem architecture in the three-document hierarchy:

1. **Platform architecture:** the platform architecture doc
2. **Subsystem architecture:** [Agents_Policy_and_Skills_Platform_Architecture.md](Agents_Policy_and_Skills_Platform_Architecture.md)
3. **Implementation plan:** [Agents_Policy_and_Skills_Implementation.md](Agents_Policy_and_Skills_Implementation.md)

This is a discussion-only architecture for Use Case 2. It is not evidence of implemented product/runtime services in this repository.

Use this document when the question is specifically how `AGENTS.md + skills + controller + evidence + policy` should work inside the broader platform.

---

## 2. Problem Framing

When teams say they want “agent rules” and “skills,” they usually mean some mix of:

1. behavior constraints,
2. reusable workflows,
3. domain knowledge,
4. execution recipes,
5. approval boundaries,
6. release discipline.

At small scale, these can live in markdown files and prompts.

At large scale, that breaks down because:

1. prose rules are not enforceable,
2. prompt-only skills are not version-safe,
3. agent behavior cannot be audited reliably,
4. tenant-specific restrictions cannot be merged safely,
5. release integrity cannot depend on memory or convention,
6. regulators and enterprise customers need durable evidence.

The system therefore must operationalize `AGENTS.md + skills` into machine-validated architecture.

---

## 3. Architectural Thesis

The correct architecture is:

1. **Policy Plane**
   - machine-enforced interpretation of `AGENTS.md`
2. **Capability Plane**
   - skill registry, packaging, rollout, trust level, and compatibility
3. **Control Plane**
   - request intake, context construction, routing, approvals, state transitions
4. **Execution Plane**
   - agent task execution, tool mediation, workflow runtime, checkpointing
5. **Evidence Plane**
   - findings, receipts, artifacts, counter-examples, and closure proofs
6. **Governance Plane**
   - tenant overlays, audit, retention, compliance hold, and policy override receipts

This subsystem should plug into the broader AaaS platform but must be architected as its own coherent stack.

---

## 4. Platform Role of AGENTS.md

`AGENTS.md` should be treated as the human-readable source expression of platform policy.

### 4.1 What AGENTS.md Contains Today

Typical `AGENTS.md` content includes:

1. coding rules,
2. test rules,
3. review expectations,
4. deployment constraints,
5. approval boundaries,
6. architecture decision gates,
7. closure integrity rules,
8. no-shortcut rules.

### 4.2 What It Must Become

At platform scale, split that content into four derived representations:

1. **Human-readable policy spec**
   - keeps intent understandable
2. **Machine-readable policy rules**
   - enforceable schemas and predicates
3. **Workflow bindings**
   - where and when a rule applies
4. **Audit events**
   - proof that a rule was evaluated and enforced

### 4.3 Example Translation

| AGENTS.md Rule | Platform Representation |
|---|---|
| Never run destructive commands without approval | `policy_rule.requires_approval = true` on destructive action class |
| No closure without evidence | task transition guard from `executing -> verified` requires evidence receipt |
| No architecture changes without explicit approval | architecture-delta detector triggers approval packet before implementation |
| No silent deferrals | controller prohibits `done` with unresolved owned items |

`AGENTS.md` remains important, but only as a source-of-truth document feeding a compiled policy system.

---

## 5. Platform Role of Skills

Skills are the reusable capability modules that tell the platform what a given agent can do and how it should do it.

### 5.1 Skills Must Become First-Class Assets

Each skill should be a governed package with:

1. identity,
2. version,
3. metadata,
4. trigger rules,
5. supported task types,
6. tool requirements,
7. permission requirements,
8. validation status,
9. rollout status,
10. trust/risk classification.

### 5.2 Skills Are Not Just Instructions

At scale, a skill must encapsulate:

1. guidance,
2. references,
3. scripts,
4. deterministic helpers,
5. validation checks,
6. operational policy bindings.

### 5.3 Skill Categories

| Category | Description | Examples |
|---|---|---|
| Domain skill | Product or business-specific knowledge | customer-intake-review, entitlement-check-policy |
| Workflow skill | Multi-step process logic | release-readiness, radar-remediation |
| Integration skill | External system adapter behavior | github-pr-review, slack-approval-routing |
| Review skill | Review discipline and evidence collection | code-review-rigorous, spec-review-rigorous |
| Control skill | Planning and routing helpers | work-decomposer, evidence-synthesizer |

---

## 6. Core Subsystems

The subsystem should be composed of the following services.

| Subsystem | Responsibility |
|---|---|
| Policy Registry | Stores policy documents, compiled rules, bindings, and versions |
| Policy Compiler | Converts prose-derived policies into enforceable rules |
| Skill Registry | Stores skill packages, versions, metadata, compatibility, and rollout state |
| Context Builder | Builds minimal task context from code, docs, tenant state, and prior evidence |
| Controller Service | Routes requests, selects skills, applies policy, manages approvals and state |
| Execution Broker | Sends typed packets to execution runtimes or peer-agent backends |
| Review Service | Manages independent review packets, findings, convergence, and reviewer state |
| Evidence Service | Stores receipts, artifacts, counter-example checks, and closure proofs |
| Approval Service | Manages explicit human approvals and override receipts |
| Audit Service | Records immutable system-of-record events for all critical actions |

---

## 7. Policy Architecture

### 7.1 Policy Layers

Effective policy must be resolved from multiple layers:

1. **Platform Global**
   - non-negotiable rules
2. **Product / Plan Tier**
   - feature and capability entitlements
3. **Tenant Policy**
   - tenant-specific restrictions
4. **Workspace / Project Policy**
   - team or matter-specific constraints
5. **Task Policy**
   - request-specific constraints

Final runtime policy should be the compiled merge of all five.

### 7.2 Policy Object Model

| Object | Purpose |
|---|---|
| `PolicyDocument` | Human-readable source text or canonical spec |
| `PolicyVersion` | Versioned release of a policy |
| `PolicyRule` | Atomic enforceable rule |
| `PolicyBinding` | Links a rule to task class, tool class, tenant tier, or workflow stage |
| `PolicyDecision` | Result of evaluating rules in a live run |
| `PolicyOverrideReceipt` | Explicit approved override for a blocked rule |

### 7.3 Policy Classes

| Class | Examples |
|---|---|
| Action safety | destructive command, external send, delete, force-push |
| Quality gates | test, build, receipt, review requirements |
| Closure integrity | no `done` without receipts, no unresolved critical findings |
| Architecture governance | explicit approval for topology or scope changes |
| Tenant governance | restricted data scope, region, retention, compliance hold |
| Tool restrictions | allowed tools, allowed domains, allowed write scopes |

### 7.4 Policy Compiler

The compiler should:

1. parse source policies,
2. normalize rule classes,
3. validate rule consistency,
4. produce machine-executable rules,
5. emit compatibility warnings for conflicting rules.

This is the component that turns `AGENTS.md` philosophy into runtime enforcement.

---

## 8. Skill Architecture

### 8.1 Skill Package Standard

Canonical structure:

```text
skill-package/
├── SKILL.md
├── manifest.json
├── agents/provider.yaml
├── references/
├── scripts/
└── assets/
```

### 8.2 Skill Registry Model

| Object | Purpose |
|---|---|
| `SkillPackage` | Logical skill identity |
| `SkillVersion` | Versioned release of a skill |
| `SkillManifest` | Structured metadata and requirements |
| `SkillValidationResult` | Lint, compatibility, and health result |
| `SkillRolloutPolicy` | Who can use the skill and under what rollout state |
| `SkillTrustProfile` | Risk level, required approvals, and allowed task classes |

### 8.3 Skill Lifecycle

1. author
2. validate
3. review
4. approve
5. publish
6. canary
7. general availability
8. deprecate
9. retire

### 8.4 Required Skill Controls

Every production skill should declare:

1. allowed task types,
2. required tools,
3. required approvals if high risk,
4. whether it can write code,
5. whether it can invoke external integrations,
6. whether it can handle regulated data classes.

---

## 9. Controller and Execution Architecture

### 9.1 Controller Responsibilities

For each request, the controller must:

1. parse the request,
2. build effective policy,
3. select the allowed skills,
4. construct a minimal context packet,
5. create a task graph,
6. route tasks to execution backends,
7. collect findings and receipts,
8. enforce transitions and approvals,
9. produce closure and release packets.

### 9.2 Controller Inputs

1. request packet,
2. effective policy,
3. available skill versions,
4. tenant profile,
5. workflow context,
6. prior evidence and review state.

### 9.3 Execution Runtime

Execution runtime should provide:

1. step checkpointing,
2. retries,
3. idempotency,
4. pause/resume,
5. timeout control,
6. tool access mediation,
7. artifact capture,
8. policy decision capture.

### 9.4 Why Workflow Runtime Matters

This subsystem should not run as a loose sequence of prompts.

It needs durable execution because:

1. tasks may run for hours,
2. approvals may pause execution,
3. retries must be safe,
4. audit trails must survive restarts,
5. release packets need exact history.

---

## 10. Data and Contract Model

The subsystem should define first-class records for:

| Record | Purpose |
|---|---|
| `AgentProfile` | Declares execution backend identity and capability envelope |
| `TaskPacket` | Unit of assigned work |
| `ReviewPacket` | Independent review findings |
| `EvidenceReceipt` | Proof of verification |
| `ApprovalReceipt` | Human authorization for gated action |
| `ExecutionRun` | Top-level runtime instance |
| `ExecutionStep` | Individual step inside a run |
| `Artifact` | File, diff, document, trace, or generated asset |
| `ReleasePacket` | Consolidated release readiness record |

### 10.1 Typed Contract Requirement

Critical packets must be schema-validated and versioned.

Recommended implementation:

1. Pydantic if Python control plane
2. Zod if TypeScript control plane

No critical state should be “best effort parsed from chat text.”

---

## 11. Multi-Tenant Isolation Model

Because the platform is multi-tenant, policy and skill behavior must be tenant-aware.

### 11.1 Isolation Requirements

1. tenant-specific policy overlays,
2. tenant-specific skill availability,
3. tenant-specific tool entitlements,
4. tenant-specific retention settings,
5. tenant-specific compliance hold and approval rules.

### 11.2 Capability Resolution

For a given task, the platform should compute:

1. what skills are globally available,
2. what skills are allowed for this plan tier,
3. what skills are enabled for this tenant,
4. what skills are disallowed by policy,
5. what skill version is pinned for this environment.

### 11.3 Data Access Boundaries

The subsystem must ensure:

1. task context only contains authorized tenant data,
2. cross-tenant memory is impossible by default,
3. skill references do not leak restricted data,
4. agent outputs inherit tenant classification labels.

---

## 12. Security, Governance, and Audit

### 12.1 Security Requirements

1. no raw secrets in skills,
2. all integration credentials routed through approved adapters,
3. destructive actions require explicit approval,
4. all high-risk actions generate immutable audit events,
5. tool access is mediated and scoped.

### 12.2 Governance Requirements

1. no self-approval by the executing agent,
2. no policy bypass without override receipt,
3. no closure with unresolved required findings,
4. no release without verification summary and approvals,
5. no undocumented scope changes.

### 12.3 Audit Requirements

Every run should capture:

1. request source,
2. effective policy version,
3. skill versions selected,
4. task packets issued,
5. tools invoked,
6. approvals received,
7. receipts generated,
8. final disposition.

This is especially important for enterprise and regulated environments.

---

## 13. Review, Verification, and Release Architecture

### 13.1 Review Model

The subsystem should support:

1. document review,
2. code review,
3. release review.

Each should use structured findings, not just free-text summaries.

### 13.2 Verification Model

Verification should be policy-bound by task class.

Examples:

1. code change -> typecheck + impacted tests + build
2. deployment change -> config validation + smoke + rollback packet
3. architecture change -> review + approval + compatibility impact

### 13.3 Release Packet Model

The release subsystem should collect:

1. verified scope,
2. open findings count,
3. risk level,
4. rollback plan,
5. final approvals.

The release packet is the trusted closure artifact for the subsystem.

---

## 14. Observability and Operations

### 14.1 Required Telemetry

The platform should emit metrics by:

1. tenant,
2. workspace,
3. policy version,
4. skill version,
5. agent profile,
6. task type,
7. review state,
8. release state.

### 14.2 Key Metrics

| Metric | Why It Matters |
|---|---|
| Skill invocation frequency | Detect dead or critical skills |
| Policy block rate | Detect excessive friction or risky activity |
| Approval latency | Measure operational bottlenecks |
| Reopen rate after closure | Measure weak verification or review quality |
| Review finding escape rate | Measure effectiveness of review controls |
| Release variance | Measure predictability |

### 14.3 Operational Requirements

1. canary rollout for skills and policies,
2. rapid rollback for broken skill versions,
3. replayable task runs,
4. incident correlation by run ID and task ID.

---

## 15. Recommended Service Topology

### 15.1 Core Services

| Service | Role |
|---|---|
| `policy-service` | Stores and resolves policies |
| `policy-compiler` | Converts source rules into executable forms |
| `skill-registry` | Stores and serves skill packages and versions |
| `controller-service` | Main orchestration brain |
| `execution-broker` | Connects controller to agent runtimes |
| `review-service` | Handles findings and convergence |
| `evidence-service` | Stores receipts and artifacts |
| `approval-service` | Human approval and overrides |
| `audit-service` | Immutable event stream and compliance export |

### 15.2 Backing Infrastructure

| Component | Recommendation |
|---|---|
| Metadata store | PostgreSQL |
| Artifact store | Object storage |
| Queue / workflow runtime | Durable workflow engine |
| Search / retrieval | Search index + vector layer |
| Metrics / logs / traces | Central observability stack |

---

## 16. Reference Request Lifecycle

For one production request, the architecture should behave like this:

1. user submits request,
2. controller creates request packet,
3. policy service resolves effective policy,
4. controller selects allowed skills,
5. context builder creates scoped context,
6. task packets are created,
7. execution broker routes tasks,
8. review packets are created if needed,
9. evidence receipts are collected,
10. approval service blocks or releases transitions,
11. controller produces closure packet,
12. audit service records the full chain.

That is the actual production implementation of `AGENTS.md + skills`.

---

## 17. Maturity Roadmap

### Stage 1: Structured but Local

1. local skill registry,
2. local policy resolution,
3. typed packets,
4. evidence receipts.

### Stage 2: Team Operating System

1. shared registry,
2. controller service,
3. review service,
4. approval workflows,
5. audit trail.

### Stage 3: Tenant-Aware SaaS Subsystem

1. tenant policy overlays,
2. skill rollout policies,
3. integration adapters,
4. multi-environment release controls.

### Stage 4: Enterprise AaaS Foundation

1. compliance export,
2. compliance hold,
3. region-aware policy,
4. skill/policy canaries,
5. large-scale observability and predictability metrics.

---

## 18. Open Architecture Decisions

| ID | Decision | Alternatives | Recommended Option | Why |
|---|---|---|---|---|
| AD-1 | Policy source of truth | Markdown only, JSON only, dual source | Dual source | Human-readable intent plus machine enforcement |
| AD-2 | Packet schema stack | Python/Pydantic, TypeScript/Zod | Either; choose one per control plane | Strong typing is mandatory |
| AD-3 | Skill storage | Filesystem only, registry DB, hybrid | Hybrid | Native compatibility with governed rollout |
| AD-4 | Controller runtime | Stateless jobs, durable workflow engine | Durable workflow engine | Needed for approvals, retries, long runs |
| AD-5 | Review enforcement | Advisory only, hard-gated | Hard-gated | Required for regulated trust model |
| AD-6 | Tenant isolation for skills | Global only, tenant overlays | Tenant overlays | Necessary for real enterprise customers |

---

## Closing Statement

The right professional implementation is not:

1. a markdown file of rules, plus
2. a folder of skills.

The right implementation is:

1. a **policy system** derived from `AGENTS.md`,
2. a **skill registry** derived from skill packages,
3. a **controller** that interprets both,
4. a **typed execution protocol**,
5. an **evidence and review system**,
6. a **governed audit trail**.

That is the proper architecture for `AGENTS.md + skills` as a platform subsystem inside an enterprise Agents-as-a-Service SaaS product.
