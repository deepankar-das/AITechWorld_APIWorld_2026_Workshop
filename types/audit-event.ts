/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Audit Event Types
 *
 * Defines the AuditEvent contract emitted by the daemon for every
 * governed action. Includes minimum schema gate fields.
 *
 * TDD Reference: Section 9, Appendix B.4 (AuditEvent)
 * PRD Reference: Appendix C, R-3
 */

import { z } from "zod";

// ── Approval Status ─────────────────────────────────────────────────────────

export const ApprovalStatus = z.enum([
  "not_required",
  "pending",
  "approved",
  "denied",
  "expired",
  "deny_timeout",
  "allow_timeout_configured",
]);
export type ApprovalStatus = z.infer<typeof ApprovalStatus>;

// ── Approval Record ─────────────────────────────────────────────────────────

export const ApprovalRecordSchema = z.object({
  status: ApprovalStatus,
  approver_id: z.string().optional(),
  rationale: z.string().optional(),
  requested_at: z.string().datetime().optional(),
  resolved_at: z.string().datetime().optional(),
  scope: z.object({
    type: z.enum(["single", "session", "time_bounded"]),
    pattern: z.string().optional(),
    expiry: z.string().datetime().optional(),
  }).optional(),
  is_break_glass: z.boolean().default(false),
});
export type ApprovalRecord = z.infer<typeof ApprovalRecordSchema>;

// ── Payload Summary ─────────────────────────────────────────────────────────

export const PayloadSummarySchema = z.object({
  redacted: z.boolean().default(false),
  content_hash: z.string().optional(),
  bytes: z.number().optional(),
});
export type PayloadSummary = z.infer<typeof PayloadSummarySchema>;

// ── AuditEvent (full contract) ──────────────────────────────────────────────

export const AuditEventSchema = z.object({
  // Core identifiers
  event_id: z.string().min(1),
  timestamp: z.string().datetime(),
  session_id: z.string().min(1),
  correlation_id: z.string().min(1),

  // Minimum schema gate fields (all required — validated before storage)
  who: z.string().min(1),       // "{user_id}|{agent_type}|{session_id}"
  what: z.string().min(1),      // "{action.type}:{attempted_action}"
  when: z.string().min(1),      // timestamp
  policy: z.string().min(1),    // "{policy_id}@{policy_version}"
  decision: z.string().min(1),  // "{decision}:{reason_code}"
  result: z.string().min(1),    // "executed (exit 0)" | "blocked" | "pending"

  // Actor
  actor: z.object({
    user_id: z.string(),
    agent_type: z.string(),
    agent_instance: z.string(),
  }),

  // Environment
  environment: z.object({
    workspace: z.string(),
    repo: z.string(),
    branch: z.string(),
    tier: z.string(),
    deployment_mode: z.string(),
  }),

  // Action (with attempted + observed per R-1)
  action: z.object({
    type: z.string(),
    attempted_action: z.string(),
    observed_effect: z.string(),        // "executed (exit 0)" | "blocked" | "pending"
  }),

  // Resource
  resource: z.object({
    kind: z.string(),
    path: z.string().optional(),
    host: z.string().optional(),
    value: z.string().optional(),
    classification: z.array(z.string()).default([]),
  }),

  // Policy detail
  policy_detail: z.object({
    policy_id: z.string(),
    policy_version: z.string(),
    decision: z.string(),
    reason_code: z.string(),
    reason_human: z.string(),
  }),

  // Approval (optional — only present when approval was involved)
  approval: ApprovalRecordSchema.optional(),

  // Payload summary (optional)
  payload_summary: PayloadSummarySchema.optional(),
});
export type AuditEvent = z.infer<typeof AuditEventSchema>;

// ── Minimum Schema Gate Fields ──────────────────────────────────────────────
// These 6 fields must be present and non-empty for an event to pass the gate.

export const MINIMUM_GATE_FIELDS = [
  "who",
  "what",
  "when",
  "policy",
  "decision",
  "result",
] as const;

export type MinimumGateField = typeof MINIMUM_GATE_FIELDS[number];