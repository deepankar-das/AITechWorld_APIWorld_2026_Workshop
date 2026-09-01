/**
 * Author: Deepankar Das
 */

package types

// ApprovalStatus tracks the lifecycle state of an approval.
type ApprovalStatus string

const (
	ApprovalNotRequired        ApprovalStatus = "not_required"
	ApprovalPending            ApprovalStatus = "pending"
	ApprovalApproved           ApprovalStatus = "approved"
	ApprovalDenied             ApprovalStatus = "denied"
	ApprovalExpired            ApprovalStatus = "expired"
	ApprovalDenyTimeout        ApprovalStatus = "deny_timeout"
	ApprovalAllowTimeoutConfig ApprovalStatus = "allow_timeout_configured"
)

// ApprovalRecord captures the approval chain for an audit event.
type ApprovalRecord struct {
	Status      ApprovalStatus `json:"status"`
	ApproverID  string         `json:"approver_id,omitempty"`
	Rationale   string         `json:"rationale,omitempty"`
	RequestedAt string         `json:"requested_at,omitempty"`
	ResolvedAt  string         `json:"resolved_at,omitempty"`
	Scope       *ApprovalScope `json:"scope,omitempty"`
	IsBreakGlass bool          `json:"is_break_glass"`
}

// PayloadSummary captures metadata about the action payload.
type PayloadSummary struct {
	Redacted    bool   `json:"redacted"`
	ContentHash string `json:"content_hash,omitempty"`
	Bytes       int    `json:"bytes,omitempty"`
}

// AuditEvent is the canonical audit record for every governed action.
type AuditEvent struct {
	EventID       string `json:"event_id"`
	Timestamp     string `json:"timestamp"`
	SessionID     string `json:"session_id"`
	CorrelationID string `json:"correlation_id"`

	// Minimum schema gate fields (all required for storage)
	Who      string `json:"who"`
	What     string `json:"what"`
	When     string `json:"when"`
	Policy   string `json:"policy"`
	Decision string `json:"decision"`
	Result   string `json:"result"`

	Actor struct {
		UserID        string `json:"user_id"`
		AgentType     string `json:"agent_type"`
		AgentInstance string `json:"agent_instance"`
	} `json:"actor"`

	Environment struct {
		Workspace      string `json:"workspace"`
		Repo           string `json:"repo"`
		Branch         string `json:"branch"`
		Tier           string `json:"tier"`
		DeploymentMode string `json:"deployment_mode"`
	} `json:"environment"`

	Action struct {
		Type            string `json:"type"`
		AttemptedAction string `json:"attempted_action"`
		ObservedEffect  string `json:"observed_effect"`
	} `json:"action"`

	Resource struct {
		Kind           string   `json:"kind"`
		Path           string   `json:"path,omitempty"`
		Host           string   `json:"host,omitempty"`
		Value          string   `json:"value,omitempty"`
		Classification []string `json:"classification"`
	} `json:"resource"`

	PolicyDetail struct {
		PolicyID      string `json:"policy_id"`
		PolicyVersion string `json:"policy_version"`
		Decision      string `json:"decision"`
		ReasonCode    string `json:"reason_code"`
		ReasonHuman   string `json:"reason_human"`
	} `json:"policy_detail"`

	Approval       *ApprovalRecord `json:"approval,omitempty"`
	PayloadSummary *PayloadSummary `json:"payload_summary,omitempty"`
}

// MinimumGateFields are the 6 fields required for audit event acceptance.
var MinimumGateFields = [6]string{"who", "what", "when", "policy", "decision", "result"}

// SessionSummary provides aggregate info for a session.
type SessionSummary struct {
	SessionID     string         `json:"session_id"`
	UserID        string         `json:"user_id,omitempty"`
	AgentType     string         `json:"agent_type,omitempty"`
	AgentInstance string         `json:"agent_instance,omitempty"`
	EventCount    int            `json:"event_count"`
	FirstEvent    string         `json:"first_event"`
	LastEvent     string         `json:"last_event"`
	Decisions     map[string]int `json:"decisions"`
}
