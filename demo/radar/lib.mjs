/**
 * Author: Deepankar Das
 */

/**
 * Enforcer demo — shared helpers for the RADAR / gate scripts.
 *
 * These scripts are a teaching implementation of the Agentic Development Model
 * pipeline (risk-aware selection, mandatory floors, run-folder receipts) sized
 * for a live workshop. They run against Enforcer's real Vitest suite — the
 * PASS / FAIL results are genuine.
 */

import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import * as fs from "node:fs";
import * as path from "node:path";
import { fileURLToPath } from "node:url";

export const REPO_ROOT = path.resolve(fileURLToPath(new URL("../..", import.meta.url)));
export const RADAR_DIR = path.join(REPO_ROOT, "demo", "radar");
export const CHANGES_DIR = path.join(REPO_ROOT, "demo", "changes");
export const RUN_ROOT = path.join(REPO_ROOT, "demo", "run-folder");

export const C = {
  dim: (s) => `\x1b[2m${s}\x1b[0m`,
  bold: (s) => `\x1b[1m${s}\x1b[0m`,
  green: (s) => `\x1b[32m${s}\x1b[0m`,
  red: (s) => `\x1b[31m${s}\x1b[0m`,
  amber: (s) => `\x1b[33m${s}\x1b[0m`,
  cyan: (s) => `\x1b[36m${s}\x1b[0m`,
};

export function readJson(p) {
  return JSON.parse(fs.readFileSync(p, "utf8"));
}

export function floors() {
  return readJson(path.join(RADAR_DIR, "floors.json")).floors;
}

export function reachabilityMap() {
  return readJson(path.join(RADAR_DIR, "map.json")).reachability;
}

/**
 * Load a change manifest by name. Returns { name, touched: string[], summary }.
 * With no name, returns a synthetic "baseline" change touching nothing.
 */
export function loadChange(name) {
  if (!name || name === "baseline") {
    return { name: "baseline", summary: "no change applied — verifying the floors on a clean tree", touched: [] };
  }
  const mf = path.join(CHANGES_DIR, name, "manifest.json");
  if (!fs.existsSync(mf)) {
    throw new Error(`no change manifest at demo/changes/${name}/manifest.json`);
  }
  const m = readJson(mf);
  return { name, summary: m.summary || "", touched: m.touched || [] };
}

/**
 * Build the selected corpus for a change: reachability hits ∪ mandatory floors.
 * Returns { selected: [{test, dimension, reason, kind}], floorsEnforced: [...] }.
 */
export function select(change) {
  const rmap = reachabilityMap();
  const byTest = new Map();

  for (const f of floors()) {
    byTest.set(f.test, { test: f.test, dimension: f.dimension, reason: f.reason, kind: "floor", invariant: f.invariant });
  }

  for (const file of change.touched) {
    for (const entry of rmap) {
      if (file === entry.match || file.startsWith(entry.match)) {
        for (const t of entry.tests) {
          if (!byTest.has(t)) {
            byTest.set(t, { test: t, dimension: entry.dimension, reason: `${entry.reason} (${file})`, kind: "reachability" });
          }
        }
      }
    }
  }

  const selected = [...byTest.values()];
  return {
    selected,
    floorsEnforced: selected.filter((s) => s.kind === "floor").map((s) => s.test),
  };
}

/** sha256 over an arbitrary object, stable key order. */
export function hashOf(obj) {
  return createHash("sha256").update(JSON.stringify(obj, Object.keys(obj).sort())).digest("hex").slice(0, 12);
}

export function newRunId(change) {
  const stamp = new Date().toISOString().replace(/[:.]/g, "-").replace("T", "_").slice(0, 19);
  return `${stamp}__${change.name}`;
}

export function runFolder(runId) {
  const dir = path.join(RUN_ROOT, runId);
  fs.mkdirSync(dir, { recursive: true });
  return dir;
}

/**
 * Run a set of Vitest files and return a normalised result:
 * { passed, failed, total, firstFailure: {file, name} | null, files: [...] }
 */
export function runVitest(testFiles, { cwd = REPO_ROOT } = {}) {
  const outFile = path.join(RUN_ROOT, `.vitest-${Date.now()}.json`);
  fs.mkdirSync(RUN_ROOT, { recursive: true });
  let raw;
  try {
    execFileSync(
      "npx",
      ["vitest", "run", ...testFiles, "--reporter=json", `--outputFile=${outFile}`],
      { cwd, stdio: ["ignore", "ignore", "inherit"] },
    );
  } catch {
    /* non-zero exit on test failure is expected — the JSON file still gets written */
  }
  try {
    raw = readJson(outFile);
  } finally {
    fs.rmSync(outFile, { force: true });
  }

  const files = (raw.testResults || []).map((tr) => {
    const rel = path.relative(cwd, tr.name);
    const assertions = tr.assertionResults || [];
    const failed = assertions.filter((a) => a.status === "failed");
    return {
      file: rel,
      passed: assertions.filter((a) => a.status === "passed").length,
      failed: failed.length,
      firstFailure: failed[0] ? failed[0].fullName || failed[0].title : null,
    };
  });

  let firstFailure = null;
  for (const f of files) {
    if (f.firstFailure) { firstFailure = { file: f.file, name: f.firstFailure }; break; }
  }

  return {
    passed: raw.numPassedTests ?? 0,
    failed: raw.numFailedTests ?? 0,
    total: raw.numTotalTests ?? 0,
    firstFailure,
    files,
  };
}

export function tsc(cwd = REPO_ROOT) {
  try {
    execFileSync("npx", ["tsc", "--noEmit"], { cwd, stdio: ["ignore", "pipe", "pipe"] });
    return { ok: true };
  } catch (e) {
    return { ok: false, output: (e.stdout?.toString() || "") + (e.stderr?.toString() || "") };
  }
}

/** Cheap secret scan over the tracked source + any applied change overlay. */
export function secretScan(cwd = REPO_ROOT) {
  const patterns = [
    /sk_live_[0-9a-zA-Z]{24,}/,
    /xox[baprs]-[0-9]{10,13}-[0-9]{10,13}-[a-zA-Z0-9]{24,}/,
    /-----BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY-----/,
    /ghp_[A-Za-z0-9]{36}/,
    /AKIA[0-9A-Z]{16}(?![0-9A-Z])/,
  ];
  const roots = [path.join(cwd, "src"), path.join(cwd, "types")];
  const hits = [];
  const walk = (d) => {
    if (!fs.existsSync(d)) return;
    for (const ent of fs.readdirSync(d, { withFileTypes: true })) {
      const p = path.join(d, ent.name);
      if (ent.isDirectory()) walk(p);
      else if (/\.(ts|tsx|js|mjs|json|ya?ml)$/.test(ent.name)) {
        const text = fs.readFileSync(p, "utf8");
        for (const re of patterns) {
          if (re.test(text)) hits.push({ file: path.relative(cwd, p), pattern: re.source });
        }
      }
    }
  };
  roots.forEach(walk);
  return { ok: hits.length === 0, hits };
}

export function writeReceipt(dir, receipt) {
  fs.writeFileSync(path.join(dir, "receipt"), JSON.stringify(receipt, null, 2) + "\n");
}
