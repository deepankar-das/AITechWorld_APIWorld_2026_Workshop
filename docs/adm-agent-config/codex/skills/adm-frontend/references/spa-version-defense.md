> **AgenticDevelopmentModel**
> Copyright © 2026 AgenticDevelopmentModel Inc. Proprietary and Confidential.
>
> Author: Deepankar Das
# SPA Version Defense

## Required layers

1. Cache headers: `index.html` non-cached, static assets content-hashed.
2. Chunk recovery: every `lazy()` import wraps `lazyRetry()`.
3. Navigation sync: periodic version checks trigger reload on navigation when mismatch detected.
4. API guard: `X-App-Version` mismatch on `/api/*` mutations forces client reload.

## Required lazy pattern

```typescript
const NewPage = lazy(() => lazyRetry(() => import("@/pages/new-page")));
```

Do not use a raw `lazy(() => import(...))` for routable pages.
