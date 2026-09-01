/**
 * Author: Deepankar Das
 */

package enforcement

import (
	"sync"

	"github.com/anthropics/enforcer/internal/types"
)

// McpGateway manages MCP server registration and tool governance.
type McpGateway struct {
	mu       sync.RWMutex
	registry map[string]types.McpServerEntry
}

// NewMcpGateway creates a gateway with default server registrations.
func NewMcpGateway() *McpGateway {
	gw := &McpGateway{
		registry: make(map[string]types.McpServerEntry),
	}
	// Register default servers
	gw.RegisterServer(types.McpServerEntry{
		ServerID:    "filesystem",
		DisplayName: "Filesystem MCP",
		Description: "Local filesystem operations",
		TrustLevel:  "trusted",
	})
	gw.RegisterServer(types.McpServerEntry{
		ServerID:         "database",
		DisplayName:      "Database MCP",
		Description:      "Database query operations",
		TrustLevel:       "warning",
		RequiresApproval: true,
	})
	gw.RegisterServer(types.McpServerEntry{
		ServerID:    "web-search",
		DisplayName: "Web Search MCP",
		Description: "Web search and browsing",
		TrustLevel:  "untrusted",
		BlockedTools: []string{"execute_script"},
	})
	return gw
}

// RegisterServer adds or updates a server entry.
func (gw *McpGateway) RegisterServer(entry types.McpServerEntry) {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	gw.registry[entry.ServerID] = entry
}

// GetServer returns a server entry by ID.
func (gw *McpGateway) GetServer(serverID string) (types.McpServerEntry, bool) {
	gw.mu.RLock()
	defer gw.mu.RUnlock()
	entry, ok := gw.registry[serverID]
	return entry, ok
}

// GetServers returns all registered servers.
func (gw *McpGateway) GetServers() []types.McpServerEntry {
	gw.mu.RLock()
	defer gw.mu.RUnlock()
	result := make([]types.McpServerEntry, 0, len(gw.registry))
	for _, entry := range gw.registry {
		result = append(result, entry)
	}
	return result
}

// EvaluateMcpCall checks an MCP tool call against the server registry.
func (gw *McpGateway) EvaluateMcpCall(call types.McpToolCall) types.McpGatewayDecision {
	gw.mu.RLock()
	defer gw.mu.RUnlock()

	entry, ok := gw.registry[call.ServerID]
	if !ok {
		return types.McpGatewayDecision{
			Allowed:     false,
			Decision:    "deny",
			ReasonCode:  "MCP_SERVER_UNKNOWN",
			ReasonHuman: "MCP server '" + call.ServerID + "' is not registered",
			PolicyID:    "system.mcp_unknown_server",
			PolicyVersion: "v1.0.0",
		}
	}

	// Check blocked tools
	for _, blocked := range entry.BlockedTools {
		if blocked == call.Tool {
			return types.McpGatewayDecision{
				Allowed:     false,
				Decision:    "deny",
				ReasonCode:  "MCP_TOOL_BLOCKED",
				ReasonHuman: "Tool '" + call.Tool + "' is blocked on server '" + call.ServerID + "'",
				PolicyID:    "system.mcp_tool_blocked",
				PolicyVersion: "v1.0.0",
			}
		}
	}

	// Check allowlist for untrusted servers
	if entry.TrustLevel == "untrusted" && len(entry.AllowedTools) > 0 {
		allowed := false
		for _, tool := range entry.AllowedTools {
			if tool == call.Tool {
				allowed = true
				break
			}
		}
		if !allowed {
			return types.McpGatewayDecision{
				Allowed:     false,
				Decision:    "deny",
				ReasonCode:  "MCP_TOOL_NOT_IN_ALLOWLIST",
				ReasonHuman: "Tool '" + call.Tool + "' is not in allowlist for untrusted server '" + call.ServerID + "'",
				PolicyID:    "system.mcp_tool_not_allowed",
				PolicyVersion: "v1.0.0",
			}
		}
	}

	// Check if approval is required
	if entry.RequiresApproval {
		return types.McpGatewayDecision{
			Allowed:     false,
			Decision:    "require_approval",
			ReasonCode:  "MCP_REQUIRES_APPROVAL",
			ReasonHuman: "MCP server '" + call.ServerID + "' requires approval for tool invocations",
			PolicyID:    "system.mcp_requires_approval",
			PolicyVersion: "v1.0.0",
		}
	}

	// Trusted server, allowed tool
	return types.McpGatewayDecision{
		Allowed:     true,
		Decision:    "allow",
		ReasonCode:  "MCP_ALLOWED",
		ReasonHuman: "MCP call allowed for trusted server '" + call.ServerID + "'",
		PolicyID:    "system.mcp_allowed",
		PolicyVersion: "v1.0.0",
	}
}
