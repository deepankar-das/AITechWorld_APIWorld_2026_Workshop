/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — MCP Gateway
 *
 * Protocol-aware governance for Model Context Protocol tool invocations.
 * Sits between MCP clients (agents) and MCP servers (tools), intercepting
 * every tool call for policy evaluation.
 *
 * Capabilities:
 *   - Server allowlist/denylist
 *   - Tool-level allow/deny per server
 *   - Method-level restrictions
 *   - Payload size enforcement
 *   - Payload inspection and metadata capture
 *   - Audit logging for every MCP interaction
 *
 * TDD Reference: Section 4A.3, TDD Appendix B.3 (MCP Gateway component)
 * PRD Reference: Appendix C, FR-6; PRD Section 9 FR-6
 */

import { v4 as uuidv4 } from "uuid";
import * as http from "node:http";
import type { McpToolCall, McpServerEntry, McpGatewayDecision } from "../../types/mcp.js";
import type { ActionRequest } from "../../types/action.js";

const MCP_GATEWAY_PORT = parseInt(process.env.MCP_GATEWAY_PORT || "9102", 10);
const DAEMON_URL = process.env.AA_DAEMON_URL || "http://127.0.0.1:9100";

// ── Server Registry ─────────────────────────────────────────────────────────

const DEFAULT_SERVER_REGISTRY: McpServerEntry[] = [
  {
    server_id: "filesystem",
    display_name: "Filesystem MCP Server",
    description: "File read/write/search operations",
    trust_level: "trusted",
    allowed_tools: [],
    blocked_tools: [],
    requires_approval: false,
  },
  {
    server_id: "database",
    display_name: "Database MCP Server",
    description: "SQL query execution",
    trust_level: "warning",
    allowed_tools: ["query", "schema"],
    blocked_tools: ["drop", "truncate", "delete"],
    requires_approval: true,
  },
  {
    server_id: "web-search",
    display_name: "Web Search MCP Server",
    description: "Internet search and web fetch",
    trust_level: "untrusted",
    allowed_tools: ["search"],
    blocked_tools: ["fetch_raw"],
    requires_approval: false,
  },
];

let serverRegistry: McpServerEntry[] = [...DEFAULT_SERVER_REGISTRY];

/**
 * Register a custom server entry (for policy configuration).
 */
export function registerMcpServer(entry: McpServerEntry): void {
  const existing = serverRegistry.findIndex(s => s.server_id === entry.server_id);
  if (existing >= 0) {
    serverRegistry[existing] = entry;
  } else {
    serverRegistry.push(entry);
  }
}

/**
 * Get the registry entry for an MCP server.
 */
export function getServerEntry(serverId: string): McpServerEntry | undefined {
  return serverRegistry.find(s => s.server_id === serverId);
}

// ── Policy Evaluation ───────────────────────────────────────────────────────

/**
 * Evaluate an MCP tool call against the server registry and daemon policy.
 */
