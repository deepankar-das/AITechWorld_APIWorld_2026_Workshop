/**
 * Author: Deepankar Das
 */

/**
 * Enforcer demo — RADAR selection.
 *
 *   node demo/radar/select.mjs [--change <name>]
 *
 * Prints the risk-weighted corpus for the change: reachability hits from
 * demo/radar/map.json unioned with the mandatory floors from
 * demo/radar/floors.json. Each row carries the reason it was selected and the
 * blast-radius dimension it covers. Writes selection.json into a fresh
 * run folder and prints that path.
 */

import * as fs from "node:fs";
import * as path from "node:path";
import { C, loadChange, select, newRunId, runFolder, RUN_ROOT } from "./lib.mjs";

const argv = process.argv.slice(2);
const changeName = (() => {
  const i = argv.indexOf("--change");
  return i !== -1 ? argv[i + 1] : null;
})();

const change = loadChange(changeName);
const { selected, floorsEnforced } = select(change);

const runId = newRunId(change);
const dir = runFolder(runId);
const selectionPath = path.join(dir, "selection.json");
fs.writeFileSync(
  selectionPath,
  JSON.stringify({ change, selected, floorsEnforced, generated: new Date().toISOString() }, null, 2) + "\n",
);
// pointer to the latest run for run.mjs
fs.writeFileSync(path.join(RUN_ROOT, ".latest"), runId + "\n");

console.log("");
console.log(C.bold("RADAR  ·  Risk-Aware Dependency Analysis for Rapid Verification"));
console.log(C.dim(`change: ${change.name}   ${change.summary ? "— " + change.summary : ""}`));
if (change.touched.length) {
  console.log(C.dim(`touched: ${change.touched.join(", ")}`));
}
console.log("");

const pad = (s, n) => (s.length >= n ? s + "  " : s + " ".repeat(n - s.length));
const TESTW = 54;
const DIMW = 26;
console.log(C.dim("  " + pad("test", TESTW + 8) + pad("dimension", DIMW) + "why"));
console.log(C.dim("  " + "-".repeat(TESTW + 8 + DIMW + 40)));
for (const s of selected) {
  const tag = s.kind === "floor" ? C.amber("[floor] ") : "        ";
  console.log("  " + tag + pad(s.test, TESTW) + pad(s.dimension, DIMW) + C.dim(s.reason));
}
console.log("");
console.log(`  selected corpus: ${C.bold(String(selected.length))} tests  ` +
  C.dim(`(${floorsEnforced.length} mandatory floors enforced, ${selected.length - floorsEnforced.length} by reachability)`));
console.log(C.dim(`  selection written to demo/run-folder/${runId}/selection.json`));
console.log("");
