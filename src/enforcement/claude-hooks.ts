/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Claude Code Hooks Adapter
 *
 * Generates the hooks configuration for Claude Code's settings.json
 * and provides the hook handler scripts that enforcement points call.
 *
 * Claude Code hooks fire on pre_tool_call and post_tool_call events.
 * Enforcer registers hooks that route tool calls to the appropriate
 * enforcement point (filesystem guard, shell proxy, or network proxy).
 *
 * TDD Reference: Section 6.2 (Claude Code Integration)
 * PRD Reference: Appendix C, C.6 (Phase 1 P0: Claude Code)
 */

import * as path from "node:path";

// ── Tool-to-Enforcement Mapping ─────────────────────────────────────────────

export interface HookMapping {
  tool: string;
  enforcementPoint: "fs-guard" | "shell-proxy" | "network-proxy";
  actionType: string;
  hookType: "pre_tool_call" | "post_tool_call";
}

/**
 * Map Claude Code tools to Enforcer enforcement points.
 */
export const TOOL_MAPPINGS: HookMapping[] = [
  // Pre-execution hooks (blocking — can prevent execution)
  { tool: "Read",      enforcementPoint: "fs-guard",      actionType: "file.read",       hookType: "pre_tool_call" },
  { tool: "Edit",      enforcementPoint: "fs-guard",      actionType: "file.write",      hookType: "pre_tool_call" },
  { tool: "Write",     enforcementPoint: "fs-guard",      actionType: "file.write",      hookType: "pre_tool_call" },
  { tool: "Bash",      enforcementPoint: "shell-proxy",   actionType: "shell.exec",      hookType: "pre_tool_call" },
  { tool: "WebFetch",  enforcementPoint: "network-proxy",  actionType: "network.request", hookType: "pre_tool_call" },
  { tool: "WebSearch", enforcementPoint: "network-proxy",  actionType: "network.request", hookType: "pre_tool_call" },
  // Post-execution hooks (non-blocking — for audit enrichment)
  { tool: "Read",      enforcementPoint: "fs-guard",      actionType: "file.read",       hookType: "post_tool_call" },
  { tool: "Edit",      enforcementPoint: "fs-guard",      actionType: "file.write",      hookType: "post_tool_call" },
  { tool: "Write",     enforcementPoint: "fs-guard",      actionType: "file.write",      hookType: "post_tool_call" },
  { tool: "Bash",      enforcementPoint: "shell-proxy",   actionType: "shell.exec",      hookType: "post_tool_call" },
];

/**
 * Get the enforcement point for a Claude Code tool name.
 */
export function getEnforcementPoint(toolName: string): HookMapping | undefined {
  return TOOL_MAPPINGS.find(
    m => m.tool === toolName && m.hookType === "pre_tool_call",
  );
}

// ── Hook Configuration Generator ────────────────────────────────────────────

export interface ClaudeHooksConfig {
  hooks: {
    pre_tool_call: Array<{
      matcher: { tool_name: string };
      hooks: Array<{ type: "command"; command: string }>;
    }>;
    post_tool_call: Array<{
      matcher: { tool_name: string };
      hooks: Array<{ type: "command"; command: string }>;
    }>;
  };
}

/**
 * Generate the Claude Code hooks configuration for settings.json.
 *
 * This configuration tells Claude Code to run Enforcer's hook handler
 * before and after each tool call. The handler script communicates with
 * the daemon to evaluate policy and capture audit data.
 *
 * @param handlerScriptPath - Absolute path to the Enforcer hook handler script
 * @param daemonUrl - Daemon base URL (default: http://127.0.0.1:9100)
 */
export function generateHooksConfig(
  handlerScriptPath: string,
  daemonUrl = "http://127.0.0.1:9100",
): ClaudeHooksConfig {
  const absoluteHandler = path.resolve(handlerScriptPath);

  // Group pre_tool_call hooks by tool
  const preToolHooks = new Map<string, string>();
  const postToolHooks = new Map<string, string>();

  for (const mapping of TOOL_MAPPINGS) {
    const command = `${absoluteHandler} --daemon-url ${daemonUrl} --hook-type ${mapping.hookType} --tool ${mapping.tool} --enforcement ${mapping.enforcementPoint}`;

    if (mapping.hookType === "pre_tool_call") {
      preToolHooks.set(mapping.tool, command);
    } else {
      postToolHooks.set(mapping.tool, command);
    }
  }

  return {
    hooks: {
      pre_tool_call: Array.from(preToolHooks.entries()).map(([tool, command]) => ({
        matcher: { tool_name: tool },
        hooks: [{ type: "command" as const, command }],
      })),
      post_tool_call: Array.from(postToolHooks.entries()).map(([tool, command]) => ({
        matcher: { tool_name: tool },
        hooks: [{ type: "command" as const, command }],
      })),
    },
  };
}

/**
 * Get a human-readable summary of hook registrations.
 */
export function getHooksSummary(): string {
  const preTools = TOOL_MAPPINGS
    .filter(m => m.hookType === "pre_tool_call")
    .map(m => `${m.tool} → ${m.enforcementPoint}`);

  const postTools = TOOL_MAPPINGS
    .filter(m => m.hookType === "post_tool_call")
    .map(m => `${m.tool} → ${m.enforcementPoint}`);

  return [
    "Enforcer Claude Code Hooks:",
    "",
    "Pre-execution (blocking):",
    ...preTools.map(t => `  ${t}`),
    "",
    "Post-execution (audit enrichment):",
    ...postTools.map(t => `  ${t}`),
  ].join("\n");
}