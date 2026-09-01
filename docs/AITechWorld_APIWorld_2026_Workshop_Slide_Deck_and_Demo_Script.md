# Securing AI-Generated APIs at the Speed of AI

## Risk-Aware Test Selection and Automated Tests

**Event:** API World \+ CloudX \+ AI TechWorld 2026  
**Session:** Tuesday, September 1, 2026 · 12:00–12:50 PM PDT  
**Location:** API World — Workshop Stage A (PRO)  
**Format:** Technical Workshop / Tutorial — 50 minutes  
**Access:** PRO Pass · PREMIUM Pass  
**Speaker:** Deepankar Das  
**Deck and demo-script draft:** v0.1  
**Structure:** 30 minutes presentation · 15 minutes live demo · 5 minutes Q\&A

---

# Run of show

| Time | Segment | Slides / demo | Primary purpose |
| :---- | ----: | :---- | :---- |
| 0:00–0:03 | Opening | Slides 1–2 | Establish the agent-speed verification problem and the six engineering lenses |
| 0:03–0:08 | Problem and model | Slides 3–5 | Explain why annual testing, occasional regression sweeps, and unbounded full-suite CI fail |
| 0:08–0:15 | Layered verification | Slides 6–9 | Show fast gates, impact selection, invariants, end-to-end checks, and milestones |
| 0:15–0:21 | Automated security testing | Slides 10–12 | Explain whitebox and blackbox coverage and the 75-test suite concept |
| 0:21–0:27 | Closure discipline | Slides 13–15 | Introduce run artifacts, first failure, counter-examples, cross-agent re-verification, and convergence |
| 0:27–0:30 | Demo setup | Slides 16–17 | Establish sample application, risky change, and demo stages |
| 0:30–0:45 | Live demo | Demo stages 1–8 | Walk through the verification system on a sample multi-tenant application |
| 0:45–0:50 | Q\&A | Slides 18–19 | Reinforce the operating model and answer questions |

---

# Presentation deck

## SLIDE 1 — Title (0:00–0:30)

### On slide

> **Securing AI-Generated APIs at the Speed of AI**  
>   
> Risk-Aware Test Selection and Automated Tests  
>   
> Deepankar Das · API World \+ CloudX \+ AI TechWorld 2026

### Speaker script

> Good afternoon. This session is about a practical problem: AI agents can now generate API code faster than conventional testing and review systems can establish whether that code is safe to ship.  
>   
> This is not a session about whether agents can write an endpoint. They can. It is about what happens next: how a team verifies API correctness, runtime behavior, authorization, tenant isolation, and security at the same pace the agents create changes.

### Speaker notes

- Do not open with a long ADM explanation.  
- Establish this as an engineering and verification session, not a product pitch.  
- Target: 30 seconds.

---

## SLIDE 2 — One change, six lenses (0:30–3:00)

### On slide

> **One AI-generated API change. Six engineering lenses.**  
>   
> API contract · Application runtime · Operations · Security · Deployment · Human accountability

Under each term, one short prompt:

| Lens | Question |
| :---- | :---- |
| API contract | Did we preserve the promise to consumers? |
| Runtime | Does the application behave correctly under real inputs? |
| Operations | Can we observe, contain, and recover? |
| Security | Did we create a new attack or exposure path? |
| Deployment | Can we safely release and roll back? |
| Human accountability | Who accepts the remaining risk? |

### Speaker script

> This workshop appears in API design and development, API operations, API security, APIs in the age of AI, developer technology, and AI innovation. That cross-listing is right, because one AI-generated API change crosses all six domains.  
>   
> It can change a public API contract. It can change a database query, cache key, authorization policy, generated SDK, deployment configuration, and observability surface. A code review that asks only “does the diff look reasonable?” is not enough.  
>   
> We need a system that makes the affected surfaces visible, selects the tests that matter, and gives the human reviewer evidence rather than confidence theater.

### Speaker notes

- Point to each lens quickly; do not teach each one yet.  
- The point is that the demo will traverse all six.  
- Target: 2 minutes 30 seconds.

---

## SLIDE 3 — The old testing rhythm (3:00–4:30)

### On slide

**Built for human-paced change**

Annual penetration test

        ↓

Release regression sweep

        ↓

Manual review

        ↓

Production

Small caption:

> AI-generated changes arrive continuously. This cadence does not.

### Speaker script

> Most API teams inherited a testing rhythm designed for a different delivery model. You do a major penetration test once or twice a year. You run broader regression sweeps before a release. You rely on code review and CI in between.  
>   
> That rhythm was never perfect. But it becomes structurally inadequate when several agents can create code, tests, configuration, contracts, and deployment artifacts continuously.  
>   
> The issue is not that penetration testing or regression testing has become unnecessary. The issue is that they are too late and too infrequent to be the only controls.

### Speaker notes

- Do not criticize security teams; position the workshop as making their work continuous and more attributable.  
- Target: 1 minute 30 seconds.

---

## SLIDE 4 — The two failure modes (4:30–6:00)

### On slide

