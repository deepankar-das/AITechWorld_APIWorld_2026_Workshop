# Known AI Test Writing Defects (CHECK EVERY TEST)

These are the specific patterns Claude defaults to. **Before submitting any test, verify it avoids every one:**

| # | AI Default (BAD) | Correct Pattern (GOOD) |
|:-:|---|---|
| 1 | `expect(body.length).toBeGreaterThan(100)` | `expect(body).toContain("Dashboard")` + `expect(body).toContain(String(apiValue))` |
| 2 | `expect(count).toBeGreaterThan(0)` | `expect(count, "count >= seed total").toBeGreaterThanOrEqual(SEED.totalRecords)` |
| 3 | `if (!await el.isVisible()) return;` (silent pass) | `await expect(el, "Element must exist").toBeVisible()` |
| 4 | Tests page loads, nothing else | Test exact KPI values, button clicks, error states, responsive |
| 5 | No error path testing | Intercept API with 500 -> verify error banner shown |
| 6 | No API-then-UI verification | Call API first -> get expected value -> verify UI shows it |
| 7 | No seed-aware assertions | Import from the `<seed-constants module>`, assert exact seed values |
| 8 | Tier 1 test without Tier 2 pair | Always create both files simultaneously |
| 9 | Backend test checks return value only | Also verify DB row created, audit log written, related records updated |
| 10 | No concurrent operation tests | `Promise.all()` for parallel writes, verify no corruption |
| 11 | No non-functional tests | Test pool exhaustion -> 503, large payload -> 413, rate limit -> 429 |
| 12 | Stopping to report numbers | Keep writing until explicitly told to stop |
| 13 | `page.waitForSelector('.el')` | `await expect(page.locator('.el')).toBeVisible()` — `waitForSelector` is deprecated |
| 14 | `page.locator('text=Submit')` | `page.getByText('Submit')` — `text=` is legacy syntax |
| 15 | `expect(x).catch(() => {})` on assertions | Remove the `.catch()` — swallowing assertion failures defeats the test |
| 16 | Per-file helper function copies | Import from the `<shared e2e navigation helpers>` |
| 17 | Sequential single-agent refactor across 50+ files | Split into 3-7 parallel agents by file batch |
| 18 | Using `rg` (ripgrep) in shell scripts | Use `grep -rl` or `find` — `rg` is not available in bash subshells or CI |
| 19 | Using `declare -A` in shell scripts | macOS ships bash 3.2 which lacks `declare -A`. Use temp files or grep-based lookups. |
| 20 | Using `script -c` (Linux syntax) for pseudo-TTY | macOS uses `script -q <logfile> <command>`. Detect with `uname` and branch. |
| 21 | `expect(dialogVisible \|\| dashboardVisible).toBe(true)` — OR escape | Separate `await expect(dialog).toBeVisible()` + `await expect(dashboard).toBeVisible()` |
| 22 | `if (await dialog.isVisible()) { realAsserts } else { trivialFallback }` | `await expect(dialog).toBeVisible()` followed by unconditional content assertions |
| 23 | `expect([400, 422, 500]).toContain(status)` — 500 in the accepted list | `expect([400, 422]).toContain(status)` — 5xx is never a valid pass state |
| 24 | `if (body.eulaText) { expect(body.eulaText.length > 100) }` — field-existence guard | `expect(body.eulaText).toBeTruthy(); expect(body.eulaText.length).toBeGreaterThan(100);` |