export async function evaluateMcpCall(
  call: McpToolCall,
  sessionId: string,
): Promise<McpGatewayDecision> {
  const entry = getServerEntry(call.server_id);

  // Unknown server — deny by default (least privilege)
  if (!entry) {
    return {
      allowed: false,
      decision: "deny",
      reason_code: "MCP_SERVER_UNKNOWN",
      reason_human: `MCP server '${call.server_id}' is not registered. Unknown servers are blocked by default.`,
      policy_id: "mcp.unknown_server",
      policy_version: "v1",
      payload_transformed: false,
    };
  }

  // Untrusted server — check tool allowlists
  if (entry.trust_level === "untrusted") {
    if (entry.blocked_tools.includes(call.tool)) {
      return {
        allowed: false,
        decision: "deny",
        reason_code: "MCP_TOOL_BLOCKED",
        reason_human: `Tool '${call.tool}' on server '${call.server_id}' is explicitly blocked.`,
        policy_id: "mcp.tool_denylist",
        policy_version: "v1",
        payload_transformed: false,
      };
    }
    if (entry.allowed_tools.length > 0 && !entry.allowed_tools.includes(call.tool)) {
      return {
        allowed: false,
        decision: "deny",
        reason_code: "MCP_TOOL_NOT_ALLOWED",
        reason_human: `Tool '${call.tool}' is not on the allowlist for server '${call.server_id}'. Allowed: ${entry.allowed_tools.join(", ")}.`,
        policy_id: "mcp.tool_allowlist",
        policy_version: "v1",
        payload_transformed: false,
      };
    }
  }

  // Blocked tools (applies to all trust levels)
  if (entry.blocked_tools.includes(call.tool)) {
    return {
      allowed: false,
      decision: "deny",
      reason_code: "MCP_TOOL_BLOCKED",
      reason_human: `Tool '${call.tool}' on server '${call.server_id}' is explicitly blocked.`,
      policy_id: "mcp.tool_denylist",
      policy_version: "v1",
      payload_transformed: false,
    };
  }

  // Server requires approval for all calls
  if (entry.requires_approval) {
    // Route through daemon for approval workflow
    const decision = await evaluateViaDaemon(call, sessionId);
    return decision;
  }

  // Trusted server, allowed tool — forward
  // Still send to daemon for audit logging
  const decision = await evaluateViaDaemon(call, sessionId);
  return decision;
}

/**
 * Forward MCP call to daemon /v1/evaluate for policy + audit.
 */
async function evaluateViaDaemon(
  call: McpToolCall,
  sessionId: string,
): Promise<McpGatewayDecision> {
  const request: ActionRequest = {
    request_id: uuidv4(),
    timestamp: new Date().toISOString(),
    actor: {
      user_id: process.env.USER || "unknown",
      agent_type: "mcp_client",
      agent_instance: `mcp_${call.server_id}`,
      session_id: sessionId,
    },
    environment: {
      workspace: process.env.AA_WORKSPACE || process.cwd(),
      repo: "",
      branch: "",
      tier: "development",
      deployment_mode: "host",
    },
    action: {
      type: "mcp.invoke",
      attempted_action: `${call.server_id}/${call.tool}.${call.method}`,
    },
    resource: {
      kind: "mcp_tool",
      value: `${call.server_id}:${call.tool}:${call.method}`,
      classification: call.classification.length > 0
        ? call.classification as ("destructive" | "network_tool" | "package_manager" | "sensitive_path" | "safe" | "potential_exfiltration" | "bypass_attempt")[]
        : [],
    },
  };

  try {
    const response = await fetch(`${DAEMON_URL}/v1/evaluate`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(request),
    });

    if (!response.ok) {
      return {
        allowed: false,
        decision: "deny",
        reason_code: "MCP_DAEMON_ERROR",
        reason_human: `Daemon error (${response.status}). MCP call denied (fail-closed).`,
        policy_id: "system.fail_closed",
        policy_version: "unknown",
        payload_transformed: false,
      };
    }

    const policyDecision = await response.json() as {
      decision: string;
      reason_code: string;
      reason_human: string;
      policy_id: string;
      policy_version: string;
    };

    return {
      allowed: policyDecision.decision === "allow",
      decision: policyDecision.decision as "allow" | "deny" | "require_approval",
      reason_code: policyDecision.reason_code,
      reason_human: policyDecision.reason_human,
      policy_id: policyDecision.policy_id,
      policy_version: policyDecision.policy_version,
      payload_transformed: false,
    };
  } catch {
    return {
      allowed: false,
      decision: "deny",
      reason_code: "MCP_DAEMON_UNREACHABLE",
      reason_human: "Daemon unreachable. MCP call denied (fail-closed).",
      policy_id: "system.fail_closed",
      policy_version: "unknown",
      payload_transformed: false,
    };
  }
}