| Failure mode | What happens | Result |
| :---- | :---- | :---- |
| **Run everything** | Full test suite becomes the critical path | Slow feedback, queueing, weak attribution |
| **Run too little** | Teams trust unit tests, an agent summary, or a plausible diff | Contract, security, and runtime defects escape |

Bottom line:

> The answer is neither “run nothing” nor “run everything.” It is **selective rigor**.

### Speaker script

> At agent speed, teams typically make one of two mistakes. The first is to run everything on every change. That creates coverage, but when feedback arrives too slowly, the organization loses the speed it adopted agents to gain—and it loses causal clarity when several changes are in flight.  
>   
> The second is to run a shallow set of tests because the agent’s code compiles, the local unit tests are green, and the pull request sounds convincing. That is where API failures escape: an authorization gap, a cache-key leak, a client incompatibility, an unsafe CORS setting, a malformed error path.  
>   
> The goal is selective rigor: the smallest credible set of tests for this change, plus non-negotiable invariant checks.

### Speaker notes

- Repeat “smallest credible set,” not “smallest set.”  
- Target: 1 minute 30 seconds.

---

## SLIDE 5 — The workshop thesis (6:00–8:00)

### On slide

> **Test at agent speed by making evidence selective, explainable, and cumulative.**

Change

  → impact map

  → selected tests \+ invariant floors

  → whitebox \+ blackbox validation

  → run artifact

  → failure closure

  → human decision

### Speaker script

> The thesis is straightforward. You do not establish trust by slowing agents down until they resemble human developers. You establish trust by making evidence selective, explainable, and cumulative.  
>   
> For every material change, we identify what it can reach. We select tests for those reachable surfaces. We add the invariant floors that cannot be skipped. We validate both the implementation and deployed behavior. We preserve the evidence. And if something fails, we do not just rerun until green—we make the test system stronger before the next run.  
>   
> The human reviewer receives that evidence packet and owns the final decision.

### Speaker notes

- This is the bridge from problem to system.  
- Target: 2 minutes.

---

## SLIDE 6 — The layered verification system (8:00–10:00)

### On slide

| Layer | Job | Cadence |
| :---- | :---- | :---- |
| Fast per-commit gates | Catch immediate, high-signal defects | Every commit / PR |
| Impact-selected tests | Cover reachable change surfaces | Every material change |
| Invariant suites | Protect non-negotiable properties | Every applicable change |
| Targeted end-to-end checks | Validate cross-layer behavior | High-risk change / pre-merge |
| Automated penetration tests | Test source-level and runtime security controls | Change-specific subset \+ milestone suite |
| Milestone validation | Re-establish broad release confidence | Release / major milestone |

### Speaker script

> This is the layered system. The important point is that these are not interchangeable test types.  
>   
> Fast gates block defects that should never wait: a secret in a commit, a malformed schema, a broken type, a failed critical authorization check.  
>   
> Impact-selected tests follow the change through its reachable surfaces. Invariant suites protect properties that cannot be skipped. End-to-end tests validate the workflows that cross several layers. Automated penetration tests inspect both implementation patterns and deployed behavior. Milestone validation is broader and slower, because it answers broader questions.  
>   
> The system works because each layer has a purpose and a cadence.

### Speaker notes

- Aim for comprehension, not exhaustive detail.  
- Target: 2 minutes.

---

## SLIDE 7 — Fast gates versus milestones (10:00–11:30)

### On slide

| Fast per-commit gates | Milestone validation |
| :---- | :---- |
| Bounded and fast | Broad and slower |
| Blocks obvious or critical regressions | Tests integrated and release-level behavior |
| Runs against the exact diff | Re-establishes confidence before release |
| Examples: schema, secrets, targeted tests, critical invariants | Examples: full regression, performance, broader pen-test suite, operational drills |

### Speaker script

> A common mistake is to insist that everything run for every commit. That turns every commit into a release candidate and slows the system down.  
>   
> The inverse mistake is to defer serious checks until a milestone. That allows risk to accumulate and makes attribution difficult.  
>   
> Fast gates must be bounded, strict, and tied to the exact diff. Milestone validation is where you test broad release behavior: performance, full regression, wider penetration coverage, long-running workflows, deployment sequencing, and operational recovery paths.  
>   
> The demo will show both. It will run fast gates against an AI-generated pull request, then show which deeper checks are reserved for the release path.

### Speaker notes

- This slide directly fulfills one of the accepted abstract’s promised takeaways.  
- Target: 1 minute 30 seconds.

---

## SLIDE 8 — Risk-aware test selection (11:30–13:30)

### On slide

> **Do not select tests only from changed files.**  
>   
> Select from changed and reachable surfaces:

Route → contract → authorization → query → cache

      → service dependencies → generated client → config → telemetry

Formula:

\[ \\text{Selected corpus} \= \\text{Reachability tests} \+ \\text{Invariant floors} \+ \\text{Risk-mandated tests} \]

### Speaker script

> A file-based selector is not enough for APIs. A route can touch a contract. A contract can affect a generated SDK. A query can affect tenant isolation. A cache key can turn a correct query into a cross-tenant data leak. A configuration change can make a secure handler unsafe at runtime.  
>   
> The selection model therefore begins with changed and reachable surfaces. Then it adds invariant floors, such as authorization, tenant isolation, contract compatibility, and secrets handling. Finally, it adds tests mandated by the risk profile—for example, a high-blast-radius change or customer-facing security surface.  
>   
> Again: the objective is not to run the fewest tests. It is to run the smallest credible corpus for the change.

