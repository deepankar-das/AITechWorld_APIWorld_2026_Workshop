---
name: adm-checkin
description: >
  AgenticDevelopmentModel commit command preparation with G1-G6 gate receipts, SCOPE
  approval receipts, and scope separation enforcement. Use whenever
  the user asks to commit, checkin, check in, land changes, prepare
  commits, stage and commit, or says "commit command", "checkin
  command", "receipt", "ready to commit", "commit with receipt", or
  "commit all". Also use when the user says "all" after a partial
  commit listing — they want the remaining commits generated to the
  same standard. This skill is MANDATORY for every commit command
  suggestion — never write a git commit command without it.
---
> **AgenticDevelopmentModel**
> Copyright © 2026 AgenticDevelopmentModel Inc. Proprietary and Confidential.
>
> Author: Deepankar Das

# AgenticDevelopmentModel Checkin Command Generator

This skill produces ready-to-paste `git add` + `git commit` commands
with all hook-required receipts pre-embedded. It exists because the
repository has two commit-msg hooks (G6 metric-threshold receipt and
SCOPE approval receipt) that reject commits missing required stanzas,
and the agent has repeatedly failed to include them when generating
commit commands from memory alone. The fix: a rigid, programmatic
procedure that checks the actual config files every time.

**This is a rigid skill. Follow every step in order. Do not skip the
programmatic check. Do not write commit commands before the check
results are in the conversation.**

## Procedure

### Step 1 — Gather workspace state

Run in parallel:

```bash
git status                  # modified + untracked files
git diff --stat             # change size summary
git diff                    # full diff (read to understand scopes)
git log --oneline -10       # recent commit style
```

Read the full diff. Understand what each file change does.

### Step 2 — Identify separate scopes

Per CLAUDE.md, bundled commits joining two scopes with `+` are BANNED.
Group files into single-purpose commits. Common scope boundaries:

- Bug fix vs. refactor vs. documentation vs. test infrastructure
- Tightly coupled files (a seed-constants change + the invariant test
  that pins it) belong together
- Unrelated changes in the same directory are still separate scopes

List the proposed commits with their file sets before proceeding.

### Step 3 — Run commit gates G1, G2, G3, G5

Run in parallel where independent:

```bash
npx tsc --noEmit                                    # G1
npx vitest run <impacted-test-files>                # G2 + G3
npx vite build                                      # G5
```

For G2/G3, select tests impacted by the diff:
- Invariant tests that pin touched modules
- Unit/component tests that import or exercise changed files
- New test files being added in this changeset

All gates must pass before proceeding. If any fail, fix first.

### Step 4 — Programmatic G6 + SCOPE check (BLOCKING)

**This step is non-negotiable.** Run the scope/metric-threshold check
covering ALL files across ALL proposed commits in a single invocation,
loading the repository's `<scope-change config>` and
`<metric-threshold allowlist config>`. For each staged file, print
whether it triggers G6 (metric-threshold class) and/or SCOPE
(exact-path, prefix, or regex match).

**Do not proceed to Step 5 until the output of this check is visible
in the conversation.** This is the step that has been missed 5+ times.

Record which files triggered G6 and which triggered SCOPE. Map each
triggered file back to its proposed commit from Step 2.

### Step 5 — Build the gate receipt table

Produce a summary table:

```
| Gate | Result |
|------|--------|
| G1 tsc --noEmit | PASS (0 errors) |
| G2 impact-selected tests | PASS — <file> N/N |
| G3 invariant tests | PASS — <file> N/N |
| G4 reviewer checklist | See below |
| G5 vite build | PASS (Xs) |
| G6 counter-example | N/A -or- required for <files> |
| SCOPE approval | N/A -or- required for <files> |
```

### Step 6 — G4 reviewer checklist

Read the `adm-commit-gates` skill for the full G4 checklist.
Answer the applicable questions based on what categories the changes
fall into (request-handling, database, pool, frontend, test).

### Step 7 — Generate commit commands

For each commit from Step 2, produce a ready-to-paste block.
Read `references/receipt-rules.md` for the exact receipt stanza
formats and embedding rules.

Commit message conventions:
- Imperative mood, no period at end of title
- Title under 72 characters
- Body explains the "why", not just the "what"
- Never include `Co-Authored-By`, agent names, or AI attribution
- Quote file paths with spaces in `git add` commands

### Step 8 — Present the full sequence

List all commits in suggested execution order with a summary table:

```
| # | Scope | Files | G6 | SCOPE |
|---|-------|-------|----|-------|
| 1 | ... | ... | N/A | N/A |
| 2 | ... | ... | N/A | Required |
```

## References

- Read `references/receipt-rules.md` when generating commit commands (Step 7).

## Reminders

- **Never run `git commit` or `git push`.** Only suggest the commands.
- **Never skip Step 4.** The programmatic check is the entire reason
  this skill exists.
- **File paths with spaces** must be double-quoted in `git add`.
- **New untracked files** that are part of a scope go in the same
  commit as the modified files they relate to.
- **The user saying "all"** after a partial listing means generate
  commit commands for every remaining uncommitted file to the same
  standard, including re-running Step 4 for any new files.
