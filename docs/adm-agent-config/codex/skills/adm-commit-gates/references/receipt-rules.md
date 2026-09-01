> **AgenticDevelopmentModel**
> Copyright © 2026 AgenticDevelopmentModel Inc. Proprietary and Confidential.
>
> Author: Deepankar Das
# Receipt Rules

## G6 receipt shape

- `Counter-example checked: <one sentence>`
- `Pre-fix grep: <command + result>`
- `Post-fix grep: <command + result>`
- `Cross-agent verifier: <approved non-agent reviewer> independently re-greped at <commit/run-id>`

Never put agent names in commit messages.

## Scope receipt shape

- `Decision: <what scope changed?>`
- `Old scope: <previous scope>`
- `New scope: <new scope>`
- `Reason: <why needed>`
- `Approved by: <developer name>`
- `Date: <YYYY-MM-DD>`

## Commit-command rule

Before suggesting a commit command, inspect the staged diff and account for local commit hooks. Do not provide a command that will predictably fail G6 or SCOPE checks.