### Speaker notes

- This should set up the impact-map view in the demo.  
- Target: 2 minutes.

---

## SLIDE 9 — Invariants are non-negotiable (13:30–15:00)

### On slide

**Examples of API invariant floors**

- Default-deny authorization behavior  
- Tenant isolation  
- API schema and compatibility rules  
- Secret leakage prevention  
- Data-integrity rules  
- Critical method and security-header controls

Bottom line:

> A selector may narrow the suite. It may not waive a critical property.

### Speaker script

> Risk-aware selection is not permission to optimize security away. Some tests are invariant floors: they always run when a relevant surface is touched.  
>   
> If authorization changes, the authorization matrix must run. If a multi-tenant query or cache changes, the tenant-isolation tests must run. If a shared API contract changes, the contract and compatibility checks must run.  
>   
> The selector narrows the suite around the change. It does not waive the properties that the platform must never violate.

### Speaker notes

- Keep this direct and concrete.  
- Target: 1 minute 30 seconds.

---

## SLIDE 10 — Whitebox security validation (15:00–17:00)

### On slide

**Whitebox: inspect the implementation and build surface**

| Category | What it looks for |
| :---- | :---- |
| SQL injection patterns | Unsafe query construction and parameter handling |
| XSS exposure | Unsafe output or rendering paths |
| Secret leakage | Credentials and sensitive values in source, logs, or artifacts |
| Auth coverage gaps | Sensitive routes without expected policy enforcement |
| Unsafe shell execution | Untrusted input influencing command execution |
| Container posture | Insecure image, privilege, or runtime settings |
| Dependency risks | Vulnerable, stale, unpinned, or risky dependencies |

### Speaker script

> The accepted session describes a 75-test automated penetration-testing suite with whitebox and blackbox validation. Whitebox checks have access to the code, configuration, build artifacts, container definitions, and dependency graph.  
>   
> This is where we can inspect patterns that a runtime test may not reliably find: unsafe query construction, an authorization path with no policy check, a secret in a build artifact, a dangerous shell boundary, a weak container setting, or a risky dependency.  
>   
> In the demo, we will use a small subset. The purpose is to show how whitebox checks are attached to a change because of the surfaces it affects.

### Speaker notes

- Mention the 75-test suite exactly once here and once on slide 12\.  
- Target: 2 minutes.

---

## SLIDE 11 — Blackbox API security validation (17:00–19:00)

### On slide

**Blackbox: test the deployed API as a client or attacker sees it**

| Category | Example |
| :---- | :---- |
| Security headers | Required headers are present and correctly configured |
| Method allowlists | Unsupported methods are rejected |
| Oversized payloads | Large inputs fail safely |
| Auth enforcement | Missing, invalid, expired, and insufficient credentials fail |
| Brute-force handling | Repeated failures receive defensive handling |
| Path traversal | Encoded traversal attempts fail |
| CORS behavior | Only intended origins, methods, and headers work |
| Session cookie attributes | Secure, HttpOnly, SameSite, and scope attributes are correct |

### Speaker script

> Blackbox checks operate against the running API. They test the behavior that a real client, an attacker, or a misconfigured integration can observe.  
>   
> This is important because source-level correctness does not guarantee deployed behavior. A gateway, proxy, environment variable, CORS policy, load balancer, cache, or deployment setting can change what the client sees.  
>   
> Whitebox and blackbox validation are complements. A pass in one is not a waiver for the other.

### Speaker notes

- Target: 2 minutes.

---

## SLIDE 12 — Security coverage by cadence (19:00–21:00)

### On slide

Every relevant PR

  • fast secret checks

  • auth coverage checks

  • selected security tests

  • targeted blackbox checks

Milestone / release

  • broader 75-test automated penetration suite

  • deeper dependency and container review

  • performance and operational validation

  • release-path and rollback verification

### Speaker script

> Not every security check needs to run on every pull request. But security cannot be deferred to an annual event either.  
>   
> The right approach divides the work by cadence. Every relevant pull request gets fast secret scanning, authorization coverage, selected security tests, and targeted blackbox checks. A release or major milestone gets the broader penetration suite, deeper dependency and container analysis, performance validation, and release-path exercises.  
>   
> The system is continuous without pretending that the same test depth is economically sensible for every commit.

### Speaker notes

- This is the final slide on the layered system before closure discipline.  
- Target: 2 minutes.

---

## SLIDE 13 — A passing run is not closure (21:00–23:00)

### On slide

> **Coverage is not closure.**  
>   
> A failure is not resolved because a later run turns green.

Four requirements:

1. Preserve the run artifact.  
2. Investigate the first failure.  
3. Add a counter-example that would catch recurrence.  
4. Re-verify independently when risk warrants it.

### Speaker script

