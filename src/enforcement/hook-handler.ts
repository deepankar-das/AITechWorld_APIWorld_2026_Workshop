#!/usr/bin/env npx tsx
/**
 * Author: Deepankar Das
 */
/**
 * Enforcer — Claude Code Hook Handler
 *
 * Called by Claude Code's hooks system (PreToolUse / PostToolUse).
 * Receives tool input via stdin as JSON, evaluates it against the daemon,
 * and exits with:
 *   0 — allow (tool proceeds)
 *   2 — block (tool is denied, stderr shown to user as reason)
 *
 * Usage (called by Claude Code, not directly):
 *   echo '{"tool":"Bash","input":{"command":"rm -rf /"}}' | npx tsx src/enforcement/hook-handler.ts pre_tool_call
 *
 * TDD Reference: Section 6.2 (Claude Code Integration)
 */

import * as path from "node:path";
import { v4 as uuidv4 } from "uuid";
import type { ActionRequest, ActionType } from "../../types/action.js";
import { classifyCommand } from "./command-classifier.js";
import { detectSecretAccess } from "./secret-detector.js";
import { detectPackageInstall } from "./package-guard.js";

const DAEMON_URL = process.env.AA_DAEMON_URL || "http://127.0.0.1:9100";
const SESSION_ID = process.env.AA_SESSION_ID || `sess_${uuidv4().slice(0, 8)}`;
const WORKSPACE = process.env.AA_WORKSPACE || process.cwd();

// ── Tool-to-Action Mapping ──────────────────────────────────────────────────

interface ToolMapping {
  actionType: ActionType;
  resourceKind: "file" | "command" | "host";
  extractPath?: (input: Record<string, unknown>) => string | undefined;
  extractValue?: (input: Record<string, unknown>) => string | undefined;
  extractHost?: (input: Record<string, unknown>) => string | undefined;
}

const TOOL_MAP: Record<string, ToolMapping> = {
  Read: {
    actionType: "file.read",
    resourceKind: "file",
    extractPath: (input) => input.file_path as string | undefined,
  },
  Edit: {
    actionType: "file.write",
    resourceKind: "file",
    extractPath: (input) => input.file_path as string | undefined,
  },
  Write: {
    actionType: "file.write",
    resourceKind: "file",
    extractPath: (input) => input.file_path as string | undefined,
  },
  Bash: {
    actionType: "shell.exec",
    resourceKind: "command",
    extractValue: (input) => input.command as string | undefined,
  },
  Glob: {
    actionType: "file.read",
    resourceKind: "file",
    // Glob searches within workspace — pass workspace as path so path_inside_project matches
    extractPath: () => WORKSPACE,
    extractValue: (input) => input.pattern as string | undefined,
  },
  Grep: {
    actionType: "file.read",
    resourceKind: "file",
    // Grep searches within workspace — pass workspace as path so path_inside_project matches
    extractPath: () => WORKSPACE,
    extractValue: (input) => input.pattern as string | undefined,
  },
  WebFetch: {
    actionType: "network.request",
    resourceKind: "host",
    extractHost: (input) => {
      const url = input.url as string | undefined;
      if (!url) return undefined;
      try { return new URL(url).hostname; } catch { return url; }
    },
    extractValue: (input) => input.url as string | undefined,
  },
  WebSearch: {
    actionType: "network.request",
    resourceKind: "host",
    extractValue: (input) => input.query as string | undefined,
  },
};

// ── Read stdin ──────────────────────────────────────────────────────────────

async function readStdin(): Promise<string> {
  return new Promise((resolve) => {
    let data = "";
    process.stdin.setEncoding("utf-8");
    process.stdin.on("data", (chunk) => { data += chunk; });
    process.stdin.on("end", () => resolve(data));
    // If stdin is empty or not piped, resolve after short timeout
    setTimeout(() => resolve(data), 100);
  });
}

// ── Build ActionRequest ─────────────────────────────────────────────────────

function buildActionRequest(
  toolName: string,
  toolInput: Record<string, unknown>,
): ActionRequest | null {
  const mapping = TOOL_MAP[toolName];
  if (!mapping) return null;

  const filePath = mapping.extractPath?.(toolInput);
  const value = mapping.extractValue?.(toolInput);
  const host = mapping.extractHost?.(toolInput);

  let classifications = mapping.actionType === "shell.exec" && value
    ? classifyCommand(value)
    : [];

  // Enrich: detect secret/credential access
  const secretCheck = detectSecretAccess(filePath, value);
  if (secretCheck.isSensitive) {
    classifications = [...classifications.filter(c => c !== "safe"), "sensitive_path"];
  }

  // Enrich: detect package install and upgrade action type
  let actionType = mapping.actionType;
  if (value) {
    const pkgCheck = detectPackageInstall(value);
    if (pkgCheck.isPackageInstall) {
      actionType = "package.install";
      classifications = [...classifications.filter(c => c !== "safe"), "package_manager"];
    }
  }

  const attemptedAction = value
    || (filePath ? `${actionType} ${filePath}` : `${toolName} tool call`);

  return {
    request_id: uuidv4(),
    timestamp: new Date().toISOString(),
    actor: {
      user_id: process.env.USER || "unknown",
      agent_type: "claude_code",
      agent_instance: "vscode_extension",
      session_id: SESSION_ID,
    },
    environment: {
      workspace: WORKSPACE,
      repo: "",
      branch: "",
      tier: "development",
      deployment_mode: "host",
    },
    action: {
      type: actionType,
      attempted_action: attemptedAction,
    },
    resource: {
      kind: mapping.resourceKind,
      path: filePath ? path.resolve(filePath) : undefined,
      host,
      value,
      classification: classifications,
    },
  };
}

