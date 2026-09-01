/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Policy Bundle Loader
 *
 * Loads YAML policy bundles from disk, parses them into typed PolicyBundle
 * objects, and supports version tracking for rollback.
 *
 * TDD Reference: Section 8
 */

import * as fs from "node:fs";
import * as path from "node:path";
import * as yaml from "js-yaml";
import { PolicyBundleSchema, type PolicyBundle } from "../../types/policy.js";

export interface LoadedBundle {
  bundle: PolicyBundle;
  loadedAt: string;
  filePath: string;
}

/**
 * Load a YAML policy bundle from a file path.
 * Validates against the PolicyBundle Zod schema.
 */
export function loadPolicyBundle(filePath: string): LoadedBundle {
  const absolutePath = path.resolve(filePath);
  const raw = fs.readFileSync(absolutePath, "utf-8");
  const parsed = yaml.load(raw) as unknown;
  const bundle = PolicyBundleSchema.parse(parsed);

  return {
    bundle,
    loadedAt: new Date().toISOString(),
    filePath: absolutePath,
  };
}

/**
 * Load all YAML files from a directory and merge them into a single bundle.
 * Files are loaded in alphabetical order. Later files can add rules but
 * the bundle_version comes from the first file.
 */
export function loadPolicyDirectory(dirPath: string): LoadedBundle {
  const absoluteDir = path.resolve(dirPath);
  const files = fs.readdirSync(absoluteDir)
    .filter(f => f.endsWith(".yaml") || f.endsWith(".yml"))
    .sort();

  if (files.length === 0) {
    throw new Error(`No YAML policy files found in ${absoluteDir}`);
  }

  let mergedBundle: PolicyBundle | null = null;

  for (const file of files) {
    const filePath = path.join(absoluteDir, file);
    const loaded = loadPolicyBundle(filePath);

    if (mergedBundle === null) {
      mergedBundle = loaded.bundle;
    } else {
      mergedBundle.rules.push(...loaded.bundle.rules);
    }
  }

  return {
    bundle: mergedBundle!,
    loadedAt: new Date().toISOString(),
    filePath: absoluteDir,
  };
}