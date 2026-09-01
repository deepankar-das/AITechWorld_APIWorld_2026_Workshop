/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — /v1/evaluate Route Handler
 *
 * Accepts an ActionRequest from enforcement points, evaluates it
 * against the policy bundle, emits an audit event, and returns
 * the PolicyDecision.
 *
 * TDD Reference: Section 7.2
 */

import { v4 as uuidv4 } from "uuid";
import { ActionRequestSchema, type ActionRequest } from "../../../types/action.js";
import { type PolicyBundle, type PolicyDecision } from "../../../types/policy.js";
import { type AuditEvent } from "../../../types/audit-event.js";
import { evaluatePolicy } from "../../policy/engine.js";
import { buildGateFields } from "../../audit/validate.js";
import { type AuditBuffer } from "../../audit/buffer.js";
import { type ApprovalService } from "../../approval/service.js";

export interface EvaluateResult {
  status: number;
  body: PolicyDecision | { error: string; details?: string[] };
}

/**
 * Handle POST /v1/evaluate
 *
 * 1. Parse and validate the ActionRequest
 * 2. Check active approval scopes (auto-approve if matching)
 * 3. Evaluate policy
 * 4. If require_approval: create approval and wait (async)
 * 4. Return PolicyDecision
 */
export function handleEvaluate(
  rawBody: string,
  policyBundle: PolicyBundle,
  auditBuffer: AuditBuffer,
  approvalService?: ApprovalService,
): EvaluateResult {
  const evaluateStart = Date.now();

  // 1. Parse request body
  let parsed: unknown;
  try {
    parsed = JSON.parse(rawBody);
  } catch {
    return {
      status: 400,
      body: { error: "Invalid JSON" },
    };
  }

  // 2. Validate against ActionRequest schema
  const validation = ActionRequestSchema.safeParse(parsed);
  if (!validation.success) {
    return {
      status: 400,
      body: {
        error: "Invalid ActionRequest",
        details: validation.error.issues.map(i => `${i.path.join(".")}: ${i.message}`),
      },
    };
  }

  const request: ActionRequest = validation.data;

  // 2b. Check active approval scopes (auto-approve without re-prompting)
  if (approvalService) {
    const scopeMatch = approvalService.checkScope(request);
    if (scopeMatch) {
      // Auto-approved by matching scope — return allow decision
      const autoDecision: PolicyDecision = {
        request_id: request.request_id,
        decision: "allow",
        reason_code: "SCOPE_AUTO_APPROVED",
        reason_human: scopeMatch.rationale || "Auto-approved by matching scope.",
        policy_id: "scope.auto",
        policy_version: policyBundle.bundle_version,
        approval_required: false,
      };

      // Build and buffer audit event for scope match
      const autoGateFields = buildGateFields({
        actor: request.actor,
        session_id: request.actor.session_id,
        action: request.action,
        timestamp: request.timestamp,
        policy_detail: { policy_id: "scope.auto", policy_version: policyBundle.bundle_version },
        decision: "allow:SCOPE_AUTO_APPROVED",
        result: "pending",
      });

      const autoAuditEvent: AuditEvent = {
        event_id: uuidv4(),
        timestamp: request.timestamp,
        session_id: request.actor.session_id,
        correlation_id: request.request_id,
        ...autoGateFields,
        actor: { user_id: request.actor.user_id, agent_type: request.actor.agent_type, agent_instance: request.actor.agent_instance },
        environment: { workspace: request.environment.workspace, repo: request.environment.repo, branch: request.environment.branch, tier: request.environment.tier, deployment_mode: request.environment.deployment_mode },
        action: { type: request.action.type, attempted_action: request.action.attempted_action, observed_effect: "pending" },
        resource: { kind: request.resource.kind, path: request.resource.path, host: request.resource.host, value: request.resource.value, classification: request.resource.classification || [] },
        policy_detail: { policy_id: "scope.auto", policy_version: policyBundle.bundle_version, decision: "allow", reason_code: "SCOPE_AUTO_APPROVED", reason_human: "Auto-approved by matching scope." },
        approval: { status: "approved", approver_id: "scope_auto", rationale: scopeMatch.rationale, is_break_glass: false },
      };

      auditBuffer.bufferEvent(autoAuditEvent);
      return { status: 200, body: autoDecision };
    }
  }

  // 3. Evaluate policy
  const decision: PolicyDecision = evaluatePolicy(request, policyBundle);

  // 4. Build audit event
  const observedEffect = decision.decision === "allow"
    ? "pending"  // Will be updated by post_tool_call hook with actual result
    : decision.decision === "deny"
      ? "blocked"
      : "pending_approval";

  const decisionString = `${decision.decision}:${decision.reason_code}`;

  const gateFields = buildGateFields({
    actor: request.actor,
    session_id: request.actor.session_id,
    action: request.action,
    timestamp: request.timestamp,
    policy_detail: {
      policy_id: decision.policy_id,
      policy_version: decision.policy_version,
    },
    decision: decisionString,
    result: observedEffect,
  });

  const auditEvent: AuditEvent = {
    event_id: uuidv4(),
    timestamp: request.timestamp,
    session_id: request.actor.session_id,
    correlation_id: request.request_id,
    ...gateFields,
    actor: {
      user_id: request.actor.user_id,
      agent_type: request.actor.agent_type,
      agent_instance: request.actor.agent_instance,
    },
    environment: {
      workspace: request.environment.workspace,
      repo: request.environment.repo,
      branch: request.environment.branch,
      tier: request.environment.tier,
      deployment_mode: request.environment.deployment_mode,
    },
    action: {
      type: request.action.type,
      attempted_action: request.action.attempted_action,
      observed_effect: observedEffect,
    },
    resource: {
      kind: request.resource.kind,
      path: request.resource.path,
      host: request.resource.host,
      value: request.resource.value,
      classification: request.resource.classification || [],
    },
    policy_detail: {
      policy_id: decision.policy_id,
      policy_version: decision.policy_version,
      decision: decision.decision,
      reason_code: decision.reason_code,
      reason_human: decision.reason_human,
    },
  };

  // 5. Buffer audit event (async — never blocks the decision path)
  auditBuffer.bufferEvent(auditEvent);

  // 6. Log latency
  const evaluateLatencyMs = Date.now() - evaluateStart;
  if (evaluateLatencyMs > 50) {
    console.warn(`[EVALUATE] Slow policy decision: ${evaluateLatencyMs}ms for ${request.action.type} (target: p95 <50ms)`);
  }

  // 7. Return decision
  return {
    status: 200,
    body: decision,
  };
}