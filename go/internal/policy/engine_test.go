/**
 * Author: Deepankar Das
 */

package policy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anthropics/enforcer/internal/types"
)

func makeRequest(actionType types.ActionType, resourceKind types.ResourceKind, opts ...func(*types.ActionRequest)) types.ActionRequest {
	req := types.ActionRequest{
		RequestID: "test-req-1",
		Timestamp: "2026-04-27T00:00:00Z",
		Actor: types.Actor{
			UserID:    "dev_001",
			AgentType: "claude_code",
		},
		Environment: types.Environment{
			Workspace: "/Users/dev/project",
		},
		Action: types.ActionDetail{
			Type:            actionType,
			AttemptedAction: "test action",
		},
		Resource: types.Resource{
			Kind: resourceKind,
		},
	}
	for _, opt := range opts {
		opt(&req)
	}
	return req
}

func denyRule(id string, actionTypes []string, resource map[string]interface{}) types.PolicyRule {
	return types.PolicyRule{
		PolicyID: id,
		Version:  "v1.0.0",
		Scope: struct {
			Level types.PolicyScopeLevel `json:"level" yaml:"level"`
		}{Level: types.ScopeOrganization},
		Subject: struct {
			AgentTypes []string `json:"agent_types" yaml:"agent_types"`
			Users      []string `json:"users" yaml:"users"`
		}{AgentTypes: []string{"*"}, Users: []string{"*"}},
		Action: struct {
			Types []string `json:"types" yaml:"types"`
		}{Types: actionTypes},
		Resource: resource,
		Effect: types.PolicyEffect{
			Decision:    types.DecisionDeny,
			ReasonCode:  "TEST_DENY",
			ReasonHuman: "Denied by test rule",
		},
	}
}

func approvalRule(id string, actionTypes []string, resource map[string]interface{}) types.PolicyRule {
	r := denyRule(id, actionTypes, resource)
	r.Effect.Decision = types.DecisionRequireApproval
	r.Effect.ReasonCode = "TEST_APPROVAL"
	r.Effect.ReasonHuman = "Approval required by test rule"
	return r
}

func allowRule(id string, actionTypes []string, resource map[string]interface{}) types.PolicyRule {
	r := denyRule(id, actionTypes, resource)
	r.Effect.Decision = types.DecisionAllow
	r.Effect.ReasonCode = "TEST_ALLOW"
	r.Effect.ReasonHuman = "Allowed by test rule"
	return r
}

func TestEvaluatePolicy_DenyBeforeAllow(t *testing.T) {
	bundle := types.PolicyBundle{
		BundleVersion: "v1.0.0",
		Rules: []types.PolicyRule{
			allowRule("allow-all", []string{"*"}, map[string]interface{}{}),
			denyRule("deny-writes", []string{"file.write"}, map[string]interface{}{"path_outside_project": true}),
		},
	}
	req := makeRequest(types.ActionFileWrite, types.ResourceFile, func(r *types.ActionRequest) {
		r.Resource.Path = "/etc/passwd"
	})
	decision := EvaluatePolicy(req, bundle)
	if decision.Decision != types.DecisionDeny {
		t.Errorf("expected deny, got %s", decision.Decision)
	}
}

func TestEvaluatePolicy_RequireApprovalBeforeAllow(t *testing.T) {
	bundle := types.PolicyBundle{
		BundleVersion: "v1.0.0",
		Rules: []types.PolicyRule{
			allowRule("allow-all", []string{"*"}, map[string]interface{}{}),
			approvalRule("approve-cmds", []string{"shell.exec"}, map[string]interface{}{
				"command_patterns": []interface{}{"rm -rf"},
			}),
		},
	}
	req := makeRequest(types.ActionShellExec, types.ResourceCommand, func(r *types.ActionRequest) {
		r.Resource.Value = "rm -rf node_modules"
	})
	decision := EvaluatePolicy(req, bundle)
	if decision.Decision != types.DecisionRequireApproval {
		t.Errorf("expected require_approval, got %s", decision.Decision)
	}
	if !decision.ApprovalRequired {
		t.Error("expected approval_required=true")
	}
}

func TestEvaluatePolicy_DefaultDeny(t *testing.T) {
	bundle := types.PolicyBundle{
		BundleVersion: "v1.0.0",
		Rules:         []types.PolicyRule{},
	}
	req := makeRequest(types.ActionFileWrite, types.ResourceFile)
	decision := EvaluatePolicy(req, bundle)
	if decision.Decision != types.DecisionDeny {
		t.Errorf("expected deny (default), got %s", decision.Decision)
	}
	if decision.ReasonCode != "DEFAULT_DENY" {
		t.Errorf("expected DEFAULT_DENY, got %s", decision.ReasonCode)
	}
}

