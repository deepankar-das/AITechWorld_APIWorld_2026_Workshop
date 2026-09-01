/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Reusable Approval Scopes
 *
 * Checks if a new action matches an existing approved scope.
 * When a scope matches, the action is auto-approved without re-prompting.
 *
 * Scope types:
 *   - single: one action only (default — no reuse)
 *   - session: all matching actions in this session
 *   - time_bounded: matching actions until expiry
 *
 * TDD Reference: Section 10.2
 * PRD Reference: Appendix C, R-4
 */

import type { ActionRequest } from "../../types/action.js";
import type { ApprovalScope } from "../../types/approval.js";

/**
 * Check if an action matches an approval scope.
 *
 * Pattern matching:
 *   - Exact match: "npm install express" matches "npm install express"
 *   - Wildcard prefix: "npm install *" matches any "npm install ..."
 *   - Action type prefix: "shell.exec:*" matches any shell command
 */
export function matchesScope(
  request: ActionRequest,
  scope: ApprovalScope,
): boolean {
  // Single-use scopes never match a second time
  if (scope.type === "single") {
    return false;
  }

  // Time-bounded scopes: check expiry
  if (scope.type === "time_bounded" && scope.expiry) {
    if (new Date(scope.expiry).getTime() < Date.now()) {
      return false; // Expired
    }
  }

  // If no pattern, scope matches everything in the session
  if (!scope.pattern) {
    return true;
  }

  const pattern = scope.pattern;
  const actionString = `${request.action.type}:${request.resource.value || request.resource.path || ""}`;

  // Exact match
  if (actionString === pattern) {
    return true;
  }

  // Wildcard matching
  if (pattern.endsWith("*")) {
    const prefix = pattern.slice(0, -1);
    if (actionString.startsWith(prefix)) {
      return true;
    }
    // Also check just the resource value
    const resourceValue = request.resource.value || request.resource.path || "";
    if (resourceValue.startsWith(prefix)) {
      return true;
    }
  }

  // Check resource value directly against pattern
  const resourceValue = request.resource.value || request.resource.path || "";
  if (resourceValue === pattern) {
    return true;
  }

  return false;
}