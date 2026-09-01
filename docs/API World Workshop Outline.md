This changes the presentation significantly. You now have a much richer story to tell — ADM as the operating model, RADAR as the test-selection paradigm with false-negative-rate targets, the Convergence Loop as the defect-elimination engine, and the pentest gate split as the CI/nightly discipline — all framed from a security posture, not just an engineering productivity one.

Let me synthesize everything from the Substack essays, the podcast prep, and the codebase inventory into a revised presentation structure.

## **Revised Narrative: The Security Expert's Framing**

The key reframe from your materials is this: the accepted abstract promises a **"layered verification system"** — but your Substack essays and podcast prep reveal that what you actually built is an **operating model where security is a continuous property of every diff, not a phase in a pipeline**. That's a DevSecOps thesis, not just an engineering one.

The audience on Workshop Stage A (PRO) paid $1,000+ and many will be API security practitioners, DevSecOps engineers, and platform architects. They live in the world of OWASP, SAST/DAST, CI gating, and shift-left promises that never delivered. Your talk should speak their language.

## **Revised 50-Minute Arc (Format D)**

### **Act 1 — The Problem, Framed as a Security Crisis (0:00–10:00)**

| Time | Slide | Content | Notes |
| ----- | ----- | ----- | ----- |
| 0:00–2:00 | **Cold open** | "Your AI agents landed 47 commits today. Annual pentests check your code once a year. How many of those 47 commits introduced a vulnerability nobody will find until the next audit?" | One slide, one question. Security framing from the first second. |
| 2:00–4:00 | **The Acceleration Whiplash** | Faros AI 2026 data: 22,000 developers, 4,000 teams. Throughput up — bugs per developer up 54%, incidents per merged PR up **242.7%**, code churn up **861%**, median PR review time up **441.5%**, 31.3% more code reaching production with **no review at all**. "Strong pre-AI engineering maturity provides no protection." | From Part 1\. This is the DevSecOps audience's nightmare in numbers. |
| 4:00–6:00 | **Why shift-left never worked** | "Shift-left has been the promise of every quality and security movement for twenty years. It never worked at scale because the loop always terminated on a schedule, not on a quality gate. Sprints ended. Deadlines hit. Code shipped with known defects." | From Part 2 (Convergence Loop section). Sets up ADM as the fix. |
| 6:00–8:00 | **Coverage is dead at agent velocity** | "Full-suite regressions on every agent-authored diff are neither fast enough nor cheap enough. Skipping tests produces the bad-quality exposure that surfaces in production. The team either runs everything and stalls, or runs a subset and hopes. Both end in the same place." | From Part 2 (RADAR section). The sharp, contrarian claim. |
| 8:00–10:00 | **Introducing ADM** | One architecture slide: "The Agentic Development Model rests on three primitives in the order a single diff flows through them: **Cross-Model Convergence Review** (the jury), **RADAR** (the risk-aware test selector), and **the Convergence Loop** (the eight-step process that drives residual defects to zero). Above them sits a commit-attested gate that admits only diffs whose receipts prove the loop terminated cleanly on that specific change." | Name all three once, early, as a unit. The audience now has the vocabulary. |

### **Act 2a — RADAR Deep Dive (10:00–22:00)**

This is where you deliver on "we will show." The deepest section of the talk.