// ── Main ────────────────────────────────────────────────────────────────────

async function main() {
  const hookType = process.argv[2] || "pre_tool_call";

  // Read tool input from stdin (needed for both pre and post hooks)
  const stdinData = await readStdin();
  if (!stdinData.trim()) {
    process.exit(0);
  }

  let parsed: Record<string, unknown>;
  try {
    parsed = JSON.parse(stdinData);
  } catch {
    process.exit(0);
  }

  // Claude Code sends: { tool_name, tool_input, session_id, hook_event_name, ... }
  // Normalize field names (support both old format and Claude Code format)
  const toolName = (parsed.tool_name || parsed.tool || "unknown") as string;
  const toolInput = (parsed.tool_input || parsed.input || {}) as Record<string, unknown>;
  const hookSessionId = (parsed.session_id || SESSION_ID) as string;

  // Post-tool hooks: report observed_effect back to daemon for audit enrichment
  if (hookType === "post_tool_call") {
    const output = (parsed.tool_output || parsed.output || {}) as Record<string, unknown>;
    const error = parsed.error as string | undefined;

    const observedEffect = error
      ? `error: ${error}`
      : output.exit_code !== undefined
        ? `executed (exit ${output.exit_code})`
        : output.content !== undefined
          ? `completed (${typeof output.content === "string" ? output.content.length : 0} chars)`
          : "completed";

    try {
      await fetch(`${DAEMON_URL}/v1/audit/enrich`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          session_id: hookSessionId,
          tool: toolName,
          observed_effect: observedEffect,
          timestamp: new Date().toISOString(),
        }),
      });
    } catch {
      // Best-effort — don't block on audit enrichment failures
    }
    process.exit(0);
  }

  // Pre-tool hook: evaluate policy before execution
  // (toolName and toolInput already set above from parsed input)

  // Check if enforcement is enabled — if not, allow everything
  try {
    const enfRes = await fetch(`${DAEMON_URL}/v1/enforcement`);
    if (enfRes.ok) {
      const enfState = await enfRes.json() as { enabled: boolean };
      if (!enfState.enabled) {
        // Enforcement disabled — allow all actions
        process.exit(0);
      }
    }
  } catch {
    // Daemon unreachable — fail-open
    process.exit(0);
  }

  // Build action request
  const request = buildActionRequest(toolName, toolInput as Record<string, unknown>);
  if (!request) {
    // Unknown tool — allow
    process.exit(0);
  }

  // Call daemon
  try {
    const response = await fetch(`${DAEMON_URL}/v1/evaluate`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(request),
    });

    if (!response.ok) {
      // Daemon error — fail-open for prototype (configurable in production)
      console.error(`[Enforcer] Daemon returned ${response.status}. Allowing action.`);
      process.exit(0);
    }

    const decision = await response.json() as {
      decision: string;
      reason_human: string;
      reason_code: string;
      policy_id: string;
    };

    if (decision.decision === "allow") {
      process.exit(0);
    }

    if (decision.decision === "deny") {
      // Exit 2 = block. Stderr is shown to the user as the reason.
      process.stderr.write(
        `\n[Enforcer] BLOCKED: ${decision.reason_human}\n` +
        `  Policy: ${decision.policy_id}\n` +
        `  Reason: ${decision.reason_code}\n`,
      );
      process.exit(2);
    }

    if (decision.decision === "require_approval") {
      // For prototype: block with message directing to approval console
      // Full approval integration (pause + wait) requires VS Code extension in Phase B7
      process.stderr.write(
        `\n[Enforcer] APPROVAL REQUIRED: ${decision.reason_human}\n` +
        `  Policy: ${decision.policy_id}\n` +
        `  Approve via console: http://localhost:6100/approvals\n`,
      );
      process.exit(2);
    }

    // Unknown decision — allow
    process.exit(0);
  } catch (err) {
    // Daemon unreachable — fail-open for prototype
    console.error(`[Enforcer] Daemon unreachable. Allowing action.`);
    process.exit(0);
  }
}

main();