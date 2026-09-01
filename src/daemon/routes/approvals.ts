/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Approval API Routes
 *
 * REST endpoints for the approval workflow:
 *   POST   /v1/approvals           — create approval request
 *   GET    /v1/approvals/:id       — get approval status + context
 *   POST   /v1/approvals/:id/resolve — approve or deny
 *   GET    /v1/approvals/pending   — list all pending approvals
 *
 * TDD Reference: Section 10.3
 */

import type { ApprovalService } from "../../approval/service.js";
import { requestBreakGlass } from "../../approval/break-glass.js";

export interface ApprovalRouteResult {
  status: number;
  body: unknown;
}

/**
 * GET /v1/approvals/pending
 */
export function handleGetPending(approvalService: ApprovalService): ApprovalRouteResult {
  const pending = approvalService.getPending();
  return {
    status: 200,
    body: { approvals: pending, count: pending.length },
  };
}

/**
 * GET /v1/approvals/:id
 */
export function handleGetApproval(
  approvalId: string,
  approvalService: ApprovalService,
): ApprovalRouteResult {
  const result = approvalService.getApproval(approvalId);
  if (!result) {
    return { status: 404, body: { error: `Approval ${approvalId} not found` } };
  }
  return {
    status: 200,
    body: {
      approval_id: result.approval.approval_id,
      status: result.status,
      context_bundle: result.approval.context_bundle,
      timeout_seconds: result.approval.timeout_seconds,
      timeout_behavior: result.approval.timeout_behavior,
      created_at: result.approval.created_at,
    },
  };
}

/**
 * POST /v1/approvals/:id/resolve
 *
 * Body: { decision: "approve" | "deny", approver_id: string, rationale?: string, scope?: object, is_break_glass?: boolean }
 */
export function handleResolveApproval(
  approvalId: string,
  rawBody: string,
  approvalService: ApprovalService,
): ApprovalRouteResult {
  let body: Record<string, unknown>;
  try {
    body = JSON.parse(rawBody);
  } catch {
    return { status: 400, body: { error: "Invalid JSON" } };
  }

  const decision = body.decision as string;
  const approverId = body.approver_id as string;

  if (!decision || !["approve", "deny"].includes(decision)) {
    return { status: 400, body: { error: "decision must be 'approve' or 'deny'" } };
  }
  if (!approverId) {
    return { status: 400, body: { error: "approver_id is required" } };
  }

  try {
    // Handle break-glass
    if (body.is_break_glass) {
      const rationale = body.rationale as string;
      if (!rationale || rationale.trim().length === 0) {
        return { status: 400, body: { error: "Break-glass requires a non-empty rationale" } };
      }
      const bgDecision = requestBreakGlass(approvalId, approverId, rationale);
      approvalService.resolveApproval(approvalId, bgDecision);
      return {
        status: 200,
        body: { approval_id: approvalId, decision: "approve", is_break_glass: true },
      };
    }

    // Normal resolution
    approvalService.resolveApproval(approvalId, {
      approval_id: approvalId,
      decision: decision as "approve" | "deny",
      approver_id: approverId,
      rationale: (body.rationale as string) || undefined,
      scope: body.scope as { type: "single" | "session" | "time_bounded"; pattern?: string; expiry?: string } | undefined,
      is_break_glass: false,
    });

    return {
      status: 200,
      body: { approval_id: approvalId, decision, approver_id: approverId },
    };
  } catch (err) {
    const message = err instanceof Error ? err.message : "Unknown error";
    return { status: 404, body: { error: message } };
  }
}

/**
 * GET /v1/approvals/metrics
 */
export function handleGetApprovalMetrics(approvalService: ApprovalService): ApprovalRouteResult {
  return {
    status: 200,
    body: approvalService.getMetrics(),
  };
}