| Time | Slide | Content | Notes |
| ----- | ----- | ----- | ----- |
| 10:00–12:00 | **What RADAR is** | "Risk-Aware Dependency Analysis for Rapid Verification. Not a random subset. Not just the dependency graph. A risk-weighted selector that scans blast radius across **multiple dimensions** and binds every test to a **false-negative-rate target**." | Definition slide. |
| 12:00–15:00 | **The seven blast-radius dimensions** | Walk through each: (1) files touched, (2) module/package boundaries crossed, (3) data structures read or written, (4) API contracts exposed or consumed, (5) runtime paths altered, (6) historical failure correlations of adjacent code, (7) invariants declared at the system level as always-on. "The output is not a single dependency-graph slice. It is a risk-weighted list of tests, each carrying the **reasons** it was selected and the **dimension** of the blast radius it covers." | Slide with the 7 dimensions as a diagram. This is the technical meat the PRO audience paid for. |
| 15:00–17:00 | **False-negative-rate targets** | "A false negative is a test that passes when the code is actually broken. Every test has a measurable historical false-negative rate. RADAR binds each test to a target that depends on priority tier. Critical invariants: near zero. High-blast-radius code: tighter. Low-priority: loosest. If a test drifts above its target, **RADAR flags the test for repair**, not the code it covers." | From Part 2\. This is the concept no other framework names. |
| 17:00–18:30 | **Mandatory floors for critical invariants** | "No authorization bypass on any privileged operation. No data loss on the primary write path. No unbounded resource consumption in any request handler. Each critical invariant has anchor tests. RADAR is **forbidden** from selecting those away, regardless of blast-radius analysis. Intelligence in test selection is welcome. Intelligence that lets a critical-invariant regression through is not." | From Part 2\. Directly speaks to the security audience. |
| 18:30–20:00 | **Pre-recorded clip: RADAR impact selection** | Show: a diff touching a file → radar-select.ts computing the blast radius → selecting \~50–200 tests from 16,000+ → the selection output showing **reasons per test** and **dimension covered**. | Clip 2\. Visual proof of the selection mechanics. |
| 20:00–22:00 | **The run folder as audit substrate** | "Every run folder holds the selection RADAR made for that diff, the reasons per test, the false-negative-rate target and observed rate per test, and the list of critical-invariant anchor tests that were enforced. Six months later, a regulator can read not just **which tests ran** but **why each one was chosen**." | From Part 2\. Directly addresses the compliance/audit audience. |

### **Act 2b — The Convergence Loop: How RADAR Results Drive Defects to Zero (22:00–30:00)**

| Time | Slide | Content | Notes |
| ----- | ----- | ----- | ----- |
| 22:00–24:00 | **The eight steps** | Walk through the Convergence Loop as a numbered procedure: (1) Agent hands off diff \+ full context, (2) Three-model jury reviews in parallel product-code and test-code lanes, (3) Test-code changes built and added to RADAR suite → RADAR runs, (4) Jury findings \+ RADAR failures → work-item list with three lanes and named owners, (5) Owners fix in parallel, (6) Each owner runs targeted re-tests, (7) Jury re-reviews fixes, (8) Follow-up RADAR run — clean \= terminate, new issues \= back to step 2\. | From Part 2\. One slide, eight numbered steps. The audience can photograph it. |
| 24:00–26:00 | **Why this is only possible now** | "In the human-paced world, iterating until residual defects reach zero was theoretically correct and economically impossible — each iteration cost days. Agent velocity moves the cost per iteration from human-days to agent-minutes and tokens. The Convergence Loop that could not be run in the human-paced world **has to be run** in the agent-paced world, because leaving residual defects in a diff wastes the one scarce resource the operating model still has: **human reviewer attention**." | From Part 2\. The economic argument. |
| 26:00–28:00 | **The convergence curve as the replacement metric** | "Coverage is dead. What replaces it is the shape of the convergence curve: how many iterations to zero, and what defect population resolved at each step." Real data from a production convergence campaign on a multi-tenant B2B SaaS platform: 2,094 initial failures → 445 after iteration 1 → 10 after iteration 2 → 0 after iteration 3\. Residual ratios: 0.21 → 0.022 → 0.0. | From Part 2\. The convergence-campaign story is your proof. |
| 28:00–30:00 | **Pre-recorded clip: gate run** | Show: radar-validate.sh running all 8 gates — tsc → RADAR self-tests → agent logic → API contracts → selection → vitest → Playwright → vite build — completing green in \~3 min. "That receipt is bound to the exact diff. A diff that no longer matches its receipt cannot pass." | Clip 1\. Visual proof of the gate discipline. |

### **Act 2c — The Pentest Gate: CI vs. Nightly (30:00–38:00)**

This is where you take the security expert stance hardest.

