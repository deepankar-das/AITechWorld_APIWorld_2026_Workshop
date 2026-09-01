---
name: adm-checkin
description: >
  AgenticDevelopmentModel commit command preparation with G1-G6 gate receipts, SCOPE
  approval receipts, and scope separation enforcement. Use when
  preparing commit commands, checking in code, or generating commit
  messages with required hook receipts.
---
> **AgenticDevelopmentModel**
> Copyright © 2026 AgenticDevelopmentModel Inc. Proprietary and Confidential.
>
> Author: Deepankar Das

# AgenticDevelopmentModel Checkin Command Generator

## When to use

Use this skill when the task involves generating commit commands,
preparing code for checkin, or producing commit messages that must
pass the G6 and SCOPE commit-msg hooks.

## Workflow

1. Gather workspace state: `git status`, `git diff --stat`, `git diff`, `git log --oneline -10`. Read the full diff.
2. Group files into single-purpose commits. No bundled scopes.
3. Run gates G1-G5 for the impacted scope.
4. **Before writing any commit command**, run the programmatic G6 + SCOPE check against ALL files. Read `references/receipt-rules.md` for the check script and receipt formats.
5. Generate commit commands with receipt stanzas embedded where required.
6. Present a summary table of all commits with G6/SCOPE applicability.

## Required checks

- `G1`: `npx tsc --noEmit`
- `G2`: impact-selected `npx vitest run <subset>`
- `G3`: impacted invariant tests
- `G4`: reviewer checklist from `adm-commit-gates` skill
- `G5`: `npx vite build`
- `G6`: receipt rules from `references/receipt-rules.md`
- `SCOPE`: receipt rules from `references/receipt-rules.md`

## References

- Read `references/receipt-rules.md` when generating commit commands — contains the programmatic check script, receipt stanza formats, and embedding rules.
