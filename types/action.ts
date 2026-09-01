/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Canonical Action Types
 *
 * Defines the ActionRequest contract used by all enforcement points
 * to submit actions to the daemon for policy evaluation.
 *
 * TDD Reference: Appendix B.4 (ActionRequest)
 * PRD Reference: Appendix C, R-1
 */

import { z } from "zod";

// ── Action Types ────────────────────────────────────────────────────────────

export const ActionType = z.enum([
  "file.read",
  "file.write",
  "file.delete",
  "file.move",
  "shell.exec",
  "network.request",
  // Phase 2 surfaces:
  "package.install",
  "credential.access",
  "mcp.invoke",
]);
export type ActionType = z.infer<typeof ActionType>;

// ── Resource Kind ───────────────────────────────────────────────────────────

export const ResourceKind = z.enum([
  "file",
  "command",
  "host",
  "mcp_tool",
  "credential",
  "database",
]);
export type ResourceKind = z.infer<typeof ResourceKind>;

// ── Resource Classification Tags ────────────────────────────────────────────

export const ResourceClassification = z.enum([
  "destructive",
  "network_tool",
  "package_manager",
  "sensitive_path",
  "safe",
  "potential_exfiltration",
  "bypass_attempt",
]);
export type ResourceClassification = z.infer<typeof ResourceClassification>;

// ── Actor ───────────────────────────────────────────────────────────────────

export const ActorSchema = z.object({
  user_id: z.string().min(1),
  agent_type: z.string().min(1),       // "claude_code" | "cursor" | "codex"
  agent_instance: z.string().min(1),    // "vscode_ext_1" | "cli_session_2"
  session_id: z.string().min(1),
});
export type Actor = z.infer<typeof ActorSchema>;

// ── Environment ─────────────────────────────────────────────────────────────

export const EnvironmentSchema = z.object({
  workspace: z.string().min(1),         // Absolute path to project root
  repo: z.string().default(""),         // "org/repo-name"
  branch: z.string().default(""),       // "main", "feature/x"
  tier: z.string().default("development"), // "development" | "staging" | "production"
  deployment_mode: z.string().default("host"), // "host" | "container" | "remote"
});
export type Environment = z.infer<typeof EnvironmentSchema>;

// ── Resource ────────────────────────────────────────────────────────────────

export const ResourceSchema = z.object({
  kind: ResourceKind,
  path: z.string().optional(),           // File path (for file ops)
  host: z.string().optional(),           // Destination host (for network ops)
  value: z.string().optional(),          // Command string, URL, etc.
  classification: z.array(ResourceClassification).default([]),
});
export type Resource = z.infer<typeof ResourceSchema>;

// ── Action ──────────────────────────────────────────────────────────────────

export const ActionDetailSchema = z.object({
  type: ActionType,
  attempted_action: z.string().min(1),   // Human-readable: "Write 247 bytes to ~/.config/settings.json"
});
export type ActionDetail = z.infer<typeof ActionDetailSchema>;

// ── ActionRequest (full contract) ───────────────────────────────────────────

export const ActionRequestSchema = z.object({
  request_id: z.string().min(1),
  timestamp: z.string().datetime(),
  actor: ActorSchema,
  environment: EnvironmentSchema,
  action: ActionDetailSchema,
  resource: ResourceSchema,
});
export type ActionRequest = z.infer<typeof ActionRequestSchema>;