> The deeper lesson from the system is closure discipline. A dashboard full of green checks does not necessarily mean the test system is honest.  
>   
> If a failure occurs and the team simply reruns until it becomes green, the cause may remain unknown. If the evidence cannot be tied to a specific diff, a reviewer cannot rely on it. If the same agent that introduced a defect also declares it fixed, the same blind spot may remain.  
>   
> Closure requires an artifact, a first-failure discipline, a durable counter-example, and independent re-verification when the risk calls for it.

### Speaker notes

- Slow down slightly. This is a key conceptual transition.  
- Target: 2 minutes.

---

## SLIDE 14 — First failure and counter-examples (23:00–25:00)

### On slide

| Assertion | Counter-example |
| :---- | :---- |
| “Authorization works” | Missing, expired, wrong-role, wrong-tenant credentials |
| “Cache is safe” | Identical request across two tenants, workspaces, and date ranges |
| “Contract is compatible” | Existing client fixture against the changed deployment |
| “Input validation works” | Malformed, oversized, encoded, and boundary inputs |

### Speaker script

> When a multi-layer test run fails, do not immediately change ten things and rerun the suite until it becomes green. Preserve the first failure in stable order. Explain it, fix or classify it, and rerun the smallest relevant corpus.  
>   
> Then add a counter-example. The point of a test is not to confirm the story you already believe. It is to create evidence that could disprove it.  
>   
> “Authorization works” is not a test. Wrong-role, wrong-tenant, expired, missing, and over-scoped credentials are tests. “The cache is safe” is not a test. Cross-tenant cache requests with shared dimensions are tests.

### Speaker notes

- Preview that the demo will show a cache-key failure and a counter-example.  
- Target: 2 minutes.

---

## SLIDE 15 — Cross-agent re-verification (25:00–27:00)

### On slide

Agent A: proposes or fixes change

         ↓

Agent B: independently challenges contract, security, and tests

         ↓

Human reviewer: resolves disagreements and approves release decision

Bottom line:

> Independent automated challenge reduces shared blind spots. Human accountability remains.

### Speaker script

> In agentic workflows, an agent can produce code, tests, and an explanation that all share the same mistaken assumption. That is a problem of correlated confidence.  
>   
> Cross-agent re-verification means a different agent, toolchain, prompt strategy, or model family challenges the proposed change. It looks for missing tests, false assumptions, contract inconsistencies, and security gaps.  
>   
> This does not replace the human reviewer. It gives the reviewer an adversarial second view and a more useful evidence packet.

### Speaker notes

- Avoid claiming model-family diversity is always required; phrase it as an available independent path.  
- Target: 2 minutes.

---

## SLIDE 16 — The convergence loop (27:00–28:30)

### On slide

Failure observed

     ↓

Preserve first-failure evidence

     ↓

Add counter-example / test expectation

     ↓

Fix cause

     ↓

Run selected corpus

     ↓

Independent re-verification

     ↓

Durable closure

### Speaker script

> This is the convergence loop. A failure becomes an input to the system, not an interruption to be hidden.  
>   
> We preserve the first failure. We add the counter-example that would expose recurrence. We fix the cause rather than only the symptom. We rerun the selected corpus, not necessarily everything. Then we independently challenge the result and close only when the evidence is durable.  
>   
> In the next fifteen minutes, I will show this exact loop on a deliberately flawed AI-generated API change.

### Speaker notes

- Transition toward demo. Keep the diagram on screen as you introduce the sample app.  
- Target: 1 minute 30 seconds.

---

## SLIDE 17 — Live demo setup (28:30–30:00)

### On slide

> **Sample application:** multi-tenant usage service  
>   
> **AI-generated change:** add `GET /v1/accounts/{accountId}/usage-summary`  
>   
> **Changed surfaces:** Route · OpenAPI contract · `usage:read` authorization · tenant query · cache key · SDK · feature flag · telemetry  
>   
> **Seeded defect:** cache key omits account identity

### Speaker script

> The sample application is a small multi-tenant usage service. An AI agent has generated a pull request that adds a customer-facing usage-summary endpoint.  
>   
> It updates the route, the OpenAPI contract, an authorization scope, a tenant-scoped aggregation query, a cache key, a generated client, a feature flag, and telemetry.  
>   
> The code will look plausible. The unit tests will mostly be green. But there is a seeded defect: the cache key omits the account identity. Under the right sequence of requests, Tenant B can receive a cached response created for Tenant A.  
>   
> We will walk through each verification stage, catch it, turn it into a durable counter-example, fix it, and produce the evidence packet a reviewer needs.

### Speaker notes

- Move to demo environment at 30:00 exactly.  
- Have slide 17 available as a fallback while changing display inputs.

---

# Live demo script

## Demo goal

Demonstrate the full layered verification process on a prebuilt sample application. The audience should see a realistic agent-generated API change move from pull request to evidence-backed approval, including a seeded failure and the convergence loop.

### Demo timing

