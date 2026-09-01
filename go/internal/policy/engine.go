/**
 * Author: Deepankar Das
 */

package policy

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/anthropics/enforcer/internal/types"
)

// ruleMatchesAction checks if a single policy rule applies to an action request.
func ruleMatchesAction(rule types.PolicyRule, request types.ActionRequest) bool {
	if !rule.IsEnabled() {
		return false
	}

	// Check action type match
	actionTypes := rule.Action.Types
	if !containsStr(actionTypes, "*") && !containsStr(actionTypes, string(request.Action.Type)) {
		return false
	}

	// Check subject match (agent type)
	agentTypes := rule.Subject.AgentTypes
	if !containsStr(agentTypes, "*") && !containsStr(agentTypes, request.Actor.AgentType) {
		return false
	}

	// Check subject match (user)
	users := rule.Subject.Users
	if !containsStr(users, "*") && !containsStr(users, request.Actor.UserID) {
		return false
	}

	// Check resource conditions
	rc := rule.Resource

	// Path-based rules: path is outside project root
	if getBool(rc, "path_outside_project") {
		filePath := request.Resource.Path
		if filePath == "" {
			return false
		}
		normalizedPath, _ := filepath.Abs(filePath)
		normalizedWorkspace, _ := filepath.Abs(request.Environment.Workspace)
		// Rule matches when path IS outside project root
		if strings.HasPrefix(normalizedPath, normalizedWorkspace+string(filepath.Separator)) || normalizedPath == normalizedWorkspace {
			return false // Path is inside project — this "outside" rule does not match
		}
		return true
	}

	// Sensitive path patterns
	if patterns, ok := getStringSlice(rc, "path_patterns"); ok {
		filePath := request.Resource.Path
		if filePath == "" {
			return false
		}
		normalizedPath, _ := filepath.Abs(filePath)
		homeDir, _ := os.UserHomeDir()
		for _, pattern := range patterns {
			expanded := strings.Replace(pattern, "~", homeDir, 1)
			if strings.HasSuffix(expanded, "/*") {
				dir := expanded[:len(expanded)-2]
				if strings.HasPrefix(normalizedPath, dir+string(filepath.Separator)) {
					return true
				}
			} else if normalizedPath == expanded {
				return true
			}
		}
		return false
	}

	// Host-based rules: check against loaded network allowlist
	if getBool(rc, "host_not_in_allowlist") {
		host := request.Resource.Host
		if host == "" {
			return false
		}
		al := GetGlobalAllowlist()
		if al != nil {
			if al.IsHostAllowlisted(host) {
				return false // Host is allowlisted — deny rule does not match
			}
			if al.IsHostInWarningList(host) {
				return false // Host is on warning list — handled by approval rule, not deny
			}
		}
		return true // Host is NOT allowlisted and NOT on warning list — deny
	}

	// Host in warning list: requires approval
	if getBool(rc, "host_in_warning_list") {
		host := request.Resource.Host
		if host == "" {
			return false
		}
		al := GetGlobalAllowlist()
		if al != nil && al.IsHostInWarningList(host) {
			return true // Host is on warning list — approval rule matches
		}
		return false
	}

	// Shell delete/move outside project rules
	if getBool(rc, "shell_delete_outside_project") {
		return shellDeleteOutsideProject(request.Resource.Value, request.Environment.Workspace)
	}
	if getBool(rc, "shell_move_outside_project") {
		return shellMoveOutsideProject(request.Resource.Value, request.Environment.Workspace)
	}

	// Command pattern rules
	if patterns, ok := getStringSlice(rc, "command_patterns"); ok {
		command := request.Resource.Value
		for _, pattern := range patterns {
			if strings.HasSuffix(pattern, "*") {
				if strings.HasPrefix(command, pattern[:len(pattern)-1]) {
					return true
				}
			} else if strings.Contains(command, pattern) {
				return true
			}
		}
		return false
	}

	// Path inside project (for allow rules)
	if getBool(rc, "path_inside_project") {
		filePath := request.Resource.Path
		if filePath == "" {
			return false
		}
		normalizedPath, _ := filepath.Abs(filePath)
		normalizedWorkspace, _ := filepath.Abs(request.Environment.Workspace)
		return strings.HasPrefix(normalizedPath, normalizedWorkspace+string(filepath.Separator)) || normalizedPath == normalizedWorkspace
	}

	// Catch-all: if no resource conditions, rule matches all resources
	if len(rc) == 0 {
		return true
	}

	return false
}

func shellDeleteOutsideProject(command, workspace string) bool {
	if strings.TrimSpace(command) == "" || strings.TrimSpace(workspace) == "" {
		return false
	}
	workspaceAbs, err := filepath.Abs(workspace)
	if err != nil {
		return false
	}

	homeDir, _ := os.UserHomeDir()
	for _, sub := range splitPolicyCommand(command) {
		tokens := strings.Fields(sub)
		if len(tokens) == 0 || tokens[0] != "rm" {
			continue
		}
		for _, token := range tokens[1:] {
			t := strings.Trim(token, "\"'")
			if t == "" || t == "--" || strings.HasPrefix(t, "-") {
				continue
			}
			target := resolveShellPathToken(t, workspaceAbs, homeDir)
			if target == "" {
				continue
			}
			if !pathIsInsideWorkspace(target, workspaceAbs) {
				return true
			}
		}
	}
	return false
}

