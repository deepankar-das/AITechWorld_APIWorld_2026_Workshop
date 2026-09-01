/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Policy Hierarchy Merge
 *
 * Merges policy bundles from org → team → repo → local.
 * Lower levels can only tighten (add restrictions), never weaken baselines.
 *
 * Tightening rules:
 *   - A lower level can add new deny or require_approval rules
 *   - A lower level can escalate an allow to require_approval or deny
 *   - A lower level CANNOT change a deny to allow or require_approval to allow
 *
 * TDD Reference: Section 8.1
 * PRD Reference: Appendix C, R-2, Principle P-4
 */

import { type PolicyBundle, type PolicyRule } from "../../types/policy.js";

/** Decision severity for tightening comparison. Higher = stricter. */
const DECISION_SEVERITY: Record<string, number> = {
  allow: 0,
  simulate: 1,
  require_approval: 2,
  redact: 3,
  quarantine: 4,
  deny: 5,
};

/**
 * Check if a rule from a lower-level bundle is a valid tightening of
 * existing rules. A rule that would weaken enforcement is rejected.
 */
function isValidTightening(existingRules: PolicyRule[], newRule: PolicyRule): boolean {
  // New deny or require_approval rules are always valid tightening
  const newSeverity = DECISION_SEVERITY[newRule.effect.decision] ?? 0;
  if (newSeverity >= 2) {
    return true; // Adding restrictions is always valid
  }

  // New allow rules must not weaken an existing deny or require_approval
  // for the same action types and resource patterns
  for (const existing of existingRules) {
    const existingSeverity = DECISION_SEVERITY[existing.effect.decision] ?? 0;
    // Check if rules overlap (same action types)
    const overlap = existing.action.types.some(
      t => t === "*" || newRule.action.types.includes(t) || newRule.action.types.includes("*"),
    );
    if (overlap && newSeverity < existingSeverity) {
      return false; // Attempting to weaken — rejected
    }
  }

  return true;
}

/**
 * Merge multiple policy bundles in hierarchy order.
 * First bundle is the org baseline. Subsequent bundles are
 * team → repo → local layers that can only tighten.
 *
 * Returns a merged bundle with the org version as the base version.
 */
export function mergeHierarchy(...bundles: (PolicyBundle | undefined)[]): PolicyBundle {
  const defined = bundles.filter((b): b is PolicyBundle => b !== undefined);

  if (defined.length === 0) {
    return {
      bundle_version: "v0.0.0",
      scope_level: "organization",
      rules: [],
    };
  }

  const base = defined[0];
  const mergedRules: PolicyRule[] = [...base.rules];

  for (let i = 1; i < defined.length; i++) {
    const overlay = defined[i];
    for (const rule of overlay.rules) {
      if (isValidTightening(mergedRules, rule)) {
        mergedRules.push(rule);
      }
      // Invalid tightenings are silently dropped.
      // In production, this should emit a warning log.
    }
  }

  return {
    bundle_version: base.bundle_version,
    scope_level: base.scope_level,
    rules: mergedRules,
  };
}