| Stage | Time | What the audience sees |
| :---- | ----: | :---- |
| 1\. Baseline and pull request | 30:00–31:30 | The sample app, API endpoint, generated diff, and scoped change description |
| 2\. Change impact map | 31:30–33:00 | Changed and reachable surfaces used for test selection |
| 3\. Fast per-commit gates | 33:00–34:30 | Fast checks and a clean initial result |
| 4\. Impact-selected and invariant tests | 34:30–36:30 | Selected test corpus; tenant-cache invariant fails |
| 5\. Whitebox / blackbox checks | 36:30–38:30 | Representative security checks and why they matter |
| 6\. Preserve first failure | 38:30–39:30 | Run artifact and failure record |
| 7\. Fix, counter-example, re-verification | 39:30–42:30 | Corrected cache key, repeat test, independent challenge |
| 8\. Final receipt and release decision | 42:30–45:00 | Reviewer evidence packet; fast gates vs milestone checks |

---

## Demo environment checklist

Prepare this before the session:

- [ ] Sample multi-tenant API application is running locally.  
- [ ] One browser window is open to the API documentation or a simple client UI.  
- [ ] One terminal is prepared for application commands and test runs.  
- [ ] One editor window is prepared at the AI-generated pull-request diff.  
- [ ] One local JSON or YAML run artifact is ready to open.  
- [ ] The seeded cache-key defect is present on the starting branch.  
- [ ] The corrected commit is prepared locally but not checked out.  
- [ ] The counter-example test is prepared in the corrected commit.  
- [ ] A screen recording of every critical command is available locally as a fallback.  
- [ ] Network-dependent tools are not required for the live path.  
- [ ] Font sizes are large enough for a conference room.

### Recommended directory shape

sample-api/

  openapi/

    usage-summary.yaml

  src/

    routes/usage-summary.ts

    services/usage-aggregation.ts

    auth/usage-scope.ts

    cache/usage-summary-cache.ts

  tests/

    contract/

    auth/

    isolation/

    cache/

    integration/

    security/

    e2e/

  radar-runs/

    demo-starting-run.json

    demo-failure-run.json

    demo-final-run.json

  failure-ledger/

    FL-017-cache-tenant-isolation.md

### Recommended representative commands

Adjust these to the real sample application. Keep command names predictable and short.

\# Start or verify the local sample application

npm run dev

\# Fast per-commit gates

npm run gate:fast

\# Build the impact map and select the test corpus

npm run radar:select \-- \--change demo/usage-summary

\# Run selected and invariant tests

npm run radar:run \-- \--change demo/usage-summary

\# Run representative whitebox checks

npm run security:whitebox \-- \--scope usage-summary

\# Run representative blackbox checks against the local API

npm run security:blackbox \-- \--base-url http://localhost:3000

\# Switch to prepared corrected state

 git checkout demo/cache-key-fixed

\# Re-run the selected corpus and independent verifier

npm run radar:run \-- \--change demo/usage-summary

npm run verify:independent \-- \--change demo/usage-summary

\# Display final evidence packet

cat radar-runs/demo-final-run.json

---

## Stage 1 — Baseline and pull request (30:00–31:30)

### Screen state

Open the pull-request diff or a side-by-side editor showing:

- `GET /v1/accounts/{accountId}/usage-summary`  
- The OpenAPI path and response schema.  
- `usage:read` scope enforcement.  
- The tenant-scoped aggregation query.  
- The cache-key function.  
- The generated SDK method.  
- Feature-flag and telemetry changes.

### Live narration

> Here is the change. The agent has added a usage-summary endpoint for account administrators. The pull request is not huge, and most of it looks familiar: a route, a schema, a scope check, a query, a cache, a client update, and telemetry.  
>   
> The important point is not the size of the diff. It is the number of surfaces it touches. An API route is not just a route. It is a contract, an authorization path, a tenant boundary, a runtime query, a cache behavior, a client surface, and an operational surface.  
>   
> If we select tests from changed files alone, we will miss indirect risk. So we start with the impact map.

### Show

Highlight the intentionally flawed cache key, but do not expose the bug yet. For example:

