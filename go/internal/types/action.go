/**
 * Author: Deepankar Das
 */

package types

// ActionType enumerates the types of actions an agent can perform.
type ActionType string

const (
	ActionFileRead       ActionType = "file.read"
	ActionFileWrite      ActionType = "file.write"
	ActionFileDelete     ActionType = "file.delete"
	ActionFileMove       ActionType = "file.move"
	ActionShellExec      ActionType = "shell.exec"
	ActionNetworkRequest ActionType = "network.request"
	ActionPackageInstall ActionType = "package.install"
	ActionCredAccess        ActionType = "credential.access"
	ActionMcpInvoke         ActionType = "mcp.invoke"
	ActionInternalOrch      ActionType = "internal.orchestration"
)

// ResourceKind classifies the resource being accessed.
type ResourceKind string

const (
	ResourceFile       ResourceKind = "file"
	ResourceCommand    ResourceKind = "command"
	ResourceHost       ResourceKind = "host"
	ResourceMcpTool    ResourceKind = "mcp_tool"
	ResourceCredential ResourceKind = "credential"
	ResourceDatabase   ResourceKind = "database"
	ResourceInternal   ResourceKind = "internal"
)

// ResourceClassification tags risk attributes of a resource.
type ResourceClassification string

const (
	ClassDestructive    ResourceClassification = "destructive"
	ClassNetworkTool    ResourceClassification = "network_tool"
	ClassPackageManager ResourceClassification = "package_manager"
	ClassSensitivePath  ResourceClassification = "sensitive_path"
	ClassSafe           ResourceClassification = "safe"
	ClassPotentialExfil ResourceClassification = "potential_exfiltration"
	ClassBypassAttempt  ResourceClassification = "bypass_attempt"
)

// Actor identifies who is performing the action.
type Actor struct {
	UserID        string `json:"user_id"`
	AgentType     string `json:"agent_type"`
	AgentInstance string `json:"agent_instance"`
	SessionID     string `json:"session_id"`
}

// Environment describes the workspace context.
type Environment struct {
	Workspace      string `json:"workspace"`
	Repo           string `json:"repo"`
	Branch         string `json:"branch"`
	Tier           string `json:"tier"`
	DeploymentMode string `json:"deployment_mode"`
}

// Resource describes the target of the action.
type Resource struct {
	Kind           ResourceKind             `json:"kind"`
	Path           string                   `json:"path,omitempty"`
	Host           string                   `json:"host,omitempty"`
	Value          string                   `json:"value,omitempty"`
	Classification []ResourceClassification `json:"classification"`
}

// ActionDetail describes what the agent is trying to do.
type ActionDetail struct {
	Type            ActionType `json:"type"`
	AttemptedAction string     `json:"attempted_action"`
}

// ActionRequest is the canonical request sent from enforcement points to the daemon.
type ActionRequest struct {
	RequestID   string       `json:"request_id"`
	Timestamp   string       `json:"timestamp"`
	Actor       Actor        `json:"actor"`
	Environment Environment  `json:"environment"`
	Action      ActionDetail `json:"action"`
	Resource    Resource     `json:"resource"`
}

// ValidateActionRequest returns a list of validation errors, or nil if valid.
func ValidateActionRequest(req *ActionRequest) []string {
	var errs []string
	if req.RequestID == "" {
		errs = append(errs, "request_id is required")
	}
	if req.Timestamp == "" {
		errs = append(errs, "timestamp is required")
	}
	if req.Actor.UserID == "" {
		errs = append(errs, "actor.user_id is required")
	}
	if req.Actor.AgentType == "" {
		errs = append(errs, "actor.agent_type is required")
	}
	if req.Action.Type == "" {
		errs = append(errs, "action.type is required")
	}
	if req.Action.AttemptedAction == "" {
		errs = append(errs, "action.attempted_action is required")
	}
	if req.Resource.Kind == "" {
		errs = append(errs, "resource.kind is required")
	}
	return errs
}
