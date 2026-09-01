> Author: Deepankar Das
# Enforcer - Codex Agent Instructions

> **Enforcer: Runtime governance for AI coding agents.**

This file configures Codex (OpenAI) agents working on this Enforcer repository. For Claude Code instructions, see [CLAUDE.md](CLAUDE.md).

---

## Repository Reality (Read First)

This repository is currently documentation-first. As of this version, it contains:
- `AGENTS.md`
- `CLAUDE.md`
- `docs/*.md` product and technical documents for Enforcer
- no `app/`, `lib/`, `components/`, `tests/`, `scripts/`, or `package.json`

When asked to implement code, first state whether work will:
1. modify docs only, or
2. introduce new repository topology (runtime, package manager, source tree, tests).

If introducing topology, present a short architecture decision with tradeoffs before implementing.

---

## Product Context

Enforcer is a security and policy layer between AI coding agents and the systems they touch. Core themes across docs: interception, policy enforcement, approval workflows, auditability, and enterprise rollout safety.

Canonical docs to read before major edits:
- [Enforcer_PRD_Final.md](docs/Enforcer_PRD_Final.md)
- [Enforcer_TDD_Final.md](docs/Enforcer_TDD_Final.md)
- [Enforcer_TDD_Final_2.md](docs/Enforcer_TDD_Final_2.md)
- [Enforcer_TDD.md](docs/Enforcer_TDD.md)
- [Enforcer_TDD_Addendum.md](docs/Enforcer_TDD_Addendum.md)
- [enforcer_market_study_mrd.md](docs/enforcer_market_study_mrd.md)
- [enforcer_prd_vscode_agents.md](docs/enforcer_prd_vscode_agents.md)

---

## Non-Negotiable Standards

These apply to every change:

| Requirement | Rule |
|---|---|
| Evidence-first | Do not claim files, commands, tests, or architecture exist unless verified in this repo. |
| No fabricated implementation state | Do not report code paths, APIs, or running systems that are not present. |
| Doc integrity | Preserve meaning and traceability when editing PRD/TDD content; do not silently remove constraints. |
| Reference integrity | Keep links and file names accurate to existing repository paths. |
| Scope control | Do not silently convert documentation tasks into codebase scaffolding without explicit approval. |

---

## Per-Commit Gates

Every commit should pass:

| Gate | Command | Blocks on |
|---|---|---|
| G1 - Repository structure check | `find . -maxdepth 2 -type d | sort` | Instructions reference non-existent top-level structure as current state |
| G2 - Docs inventory check | `ls -1 docs | sort` | Missing or incorrect doc references |
| G3 - Reviewer checklist | See below | Any unchecked item |

### Reviewer Checklist (G3)

For `docs/*.md`, `AGENTS.md`, `CLAUDE.md` changes:
- [ ] All referenced files are present in this repository
- [ ] Claims are marked as Verified, Inferred, or Unknown when certainty matters
- [ ] Dates and version statements are concrete where needed
- [ ] If multiple PRD/TDD variants exist, the target document is explicitly named
- [ ] Constraints were preserved (no silent requirement drop)

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

```text
Enforcer/
├── AGENTS.md
├── CLAUDE.md
└── docs/
    ├── Enforcer Prompt.md
    ├── Enforcer_PRD_Final.md
    ├── Enforcer_Prompt.md
    ├── Enforcer_Prompt_aifund.md
    ├── Enforcer_TDD.md
    ├── Enforcer_TDD_Addendum.md
    ├── Enforcer_TDD_Final.md
    ├── Enforcer_TDD_Final_2.md
    ├── enforcer_market_study_mrd.md
    ├── enforcer_prd_1.md
    ├── enforcer_prd_2.md
    ├── enforcer_prd_3.md
    └── enforcer_prd_vscode_agents.md
```

---

## Working Ownership (Current Repo State)

| Owner | Scope |
|---|---|
| Codex/Claude agents | Documentation updates, consistency checks, requirements clarification, architecture tradeoff capture |
| Developer | Final decisions on canonical PRD/TDD versions, repo topology changes, implementation sequencing |

Do not assume split code ownership modules exist yet; this repo does not currently contain implementation modules.

---

## Running and Verification

No runnable application commands are currently present in this repository.

---

## Key Commands

| Task | Command |
|---|---|
| List repository files | `rg --files` |
| List docs | `ls -1 docs | sort` |
| Find stale references | `rg -n "Syntra_|syntra/|app/|lib/|tests/|scripts/" AGENTS.md CLAUDE.md` |
| Basic structure check | `find . -maxdepth 2 -type d | sort` |

---

## Evidence-First Decision Making

- Never assert something as fact without evidence (file read, command output, grep result).
- Never guess at file contents or function signatures; read files first.
- Never assume a state is verified without running the check.
- Classify claims as Verified, Inferred, or Unknown.
- When uncertain, say "I do not know" and investigate.