| Time | Slide | Content | Notes |
| ----- | ----- | ----- | ----- |
| 30:00–32:00 | **Architecture-first, not testing-first** | "Security isn't purely a testing exercise. It's removing the attack surface architecturally first, then automating verification that it stays removed." Walk through what structurally can't happen: no admin panel (none exists), no Redis session risk (Postgres), no GraphQL introspection (REST-only), no exposed database (Cloud SQL → Unix socket, no public IP), no sequential-ID enumeration (UUIDs), no cross-tenant leakage (schema-per-tenant enforced by middleware). | From podcast prep section 11\. Speaks directly to the security audience. |
| 32:00–34:00 | **The CI/Nightly split — which tests run when and why** | **Every commit (35 whitebox, \~10 seconds):** Static code analysis via Vitest. No server required. Checks SQL injection patterns (string interpolation near user input), XSS audit (every dangerous render must have a sanitizer), secret leakage scanner (regex for API keys, hardcoded passwords, secrets in browser storage), auth middleware coverage (every route enforces auth), container security (distroless, no source maps), supply chain (dependency pinning, audit signatures), runtime-specific (no evil regex, no synchronous I/O in request handlers, no shell execution in routes). **Why CI:** These are pattern-matching tests against the source code — deterministic, fast (\<10s), no infrastructure dependency. A developer gets feedback before their diff leaves their machine. | Slide: two-column table, CI vs Nightly, with categories. |
| 34:00–36:00 | **The nightly suite (40+ blackbox, full 75+)** | **Nightly (40+ blackbox, requires running server):** Security header validation (HSTS, CSP, X-Frame-Options, Permissions-Policy), auth enforcement on 12 protected endpoints, HTTP method allowlist (TRACE → 405), body size limits (413 on oversized payload), password reset security (token not disclosed, identical response for known/unknown emails), brute force protection (429 after repeated failures), path traversal (4 payloads), CORS validation (evil origin not reflected), injection payloads handled gracefully, host header injection not reflected, session cookie attributes (HttpOnly, SameSite), configuration hardening (no backup files served, no API docs exposed). **Why nightly, not CI:** These require a live server with a real database, network stack, and HTTP listener. Running them on every commit would add 3–5 minutes of server spinup/teardown per diff and create infrastructure contention across concurrent developers. Running nightly gives full coverage at zero developer friction, with a failure alerting into the morning briefing. | Continue the two-column slide. |
| 36:00–37:00 | **Live demo: whitebox pentest run** | Run vitest run server/tests/pentest-whitebox.test.ts live. 42 tests, named OWASP categories visible in terminal output, completes in \~10 seconds. "This is what runs on every single commit. If it fails, the commit doesn't land." | **The anchor moment.** Live demo. |
| 37:00–38:00 | **The catch that proves it works** | The staffId SQL injection story from the podcast prep: "An AI-assisted internal pentest found one query in analytics.ts using quote-escaping instead of parameterization with real user input. Not exploitable in the tested configuration — but it was the one place in the entire codebase where a future developer copying that pattern could introduce a real injection. We fixed it, and the finding class is now a permanent regression test. That specific mistake can never ship again." | From podcast prep section 11\. Concrete, honest, credible. |

### **Act 3 — Closure Discipline and What We Didn't Automate (38:00–45:00)**

| Time | Slide | Content | Notes |
| ----- | ----- | ----- | ----- |
| 38:00–40:00 | **Counter-example discipline and cross-agent verification** | "Every fix must state what would prove it wrong. Every claim by one agent is verified by another agent from a different model family." The cross-model jury: Claude catches structural/contract issues, GPT catches edge-case reasoning, Gemini catches cross-file couplings. "They don't vote. If you let the majority win, you lose the one juror who was actually right." Research: diversity-preserving aggregation captures \~95% of the theoretical ceiling; naive majority voting captures almost none (arXiv 2510.21513). Cross-LLM secure-code evaluation: up to 47% improvement over single-model baselines (arXiv 2603.22717). | From Part 2 and podcast prep Q4.5. |
| 40:00–42:00 | **The commit-attested gate** | "Nothing in most repositories binds a passing test run to the specific change that shipped. At agent velocity, with an order of magnitude more commits per day, that loose coupling is a load-bearing weakness. Our gate admits a commit only if it carries an attestation identifying a specific test run whose recorded diff matches the staged diff and whose verdict is a pass. The receipt is hashed against the diff. The run folder carries the RADAR selection, the executor logs, the jury findings, and the receipt itself." | From Part 2 (commit-attested gate section). |
| 42:00–45:00 | **What we did not automate and why** | The 21 OWASP WSTG test cases left to humans: reconnaissance, timing attacks, workflow bypass, manual logic testing. "These genuinely need a human — so that's what still gets scoped to a third-party vendor rather than faked as automated. The automated suite tells you what changed since the last vendor engagement. The vendor engagement tells you what a skilled human can find with more creativity than a script. Neither replaces the other." Also: the Ford story as the closing parable — "Ford rehired 350 veteran quality inspectors after going AI-only. Then they became \#1 in J.D. Power's Initial Quality Study for the first time since 2010\. Automation **plus** deliberate human control points, not automation **instead of** them." | From podcast prep sections 11 and Q4.9. |

