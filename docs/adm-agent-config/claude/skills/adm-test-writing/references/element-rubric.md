# Minimum Test Depth Rubric (Per Element Type)

Every UI element and API endpoint must have a minimum number of tests. Do not write fewer.

| Element Type | Min Tests | Required Coverage |
|---|:-:|---|
| **Button** | 4 | Click -> state change, disabled condition, confirm/cancel dialog (if applicable), responsive at 375px |
| **Input field** | 5 | Valid input, empty/required, invalid format, XSS payload, boundary value (max length) |
| **API data display** | 4 | API-then-UI exact value match, seed-aware assertion, error state (500), responsive |
| **Page** | 5 | Loads with heading, no overflow at 375px/768px/1280px, axe-core a11y scan |
| **Table** | 5 | Row count matches API, sort click reorders, filter reduces count, pagination page 2 different, empty state |
| **Form** | 6 | Valid submit -> success, empty submit -> error, cancel -> no side effect, XSS safe, submit button state (disabled->enabled), API response schema |
| **Dialog** | 3 | Opens on trigger, Escape closes, submit works |