// shellMoveOutsideProject returns true when the command contains a mv/rename
// whose source OR destination is outside the project workspace.  Moving a file
// out of the project is data exfiltration; moving something in from outside can
// inject untrusted content.
func shellMoveOutsideProject(command, workspace string) bool {
	if strings.TrimSpace(command) == "" || strings.TrimSpace(workspace) == "" {
		return false
	}
	workspaceAbs, err := filepath.Abs(workspace)
	if err != nil {
		return false
	}

	homeDir, _ := os.UserHomeDir()
	for _, sub := range splitPolicyCommand(command) {
		tokens := strings.Fields(sub)
		if len(tokens) == 0 {
			continue
		}
		cmd := tokens[0]
		if cmd != "mv" && cmd != "rename" && cmd != "cp" {
			continue
		}
		for _, token := range tokens[1:] {
			t := strings.Trim(token, "\"'")
			if t == "" || t == "--" || strings.HasPrefix(t, "-") {
				continue
			}
			target := resolveShellPathToken(t, workspaceAbs, homeDir)
			if target == "" {
				continue
			}
			if !pathIsInsideWorkspace(target, workspaceAbs) {
				return true
			}
		}
	}
	return false
}

func splitPolicyCommand(command string) []string {
	normalized := strings.NewReplacer("&&", ";", "||", ";", "|", ";").Replace(command)
	parts := strings.Split(normalized, ";")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func resolveShellPathToken(token, workspaceAbs, homeDir string) string {
	path := token
	if strings.HasPrefix(path, "~/") && homeDir != "" {
		path = filepath.Join(homeDir, strings.TrimPrefix(path, "~/"))
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(workspaceAbs, path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	return abs
}

func pathIsInsideWorkspace(targetAbs, workspaceAbs string) bool {
	cleanTarget := filepath.Clean(targetAbs)
	cleanWorkspace := filepath.Clean(workspaceAbs)
	return strings.HasPrefix(cleanTarget, cleanWorkspace+string(filepath.Separator)) || cleanTarget == cleanWorkspace
}

// EvaluatePolicy evaluates an ActionRequest against a PolicyBundle.
// Follows: deny → require_approval → allow → default deny (least privilege).
func EvaluatePolicy(request types.ActionRequest, bundle types.PolicyBundle) types.PolicyDecision {
	var denyRules, approvalRules, allowRules []types.PolicyRule

	for _, rule := range bundle.Rules {
		switch types.PolicyDecisionType(rule.Effect.Decision) {
		case types.DecisionDeny:
			denyRules = append(denyRules, rule)
		case types.DecisionRequireApproval:
			approvalRules = append(approvalRules, rule)
		case types.DecisionAllow:
			allowRules = append(allowRules, rule)
		}
	}

	// 1. Check deny rules first
	for _, rule := range denyRules {
		if ruleMatchesAction(rule, request) {
			return types.PolicyDecision{
				RequestID:        request.RequestID,
				Decision:         types.DecisionDeny,
				ReasonCode:       rule.Effect.ReasonCode,
				ReasonHuman:      rule.Effect.ReasonHuman,
				PolicyID:         rule.PolicyID,
				PolicyVersion:    rule.Version,
				ApprovalRequired: false,
			}
		}
	}

	// 2. Check require_approval rules
	for _, rule := range approvalRules {
		if ruleMatchesAction(rule, request) {
			return types.PolicyDecision{
				RequestID:        request.RequestID,
				Decision:         types.DecisionRequireApproval,
				ReasonCode:       rule.Effect.ReasonCode,
				ReasonHuman:      rule.Effect.ReasonHuman,
				PolicyID:         rule.PolicyID,
				PolicyVersion:    rule.Version,
				ApprovalRequired: true,
			}
		}
	}

	// 3. Check allow rules
	for _, rule := range allowRules {
		if ruleMatchesAction(rule, request) {
			return types.PolicyDecision{
				RequestID:        request.RequestID,
				Decision:         types.DecisionAllow,
				ReasonCode:       rule.Effect.ReasonCode,
				ReasonHuman:      rule.Effect.ReasonHuman,
				PolicyID:         rule.PolicyID,
				PolicyVersion:    rule.Version,
				ApprovalRequired: false,
			}
		}
	}

	// 4. Default deny (least privilege)
	return types.PolicyDecision{
		RequestID:        request.RequestID,
		Decision:         types.DecisionDeny,
		ReasonCode:       "DEFAULT_DENY",
		ReasonHuman:      "No matching policy rule. Default: deny (least privilege).",
		PolicyID:         "system.default_deny",
		PolicyVersion:    bundle.BundleVersion,
		ApprovalRequired: false,
	}
}

// SimulatePolicy evaluates a policy without enforcing it.
func SimulatePolicy(request types.ActionRequest, bundle types.PolicyBundle) types.PolicyDecision {
	result := EvaluatePolicy(request, bundle)
	simulated := result
	simulated.Decision = types.DecisionSimulate
	simulated.ReasonHuman = "[SIMULATED] " + result.ReasonHuman + " (would be: " + string(result.Decision) + ")"
	return simulated
}

// Helper functions for map[string]interface{} resource conditions

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func getStringSlice(m map[string]interface{}, key string) ([]string, bool) {
	v, ok := m[key]
	if !ok {
		return nil, false
	}
	switch typed := v.(type) {
	case []string:
		return typed, true
	case []interface{}:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result, true
	}
	return nil, false
}
