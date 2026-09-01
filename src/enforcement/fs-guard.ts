/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Filesystem Guard
 *
 * Intercepts file read/write/delete operations, builds an ActionRequest,
 * and evaluates it against the daemon's policy engine.
 *
 * TDD Reference: Section 6.1 (file system interception)
 * PRD Reference: Appendix C, R-1
 */

import * as path from "node:path";
import { v4 as uuidv4 } from "uuid";
import type { ActionRequest, ActionType } from "../../types/action.js";
import type { PolicyDecision } from "../../types/policy.js";
import type { Environment } from "../../types/action.js";

/**
 * Map file operation to action type.
 */
function fileOpToActionType(op: "read" | "write" | "delete" | "move"): ActionType {
  switch (op) {
    case "read": return "file.read";
    case "write": return "file.write";
    case "delete": return "file.delete";
    case "move": return "file.move";
  }
}

export interface FsGuardOptions {
  /** Daemon base URL */
  daemonUrl: string;
  /** Actor metadata */
  actor: {
    user_id: string;
    agent_type: string;
    agent_instance: string;
    session_id: string;
  };
  /** Environment context */
  environment: Environment;
}

/**
 * Intercept a file operation and evaluate it against policy.
 *
 * Returns the PolicyDecision. On "deny", the caller should block the operation
 * and show the rationale to the agent/developer.
 */
export async function interceptFileOp(
  op: "read" | "write" | "delete" | "move",
  filePath: string,
  options: FsGuardOptions,
): Promise<PolicyDecision> {
  const absolutePath = path.resolve(filePath);
  const actionType = fileOpToActionType(op);

  const request: ActionRequest = {
    request_id: uuidv4(),
    timestamp: new Date().toISOString(),
    actor: options.actor,
    environment: options.environment,
    action: {
      type: actionType,
      attempted_action: `${actionType} ${absolutePath}`,
    },
    resource: {
      kind: "file",
      path: absolutePath,
      classification: [],
    },
  };

  const response = await fetch(`${options.daemonUrl}/v1/evaluate`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(request),
  });

  if (!response.ok) {
    // Fail-closed: treat daemon errors as deny
    return {
      request_id: request.request_id,
      decision: "deny",
      reason_code: "DAEMON_ERROR",
      reason_human: `Daemon returned ${response.status}. Fail-closed: action denied.`,
      policy_id: "system.fail_closed",
      policy_version: "unknown",
      approval_required: false,
    };
  }

  return response.json() as Promise<PolicyDecision>;
}