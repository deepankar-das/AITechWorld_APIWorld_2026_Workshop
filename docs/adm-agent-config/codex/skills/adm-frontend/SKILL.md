---
name: adm-frontend
description: >
  AgenticDevelopmentModel frontend route, SPA-version-defense, UI regression, and React page
  development procedures. Use when modifying client code, routes, page
  composition, lazy imports, responsive behavior, or SPA deploy behavior.
---
> **AgenticDevelopmentModel**
> Copyright © 2026 AgenticDevelopmentModel Inc. Proprietary and Confidential.
>
> Author: Deepankar Das

# AgenticDevelopmentModel Frontend Development

## When to use

Use this skill for React routes, lazy imports, UI flows, responsive layout, SPA cache/version behavior, or frontend regression analysis.

## Workflow

1. Confirm the page type and the matching test expectations.
2. If adding or changing a lazy route, wrap it with `lazyRetry()`.
3. Preserve the four-layer SPA version-defense contract.
4. Pair UI changes with data assertions, error-path checks, responsive validation, and accessibility coverage.

## Core modules

- `<client route registry>`
- `<version-manager module>`
- `<API client module>`
- `<server entrypoint>`

## References

Read `references/spa-version-defense.md` for the required lazy-route and version-mismatch behavior.
