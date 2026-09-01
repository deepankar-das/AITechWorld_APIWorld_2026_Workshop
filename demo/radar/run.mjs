/**
 * Author: Deepankar Das
 */

/**
 * Enforcer demo — run the RADAR-selected corpus.
 *
 *   node demo/radar/run.mjs [--change <name>] [--run <runId>]
 *
 * Runs the selected Vitest files, prints PASS / FAIL with the FIRST FAILURE
 * preserved in stable order, and writes run.json + a receipt (hashed against
 * the change + selection) into the run folder.
 */

import * as fs from "node:fs";
import * as path from "node:path";
import {
  C, RUN_ROOT, loadChange, select, newRunId, runFolder,
  runVitest, hashOf, writeReceipt, readJson,
} from "./lib.mjs";

const argv = process.argv.slice(2);
const arg = (k) => { const i = argv.indexOf(k); return i !== -1 ? argv[i + 1] : null; };

let runId = arg("--run");
let change, selected, floorsEnforced;

if (runId) {
  const sel = readJson(path.join(RUN_ROOT, runId, "selection.json"));
  ({ change, selected, floorsEnforced } = sel);
} else if (fs.existsSync(path.join(RUN_ROOT, ".latest")) && !arg("--change")) {
  runId = fs.readFileSync(path.join(RUN_ROOT, ".latest"), "utf8").trim();
  const sel = readJson(path.join(RUN_ROOT, runId, "selection.json"));
  ({ change, selected, floorsEnforced } = sel);
} else {
  change = loadChange(arg("--change"));
  ({ selected, floorsEnforced } = select(change));
  runId = newRunId(change);
}

const dir = runFolder(runId);
const testFiles = selected.map((s) => s.test);

console.log("");
console.log(C.bold(`RADAR run  ·  ${change.name}  ·  ${testFiles.length} tests`));
console.log(C.dim(`  ${runId}`));
console.log("");

const result = runVitest(testFiles);

for (const f of result.files) {
  const ok = f.failed === 0;
  const mark = ok ? C.green("PASS") : C.red("FAIL");
  console.log(`  ${mark}  ${f.file}  ${C.dim(`${f.passed}/${f.passed + f.failed}`)}`);
  if (!ok && f.firstFailure) console.log(C.red(`        ↳ ${f.firstFailure}`));
}

const verdict = result.failed === 0 ? "PASS" : "FAIL";
console.log("");
console.log(`  VERDICT: ${verdict === "PASS" ? C.green(verdict) : C.red(verdict)}   ` +
  C.dim(`${result.passed} passed, ${result.failed} failed, ${result.total} total`));
if (result.firstFailure) {
  console.log(C.red(`  FIRST FAILURE: ${result.firstFailure.file}`));
  console.log(C.red(`                 ${result.firstFailure.name}`));
}

const receipt = {
  run_id: runId,
  change: change.name,
  change_hash: hashOf({ name: change.name, touched: change.touched }),
  selection_hash: hashOf({ tests: testFiles.sort() }),
  floors_enforced: floorsEnforced,
  results: { verdict, passed: result.passed, failed: result.failed, total: result.total },
  first_failure: result.firstFailure,
  admissible_for_merge: verdict === "PASS",
  generated: new Date().toISOString(),
};

fs.writeFileSync(path.join(dir, "run.json"), JSON.stringify({ ...receipt, files: result.files }, null, 2) + "\n");
writeReceipt(dir, receipt);

console.log("");
console.log(C.dim(`  run.json + receipt written to demo/run-folder/${runId}/`));
console.log(`  ${verdict === "PASS" ? C.green("admissible_for_merge: true") : C.red("admissible_for_merge: false")}` +
  C.dim("   (milestone validation — broader pen-test suite, load, rollback — still required before rollout)"));
console.log("");

process.exit(verdict === "PASS" ? 0 : 1);