func TestEvaluatePolicy_AllowInsideProject(t *testing.T) {
	bundle := types.PolicyBundle{
		BundleVersion: "v1.0.0",
		Rules: []types.PolicyRule{
			allowRule("allow-project", []string{"file.write"}, map[string]interface{}{"path_inside_project": true}),
		},
	}
	absProject, _ := filepath.Abs("/Users/dev/project")
	req := makeRequest(types.ActionFileWrite, types.ResourceFile, func(r *types.ActionRequest) {
		r.Environment.Workspace = absProject
		r.Resource.Path = filepath.Join(absProject, "src", "main.ts")
	})
	decision := EvaluatePolicy(req, bundle)
	if decision.Decision != types.DecisionAllow {
		t.Errorf("expected allow, got %s", decision.Decision)
	}
}

func TestEvaluatePolicy_DenyOutsideProject(t *testing.T) {
	bundle := types.PolicyBundle{
		BundleVersion: "v1.0.0",
		Rules: []types.PolicyRule{
			denyRule("deny-outside", []string{"file.write"}, map[string]interface{}{"path_outside_project": true}),
		},
	}
	absProject, _ := filepath.Abs("/Users/dev/project")
	req := makeRequest(types.ActionFileWrite, types.ResourceFile, func(r *types.ActionRequest) {
		r.Environment.Workspace = absProject
		r.Resource.Path = "/etc/passwd"
	})
	decision := EvaluatePolicy(req, bundle)
	if decision.Decision != types.DecisionDeny {
		t.Errorf("expected deny, got %s", decision.Decision)
	}
}

func TestEvaluatePolicy_SensitivePathPatterns(t *testing.T) {
	homeDir, _ := os.UserHomeDir()
	bundle := types.PolicyBundle{
		BundleVersion: "v1.0.0",
		Rules: []types.PolicyRule{
			denyRule("deny-sensitive", []string{"file.read"}, map[string]interface{}{
				"path_patterns": []interface{}{"~/.ssh/*", "~/.aws/*"},
			}),
		},
	}
	req := makeRequest(types.ActionFileRead, types.ResourceFile, func(r *types.ActionRequest) {
		r.Resource.Path = filepath.Join(homeDir, ".ssh", "id_rsa")
	})
	decision := EvaluatePolicy(req, bundle)
	if decision.Decision != types.DecisionDeny {
		t.Errorf("expected deny for sensitive path, got %s", decision.Decision)
	}
}

func TestEvaluatePolicy_HostNotInAllowlist(t *testing.T) {
	// Set up allowlist
	SetGlobalAllowlist(&NetworkAllowlist{
		Allowlist:   []string{"api.openai.com", "github.com", "*.googleapis.com"},
		WarningList: []string{"pastebin.com"},
	})
	defer SetGlobalAllowlist(nil)

	bundle := types.PolicyBundle{
		BundleVersion: "v1.0.0",
		Rules: []types.PolicyRule{
			denyRule("deny-host", []string{"network.request"}, map[string]interface{}{"host_not_in_allowlist": true}),
			allowRule("allow-all", []string{"*"}, map[string]interface{}{}),
		},
	}

	tests := []struct {
		name string
		host string
		want types.PolicyDecisionType
	}{
		{"unknown host denied", "evil.com", types.DecisionDeny},
		{"allowlisted host allowed", "api.openai.com", types.DecisionAllow},
		{"allowlisted host github", "github.com", types.DecisionAllow},
		{"wildcard allowlist match", "storage.googleapis.com", types.DecisionAllow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := makeRequest(types.ActionNetworkRequest, types.ResourceHost, func(r *types.ActionRequest) {
				r.Resource.Host = tt.host
			})
			decision := EvaluatePolicy(req, bundle)
			if decision.Decision != tt.want {
				t.Errorf("host=%s: expected %s, got %s", tt.host, tt.want, decision.Decision)
			}
		})
	}
}

func TestEvaluatePolicy_HostInWarningList(t *testing.T) {
	SetGlobalAllowlist(&NetworkAllowlist{
		Allowlist:   []string{"github.com"},
		WarningList: []string{"pastebin.com", "gist.github.com"},
	})
	defer SetGlobalAllowlist(nil)

	bundle := types.PolicyBundle{
		BundleVersion: "v1.0.0",
		Rules: []types.PolicyRule{
			approvalRule("approve-warning", []string{"network.request"}, map[string]interface{}{"host_in_warning_list": true}),
			denyRule("deny-host", []string{"network.request"}, map[string]interface{}{"host_not_in_allowlist": true}),
		},
	}

	req := makeRequest(types.ActionNetworkRequest, types.ResourceHost, func(r *types.ActionRequest) {
		r.Resource.Host = "pastebin.com"
	})
	decision := EvaluatePolicy(req, bundle)
	if decision.Decision != types.DecisionRequireApproval {
		t.Errorf("expected require_approval for warning list host, got %s", decision.Decision)
	}
}

