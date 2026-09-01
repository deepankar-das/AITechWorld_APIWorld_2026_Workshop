> **AgenticDevelopmentModel**
> Copyright © 2026 AgenticDevelopmentModel Inc. Proprietary and Confidential.
>
> Author: Deepankar Das
# Reviewer Checklist

## Request and middleware changes

- Pin middleware order assumptions with test evidence.
- Confirm every non-2xx path is tested.
- Verify reads of `req.session`, `req.user`, `req.tenantContext`, or the tenant id only happen after the middleware that sets them.

## Schema changes

- Confirm the `<shared schema module>` matches the `<platform-schema bootstrap module>` or `<tenant-schema module>` as applicable.
- Confirm idempotent DDL and verification checks exist.

## Pool and concurrency changes

- Confirm capacity math and thresholds are pinned by tests.

## Frontend route changes

- Confirm new lazy routes use `lazyRetry()`.
- Confirm SPA version-defense expectations remain intact.

## Test changes

- Reject escape hatches: `skip`, conditional pass paths, permissive assertions, swallowed failures, or empty-state-only checks.
