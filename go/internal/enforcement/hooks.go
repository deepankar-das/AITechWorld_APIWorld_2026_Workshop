/**
 * Author: Deepankar Das
 */

package enforcement

// HookMapping maps a Claude Code tool to an enforcement point.
type HookMapping struct {
	ToolName         string
	EnforcementPoint string // "fs-guard", "shell-proxy", "network-proxy"
	ActionType       string // "file.read", "file.write", "shell.exec", "network.request"
}

// ToolMappings defines how Claude Code tools map to enforcement points.
// Every tool — including internal orchestration tools — must be mapped so
// the policy engine can evaluate it.  The Hub admin controls what's allowed
// or denied through policy rules, not through hardcoded bypasses.
var ToolMappings = []HookMapping{
	// System-affecting tools (governed by specific policy rules)
	{ToolName: "Read", EnforcementPoint: "fs-guard", ActionType: "file.read"},
	{ToolName: "Edit", EnforcementPoint: "fs-guard", ActionType: "file.write"},
	{ToolName: "Write", EnforcementPoint: "fs-guard", ActionType: "file.write"},
	{ToolName: "Bash", EnforcementPoint: "shell-proxy", ActionType: "shell.exec"},
	{ToolName: "WebFetch", EnforcementPoint: "network-proxy", ActionType: "network.request"},
	{ToolName: "WebSearch", EnforcementPoint: "network-proxy", ActionType: "network.request"},
	{ToolName: "Glob", EnforcementPoint: "fs-guard", ActionType: "file.read"},
	{ToolName: "Grep", EnforcementPoint: "fs-guard", ActionType: "file.read"},

	// Internal orchestration tools (allowed by org.allow_internal_tools policy)
	{ToolName: "Agent", EnforcementPoint: "internal", ActionType: "internal.orchestration"},
	{ToolName: "TodoWrite", EnforcementPoint: "internal", ActionType: "internal.orchestration"},
	{ToolName: "Skill", EnforcementPoint: "internal", ActionType: "internal.orchestration"},
	{ToolName: "ToolSearch", EnforcementPoint: "internal", ActionType: "internal.orchestration"},
	{ToolName: "ScheduleWakeup", EnforcementPoint: "internal", ActionType: "internal.orchestration"},
	{ToolName: "NotebookEdit", EnforcementPoint: "internal", ActionType: "internal.orchestration"},
	{ToolName: "Monitor", EnforcementPoint: "internal", ActionType: "internal.orchestration"},
	{ToolName: "SendMessage", EnforcementPoint: "internal", ActionType: "internal.orchestration"},
	{ToolName: "ExitPlanMode", EnforcementPoint: "internal", ActionType: "internal.orchestration"},
	{ToolName: "EnterPlanMode", EnforcementPoint: "internal", ActionType: "internal.orchestration"},
	{ToolName: "TaskOutput", EnforcementPoint: "internal", ActionType: "internal.orchestration"},
	{ToolName: "TaskStop", EnforcementPoint: "internal", ActionType: "internal.orchestration"},
	{ToolName: "AskUserQuestion", EnforcementPoint: "internal", ActionType: "internal.orchestration"},
	{ToolName: "ExitWorktree", EnforcementPoint: "internal", ActionType: "internal.orchestration"},
	{ToolName: "EnterWorktree", EnforcementPoint: "internal", ActionType: "internal.orchestration"},
	{ToolName: "CronCreate", EnforcementPoint: "internal", ActionType: "internal.orchestration"},
	{ToolName: "CronDelete", EnforcementPoint: "internal", ActionType: "internal.orchestration"},
	{ToolName: "CronList", EnforcementPoint: "internal", ActionType: "internal.orchestration"},
	{ToolName: "RemoteTrigger", EnforcementPoint: "internal", ActionType: "internal.orchestration"},
}

// GetEnforcementPoint returns the mapping for a Claude Code tool name.
func GetEnforcementPoint(toolName string) *HookMapping {
	for i := range ToolMappings {
		if ToolMappings[i].ToolName == toolName {
			return &ToolMappings[i]
		}
	}
	return nil
}

// GenerateHooksConfig produces Claude Code settings.json hooks config.
func GenerateHooksConfig(handlerPath string) map[string]interface{} {
	preTools := []string{"Read", "Edit", "Write", "Bash", "WebFetch", "WebSearch", "Glob", "Grep"}
	postTools := []string{"Read", "Edit", "Write", "Bash"}

	hooks := make(map[string]interface{})
	for _, tool := range preTools {
		key := "PreToolUse"
		if _, ok := hooks[key]; !ok {
			hooks[key] = []interface{}{}
		}
		hooks[key] = append(hooks[key].([]interface{}), map[string]interface{}{
			"type":          "command",
			"command":       handlerPath + " pre_tool_call",
			"tool":          tool,
			"timeout":       300,
			"statusMessage": "Enforcer: Evaluating policy...",
		})
	}
	for _, tool := range postTools {
		key := "PostToolUse"
		if _, ok := hooks[key]; !ok {
			hooks[key] = []interface{}{}
		}
		hooks[key] = append(hooks[key].([]interface{}), map[string]interface{}{
			"type":    "command",
			"command": handlerPath + " post_tool_call",
			"tool":    tool,
		})
	}
	return hooks
}
