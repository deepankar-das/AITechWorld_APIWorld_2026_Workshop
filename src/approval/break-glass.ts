/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Break-Glass Emergency Override
 *
 * Bypasses normal approval routing when an emergency requires immediate action.
 * Requires explicit rationale (non-empty). Logged with elevated severity.
 *
 * TDD Reference: Section 10.2
 * PRD Reference: Appendix C, R-4
 */

import { v4 as uuidv4 } from "uuid";
import type { ApprovalDecision } from "../../types/approval.js";

/**
 * Create a break-glass approval decision.
 *
 * Validates that rationale is provided (non-empty).
 * The caller is responsible for logging this with elevated audit severity.
 */
export function requestBreakGlass(
  approvalId: string,
  approverId: string,
  rationale: string,
): ApprovalDecision {
  if (!rationale || rationale.trim().length === 0) {
    throw new Error("Break-glass access requires a non-empty rationale.");
  }

  return {
    approval_id: approvalId || `bg_${uuidv4().slice(0, 8)}`,
    decision: "approve",
    approver_id: approverId,
    rationale: `[BREAK-GLASS] ${rationale}`,
    is_break_glass: true,
  };
}