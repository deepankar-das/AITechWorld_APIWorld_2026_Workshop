/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Shell Proxy
 *
 * Intercepts shell command execution, classifies the command,
 * builds an ActionRequest, and evaluates it against policy.
 *
 * On "deny": blocks execution and returns rationale.
 * On "require_approval": pauses until approval resolves.
 * On "allow": executes and captures exit code as observed_effect.
 *
 * TDD Reference: Section 6.1 (shell command interception)
 * PRD Reference: Appendix C, R-1
 */

import { v4 as uuidv4 } from "uuid";
import { classifyCommand } from "./command-classifier.js";
import type { ActionRequest } from "../../types/action.js";
import type { PolicyDecision } from "../../types/policy.js";
import type { Environment } from "../../types/action.js";

export interface ShellProxyOptions {
  daemonUrl: string;
  actor: {
    user_id: string;
    agent_type: string;
    agent_instance: string;
    session_id: string;
  };
  environment: Environment;
}

/**
 * Intercept a shell command and evaluate it against policy.
 *
 * Returns the PolicyDecision. The caller is responsible for:
 *   - Blocking execution on "deny"
 *   - Pausing execution on "require_approval" until resolved
 *   - Executing on "allow" and reporting observed_effect
 */
export async function interceptCommand(
  command: string,
  cwd: string,
  options: ShellProxyOptions,
): Promise<PolicyDecision> {
  const classifications = classifyCommand(command);

  const request: ActionRequest = {
    request_id: uuidv4(),
    timestamp: new Date().toISOString(),
    actor: options.actor,
    environment: options.environment,
    action: {
      type: "shell.exec",
      attempted_action: command,
    },
    resource: {
      kind: "command",
      value: command,
      path: cwd,
      classification: classifications,
    },
  };

  const response = await fetch(`${options.daemonUrl}/v1/evaluate`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(request),
  });

  if (!response.ok) {
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