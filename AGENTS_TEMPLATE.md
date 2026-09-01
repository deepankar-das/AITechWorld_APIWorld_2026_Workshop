> Copyright © {{YEAR}} {{AUTHOR}}. Proprietary and Confidential.
> Author: {{AUTHOR}}

# {{PROJECT_NAME}} -- Codex Agent Instructions

> **{{PROJECT_NAME}}: {{ONE_LINE_DESCRIPTION}}**

This file configures Codex (OpenAI) agents working on this repository. For Claude Code instructions, see [CLAUDE.md](CLAUDE.md).

---

## Repository Reality (Read First)

<!-- Describe what actually exists in the repo right now. Update this section as the repo evolves. This prevents agents from hallucinating structure that doesn't exist. -->

This repository currently contains:

{{REPO_CONTENTS_LIST}}

When asked to implement code, first state whether work will:
1. modify existing files only, or
2. introduce new repository topology (runtime, package manager, source tree, tests).

If introducing topology, present a short architecture decision with tradeoffs before implementing.

---

## Product Context

{{PRODUCT_SUMMARY}}

Canonical docs to read before major edits:

<!-- List only documents that actually exist. Remove any that don't. -->

| Document | Purpose |
|----------|---------|
| {{DOC_LINK}} | {{DOC_PURPOSE}} |

---

## Non-Negotiable Standards

These apply to every change:

| Requirement | Rule |
|---|---|
| **Evidence-first** | Do not claim files, commands, tests, or architecture exist unless verified in this repo. |
| **No fabricated implementation state** | Do not report code paths, APIs, or running systems that are not present. |
| **Doc integrity** | Preserve meaning and traceability when editing PRD/TDD content; do not silently remove constraints. |
| **Reference integrity** | Keep links and file names accurate to existing repository paths. |
| **Scope control** | Do not silently convert documentation tasks into codebase scaffolding without explicit approval. |

<!-- Add project-specific standards below. Examples: -->
<!-- | **Audit trail immutability** | No UPDATE or DELETE on audit events. Append-only. | -->
<!-- | **Secrets never in logs** | Redact credentials and tokens from all output. | -->

---

## Per-Commit Gates

Every commit should pass:

| Gate | Command | Blocks on |
|---|---|---|
| G1 -- Structure check | `find . -maxdepth 2 -type d \| sort` | Instructions reference non-existent structure as current state |
| G2 -- File inventory check | `ls -1 {{DOCS_DIR}} \| sort` | Missing or incorrect file references |
| G3 -- Reviewer checklist | See below | Any unchecked item |

<!-- Once code exists, add build/test/lint gates: -->
<!-- | G4 -- Type check | `{{TYPECHECK_CMD}}` | Any new error | -->
<!-- | G5 -- Unit tests | `{{TEST_CMD}}` | Any failure | -->
<!-- | G6 -- Build | `{{BUILD_CMD}}` | Failure | -->

### Reviewer Checklist (G3)

**Documentation and configuration changes:**
- [ ] All referenced files are present in this repository
- [ ] Claims are marked as Verified, Inferred, or Unknown when certainty matters
- [ ] Dates and version statements are concrete where needed
- [ ] If multiple document variants exist, the target document is explicitly named
- [ ] Constraints were preserved (no silent requirement drop)

<!-- Once code exists, add code-specific checklists: -->
<!-- **API changes:** -->
<!-- - [ ] Is the endpoint authenticated? -->
<!-- - [ ] Are error responses sanitized? -->
<!-- - [ ] Is the endpoint covered by at least one test? -->

---

## No Dropped Commitments

Every item the user asks for must ship. No silent deferrals.

- Never drop, defer, or remove an item without explicit permission.
- When narrowing scope, ask first.
- Keep unfinished commitments visible in the task list.
- If an item was dropped, acknowledge it and complete it.

---

## Banned Patterns

- Bundled commits that join multiple scopes.
- Referencing a non-existent source tree as current implemented state.
- Invented command gates for toolchains not present in this repo.
- Silent scope switch from documentation edits to architecture implementation.
- Reporting verification that was not actually run.

<!-- Add project-specific banned patterns below. Examples: -->
<!-- - Hard DELETE on any table containing {{SENSITIVE_DATA_TYPE}}. -->
<!-- - `test.skip` / `test.todo` -- skipped tests provide zero coverage. -->

---

## Git Commit Rules

- Never run `git commit`, `git push`, or any git write commands. The developer handles all commits.
- Never add `Co-Authored-By` trailers or AI attribution to commit messages.
- When asked about commits, only suggest the commit message; do not execute it.

---

## Architecture Decision Gate

Before implementing any change that affects repository topology (new runtime code, package manager, CI layout, test harness, database schema, auth flow, or public interfaces), present the decision, alternatives, and tradeoffs for developer approval.

---

## Project Structure

<!-- Keep this accurate to the actual repo state. Update as files are added. -->

```text
{{PROJECT_NAME}}/
├── AGENTS.md
├── CLAUDE.md
└── {{DOCS_DIR}}/
    └── ...
```

---

## Ownership

<!-- Define who builds what when multiple agents or developers are involved. -->

| Owner | Scope |
|---|---|
| **Codex** | {{CODEX_SCOPE}} |
| **Claude** | {{CLAUDE_SCOPE}} |
| **Developer** | Final decisions on canonical documents, repo topology, implementation sequencing |

<!-- If no code exists yet: -->
<!-- Do not assume split code ownership modules exist yet; this repo does not currently contain implementation modules. -->

---

## Running and Verification

<!-- List actual runnable commands. If none exist yet, say so explicitly. -->

{{COMMANDS_OR_NONE}}

---

## Key Commands

| Task | Command |
|---|---|
| List repository files | `rg --files` |
| List docs | `ls -1 {{DOCS_DIR}} \| sort` |
| Find stale references | `rg -n "{{STALE_PATTERN}}" AGENTS.md CLAUDE.md` |
| Basic structure check | `find . -maxdepth 2 -type d \| sort` |

---

## Evidence-First Decision Making

- Never assert something as fact without evidence (file read, command output, grep result).
- Never guess at file contents or function signatures; read files first.
- Never assume a state is verified without running the check.
- Classify claims as Verified, Inferred, or Unknown.
- When uncertain, say "I do not know" and investigate.
