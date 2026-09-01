/**
 * Author: Deepankar Das
 */

package types

// PolicyDecisionType enumerates possible policy decisions.
type PolicyDecisionType string

const (
	DecisionAllow           PolicyDecisionType = "allow"
	DecisionDeny            PolicyDecisionType = "deny"
	DecisionRequireApproval PolicyDecisionType = "require_approval"
	DecisionAllowDegraded   PolicyDecisionType = "allow_degraded"
	DecisionRedact          PolicyDecisionType = "redact"
	DecisionQuarantine      PolicyDecisionType = "quarantine"
	DecisionSimulate        PolicyDecisionType = "simulate"
)

// PolicyScopeLevel defines the hierarchy level of a policy rule.
type PolicyScopeLevel string

const (
	ScopeOrganization PolicyScopeLevel = "organization"
	ScopeTeam         PolicyScopeLevel = "team"
	ScopeRepository   PolicyScopeLevel = "repository"
	ScopeLocal        PolicyScopeLevel = "local"
)

// PolicyEffect specifies the enforcement action for a matched rule.
type PolicyEffect struct {
	Decision    PolicyDecisionType `json:"decision" yaml:"decision"`
	ReasonCode  string             `json:"reason_code" yaml:"reason_code"`
	ReasonHuman string             `json:"reason_human" yaml:"reason_human"`
}

// PolicyRule defines a single governance rule.
type PolicyRule struct {
	PolicyID string `json:"policy_id" yaml:"policy_id"`
	Version  string `json:"version" yaml:"version"`
	Scope    struct {
		Level PolicyScopeLevel `json:"level" yaml:"level"`
	} `json:"scope" yaml:"scope"`
	Subject struct {
		AgentTypes []string `json:"agent_types" yaml:"agent_types"`
		Users      []string `json:"users" yaml:"users"`
	} `json:"subject" yaml:"subject"`
	Action struct {
		Types []string `json:"types" yaml:"types"`
	} `json:"action" yaml:"action"`
	Resource   map[string]interface{} `json:"resource" yaml:"resource"`
	Conditions map[string]interface{} `json:"conditions" yaml:"conditions"`
	Effect     PolicyEffect           `json:"effect" yaml:"effect"`
	Logging    struct {
		Mode string `json:"mode" yaml:"mode"`
	} `json:"logging" yaml:"logging"`
	Approval struct {
		Required bool `json:"required" yaml:"required"`
	} `json:"approval" yaml:"approval"`
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

// IsEnabled returns whether the rule is active (defaults to true if not set).
func (r *PolicyRule) IsEnabled() bool {
	if r.Enabled == nil {
		return true
	}
	return *r.Enabled
}

// PolicyBundle is a versioned collection of policy rules.
type PolicyBundle struct {
	BundleVersion string           `json:"bundle_version" yaml:"bundle_version"`
	ScopeLevel    PolicyScopeLevel `json:"scope_level" yaml:"scope_level"`
	Rules         []PolicyRule     `json:"rules" yaml:"rules"`
}

// PolicyDecision is the daemon's response to an enforcement point.
type PolicyDecision struct {
	RequestID        string             `json:"request_id"`
	Decision         PolicyDecisionType `json:"decision"`
	ReasonCode       string             `json:"reason_code"`
	ReasonHuman      string             `json:"reason_human"`
	PolicyID         string             `json:"policy_id"`
	PolicyVersion    string             `json:"policy_version"`
	ApprovalRequired bool               `json:"approval_required"`
}
