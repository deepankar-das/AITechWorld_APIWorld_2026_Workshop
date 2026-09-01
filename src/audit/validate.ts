/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Minimum Schema Validation Gate
 *
 * Every audit event must pass this gate before acceptance into the store.
 * Rejects events missing any of the 6 minimum gate fields.
 *
 * TDD Reference: Section 9.2
 * PRD Reference: Appendix C, R-3
 */

import { MINIMUM_GATE_FIELDS, type AuditEvent } from "../../types/audit-event.js";

export interface ValidationResult {
  valid: boolean;
  errors: string[];
}

/**
 * Validates that an audit event contains all minimum schema gate fields.
 * Each field must be present and non-empty.
 *
 * Gate fields: who, what, when, policy, decision, result
 */
export function validateAuditEvent(event: unknown): ValidationResult {
  const errors: string[] = [];

  if (event === null || event === undefined || typeof event !== "object") {
    return { valid: false, errors: ["Event is null, undefined, or not an object"] };
  }

  const record = event as Record<string, unknown>;

  for (const field of MINIMUM_GATE_FIELDS) {
    const value = record[field];
    if (value === undefined || value === null) {
      errors.push(`Missing required field: ${field}`);
    } else if (typeof value !== "string") {
      errors.push(`Field ${field} must be a string, got ${typeof value}`);
    } else if (value.trim().length === 0) {
      errors.push(`Field ${field} is empty`);
    }
  }

  return {
    valid: errors.length === 0,
    errors,
  };
}

/**
 * Builds the minimum gate fields from a full AuditEvent.
 * Used when constructing events to ensure gate fields are populated.
 */
export function buildGateFields(event: {
  actor: { user_id: string; agent_type: string };
  session_id: string;
  action: { type: string; attempted_action: string };
  timestamp: string;
  policy_detail: { policy_id: string; policy_version: string };
  decision: string;
  result: string;
}): Pick<AuditEvent, "who" | "what" | "when" | "policy" | "decision" | "result"> {
  return {
    who: `${event.actor.user_id}|${event.actor.agent_type}|${event.session_id}`,
    what: `${event.action.type}:${event.action.attempted_action}`,
    when: event.timestamp,
    policy: `${event.policy_detail.policy_id}@${event.policy_detail.policy_version}`,
    decision: event.decision,
    result: event.result,
  };
}