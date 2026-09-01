/**
 * Author: Deepankar Das
 */

package types

// ContextBundle provides context for an approval reviewer.
type ContextBundle struct {
	Actor          string `json:"actor"`
	Resource       string `json:"resource"`
	RiskRationale  string `json:"risk_rationale"`
	PolicyRule     string `json:"policy_rule"`
	AgentIdentity  string `json:"agent_identity"`
	SessionSummary string `json:"session_summary"`
}

// ApprovalScopeType defines how an approval scope is reused.
type ApprovalScopeType string

const (
	ScopeSingle      ApprovalScopeType = "single"
	ScopeSession     ApprovalScopeType = "session"
	ScopeTimeBounded ApprovalScopeType = "time_bounded"
)

// ApprovalScope defines the reuse boundaries of an approval.
type ApprovalScope struct {
	Type    ApprovalScopeType `json:"type"`
	Pattern string            `json:"pattern,omitempty"`
	Expiry  string            `json:"expiry,omitempty"`
}

// TimeoutBehavior defines what happens when an approval times out.
type TimeoutBehavior string

const (
	TimeoutDeny  TimeoutBehavior = "deny"
	TimeoutAllow TimeoutBehavior = "allow"
)

// ApprovalRequest is created when a policy decision is require_approval.
type ApprovalRequest struct {
	ApprovalID      string          `json:"approval_id"`
	RequestID       string          `json:"request_id"`
	ContextBundle   ContextBundle   `json:"context_bundle"`
	TimeoutSeconds  int             `json:"timeout_seconds"`
	TimeoutBehavior TimeoutBehavior `json:"timeout_behavior"`
	CreatedAt       string          `json:"created_at"`
}

// ApprovalDecision is the reviewer's response to an approval request.
type ApprovalDecision struct {
	ApprovalID   string          `json:"approval_id"`
	Decision     string          `json:"decision"` // "approve" or "deny"
	ApproverID   string          `json:"approver_id"`
	Rationale    string          `json:"rationale,omitempty"`
	Scope        *ApprovalScope  `json:"scope,omitempty"`
	IsBreakGlass bool            `json:"is_break_glass"`
}
