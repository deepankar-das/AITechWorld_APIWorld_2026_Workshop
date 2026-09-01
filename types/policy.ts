/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Policy Types
 *
 * Defines the PolicyDecision contract returned by the daemon to enforcement
 * points, and the PolicyRule schema used in YAML policy bundles.
 *
 * TDD Reference: Appendix B.4 (PolicyDecision), Section 8
 * PRD Reference: Appendix C, R-2
 */

import { z } from "zod";

// ── Decision Types ──────────────────────────────────────────────────────────

export const PolicyDecisionType = z.enum([
  "allow",
  "deny",
  "require_approval",
  "allow_degraded",    // Fail-open mode when policy cache is unavailable
  "redact",            // Phase 2
  "quarantine",        // Phase 2
  "simulate",          // Phase 2 (dry-run / log-only)
]);
export type PolicyDecisionType = z.infer<typeof PolicyDecisionType>;

// ── Policy Scope Level ──────────────────────────────────────────────────────

export const PolicyScopeLevel = z.enum([
  "organization",
  "team",
  "repository",
  "local",
]);
export type PolicyScopeLevel = z.infer<typeof PolicyScopeLevel>;

// ── Policy Effect (from YAML rule) ──────────────────────────────────────────

export const PolicyEffectSchema = z.object({
  decision: PolicyDecisionType,
  reason_code: z.string().min(1),
  reason_human: z.string().min(1),
});
export type PolicyEffect = z.infer<typeof PolicyEffectSchema>;

// ── Policy Rule (from YAML bundle) ──────────────────────────────────────────

export const PolicyRuleSchema = z.object({
  policy_id: z.string().min(1),
  version: z.string().min(1),
  scope: z.object({
    level: PolicyScopeLevel,
  }),
  subject: z.object({
    agent_types: z.array(z.string()).default(["*"]),
    users: z.array(z.string()).default(["*"]),
  }),
  action: z.object({
    types: z.array(z.string()),
  }),
  resource: z.record(z.unknown()).default({}),
  conditions: z.record(z.unknown()).default({}),
  effect: PolicyEffectSchema,
  logging: z.object({
    mode: z.enum(["full", "metadata_only", "redacted"]).default("full"),
  }).default({ mode: "full" }),
  approval: z.object({
    required: z.boolean().default(false),
  }).default({ required: false }),
});
export type PolicyRule = z.infer<typeof PolicyRuleSchema>;

// ── Policy Bundle (collection of rules with version) ────────────────────────

export const PolicyBundleSchema = z.object({
  bundle_version: z.string().min(1),
  scope_level: PolicyScopeLevel,
  rules: z.array(PolicyRuleSchema),
});
export type PolicyBundle = z.infer<typeof PolicyBundleSchema>;

// ── PolicyDecision (returned to enforcement point) ──────────────────────────

export const PolicyDecisionSchema = z.object({
  request_id: z.string().min(1),
  decision: PolicyDecisionType,
  reason_code: z.string().min(1),
  reason_human: z.string().min(1),
  policy_id: z.string().min(1),
  policy_version: z.string().min(1),
  approval_required: z.boolean(),
});
export type PolicyDecision = z.infer<typeof PolicyDecisionSchema>;