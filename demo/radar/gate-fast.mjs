/**
 * Author: Deepankar Das
 */

/**
 * Enforcer demo — fast per-commit gates.
 *
 *   node demo/radar/gate-fast.mjs [--change <name>]
 *
 * G1  TypeScript check          npx tsc --noEmit
 * G2  Secret-leak scan          key-shaped literals in src/ and types/
 * G3  Touched-module tests      the RADAR-selected corpus for the change
 *
 * Prints a gate receipt and exits non-zero if any gate fails. This is a fast
 * filter, not full verification — it does not run the whole suite, Playwright,
 * or the Go tests.
 */

import { C, loadChange, select, tsc, secretScan, runVitest } from "./lib.mjs";

const argv = process.argv.slice(2);
const i = argv.indexOf("--change");
const change = loadChange(i !== -1 ? argv[i + 1] : null);
const { selected } = select(change);

const t0 = Date.now();
console.log("");
console.log(C.bold(`FAST GATES  ·  ${change.name}`));
console.log("");

const rows = [];

process.stdout.write("  G1  tsc --noEmit ... ");
const g1 = tsc();
console.log(g1.ok ? C.green("PASS") : C.red("FAIL"));
if (!g1.ok) console.log(C.dim(g1.output.split("\n").slice(0, 8).map((l) => "        " + l).join("\n")));
rows.push(["G1  TypeScript", g1.ok]);

process.stdout.write("  G2  secret-leak scan ... ");
const g2 = secretScan();
console.log(g2.ok ? C.green("PASS") : C.red("FAIL"));
if (!g2.ok) g2.hits.forEach((h) => console.log(C.red(`        ${h.file}  (${h.pattern})`)));
rows.push(["G2  Secret scan", g2.ok]);

process.stdout.write(`  G3  touched-module tests (${selected.length}) ... `);
const g3 = runVitest(selected.map((s) => s.test));
const g3ok = g3.failed === 0;
console.log(g3ok ? C.green(`PASS ${g3.passed}/${g3.total}`) : C.red(`FAIL ${g3.failed} failing`));
if (!g3ok && g3.firstFailure) console.log(C.red(`        FIRST FAILURE: ${g3.firstFailure.file} — ${g3.firstFailure.name}`));
rows.push(["G3  Touched-module tests", g3ok]);

const allOk = rows.every(([, ok]) => ok);
const secs = ((Date.now() - t0) / 1000).toFixed(1);

console.log("");
for (const [name, ok] of rows) console.log(`  ${ok ? C.green("✓") : C.red("✗")}  ${name}`);
console.log("");
console.log(`  FAST GATE VERDICT: ${allOk ? C.green("PASS") : C.red("FAIL")}   ${C.dim(secs + "s")}`);
console.log("");

process.exit(allOk ? 0 : 1);
