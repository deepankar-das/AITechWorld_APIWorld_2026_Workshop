/**
 * Author: Deepankar Das
 */

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/enforcer/internal/daemon"
	"github.com/anthropics/enforcer/internal/enforcement"
	"github.com/anthropics/enforcer/internal/types"
	"github.com/google/uuid"
)

var daemonURL = envOr("AA_DAEMON_URL", "http://127.0.0.1:9100")

// hookLog writes to both stderr (for Claude Code) and a log file the developer
// can read.  The log file is in the user's home directory so it doesn't require
// root access.
var hookLogger *log.Logger

func init() {
	hookLogger = log.New(os.Stderr, "", 0)

	logDir := filepath.Join(envOr("HOME", "/tmp"), ".enforcer")
	_ = os.MkdirAll(logDir, 0755)
	logPath := filepath.Join(logDir, "hook.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		hookLogger = log.New(io.MultiWriter(os.Stderr, f), "", 0)
	}
}

func hookLog(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	hookLogger.Printf("%s %s", time.Now().UTC().Format("2006-01-02T15:04:05Z"), msg)
}

func daemonAuthToken() string {
	if token := strings.TrimSpace(os.Getenv("AA_OPERATOR_TOKEN")); token != "" {
		return token
	}
	if token := strings.TrimSpace(os.Getenv("AA_ADMIN_TOKEN")); token != "" {
		return token
	}
	if data, err := os.ReadFile("/etc/enforcer/.operator_token"); err == nil {
		if token := strings.TrimSpace(string(data)); token != "" {
			return token
		}
	}
	if data, err := os.ReadFile("/etc/enforcer/.admin_token"); err == nil {
		if token := strings.TrimSpace(string(data)); token != "" {
			return token
		}
	}
	return ""
}

func postToDaemon(path string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, daemonURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token := daemonAuthToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-AA-Token", token)
	}
	return http.DefaultClient.Do(req)
}

func main() {
	if len(os.Args) < 2 {
		hookLog("[Enforcer] Usage: enforcer-hook <pre_tool_call|post_tool_call>")
		os.Exit(0)
	}

	hookType := os.Args[1]
	hookLog("[Enforcer] Hook invoked: %s", hookType)

	strictMode := daemon.IsStrictMode()

	// Read tool input from stdin
	stdinData, err := readStdin()
	if err != nil || len(stdinData) == 0 {
		if strictMode {
			hookLog("[Enforcer] STRICT MODE: No tool input received — action blocked")
			os.Exit(2)
		}
		os.Exit(0) // Allow on stdin error
	}

	var toolInput map[string]interface{}
	if err := json.Unmarshal(stdinData, &toolInput); err != nil {
		if strictMode {
			hookLog("[Enforcer] STRICT MODE: Invalid tool input — action blocked")
			os.Exit(2)
		}
		os.Exit(0) // Allow on parse error
	}

	toolName, _ := toolInput["tool_name"].(string)
	input, _ := toolInput["tool_input"].(map[string]interface{})
	hookLog("[Enforcer] Tool: %s, AuthToken: %v", toolName, daemonAuthToken() != "")

	if hookType == "post_tool_call" {
		handlePostToolCall(toolName, input, toolInput)
		os.Exit(0)
	}

	// Pre-tool-call: check enforcement state
	enabled, reachable := checkEnforcement()
	if !reachable && strictMode {
		hookLog("[Enforcer] STRICT MODE: Daemon unreachable — action blocked")
		os.Exit(2)
	}
	if !enabled {
		// Enforcement disabled is an explicit admin decision; allow even in strict mode.
		os.Exit(0)
	}

	// Build action request.  Every tool must be in the ToolMappings so the
	// policy engine can evaluate it.  Unknown tools are blocked in strict mode,
	// allowed otherwise.
	actionReq := buildActionRequest(toolName, input)
	if actionReq == nil {
		if strictMode {
			hookLog("[Enforcer] STRICT MODE: Unknown tool %q — action blocked", toolName)
			os.Exit(2)
		}
		hookLog("[Enforcer] Unknown tool %q — no policy mapping, allowing", toolName)
		os.Exit(0)
	}

	// Send to daemon for evaluation
	decision, err := evaluateAction(actionReq)
	if err != nil {
		hookLog("[Enforcer] Daemon error: %v — fail-closed\n", err)
		os.Exit(2) // Fail-closed on daemon error
	}

	switch decision.Decision {
	case types.DecisionAllow, types.DecisionAllowDegraded, types.DecisionSimulate:
		os.Exit(0)
	case types.DecisionDeny:
		hookLog("[Enforcer] BLOCKED: %s\nPolicy: %s\nReason: %s\n",
			decision.ReasonHuman, decision.PolicyID, decision.ReasonCode)
		os.Exit(2)
	case types.DecisionRequireApproval:
		approvalID := decision.ApprovalID // from evaluateResponse
		if approvalID == "" {
			hookLog("[Enforcer] BLOCKED: %s (no approval ID returned)\nPolicy: %s\n",
				decision.ReasonHuman, decision.PolicyID)
			os.Exit(2)
		}
		// Non-blocking: exit immediately so the developer can keep working.
		// The approval request is forwarded to the Hub admin by the Sentinel
		// client agent.  When approved, the admin's decision is stored as a
		// reusable scope in the daemon's approval service.  The next time the
		// developer retries the same action, CheckScope() finds the pre-approval
		// and the daemon returns allow.
		hookLog("[Enforcer] APPROVAL REQUIRED: %s\n"+
			"Policy: %s\n"+
			"Request ID: %s\n"+
			"An approval request has been sent to your security admin.\n"+
			"Re-run this command after the admin approves it.\n",
			decision.ReasonHuman, decision.PolicyID, approvalID)
		os.Exit(2)
	default:
		os.Exit(0)
	}
}

