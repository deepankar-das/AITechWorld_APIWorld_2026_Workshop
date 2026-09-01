/**
 * Author: Deepankar Das
 */

package types

// McpToolCall represents an MCP tool invocation to be governed.
type McpToolCall struct {
	ServerID       string                 `json:"server_id"`
	Tool           string                 `json:"tool"`
	Method         string                 `json:"method"`
	Params         map[string]interface{} `json:"params"`
	Classification []string               `json:"classification"`
}

// McpPolicyConditions define per-server MCP governance rules.
type McpPolicyConditions struct {
	ServerAllowlist []string `json:"server_allowlist,omitempty"`
	ServerDenylist  []string `json:"server_denylist,omitempty"`
	ToolAllowlist   []string `json:"tool_allowlist,omitempty"`
	ToolDenylist    []string `json:"tool_denylist,omitempty"`
	MethodAllowlist []string `json:"method_allowlist,omitempty"`
	MaxPayloadBytes int      `json:"max_payload_bytes,omitempty"`
}

// McpGatewayDecision is the gateway's response to an MCP call evaluation.
type McpGatewayDecision struct {
	Allowed            bool   `json:"allowed"`
	Decision           string `json:"decision"` // "allow", "deny", "require_approval"
	ReasonCode         string `json:"reason_code"`
	ReasonHuman        string `json:"reason_human"`
	PolicyID           string `json:"policy_id"`
	PolicyVersion      string `json:"policy_version"`
	PayloadTransformed bool   `json:"payload_transformed"`
	TransformationNote string `json:"transformation_note,omitempty"`
}

// McpServerEntry represents a registered MCP server in the gateway.
type McpServerEntry struct {
	ServerID        string   `json:"server_id"`
	DisplayName     string   `json:"display_name"`
	Description     string   `json:"description"`
	TrustLevel      string   `json:"trust_level"` // "trusted", "untrusted", "warning"
	AllowedTools    []string `json:"allowed_tools"`
	BlockedTools    []string `json:"blocked_tools"`
	RequiresApproval bool   `json:"requires_approval"`
}
