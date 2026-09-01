/**
 * Author: Deepankar Das
 */

package enforcement

import (
	"testing"

	"github.com/anthropics/enforcer/internal/types"
)

func TestMcpGateway_TrustedServer(t *testing.T) {
	gw := NewMcpGateway()
	decision := gw.EvaluateMcpCall(types.McpToolCall{
		ServerID: "filesystem",
		Tool:     "read_file",
		Method:   "execute",
	})
	if !decision.Allowed || decision.Decision != "allow" {
		t.Errorf("trusted server should be allowed, got %s", decision.Decision)
	}
}

func TestMcpGateway_UnknownServer(t *testing.T) {
	gw := NewMcpGateway()
	decision := gw.EvaluateMcpCall(types.McpToolCall{
		ServerID: "unknown-server",
		Tool:     "do_stuff",
	})
	if decision.Allowed {
		t.Error("unknown server should be denied")
	}
	if decision.ReasonCode != "MCP_SERVER_UNKNOWN" {
		t.Errorf("expected MCP_SERVER_UNKNOWN, got %s", decision.ReasonCode)
	}
}

func TestMcpGateway_BlockedTool(t *testing.T) {
	gw := NewMcpGateway()
	decision := gw.EvaluateMcpCall(types.McpToolCall{
		ServerID: "web-search",
		Tool:     "execute_script",
	})
	if decision.Allowed {
		t.Error("blocked tool should be denied")
	}
	if decision.ReasonCode != "MCP_TOOL_BLOCKED" {
		t.Errorf("expected MCP_TOOL_BLOCKED, got %s", decision.ReasonCode)
	}
}

func TestMcpGateway_RequiresApproval(t *testing.T) {
	gw := NewMcpGateway()
	decision := gw.EvaluateMcpCall(types.McpToolCall{
		ServerID: "database",
		Tool:     "query",
	})
	if decision.Allowed {
		t.Error("approval-required server should not be directly allowed")
	}
	if decision.Decision != "require_approval" {
		t.Errorf("expected require_approval, got %s", decision.Decision)
	}
}

func TestMcpGateway_CustomServerRegistration(t *testing.T) {
	gw := NewMcpGateway()
	gw.RegisterServer(types.McpServerEntry{
		ServerID:    "custom-api",
		DisplayName: "Custom API",
		TrustLevel:  "trusted",
	})
	decision := gw.EvaluateMcpCall(types.McpToolCall{
		ServerID: "custom-api",
		Tool:     "call",
	})
	if !decision.Allowed {
		t.Error("custom registered trusted server should be allowed")
	}
}

func TestMcpGateway_UntrustedAllowlist(t *testing.T) {
	gw := NewMcpGateway()
	gw.RegisterServer(types.McpServerEntry{
		ServerID:     "restricted",
		DisplayName:  "Restricted MCP",
		TrustLevel:   "untrusted",
		AllowedTools: []string{"safe_tool"},
	})
	// Allowed tool
	d1 := gw.EvaluateMcpCall(types.McpToolCall{ServerID: "restricted", Tool: "safe_tool"})
	if !d1.Allowed {
		t.Error("allowed tool on untrusted server should be permitted")
	}
	// Disallowed tool
	d2 := gw.EvaluateMcpCall(types.McpToolCall{ServerID: "restricted", Tool: "dangerous_tool"})
	if d2.Allowed {
		t.Error("non-allowlisted tool on untrusted server should be denied")
	}
	if d2.ReasonCode != "MCP_TOOL_NOT_IN_ALLOWLIST" {
		t.Errorf("expected MCP_TOOL_NOT_IN_ALLOWLIST, got %s", d2.ReasonCode)
	}
}

func TestMcpGateway_GetServers(t *testing.T) {
	gw := NewMcpGateway()
	servers := gw.GetServers()
	if len(servers) < 3 {
		t.Errorf("expected at least 3 default servers, got %d", len(servers))
	}
}