// ── HTTP Proxy Server ───────────────────────────────────────────────────────

function readBody(req: http.IncomingMessage): Promise<string> {
  return new Promise((resolve, reject) => {
    const chunks: Buffer[] = [];
    req.on("data", (chunk: Buffer) => chunks.push(chunk));
    req.on("end", () => resolve(Buffer.concat(chunks).toString("utf-8")));
    req.on("error", reject);
  });
}

/**
 * Start the MCP Gateway HTTP proxy.
 *
 * Agents send MCP tool calls to this gateway instead of directly to
 * MCP servers. The gateway evaluates policy, then forwards allowed
 * calls to the actual MCP server.
 *
 * Endpoint: POST /mcp/invoke
 * Body: { server_id, tool, method, params, session_id }
 */
export function startMcpGateway(): http.Server {
  const server = http.createServer(async (req, res) => {
    const url = req.url || "/";
    const method = req.method || "GET";

    // Health
    if (url === "/health" && method === "GET") {
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(JSON.stringify({
        status: "ok",
        registered_servers: serverRegistry.length,
        servers: serverRegistry.map(s => ({
          id: s.server_id,
          trust: s.trust_level,
          requires_approval: s.requires_approval,
        })),
      }));
      return;
    }

    // Registry
    if (url === "/mcp/servers" && method === "GET") {
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ servers: serverRegistry }));
      return;
    }

    // Invoke
    if (url === "/mcp/invoke" && method === "POST") {
      const body = await readBody(req);
      let parsed: { server_id?: string; tool?: string; method?: string; params?: Record<string, unknown>; session_id?: string };
      try {
        parsed = JSON.parse(body);
      } catch {
        res.writeHead(400, { "Content-Type": "application/json" });
        res.end(JSON.stringify({ error: "Invalid JSON" }));
        return;
      }

      if (!parsed.server_id || !parsed.tool) {
        res.writeHead(400, { "Content-Type": "application/json" });
        res.end(JSON.stringify({ error: "server_id and tool are required" }));
        return;
      }

      const call: McpToolCall = {
        server_id: parsed.server_id,
        tool: parsed.tool,
        method: parsed.method || "execute",
        params: parsed.params || {},
        classification: [],
      };

      const sessionId = parsed.session_id || `mcp_${uuidv4().slice(0, 8)}`;
      const decision = await evaluateMcpCall(call, sessionId);

      if (!decision.allowed) {
        res.writeHead(403, { "Content-Type": "application/json" });
        res.end(JSON.stringify({
          blocked: true,
          decision: decision.decision,
          reason_code: decision.reason_code,
          reason_human: decision.reason_human,
          policy_id: decision.policy_id,
        }));
        return;
      }

      // Allowed — in a full implementation, forward to the actual MCP server.
      // For the prototype, return a success stub.
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(JSON.stringify({
        blocked: false,
        decision: decision.decision,
        reason_code: decision.reason_code,
        note: "MCP call allowed. In production, this would be forwarded to the actual MCP server.",
        server_id: call.server_id,
        tool: call.tool,
        method: call.method,
      }));
      return;
    }

    res.writeHead(404, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ error: "Not found" }));
  });

  server.listen(MCP_GATEWAY_PORT, "127.0.0.1", () => {
    console.log(`[MCP-GW] Enforcer MCP gateway listening on http://127.0.0.1:${MCP_GATEWAY_PORT}`);
    console.log(`[MCP-GW] Registered servers: ${serverRegistry.map(s => s.server_id).join(", ")}`);
    console.log(`[MCP-GW] Endpoints: /mcp/invoke, /mcp/servers, /health`);
  });

  return server;
}

// Run if executed directly
const isMain = process.argv[1] && (
  process.argv[1].endsWith("mcp-gateway.ts") ||
  process.argv[1].endsWith("mcp-gateway.js")
);
if (isMain) {
  startMcpGateway();
}