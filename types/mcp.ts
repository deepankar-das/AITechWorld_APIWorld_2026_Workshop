/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — MCP Gateway Types
 *
 * Types for Model Context Protocol governance. The MCP gateway intercepts
 * tool invocations between an MCP client (agent) and MCP servers (tools),
 * evaluating each call against policy before forwarding.
 *
 * TDD Reference: Section 4A.3 (Multi-Agent and Delegation Patterns)
 * PRD Reference: Appendix C, FR-6 (MCP Governance)
 */

import { z } from "zod";

// ── MCP Tool Call ───────────────────────────────────────────────────────────

export const McpToolCallSchema = z.object({
  /** MCP server identifier (e.g., "filesystem", "database", "search") */
  server_id: z.string().min(1),
  /** Tool name on the server (e.g., "read_file", "query", "search_web") */
  tool: z.string().min(1),
  /** Method being invoked (e.g., "execute", "read", "write") */
  method: z.string().default("execute"),
  /** Input parameters sent to the tool */
  params: z.record(z.unknown()).default({}),
  /** Classification tags for the tool call */
  classification: z.array(z.string()).default([]),
});
export type McpToolCall = z.infer<typeof McpToolCallSchema>;

// ── MCP Policy Rule Extensions ──────────────────────────────────────────────

export const McpPolicyConditions = z.object({
  /** Allowed MCP server IDs (empty = all blocked by default) */
  server_allowlist: z.array(z.string()).optional(),
  /** Blocked MCP server IDs */
  server_denylist: z.array(z.string()).optional(),
  /** Allowed tool names on the server */
  tool_allowlist: z.array(z.string()).optional(),
  /** Blocked tool names */
  tool_denylist: z.array(z.string()).optional(),
  /** Method-level restrictions */
  method_allowlist: z.array(z.string()).optional(),
  /** Payload size limit (bytes) — deny if exceeded */
  max_payload_bytes: z.number().optional(),
});
export type McpPolicyConditions = z.infer<typeof McpPolicyConditions>;

// ── MCP Gateway Decision ────────────────────────────────────────────────────

export interface McpGatewayDecision {
  allowed: boolean;
  decision: "allow" | "deny" | "require_approval";
  reason_code: string;
  reason_human: string;
  policy_id: string;
  policy_version: string;
  /** If payload was transformed (masked, truncated), note it here */
  payload_transformed: boolean;
  transformation_note?: string;
}

// ── MCP Server Registry ─────────────────────────────────────────────────────

export interface McpServerEntry {
  server_id: string;
  display_name: string;
  description: string;
  trust_level: "trusted" | "untrusted" | "warning";
  allowed_tools: string[]; // empty = all tools allowed
  blocked_tools: string[]; // takes precedence over allowed
  requires_approval: boolean; // if true, all calls to this server need approval
}