/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — MCP Gateway Tests
 *
 * Tests MCP tool call governance: server registry, tool allowlists/denylists,
 * trust levels, and policy routing.
 *
 * PRD Reference: Appendix C, FR-6 (MCP Governance)
 * TDD Reference: Section 4A.3
 */

import { describe, it, expect } from "vitest";
import {
  evaluateMcpCall,
  registerMcpServer,
  getServerEntry,
} from "../src/enforcement/mcp-gateway.js";
import type { McpToolCall } from "../types/mcp.js";

// ── Helpers ─────────────────────────────────────────────────────────────────

function makeCall(overrides: Partial<McpToolCall> = {}): McpToolCall {
  return {
    server_id: "filesystem",
    tool: "read_file",
    method: "execute",
    params: { path: "/tmp/test.txt" },
    classification: [],
    ...overrides,
  };
}

// ── Server Registry ─────────────────────────────────────────────────────────

describe("MCP Server Registry", () => {
  it("has default servers registered", () => {
    const fs = getServerEntry("filesystem");
    expect(fs).toBeDefined();
    expect(fs!.trust_level).toBe("trusted");

    const db = getServerEntry("database");
    expect(db).toBeDefined();
    expect(db!.trust_level).toBe("warning");
    expect(db!.requires_approval).toBe(true);

    const web = getServerEntry("web-search");
    expect(web).toBeDefined();
    expect(web!.trust_level).toBe("untrusted");
  });

  it("registers custom server entries", () => {
    registerMcpServer({
      server_id: "custom-tool",
      display_name: "Custom Tool",
      description: "A custom MCP server",
      trust_level: "trusted",
      allowed_tools: ["action1"],
      blocked_tools: [],
      requires_approval: false,
    });

    const custom = getServerEntry("custom-tool");
    expect(custom).toBeDefined();
    expect(custom!.display_name).toBe("Custom Tool");
  });

  it("returns undefined for unknown servers", () => {
    expect(getServerEntry("nonexistent-server")).toBeUndefined();
  });
});

// ── Policy Evaluation ───────────────────────────────────────────────────────

describe("MCP Gateway Policy", () => {
  it("denies calls to unknown servers", async () => {
    const call = makeCall({ server_id: "evil-server" });
    const decision = await evaluateMcpCall(call, "sess_test");

    expect(decision.allowed).toBe(false);
    expect(decision.decision).toBe("deny");
    expect(decision.reason_code).toBe("MCP_SERVER_UNKNOWN");
  });

  it("denies blocked tools on any server", async () => {
    const call = makeCall({ server_id: "database", tool: "drop" });
    const decision = await evaluateMcpCall(call, "sess_test");

    expect(decision.allowed).toBe(false);
    expect(decision.decision).toBe("deny");
    expect(decision.reason_code).toBe("MCP_TOOL_BLOCKED");
  });

  it("denies tools not on allowlist for untrusted servers", async () => {
    const call = makeCall({ server_id: "web-search", tool: "fetch_raw" });
    const decision = await evaluateMcpCall(call, "sess_test");

    expect(decision.allowed).toBe(false);
    expect(decision.decision).toBe("deny");
    expect(decision.reason_code).toBe("MCP_TOOL_BLOCKED");
  });

  it("denies non-allowlisted tools on untrusted servers", async () => {
    const call = makeCall({ server_id: "web-search", tool: "unknown_tool" });
    const decision = await evaluateMcpCall(call, "sess_test");

    expect(decision.allowed).toBe(false);
    expect(decision.decision).toBe("deny");
    expect(decision.reason_code).toBe("MCP_TOOL_NOT_ALLOWED");
  });

  it("allows allowlisted tools on untrusted servers", async () => {
    // web-search allows "search" tool
    const call = makeCall({ server_id: "web-search", tool: "search" });
    // This will try to call daemon — will fail in test environment,
    // but the registry check passes so it gets to the daemon call
    const decision = await evaluateMcpCall(call, "sess_test");

    // Without a running daemon, this returns deny (daemon unreachable)
    // The important test is that it got PAST the registry check
    // (didn't get MCP_TOOL_BLOCKED or MCP_TOOL_NOT_ALLOWED)
    expect(decision.reason_code).not.toBe("MCP_TOOL_BLOCKED");
    expect(decision.reason_code).not.toBe("MCP_TOOL_NOT_ALLOWED");
    expect(decision.reason_code).not.toBe("MCP_SERVER_UNKNOWN");
  });

  it("routes approval-required servers through daemon", async () => {
    // database server requires_approval = true
    const call = makeCall({ server_id: "database", tool: "query" });
    const decision = await evaluateMcpCall(call, "sess_test");

    // Without daemon, gets DAEMON_UNREACHABLE — but the point is it
    // wasn't blocked by the registry (the tool "query" is allowed)
    expect(decision.reason_code).not.toBe("MCP_TOOL_BLOCKED");
    expect(decision.reason_code).not.toBe("MCP_SERVER_UNKNOWN");
  });
});

// ── Payload and Metadata ────────────────────────────────────────────────────

describe("MCP Call Metadata", () => {
  it("captures server, tool, and method in decision", async () => {
    const call = makeCall({
      server_id: "evil-server",
      tool: "steal_data",
      method: "execute",
    });
    const decision = await evaluateMcpCall(call, "sess_test");

    expect(decision.reason_human).toContain("evil-server");
  });

  it("includes tool name in denial message", async () => {
    const call = makeCall({ server_id: "database", tool: "drop" });
    const decision = await evaluateMcpCall(call, "sess_test");

    expect(decision.reason_human).toContain("drop");
    expect(decision.reason_human).toContain("database");
  });

  it("tracks payload_transformed flag", async () => {
    const call = makeCall({ server_id: "evil-server" });
    const decision = await evaluateMcpCall(call, "sess_test");

    expect(decision.payload_transformed).toBe(false);
  });
});