func readStdin() ([]byte, error) {
	return io.ReadAll(os.Stdin)
}

// checkEnforcement queries the daemon for the current enforcement state.
// Returns (enabled, reachable). When the daemon is unreachable, enabled
// defaults to true (fail-closed) and reachable is false.
func checkEnforcement() (enabled bool, reachable bool) {
	resp, err := http.Get(daemonURL + "/v1/enforcement")
	if err != nil {
		return true, false // Assume enabled, but flag unreachable
	}
	defer resp.Body.Close()
	var state struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		return true, false
	}
	return state.Enabled, true
}

func buildActionRequest(toolName string, input map[string]interface{}) *types.ActionRequest {
	mapping := enforcement.GetEnforcementPoint(toolName)
	if mapping == nil {
		return nil
	}

	workspace := envOr("AA_WORKSPACE", "")
	if workspace == "" {
		workspace, _ = os.Getwd()
	}
	// Walk up to find the project root (contains .git/ or .claude/).
	// os.Getwd() may return a subdirectory (e.g. go/) if Claude Code
	// changed the shell cwd, causing files in sibling dirs to appear
	// outside the project and trigger default deny.
	workspace = findProjectRoot(workspace)

	req := &types.ActionRequest{
		RequestID: uuid.New().String(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Actor: types.Actor{
			UserID:    envOr("USER", "unknown"),
			AgentType: "claude_code",
			SessionID: envOr("AA_SESSION_ID", uuid.New().String()),
		},
		Environment: types.Environment{
			Workspace:      workspace,
			Tier:           "development",
			DeploymentMode: "host",
		},
		Action: types.ActionDetail{
			Type:            types.ActionType(mapping.ActionType),
			AttemptedAction: buildAttemptedAction(toolName, input),
		},
		Resource: buildResource(mapping, input, workspace),
	}

	// Enrich with command classification
	if mapping.ActionType == "shell.exec" {
		cmd, _ := input["command"].(string)
		classifications := enforcement.ClassifyCommand(cmd)
		for _, c := range classifications {
			req.Resource.Classification = append(req.Resource.Classification, c)
		}

		// Check for package installs
		result := enforcement.DetectPackageInstall(cmd)
		if result.IsPackageInstall {
			req.Action.Type = types.ActionPackageInstall
		}

		// Check for secret access
		detection := enforcement.DetectSecretCommandAccess(cmd)
		if detection.IsSecret {
			req.Action.Type = types.ActionCredAccess
		}
	}

	// Check for sensitive file paths
	if mapping.ActionType == "file.read" || mapping.ActionType == "file.write" {
		filePath, _ := input["file_path"].(string)
		if filePath == "" {
			filePath, _ = input["path"].(string)
		}
		detection := enforcement.DetectSensitiveFilePath(filePath)
		if detection.IsSecret {
			req.Resource.Classification = append(req.Resource.Classification, types.ClassSensitivePath)
		}
	}

	return req
}

func buildAttemptedAction(toolName string, input map[string]interface{}) string {
	switch toolName {
	case "Read":
		path, _ := input["file_path"].(string)
		return "Read " + path
	case "Edit", "Write":
		path, _ := input["file_path"].(string)
		return "Write " + path
	case "Bash":
		cmd, _ := input["command"].(string)
		if len(cmd) > 100 {
			cmd = cmd[:100] + "..."
		}
		return "Execute: " + cmd
	case "WebFetch":
		url, _ := input["url"].(string)
		return "Fetch " + url
	case "WebSearch":
		query, _ := input["query"].(string)
		return "Search: " + query
	case "Glob", "Grep":
		pattern, _ := input["pattern"].(string)
		return toolName + ": " + pattern
	default:
		return toolName
	}
}

func buildResource(mapping *enforcement.HookMapping, input map[string]interface{}, workspace string) types.Resource {
	switch mapping.EnforcementPoint {
	case "fs-guard":
		filePath, _ := input["file_path"].(string)
		if filePath == "" {
			filePath, _ = input["path"].(string)
		}
		if filePath == "" {
			filePath = workspace
		}
		absPath, _ := filepath.Abs(filePath)
		return types.Resource{
			Kind: types.ResourceFile,
			Path: absPath,
		}
	case "shell-proxy":
		cmd, _ := input["command"].(string)
		return types.Resource{
			Kind:  types.ResourceCommand,
			Value: cmd,
		}
	case "network-proxy":
		url, _ := input["url"].(string)
		host := extractHost(url)
		return types.Resource{
			Kind: types.ResourceHost,
			Host: host,
		}
	case "internal":
		return types.Resource{
			Kind:  types.ResourceInternal,
			Value: mapping.ToolName,
		}
	default:
		return types.Resource{Kind: types.ResourceFile}
	}
}

func extractHost(urlStr string) string {
	// Simple host extraction
	urlStr = strings.TrimPrefix(urlStr, "https://")
	urlStr = strings.TrimPrefix(urlStr, "http://")
	if idx := strings.Index(urlStr, "/"); idx > 0 {
		urlStr = urlStr[:idx]
	}
	if idx := strings.Index(urlStr, ":"); idx > 0 {
		urlStr = urlStr[:idx]
	}
	return urlStr
}

// evaluateResponse extends PolicyDecision with approval-specific fields
// returned by the daemon when the decision is require_approval.
type evaluateResponse struct {
	types.PolicyDecision
	ApprovalID     string `json:"approval_id,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

func evaluateAction(req *types.ActionRequest) (*evaluateResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	resp, err := postToDaemon("/v1/evaluate", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("daemon /v1/evaluate returned %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var decision evaluateResponse
	if err := json.NewDecoder(resp.Body).Decode(&decision); err != nil {
		return nil, err
	}
	return &decision, nil
}

func handlePostToolCall(toolName string, input map[string]interface{}, toolInput map[string]interface{}) {
	sessionID := envOr("AA_SESSION_ID", "")
	if sessionID == "" {
		return
	}

	effect := "executed"
	if exitCode, ok := toolInput["exit_code"].(float64); ok && exitCode != 0 {
		effect = fmt.Sprintf("error (exit %d)", int(exitCode))
	}

	enrichReq := map[string]string{
		"session_id":      sessionID,
		"tool":            toolName,
		"observed_effect": effect,
		"timestamp":       time.Now().UTC().Format(time.RFC3339),
	}

	body, _ := json.Marshal(enrichReq)
	_, _ = postToDaemon("/v1/audit/enrich", body)
}

// findProjectRoot walks up from the given directory to find the project root
// by looking for .git/ or .claude/ markers.  Returns the original path if no
// marker is found.
func findProjectRoot(start string) string {
	dir, _ := filepath.Abs(start)
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, ".claude")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return start // reached filesystem root, use original
		}
		dir = parent
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