function usageSummaryCacheKey(workspaceId: string, startDate: string, endDate: string) {

  return \`usage-summary:${workspaceId}:${startDate}:${endDate}\`;

}

Do not say it is flawed until the invariant test fails.

---

## Stage 2 — Change impact map (31:30–33:00)

### Screen state

Open a prepared impact-map file or generated RADAR selection report.

changed\_surfaces:

  \- api\_route

  \- openapi\_contract

  \- usage\_read\_authorization

  \- tenant\_scoped\_query

  \- usage\_summary\_cache

  \- generated\_client\_sdk

  \- feature\_flag

  \- observability\_metrics

reachable\_risks:

  \- contract\_compatibility

  \- authorization\_default\_deny

  \- tenant\_isolation

  \- cache\_partitioning

  \- date\_boundary\_correctness

  \- aggregation\_service\_error\_handling

  \- client\_generation\_compatibility

  \- rollout\_and\_rollback\_behavior

### Live narration

> This is the impact map. It begins with the changed surfaces and follows the relationships that matter for this API change.  
>   
> The new route reaches the OpenAPI contract and generated SDK. The scope check reaches the authorization matrix. The tenant query and cache reach isolation guarantees. The aggregation service reaches date-boundary behavior and service error handling. The feature flag and telemetry reach deployment and operations.  
>   
> The test corpus comes from this map, plus invariant floors. That is the difference between running a few nearby unit tests and producing credible evidence for an API change.

### Show

Scroll or point through each changed surface once. Keep this stage visual and fast.

---

## Stage 3 — Fast per-commit gates (33:00–34:30)

### Screen state

Run:

npm run gate:fast

### Expected terminal output

✓ Type check

✓ Lint

✓ OpenAPI schema validation

✓ Secret-leak scan

✓ Route-to-auth-policy coverage

✓ Targeted route unit tests

FAST GATE VERDICT: PASS

Duration: 42s

### Live narration

> The fast gates pass. This is intentional. The code compiles, the API schema validates, no secrets were introduced, the route has an authorization policy, and its targeted unit tests are green.  
>   
> This is useful evidence, but it is not sufficient evidence. A green fast gate does not prove the cache is safely partitioned across tenants. It does not prove the deployed API’s CORS behavior. It does not prove a generated client remains compatible.  
>   
> This is why the fast gate is a layer, not the finish line.

### Demo safety note

If a command runs long, show a pre-captured terminal log. Do not wait more than 20 seconds for a live command.

---

## Stage 4 — Impact-selected tests and invariant failure (34:30–36:30)

### Screen state

Run the selector:

npm run radar:select \-- \--change demo/usage-summary

### Expected terminal output

RADAR impact selection

Change: demo/usage-summary

Selected by reachability:

  contract.usage\_summary.schema

  integration.usage\_aggregation.date\_boundaries

  sdk.usage\_summary.client\_smoke

Invariant floors:

  auth.usage\_summary.scope\_matrix

  isolation.usage\_summary.cross\_tenant

  cache.usage\_summary.key\_partition

  compatibility.api\_v1.existing\_clients

Risk-mandated:

  blackbox.usage\_summary.method\_allowlist

  blackbox.usage\_summary.auth\_enforcement

Selected corpus: 9 tests

Selection rationale written to radar-runs/demo-selection.json

Then run:

npm run radar:run \-- \--change demo/usage-summary

### Expected terminal output

PASS contract.usage\_summary.schema

PASS auth.usage\_summary.scope\_matrix

PASS isolation.usage\_summary.cross\_tenant

FAIL cache.usage\_summary.key\_partition

PASS integration.usage\_aggregation.date\_boundaries

PASS compatibility.api\_v1.existing\_clients

PASS sdk.usage\_summary.client\_smoke

PASS blackbox.usage\_summary.method\_allowlist

PASS blackbox.usage\_summary.auth\_enforcement

FIRST FAILURE: cache.usage\_summary.key\_partition

VERDICT: FAIL

Duration: 1m 18s

### Live narration

> RADAR selected nine tests. Notice the distinction: some tests were selected because the impact map reaches them. Others are invariant floors. We did not choose the cache-isolation test because it was convenient. We chose it because a tenant-scoped query plus a new cache creates a direct isolation risk.  
>   
> Now the critical moment. The suite is mostly green, but the cache-partition invariant fails. This is exactly the sort of defect a fast unit-test pass can miss: the query is tenant-scoped, but the cache key is not.  
>   
> We do not treat this as a reason to rerun until green. We preserve the first failure and make it the next unit of work.

### Optional visual demonstration

Use a local client or curl commands to show the data leak:

\# Tenant A primes cache

curl \-H "Authorization: Bearer tenant-a-admin" \\

  "http://localhost:3000/v1/accounts/acct-a/usage-summary?workspaceId=ws-1\&startDate=2026-08-01\&endDate=2026-08-07"

\# Tenant B requests matching dimensions and incorrectly gets Tenant A result

curl \-H "Authorization: Bearer tenant-b-admin" \\

  "http://localhost:3000/v1/accounts/acct-b/usage-summary?workspaceId=ws-1\&startDate=2026-08-01\&endDate=2026-08-07"

Only show this if it is deterministic and visually clear. Otherwise, rely on the failed test output.

---

## Stage 5 — Whitebox and blackbox checks (36:30–38:30)

### Screen state

Run representative whitebox checks:

npm run security:whitebox \-- \--scope usage-summary

Expected output:

WHITEBOX CHECKS

✓ Query parameterization pattern

✓ Auth coverage for new route

✓ Secret leakage scan

✓ Dependency manifest unchanged

✓ No unsafe shell execution path

WHITEBOX VERDICT: PASS

Run representative blackbox checks:

npm run security:blackbox \-- \--base-url http://localhost:3000 \-- \--scope usage-summary

Expected output:

BLACKBOX CHECKS

✓ Security headers

✓ Method allowlist

✓ Missing credential denied

✓ Invalid credential denied

✓ Insufficient role denied

✓ Oversized request rejected safely

✓ CORS preflight constrained

BLACKBOX VERDICT: PASS

### Live narration

> The whitebox checks pass. They inspect the implementation: parameterization, route-to-policy coverage, secret leakage, dependency changes, unsafe execution boundaries.  
>   
> The blackbox checks also pass. They test what the deployed API exposes: headers, method restrictions, invalid and insufficient credentials, oversized requests, and CORS behavior.  
>   
> This is a useful reminder: security validation can be green while a system is still unsafe in a different layer. Here, the security controls pass but cache partitioning fails. That is why the layered system exists.

### Speaker notes

- Mention that the live demo represents the broader 75-test suite; do not claim the subset is exhaustive.  
- If time is tight, show pre-captured summaries rather than live commands.

---

## Stage 6 — Preserve the first failure (38:30–39:30)

### Screen state

Open the failure artifact:

{

  "run\_id": "demo-2026-09-01-usage-summary-a1b2c3",

  "change": "demo/usage-summary",

  "first\_failure": "cache.usage\_summary.key\_partition",

  "invariant": "tenant\_isolation",

  "observed\_behavior": "Tenant B received a cached response populated by Tenant A.",

  "suspected\_cause": "Cache key lacks accountId.",

  "required\_counter\_example": "Same workspace and date range across distinct accounts must never share cache entries.",

  "closure\_state": "open"

}

### Live narration

> This is the run artifact. It identifies the exact change, the first failing test, the violated invariant, the observed behavior, the suspected cause, and the counter-example required before we call the issue closed.  
>   
> The first-failure discipline matters because it preserves causal clarity. We are not going to make unrelated changes or rerun the whole world. We are going to correct this boundary, retain the counter-example, and rerun the affected corpus.

---

## Stage 7 — Fix, counter-example, and cross-agent re-verification (39:30–42:30)

### Screen state

Show the corrected cache key:

function usageSummaryCacheKey(

  accountId: string,

  workspaceId: string,

  startDate: string,

  endDate: string,

  role: string,

) {

  return \`usage-summary:${accountId}:${workspaceId}:${role}:${startDate}:${endDate}\`;

}

Show the new or strengthened counter-example test:

it('never returns a cached usage summary across account boundaries', async () \=\> {

  await requestAs('tenant-a-admin')

    .get('/v1/accounts/acct-a/usage-summary')

    .query(sharedUsageQuery)

    .expect(200)

    .expect(response \=\> expect(response.body.accountId).toBe('acct-a'));

  await requestAs('tenant-b-admin')

    .get('/v1/accounts/acct-b/usage-summary')

    .query(sharedUsageQuery)

    .expect(200)

    .expect(response \=\> expect(response.body.accountId).toBe('acct-b'));

});

Switch to the prepared fixed branch:

git checkout demo/cache-key-fixed

npm run radar:run \-- \--change demo/usage-summary

npm run verify:independent \-- \--change demo/usage-summary

### Expected terminal output

RADAR SELECTED CORPUS: 9 tests

PASS: 9

FAIL: 0

Duration: 1m 21s

INDEPENDENT VERIFICATION

✓ Checked route and cache-key boundaries

✓ Challenged account, workspace, role, and date isolation

✓ Confirmed counter-example is present

✓ No additional contract or auth coverage gap found

INDEPENDENT VERDICT: PASS

### Live narration

> The fix adds account identity—and, in this sample, role—to the cache boundary. More importantly, the test system has changed. We added the counter-example that reproduces the class of failure, not merely the exact request that happened to fail today.  
>   
> We rerun the impact-selected corpus. It passes. Then an independent verification path challenges the fix: does the cache key include every identity boundary that matters? Are account, workspace, role, and date separation actually tested?  
>   
> That is cross-agent re-verification in practice. It is not a substitute for human approval. It is a way to prevent one agent’s confident repair from becoming the only evidence.

### Time-management note

- If the test run takes more than 30 seconds, stop the live command and switch to the saved output.  
- Do not sacrifice the final receipt and release decision for live terminal time.

---

## Stage 8 — Final receipt and release decision (42:30–45:00)

### Screen state

Open a concise final receipt:

run\_id: demo-2026-09-01-usage-summary-d4e5f6

change:

  commit\_or\_diff\_hash: "d4e5f6"

  specification: "USAGE-API-01"

changed\_surfaces:

  \- route

  \- openapi\_contract

  \- authorization\_policy

  \- tenant\_query

  \- cache\_key

  \- client\_sdk

  \- feature\_flag

  \- telemetry

invariant\_floors:

  \- contract\_schema

  \- authorization\_default\_deny

  \- tenant\_isolation

  \- cache\_partitioning

  \- client\_compatibility

results:

  selected\_tests: 9

  passed: 9

  failed: 0

  whitebox\_verdict: pass

  blackbox\_verdict: pass

  first\_failure\_from\_previous\_run: resolved

  counter\_example\_added: cache.usage\_summary.key\_partition

  independent\_verification: pass

release:

  fast\_gate\_decision: admissible\_for\_merge

  milestone\_requirements:

    \- broader\_penetration\_suite

    \- production\_like\_load\_test

    \- feature\_flag\_rollout

    \- rollback\_verification

  human\_reviewer: required

### Live narration

> This is the artifact that makes the workflow reviewable. It tells the reviewer what changed, what surfaces were affected, which tests were selected, which invariants were mandatory, what failed before, what counter-example was added, what passed now, and what still belongs in milestone validation.  
>   
> Notice the result: admissible for merge is not the same as ready for unrestricted rollout. The fast gates and selected corpus support merge. The broader penetration suite, production-like load test, feature-flag rollout, and rollback verification still belong in the milestone path.  
>   
> The human reviewer has an evidence packet, a defined decision, and a clear remaining-risk boundary.

### Demo close

> That is the operating model: not annual penetration testing, not a full suite on every diff, and not trust in an agent’s summary. Layered validation, risk-aware selection, durable failure closure, independent challenge, and human accountability.

---

# Q\&A slides

## SLIDE 18 — The system in one picture (45:00–45:30)

### On slide

AI-generated change

        ↓

Fast gates

        ↓

Impact map \+ invariant floors

        ↓

Selected correctness and security tests

        ↓

Run artifact \+ first-failure discipline

        ↓

Counter-example \+ cross-agent re-verification

        ↓

Human approval

        ↓

Milestone validation and staged release

### Speaker script

> Before questions, here is the system in one picture. The goal is not to remove human judgment. It is to reduce the amount of unstructured, unsupported judgment a human reviewer has to make.

---

## SLIDE 19 — Questions (45:30–50:00)

### On slide

> **Questions**  
>   
> What sensitive API surface would you use as the first pilot?

Small footer:

> Start with one repository, one invariant suite, one run artifact, and one class of AI-generated change.

### Q\&A preparation

| Likely question | Suggested answer |
| :---- | :---- |
| Why not simply run the full suite on every change? | Use the full suite where its broad assurance is justified, especially milestones. At agent speed, per-change verification also needs fast, attributable, risk-aware feedback. A full suite alone is often too slow and too weakly tied to the precise change. |
| How do you know the selected corpus is sufficient? | You do not claim mathematical certainty from selection alone. You combine reachability with mandatory invariant floors, risk-mandated tests, milestone validation, and a learning loop that turns escaped failures into new tests or rules. |
| Does this replace penetration testing? | No. It makes security validation more continuous and change-aware. Broader penetration testing and human security review remain important milestone controls. |
| Can the same approach work without multiple AI agents? | Yes. The core is layered verification, impact selection, run artifacts, and closure. Cross-agent re-verification is an additional independent challenge path for workflows that use agents. |
| What is the smallest way to start? | Pick one sensitive API module, define its invariants, preserve run artifacts, require counter-examples for failures, and use a simple change-to-test selection record in pull requests. |
| What should never be automated? | Risk acceptance, business authorization, customer-impact trade-offs, ambiguous incident decisions, and final approval of high-blast-radius or irreversible production actions. |

---

# Facilitator preparation

## Required demo artifacts

Prepare the following artifacts in advance so every stage is deterministic:

1. A small multi-tenant sample API application with a running local environment.  
2. An AI-generated-looking pull request or commit containing the usage-summary endpoint change.  
3. A starting version with a deliberately omitted account identifier in the cache key.  
4. A corrected version with the cache-key repair and durable counter-example test.  
5. Fast-gate output for the starting version.  
6. Impact-selection output showing why the selected tests were chosen.  
7. A failing cache-isolation test and a saved run artifact.  
8. Representative whitebox and blackbox security outputs.  
9. An independent verification output.  
10. A final receipt that distinguishes merge admissibility from milestone release requirements.

## Demo design principles

- Keep the sample application small enough to understand in under two minutes.  
- Seed only one defect for the live path. One clearly explained failure is stronger than five rushed ones.  
- Make the bug realistic: a cache key omitting a tenant/account boundary cleanly connects API runtime behavior, security, tenant isolation, and operations.  
- Do not run a real network-dependent penetration test during the live demo.  
- Pre-stage every command and preserve screenshots or recordings as contingency.  
- Make the live tests readable: one command, one result, one explanation.

## If time runs short

Cut in this order:

1. The optional live curl demonstration of the cached cross-tenant response.  
2. The live whitebox command; show the prepared output instead.  
3. The live blackbox command; show the prepared output instead.  
4. Detailed explanation of the final YAML receipt.

Do not cut:

- The impact map.  
- The selected-test and invariant-floor distinction.  
- The seeded failure.  
- The counter-example.  
- The rerun and independent re-verification.  
- The fast-gate versus milestone-validation distinction.

---

# Presenter checklist

## Before the session

- [ ] Confirm exact room AV and display inputs.  
- [ ] Increase terminal and editor font size.  
- [ ] Disable notifications, VPN popups, auto-updates, and sleep mode.  
- [ ] Start the sample application before the session begins.  
- [ ] Verify the seeded defect still fails deterministically.  
- [ ] Verify the corrected branch passes deterministically.  
- [ ] Keep the fast-gate, failure, and final-receipt artifacts open in separate tabs.  
- [ ] Copy all commands to a local scratch file so no command must be typed from memory.  
- [ ] Test the 15-minute demo under conference-network-independent conditions.  
- [ ] Have local screenshots or recordings of every critical output.

## During the session

- [ ] Reach the demo setup slide by minute 28:30.  
- [ ] Switch to live demo at minute 30:00.  
- [ ] Preserve the failure before showing the fix.  
- [ ] Explain the counter-example before rerunning.  
- [ ] Stop live command execution if it risks cutting into Q\&A.  
- [ ] Start Q\&A no later than minute 45:00.

---

*End of slide deck and demo script draft v0.1.*  
