/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Approval Types
 *
 * Defines the ApprovalRequest and ApprovalDecision contracts
 * for the human-in-the-loop approval workflow.
 *
 * TDD Reference: Appendix B.4 (ApprovalRequest, ApprovalDecision), Section 10
 * PRD Reference: Appendix C, R-4
 */

import { z } from "zod";

// ── Context Bundle (presented to reviewer) ──────────────────────────────────

export const ContextBundleSchema = z.object({
  actor: z.string(),                     // "dev_001 via claude_code"
  resource: z.string(),                  // "rm -rf node_modules"
  risk_rationale: z.string(),            // "Matches destructive command pattern"
  policy_rule: z.string(),               // "org.approve_destructive_commands"
  agent_identity: z.string(),            // "claude_code (vscode_ext_1)"
  session_summary: z.string(),           // "12 actions, 0 blocks, 0 prior approvals"
});
export type ContextBundle = z.infer<typeof ContextBundleSchema>;

// ── Approval Scope ──────────────────────────────────────────────────────────

export const ApprovalScopeSchema = z.object({
  type: z.enum(["single", "session", "time_bounded"]),
  pattern: z.string().optional(),         // "npm install * from registry.npmjs.org"
  expiry: z.string().datetime().optional(), // ISO 8601 for time-bounded
});
export type ApprovalScope = z.infer<typeof ApprovalScopeSchema>;

// ── Timeout Behavior ────────────────────────────────────────────────────────

export const TimeoutBehavior = z.enum(["deny", "allow"]);
export type TimeoutBehavior = z.infer<typeof TimeoutBehavior>;

// ── ApprovalRequest ─────────────────────────────────────────────────────────

export const ApprovalRequestSchema = z.object({
  approval_id: z.string().min(1),
  request_id: z.string().min(1),
  context_bundle: ContextBundleSchema,
  timeout_seconds: z.number().int().positive().default(300),
  timeout_behavior: TimeoutBehavior.default("deny"),
  created_at: z.string().datetime(),
});
export type ApprovalRequest = z.infer<typeof ApprovalRequestSchema>;

// ── ApprovalDecision (from reviewer) ────────────────────────────────────────

export const ApprovalDecisionSchema = z.object({
  approval_id: z.string().min(1),
  decision: z.enum(["approve", "deny"]),
  approver_id: z.string().min(1),
  rationale: z.string().optional(),
  scope: ApprovalScopeSchema.optional(),
  is_break_glass: z.boolean().default(false),
});
export type ApprovalDecision = z.infer<typeof ApprovalDecisionSchema>;