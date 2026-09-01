> **AgenticDevelopmentModel**
> Copyright © 2026 AgenticDevelopmentModel Inc. Proprietary and Confidential.
>
> Author: Deepankar Das

# Receipt Rules for Commit Commands

## G6 receipt shape

Required when a commit contains files listed in the
`<metric-threshold allowlist config>`.

Append to the commit message body:

```
Counter-example checked: <one sentence describing what was verified>
Pre-fix grep: <command + result>
Post-fix grep: <command + result>
Cross-agent verifier: self-verified — not independently checked
```

Never put agent names (Claude, Codex, AI) in the Cross-agent verifier
line or anywhere in the commit message.

## SCOPE receipt shape

Required when a commit contains files matching any rule in the
`<scope-change config>` (exact paths, prefix matches, or regex
patterns).

Append to the commit message body:

```
Decision: <what enforcement/status/test-execution scope changed>
Old scope: <previous scope>
New scope: <new scope>
Reason: <why the scope change was needed>
Approved by: Deepankar Das
Date: <YYYY-MM-DD>
```

## Embedding rules

- If a commit triggers G6 only: include G6 stanza.
- If a commit triggers SCOPE only: include SCOPE stanza.
- If a commit triggers BOTH: include both stanzas.
- If a commit triggers NEITHER: no stanza needed.

## Commit command format

```bash
git add <file1> <file2> ...
git commit -m "<title line>

<body paragraph>

<receipt stanzas if required>"
```

- Quote file paths containing spaces with double quotes.
- Imperative mood in title, no period at end.
- Title under 72 characters.
- Body explains the "why", not just the "what".
- Never include `Co-Authored-By`, agent names, or AI attribution.
