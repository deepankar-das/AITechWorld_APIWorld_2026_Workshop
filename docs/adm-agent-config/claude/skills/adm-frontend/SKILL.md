---
name: adm-frontend
description: >
  AgenticDevelopmentModel frontend development procedures for React SPA. Use when
  modifying client code, adding routes, changing components, or when
  the user says "frontend", "React", "component", "route", "lazy",
  "CSS", "UI", "page", "sidebar", "SPA", or "version defense".
  Covers lazyRetry wrapping, SPA version defense (4 layers),
  version-manager, API guard, and the client route registry.
---

# AgenticDevelopmentModel Frontend Development

## SPA Version Defense (MANDATORY for all frontend changes)

After every deploy, users with the SPA open must seamlessly get new code. AgenticDevelopmentModel uses a 4-layer automatic defense — **no manual cache clear or banner click required**.

- **Layer 1 (Cache Headers)**: `index.html` = `no-cache`; JS/CSS = content-hashed + `immutable`. Already configured in the `<static-asset middleware module>`.
- **Layer 2 (Chunk Recovery)**: All `lazy()` imports MUST be wrapped with `lazyRetry()`.
- **Layer 3 (Navigation Sync)**: Version polling every 60s; auto-reload on next route change when mismatch detected.
- **Layer 4 (API Guard)**: `X-App-Version` header on every `/api/*` response. Client interceptor reloads immediately on mutation mismatch.

## lazyRetry() Wrapping Requirement

**When adding new lazy-loaded pages:** Always wrap with `lazyRetry()`:

```typescript
// Correct
const NewPage = lazy(() => lazyRetry(() => import("@/pages/new-page")));

// Wrong — will cause white screen after deploy
const NewPage = lazy(() => import("@/pages/new-page"));
```

This is enforced by invariant test INV6: `<lazy-retry invariant test>`.

## Key Modules

| Module | Purpose |
|--------|---------|
| `<client route registry>` | Client route registry — all `lazy()` imports go here |
| `<version-manager module>` | SPA version defense (`lazyRetry`, polling, API guard) |
| `<API client module>` | API interceptor with `X-App-Version` mismatch detection |
| `<server entrypoint>` | `X-App-Version` middleware |
| `<static-asset middleware module>` | Cache header configuration |

## Troubleshooting Stale SPA

See the `<debugging FAQ>` Section 4 for troubleshooting stale SPA issues after deploy.
