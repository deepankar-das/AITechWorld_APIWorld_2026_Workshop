---
name: adm-implementation-plan
description: >
  AgenticDevelopmentModel implementation plan creation using standard templates.
  Use when creating a plan, designing a feature, writing an
  implementation document, or when the user says "implementation plan",
  "plan", "design doc", "phased table", "Template A", or "Template B".
  Covers Template A (feature/code) and Template B (test writing)
  with required structure, phase table format, and column semantics.
---

# AgenticDevelopmentModel Implementation Plan Templates

## Template Selection

- **Template A — Feature/Code Implementation**: Building features, fixing bugs, refactoring, adding endpoints, schema changes.
  - Reference: the `<feature-plan reference doc>`
- **Template B — Test Writing Implementation**: Writing new E2E tests, closing test gaps, building test matrices.
  - Reference: the `<test-writing plan reference doc>`

Full template documentation: the `<implementation-plan templates doc>`

## Required Document Structure (Template A)

1. Header: copyright, version, parent doc, related docs, status
2. Table of Contents
3. Section 1 — Overview: brief description + "Existing Infrastructure (Already Built)" table
4. Section 2 — Current Infrastructure: "Key Files" table + "Database Tables (Existing)" table
5. Section 3 — Implementation Plan: phased tables (see format below)
6. Progress Summary table
7. Priority Legend and Tests Column Legend

## Required Document Structure (Template B)

1. Mandatory Rules reference table (R1-R18 mapped to CLAUDE.md lines)
2. Per-Page UI Element Inventory
3. Per-Page Test Matrix: Rules x Elements — each cell = one test to write
4. Cross-Page Journey Tests
5. Backend Test Matrix: Tier 1 + Tier 2 pairing
6. Execution and Verification Plan
7. Progress Summary

## Phase Table Format

```markdown
### Phase X: Phase Name — Layer/Scope

One-sentence description of what this phase does.

| # | Item | File(s) | Description | Pri | Deps | Status | Tests |
|:-:|------|---------|-------------|:---:|:----:|:------:|:-----:|
| X1 | Item name | `file/path.ts` | Detailed description | High | None | Not Started | vitest + E2E |
```

## Column Semantics

- **#**: Phase letter + number (e.g., A1, B2, C3)
- **Item**: Short name (2-5 words)
- **File(s)**: Exact file paths with backticks
- **Description**: Detailed — include function names, API endpoints, field names, behavior
- **Pri**: High / Medium / Low
- **Deps**: Other item IDs this depends on (e.g., "A1, A2") or "None"
- **Status**: "Not Started" initially, then "**Done**" when complete with artifact receipt
- **Tests**: "vitest", "E2E", "vitest + E2E", "Covered", or "Build passes"
