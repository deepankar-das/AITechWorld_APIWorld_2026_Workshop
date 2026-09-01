> **AgenticDevelopmentModel**
> Copyright © 2026 AgenticDevelopmentModel Inc. Proprietary and Confidential.
>
> Author: Deepankar Das

# Receipt Rules for Commit Commands

## Programmatic G6 + SCOPE check (BLOCKING)

Before writing any commit command, run the scope/metric-threshold check
covering ALL files across ALL proposed commits in a single invocation.
Load the repository's `<scope-change config>` and
`<metric-threshold allowlist config>`, then for each staged file print
whether it triggers G6 (it appears in the metric-threshold class list)
and/or SCOPE (it matches an exact path, prefix, or regex rule).

Do not generate commit commands until the output of this check is
visible. This check uses the live config files — do not guess from
memory.

## G6 receipt shape

Required when a commit contains files listed in the
`<metric-threshold allowlist config>`.

```
Counter-example checked: <one sentence>
Pre-fix grep: <command + result>
Post-fix grep: <command + result>
Cross-agent verifier: self-verified — not independently checked
```

Never put agent names in commit messages.

## SCOPE receipt shape

Required when a commit contains files matching any rule in the
`<scope-change config>`.

```
Decision: <what scope changed>
Old scope: <previous scope>
New scope: <new scope>
Reason: <why needed>
Approved by: Deepankar Das
Date: <YYYY-MM-DD>
```

## Embedding rules

- G6 only: include G6 stanza.
- SCOPE only: include SCOPE stanza.
- Both: include both stanzas.
- Neither: no stanza needed.

## Commit message conventions

- Imperative mood, no period at end of title.
- Title under 72 characters.
- Body explains the "why".
- Never include `Co-Authored-By`, agent names, or AI attribution.
- Quote file paths with spaces in `git add`.