func TestEvaluatePolicy_CommandPatterns(t *testing.T) {
	bundle := types.PolicyBundle{
		BundleVersion: "v1.0.0",
		Rules: []types.PolicyRule{
			approvalRule("approve-destructive", []string{"shell.exec"}, map[string]interface{}{
				"command_patterns": []interface{}{"rm -rf", "git push --force", "git reset --hard"},
			}),
		},
	}
	tests := []struct {
		name    string
		command string
		want    types.PolicyDecisionType
	}{
		{"rm -rf matches", "rm -rf node_modules", types.DecisionRequireApproval},
		{"git push --force matches", "git push --force origin main", types.DecisionRequireApproval},
		{"safe command does not match", "npm test", types.DecisionDeny}, // default deny
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := makeRequest(types.ActionShellExec, types.ResourceCommand, func(r *types.ActionRequest) {
				r.Resource.Value = tt.command
			})
			decision := EvaluatePolicy(req, bundle)
			if decision.Decision != tt.want {
				t.Errorf("expected %s, got %s", tt.want, decision.Decision)
			}
		})
	}
}

func TestEvaluatePolicy_ShellDeleteOutsideProject(t *testing.T) {
	workspace := t.TempDir()
	outside := filepath.Join(os.TempDir(), "outside-delete-target.txt")
	inside := filepath.Join(workspace, "inside-delete-target.txt")

	bundle := types.PolicyBundle{
		BundleVersion: "v1.0.0",
		Rules: []types.PolicyRule{
			denyRule("deny-rm-outside", []string{"shell.exec"}, map[string]interface{}{
				"shell_delete_outside_project": true,
			}),
			allowRule("allow-shell", []string{"shell.exec"}, map[string]interface{}{}),
		},
	}

	tests := []struct {
		name     string
		command  string
		expected types.PolicyDecisionType
	}{
		{
			name:     "deny absolute outside delete",
			command:  "rm " + outside,
			expected: types.DecisionDeny,
		},
		{
			name:     "deny home relative outside delete",
			command:  "rm ~/Downloads/test.txt",
			expected: types.DecisionDeny,
		},
		{
			name:     "allow inside workspace delete",
			command:  "rm " + inside,
			expected: types.DecisionAllow,
		},
		{
			name:     "allow non-rm command",
			command:  "npm test",
			expected: types.DecisionAllow,
		},
		{
			name:     "deny in compound command",
			command:  "echo ok && rm " + outside,
			expected: types.DecisionDeny,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := makeRequest(types.ActionShellExec, types.ResourceCommand, func(r *types.ActionRequest) {
				r.Environment.Workspace = workspace
				r.Resource.Value = tc.command
			})
			decision := EvaluatePolicy(req, bundle)
			if decision.Decision != tc.expected {
				t.Fatalf("command=%q expected=%s got=%s", tc.command, tc.expected, decision.Decision)
			}
		})
	}
}

func TestEvaluatePolicy_ShellMoveOutsideProject(t *testing.T) {
	workspace := t.TempDir()
	outside := filepath.Join(os.TempDir(), "outside-move-target.txt")
	inside := filepath.Join(workspace, "inside-move-target.txt")

	bundle := types.PolicyBundle{
		BundleVersion: "v1.0.0",
		Rules: []types.PolicyRule{
			denyRule("deny-mv-outside", []string{"shell.exec"}, map[string]interface{}{
				"shell_move_outside_project": true,
			}),
			allowRule("allow-shell", []string{"shell.exec"}, map[string]interface{}{}),
		},
	}

	tests := []struct {
		name     string
		command  string
		expected types.PolicyDecisionType
	}{
		{
			name:     "deny mv to absolute outside path",
			command:  "mv " + inside + " " + outside,
			expected: types.DecisionDeny,
		},
		{
			name:     "deny mv to home relative path",
			command:  "mv file.txt ~/Desktop/",
			expected: types.DecisionDeny,
		},
		{
			name:     "deny cp to outside path",
			command:  "cp important.txt " + outside,
			expected: types.DecisionDeny,
		},
		{
			name:     "deny cp from outside path",
			command:  "cp " + outside + " .",
			expected: types.DecisionDeny,
		},
		{
			name:     "allow mv within workspace",
			command:  "mv " + inside + " " + filepath.Join(workspace, "renamed.txt"),
			expected: types.DecisionAllow,
		},
		{
			name:     "allow non-mv command",
			command:  "npm test",
			expected: types.DecisionAllow,
		},
		{
			name:     "deny mv in compound command",
			command:  "echo ok && mv file.txt " + outside,
			expected: types.DecisionDeny,
		},
		{
			name:     "deny rename outside project",
			command:  "rename " + outside + " newname.txt",
			expected: types.DecisionDeny,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := makeRequest(types.ActionShellExec, types.ResourceCommand, func(r *types.ActionRequest) {
				r.Environment.Workspace = workspace
				r.Resource.Value = tc.command
			})
			decision := EvaluatePolicy(req, bundle)
			if decision.Decision != tc.expected {
				t.Fatalf("command=%q expected=%s got=%s", tc.command, tc.expected, decision.Decision)
			}
		})
	}
}

