> **AgenticDevelopmentModel**
> Copyright © 2026 AgenticDevelopmentModel Inc. Proprietary and Confidential.
>
> Author: Deepankar Das
# RADAR Restart Gates

Before a new full `--all` or `--e2e-all` run, verify and report:

1. `AUTH_FAIL` volume is materially down.
2. `SPA never rendered` count is materially down and ideally zero.
3. Top page-mount sentinel failures are materially down.
4. Pool-exhaustion margin is comfortably below threshold.

Additional rules:

- Do not use smoke-only artifacts to close load-scaled cascades.
- Perform full-distribution sampling before claiming a dominant pattern.
- Record one counter-example check before flipping any row to `Done`.