### **Act 4 — Takeaways (45:00–50:00)**

| Time | Slide | Content | Notes |
| ----- | ----- | ----- | ----- |
| 45:00–47:00 | **Three things to take home** | (1) **The layered model:** fast per-commit gates (\< 3 min) catch 80% of regressions; RADAR impact-selects the risk-weighted subset; the Convergence Loop drives residual defects to zero; the nightly pentest suite validates the full security posture. (2) **The split:** CI \= deterministic, fast, no infrastructure (whitebox). Nightly \= live server, full HTTP stack, full attack simulation (blackbox). Know which is which and why. (3) **Closure discipline:** every test carries a false-negative-rate target; every critical invariant has a mandatory floor; every commit carries a receipt bound to the exact diff it attests; every fix states what would prove it wrong. "If you can't name your guardrails, you're not doing AI governance — you're funding next year's cancelled project." | Three numbered points. Photographable. |
| 47:00–48:30 | **Resources** | QR code to: agenticfactory.ai (framework), deepankardas.substack.com (the three-part series), the OWASP WSTG coverage matrix. "The three-part series on Substack walks through every primitive in full — the Cross-Model Convergence Review, RADAR with false-negative-rate targets, the eight-step Convergence Loop, and the commit-attested gate. Part 1 is the diagnosis. Part 2 is the manual. Part 3 is multi-agent coordination at scale." | Link to the published work. |
| 48:30–50:00 | **Q\&A buffer** |  | Open. |

---

## **Summary of Changes vs. Previous Arc**

| Previous Arc | Revised Arc | Why |
| ----- | ----- | ----- |
| Engineering productivity framing | **DevSecOps/security expert framing** | Audience is API Security track, PRO workshop |
| ADM mentioned briefly at the start | **ADM named as the operating model, all three primitives introduced as a unit** | User request \+ the Substack essays provide the vocabulary |
| RADAR shown as "impact selection demo" | **RADAR deep dive: 7 blast-radius dimensions, false-negative-rate targets, mandatory floors, run folder as audit substrate** | User request \+ Part 2 provides the full mechanism |
| Gates shown as a single clip | **Convergence Loop 8 steps walked through, convergence-campaign data, gate clip as proof** | User request \+ Part 2 provides the full procedure |
| Pentest suite as a single section | **CI/nightly split with explicit rationale for which tests run when** | User request |
| "What we didn't automate" as a short section | **21 OWASP gaps \+ Ford story as the closing parable** | Security expert stance demands honesty about limits |
| No mention of cross-model review research | **arXiv citations on ensemble diversity, PoLL jury study, cross-LLM secure-code evaluation** | Credibility for the DevSecOps audience |

---

## **Pre-Recorded Clips Needed (3 clips)**

| Clip | Duration | What to Record |
| ----- | ----- | ----- |
| **Clip 1: RADAR impact selection** | \~2 min | Diff a file → radar-select.ts runs → show the selection output with reasons per test and dimensions covered → contrast with "16,000+ total" |
| **Clip 2: Full gate run** | \~3 min | radar-validate.sh running all 8 gates, completing green, showing the receipt |
| **Clip 3: Backup — blackbox pentest** | \~2 min | playwright test e2e/pentest-blackbox.spec.ts running the 35 blackbox tests against a live server (use only if time permits or as Q\&A visual) |

## **Live Demo (1 moment)**

**Whitebox pentest run:** vitest run server/tests/pentest-whitebox.test.ts — 42 tests, \~10 seconds, OWASP categories in terminal. Lowest risk, highest credibility.

---

Want me to start drafting the actual slide content and speaker notes for this arc, or do you want to adjust the structure first?