func TestEvaluatePolicy_ReasonCodeAndVersion(t *testing.T) {
	bundle := types.PolicyBundle{
		BundleVersion: "v2026.04.27.1",
		Rules: []types.PolicyRule{
			denyRule("org.block_writes", []string{"file.write"}, map[string]interface{}{"path_outside_project": true}),
		},
	}
	req := makeRequest(types.ActionFileWrite, types.ResourceFile, func(r *types.ActionRequest) {
		r.Resource.Path = "/tmp/evil.sh"
	})
	decision := EvaluatePolicy(req, bundle)
	if decision.ReasonCode != "TEST_DENY" {
		t.Errorf("expected TEST_DENY reason code, got %s", decision.ReasonCode)
	}
	if decision.PolicyVersion != "v1.0.0" {
		t.Errorf("expected v1.0.0 policy version, got %s", decision.PolicyVersion)
	}
	if decision.PolicyID != "org.block_writes" {
		t.Errorf("expected org.block_writes policy_id, got %s", decision.PolicyID)
	}
}

func TestEvaluatePolicy_DisabledRuleSkipped(t *testing.T) {
	disabled := false
	bundle := types.PolicyBundle{
		BundleVersion: "v1.0.0",
		Rules: []types.PolicyRule{
			{
				PolicyID: "disabled-rule",
				Version:  "v1.0.0",
				Scope: struct {
					Level types.PolicyScopeLevel `json:"level" yaml:"level"`
				}{Level: types.ScopeOrganization},
				Subject: struct {
					AgentTypes []string `json:"agent_types" yaml:"agent_types"`
					Users      []string `json:"users" yaml:"users"`
				}{AgentTypes: []string{"*"}, Users: []string{"*"}},
				Action: struct {
					Types []string `json:"types" yaml:"types"`
				}{Types: []string{"*"}},
				Resource: map[string]interface{}{},
				Effect:   types.PolicyEffect{Decision: types.DecisionDeny, ReasonCode: "DISABLED"},
				Enabled:  &disabled,
			},
		},
	}
	req := makeRequest(types.ActionFileRead, types.ResourceFile)
	decision := EvaluatePolicy(req, bundle)
	if decision.ReasonCode != "DEFAULT_DENY" {
		t.Errorf("disabled rule should be skipped, got reason_code=%s", decision.ReasonCode)
	}
}

func TestSimulatePolicy(t *testing.T) {
	bundle := types.PolicyBundle{
		BundleVersion: "v1.0.0",
		Rules: []types.PolicyRule{
			denyRule("deny-all", []string{"*"}, map[string]interface{}{}),
		},
	}
	req := makeRequest(types.ActionFileWrite, types.ResourceFile)
	decision := SimulatePolicy(req, bundle)
	if decision.Decision != types.DecisionSimulate {
		t.Errorf("expected simulate decision, got %s", decision.Decision)
	}
}

func TestEvaluatePolicy_AgentTypeFilter(t *testing.T) {
	bundle := types.PolicyBundle{
		BundleVersion: "v1.0.0",
		Rules: []types.PolicyRule{
			{
				PolicyID: "cursor-only",
				Version:  "v1.0.0",
				Scope: struct {
					Level types.PolicyScopeLevel `json:"level" yaml:"level"`
				}{Level: types.ScopeOrganization},
				Subject: struct {
					AgentTypes []string `json:"agent_types" yaml:"agent_types"`
					Users      []string `json:"users" yaml:"users"`
				}{AgentTypes: []string{"cursor"}, Users: []string{"*"}},
				Action: struct {
					Types []string `json:"types" yaml:"types"`
				}{Types: []string{"*"}},
				Resource: map[string]interface{}{},
				Effect:   types.PolicyEffect{Decision: types.DecisionDeny, ReasonCode: "CURSOR_ONLY"},
			},
		},
	}
	// Claude Code should NOT match cursor-only rule
	req := makeRequest(types.ActionFileRead, types.ResourceFile)
	decision := EvaluatePolicy(req, bundle)
	if decision.ReasonCode == "CURSOR_ONLY" {
		t.Error("claude_code agent should not match cursor-only rule")
	}
}
