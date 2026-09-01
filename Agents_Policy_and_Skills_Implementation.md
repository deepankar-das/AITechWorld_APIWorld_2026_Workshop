> **AgenticDevelopmentModel**
> Copyright © 2026 AgenticDevelopmentModel Inc. Proprietary and Confidential.
>
> Author: Deepankar Das

# Agents Policy and Skills Framework — Use Case 1 Implementation

**Version:** 1.2
**Date:** 2026-05-21
**Parent Context:** [Codex_Use_Cases.md](docs/Codex_Use_Cases.md)
**Related:** [AGENTS.md](AGENTS.md), the Claude skills implementation doc, the platform architecture doc, [Agents_Policy_and_Skills_Platform_Architecture.md](Agents_Policy_and_Skills_Platform_Architecture.md)
**Status:** Use Case 1 Implemented; Use Case 2 Deferred
**Scope:** Codex-side operating framework for building AgenticDevelopmentModel more correctly, more consistently, and with lower always-loaded context cost

---

## Table of Contents

1. [Overview](#1-overview)
2. [Use Case Boundary](#2-use-case-boundary)
3. [Goals](#3-goals)
4. [Implemented Components](#4-implemented-components)
5. [What Was Explicitly Removed](#5-what-was-explicitly-removed)
6. [Phase 1 Deliverables](#6-phase-1-deliverables)
7. [Verification](#7-verification)
8. [Outcome](#8-outcome)
9. [Deferred Discussion](#9-deferred-discussion)

---

## 1. Overview

This document records the implemented Codex-side framework for **Use Case 1** only:

1. using Codex more effectively to build AgenticDevelopmentModel,
2. reducing token waste from oversized root instruction files,
3. making repeated engineering workflows reusable,
4. improving consistency for code, tests, documentation, debugging, and release-readiness support.

This document does **not** describe a product/runtime implementation of multi-agent services inside AgenticDevelopmentModel.

---

## 2. Use Case Boundary

The two use cases are defined in [Codex_Use_Cases.md](docs/Codex_Use_Cases.md).

### 2.1 In Scope Here

This implementation covers:

1. `AGENTS.md` compaction,
2. Codex skill extraction,
3. Codex skill install/lint/validate tooling,
4. reference decomposition and trigger audits,
5. Phase 1 documentation proving the refactor boundary.

### 2.2 Out of Scope Here

This implementation does not include:

1. controller services,
2. typed packet runtime contracts,
3. review/evidence/release services,
4. CLI orchestration framework,
5. productized multi-agent execution surfaces.

Any such material belongs to **Use Case 2** discussion only and is not part of the implemented Codex framework in this repository cleanup.

---

## 3. Goals

### 3.1 Primary Goals

1. keep [AGENTS.md](AGENTS.md) small enough to be an always-loaded policy shell,
2. move procedural detail into reusable Codex skills,
3. keep heavy detail in references instead of the root shell,
4. make repeated AgenticDevelopmentModel workflows faster and more consistent,
5. validate that no important root-file instruction was silently dropped.

### 3.2 Success Criteria

1. Codex can route common AgenticDevelopmentModel work through skills instead of a bloated root file.
2. The root shell retains non-negotiable policy and review constraints.
3. Skills are linted and validated locally.
4. The implementation is clearly scoped to developer-assistant use, not product/runtime agent infrastructure.

---

## 4. Implemented Components

### 4.1 Compact Root Policy Shell

Implemented:

1. a compact [AGENTS.md](AGENTS.md),
2. retention of non-negotiable policy, verification, quality-gate, approval-boundary, and file-header rules,
3. removal of long procedural repetition from the always-loaded shell.

### 4.2 Codex Skill Library

Implemented skill library under [.codex/skills](.codex/skills):

1. `adm-commit-gates`
2. `adm-controller-core`
3. `adm-deploy`
4. `adm-doc-review`
5. `adm-frontend`
6. `adm-implementation-plan`
7. `adm-radar`
8. `adm-release-readiness`
9. `adm-root-cause-debugging`
10. `adm-schema-change`
11. `adm-test-writing`

Note: `adm-controller-core` is retained as a **planning skill** for request decomposition and work routing during AgenticDevelopmentModel engineering work. It is not a shipped controller service.

### 4.3 Skill Tooling

Implemented:

1. skill installer: [install-skills.sh](scripts/codex/install-skills.sh)
2. skill linter: [lint_codex_skills.mjs](tools/skill_lint/lint_codex_skills.mjs)
3. skill validator: [validate_codex_skills.mjs](tools/skill_validate/validate_codex_skills.mjs)
4. skill manifest: [manifest.json](.codex/skills/manifest.json)
5. skill manifest reference doc: [skill-manifest-spec.md](docs/skills/skill-manifest-spec.md)

### 4.4 Phase 1 Audit Artifacts

Implemented documentation artifacts:

1. [instruction-decomposition-map.md](docs/phase1/instruction-decomposition-map.md)
2. [skill-trigger-matrix.md](docs/phase1/skill-trigger-matrix.md)
3. [coverage-audit.md](docs/phase1/coverage-audit.md)
4. [token-budget-report.md](docs/phase1/token-budget-report.md)

---

## 5. What Was Explicitly Removed

As part of this cleanup, previously introduced **Use Case 2** implementation code was removed.

Removed from implemented scope:

1. controller service code,
2. typed packet implementation code,
3. review/evidence/release service code,
4. controller CLI code,
5. product/runtime dogfood artifacts and related tests.

This cleanup was necessary because those implementations belonged to the separate product/runtime discussion, not to the immediate goal of improving Codex as an engineering agent for building AgenticDevelopmentModel.

---

## 6. Phase 1 Deliverables

| # | Deliverable | File(s) | Description | Status |
|:-:|-------------|---------|-------------|:------:|
| P1 | Root shell compaction | [AGENTS.md](AGENTS.md) | Retain mandatory policy while removing repeated procedural bulk from the always-loaded shell. | **Done** |
| P2 | Codex skill extraction | [.codex/skills](.codex/skills) | Move repeated AgenticDevelopmentModel engineering workflows into reusable Codex skills. | **Done** |
| P3 | Skill installation tooling | [install-skills.sh](scripts/codex/install-skills.sh) | Install repo-tracked skills into the Codex skill directory. | **Done** |
| P4 | Skill lint and validation | [lint_codex_skills.mjs](tools/skill_lint/lint_codex_skills.mjs), [validate_codex_skills.mjs](tools/skill_validate/validate_codex_skills.mjs) | Validate skill structure, metadata, and repo conventions. | **Done** |
| P5 | Coverage and trigger audit | [docs/phase1](docs/phase1) | Prove the root-file decomposition and skill-trigger coverage. | **Done** |
| P6 | Use-case boundary cleanup | [Codex_Use_Cases.md](docs/Codex_Use_Cases.md), this document | Separate implemented Codex-assistant scope from deferred product/runtime discussion. | **Done** |

---

## 7. Verification

The Codex-side Use Case 1 framework is verified by:

1. `node tools/skill_lint/lint_codex_skills.mjs`
2. `node tools/skill_validate/validate_codex_skills.mjs`
3. `bash -n scripts/codex/install-skills.sh`
4. `npx tsc --noEmit`
5. `npx vite build`

The purpose of `npx tsc --noEmit` and `npx vite build` here is not to prove a product/runtime controller system. It is to prove that the repository remains healthy after narrowing the implementation back to Use Case 1 only.

---

## 8. Outcome

The implemented outcome is:

1. Codex usage in AgenticDevelopmentModel is more organized,
2. repeated engineering workflows are skill-based instead of repeatedly stuffed into `AGENTS.md`,
3. the root shell is smaller and cheaper to keep loaded,
4. the repository no longer over-claims a product/runtime controller implementation that the user did not want built at this stage.

---

## 9. Deferred Discussion

The broader product/runtime architecture discussion remains documented, but it is **not implemented** as part of this cleanup.

Discussion-only references:

1. the platform architecture doc
2. [Agents_Policy_and_Skills_Platform_Architecture.md](Agents_Policy_and_Skills_Platform_Architecture.md)

Those documents should be used only when the separate Use Case 2 discussion is intentionally reopened.
