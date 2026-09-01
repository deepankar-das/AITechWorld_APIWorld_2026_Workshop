/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Policy Evaluation Engine
 *
 * Evaluates an ActionRequest against the loaded policy bundle and returns
 * a PolicyDecision with reason code and policy version.
 *
 * Evaluation order (TDD Section 8.3):
 *   1. Deny rules first
 *   2. Require_approval rules
 *   3. Allow rules
 *   4. Default deny (least privilege)
 *
 * TDD Reference: Section 8
 * PRD Reference: Appendix C, R-2
 */

import * as path from "node:path";
import { type ActionRequest } from "../../types/action.js";
import { PolicyDecisionType, type PolicyBundle, type PolicyDecision, type PolicyRule } from "../../types/policy.js";

/**
 * Match a single rule against an action request.
 * Returns true if the rule applies to this action.
 */
function ruleMatchesAction(rule: PolicyRule, request: ActionRequest): boolean {
  // Check action type match
  const actionTypes = rule.action.types;
  if (!actionTypes.includes("*") && !actionTypes.includes(request.action.type)) {
    return false;
  }

  // Check subject match (agent type)
  const agentTypes = rule.subject.agent_types;
  if (!agentTypes.includes("*") && !agentTypes.includes(request.actor.agent_type)) {
    return false;
  }

  // Check subject match (user)
  const users = rule.subject.users;
  if (!users.includes("*") && !users.includes(request.actor.user_id)) {
    return false;
  }

  // Check resource conditions
  const resourceConditions = rule.resource as Record<string, unknown>;

  // Path-based rules: check if path is inside or outside project root
  if (resourceConditions.path_outside_project === true) {
    const filePath = request.resource.path;
    if (!filePath) return false;
    const normalizedPath = path.resolve(filePath);
    const normalizedWorkspace = path.resolve(request.environment.workspace);
    // Rule matches when path IS outside project root
    if (normalizedPath.startsWith(normalizedWorkspace + path.sep) || normalizedPath === normalizedWorkspace) {
      return false; // Path is inside project — this "outside" rule does not match
    }
    return true;
  }

  // Sensitive path patterns
  if (resourceConditions.path_patterns) {
    const patterns = resourceConditions.path_patterns as string[];
    const filePath = request.resource.path;
    if (!filePath) return false;
    const normalizedPath = path.resolve(filePath);
    const homeDir = process.env.HOME || process.env.USERPROFILE || "";
    const matchesAny = patterns.some(pattern => {
      // Expand ~ to home directory
      const expandedPattern = pattern.replace(/^~/, homeDir);
      // Simple glob: * matches any segment
      if (expandedPattern.endsWith("/*")) {
        const dir = expandedPattern.slice(0, -2);
        return normalizedPath.startsWith(dir + path.sep);
      }
      return normalizedPath === expandedPattern;
    });
    return matchesAny;
  }

  // Host-based rules
  if (resourceConditions.host_not_in_allowlist === true) {
    // This rule matches when the host is NOT in the allowlist.
    // The allowlist check is done externally — if this rule is reached,
    // the enforcement point has already determined the host is not allowlisted.
    return request.resource.host !== undefined;
  }

  // Command pattern rules
  if (resourceConditions.command_patterns) {
    const patterns = resourceConditions.command_patterns as string[];
    const command = request.resource.value || "";
    return patterns.some(pattern => {
      if (pattern.endsWith("*")) {
        return command.startsWith(pattern.slice(0, -1));
      }
      return command.includes(pattern);
    });
  }

  // Path inside project (for allow rules)
  if (resourceConditions.path_inside_project === true) {
    const filePath = request.resource.path;
    if (!filePath) return false;
    const normalizedPath = path.resolve(filePath);
    const normalizedWorkspace = path.resolve(request.environment.workspace);
    return normalizedPath.startsWith(normalizedWorkspace + path.sep) || normalizedPath === normalizedWorkspace;
  }

  // Catch-all: if no resource conditions, rule matches all resources
  if (Object.keys(resourceConditions).length === 0) {
    return true;
  }

  return false;
}

/**
 * Evaluate an ActionRequest against a PolicyBundle.
 *
 * Returns a PolicyDecision with reason code and policy version.
 * Follows evaluation order: deny → require_approval → allow → default deny.
 */
export interface EvaluateOptions {
  /** If true, evaluate but don't enforce — log as "simulated" */
  simulate?: boolean;
}

export function evaluatePolicy(
  request: ActionRequest,
  bundle: PolicyBundle,
  options?: EvaluateOptions,
): PolicyDecision {
  // Partition rules by decision type
  const denyRules: PolicyRule[] = [];
  const approvalRules: PolicyRule[] = [];
  const allowRules: PolicyRule[] = [];

  for (const rule of bundle.rules) {
    switch (rule.effect.decision) {
      case "deny":
        denyRules.push(rule);
        break;
      case "require_approval":
        approvalRules.push(rule);
        break;
      case "allow":
        allowRules.push(rule);
        break;
      // Other decision types (redact, quarantine, simulate) handled in Phase 2
    }
  }

  // 1. Check deny rules first
  for (const rule of denyRules) {
    if (ruleMatchesAction(rule, request)) {
      return {
        request_id: request.request_id,
        decision: "deny",
        reason_code: rule.effect.reason_code,
        reason_human: rule.effect.reason_human,
        policy_id: rule.policy_id,
        policy_version: rule.version,
        approval_required: false,
      };
    }
  }

  // 2. Check require_approval rules
  for (const rule of approvalRules) {
    if (ruleMatchesAction(rule, request)) {
      return {
        request_id: request.request_id,
        decision: "require_approval",
        reason_code: rule.effect.reason_code,
        reason_human: rule.effect.reason_human,
        policy_id: rule.policy_id,
        policy_version: rule.version,
        approval_required: true,
      };
    }
  }

  // 3. Check allow rules
  for (const rule of allowRules) {
    if (ruleMatchesAction(rule, request)) {
      return {
        request_id: request.request_id,
        decision: "allow",
        reason_code: rule.effect.reason_code,
        reason_human: rule.effect.reason_human,
        policy_id: rule.policy_id,
        policy_version: rule.version,
        approval_required: false,
      };
    }
  }

  // 4. Default deny (least privilege — Principle P-9)
  const defaultDeny: PolicyDecision = {
    request_id: request.request_id,
    decision: "deny",
    reason_code: "DEFAULT_DENY",
    reason_human: "No matching policy rule. Default: deny (least privilege).",
    policy_id: "system.default_deny",
    policy_version: bundle.bundle_version,
    approval_required: false,
  };

  // In simulation mode: return the decision but mark it as simulated
  // The actual enforcement is skipped — action proceeds regardless
  if (options?.simulate) {
    return {
      ...defaultDeny,
      decision: "simulate" as PolicyDecisionType,
      reason_human: `[SIMULATED] ${defaultDeny.reason_human} (would have been: ${defaultDeny.decision})`,
    };
  }

  return defaultDeny;
}

/**
 * Evaluate in simulation mode — returns what WOULD happen without enforcing.
 * Used for dry-run policy testing.
 */
export function simulatePolicy(
  request: ActionRequest,
  bundle: PolicyBundle,
): PolicyDecision & { simulated_decision: string } {
  // Run the real evaluation
  const realResult = evaluatePolicy(request, bundle);

  // Return the result but flag it as simulated
  return {
    ...realResult,
    decision: "simulate" as PolicyDecisionType,
    reason_human: `[SIMULATED] ${realResult.reason_human} (would be: ${realResult.decision})`,
    simulated_decision: realResult.decision,
  };
}