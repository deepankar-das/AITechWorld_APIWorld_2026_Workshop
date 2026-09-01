/**
 * Author: Deepankar Das
 */

package enforcement

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/anthropics/enforcer/internal/types"
)

// BuildEnforcementContext creates an Environment from env vars and git metadata.
func BuildEnforcementContext(workspaceOverride string) types.Environment {
	workspace := envOrDefault("AA_WORKSPACE", workspaceOverride)
	if workspace == "" {
		workspace, _ = os.Getwd()
	}
	workspace, _ = filepath.Abs(workspace)

	return types.Environment{
		Workspace:      workspace,
		Repo:           envOrDefault("AA_REPO", detectRepo(workspace)),
		Branch:         envOrDefault("AA_BRANCH", detectBranch(workspace)),
		Tier:           envOrDefault("AA_TIER", envOrDefault("NODE_ENV", "development")),
		DeploymentMode: envOrDefault("AA_DEPLOYMENT_MODE", detectDeploymentMode()),
	}
}

func detectRepo(workspace string) string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = workspace
	out, err := cmd.Output()
	if err != nil {
		return "unknown/unknown"
	}
	remote := strings.TrimSpace(string(out))
	// Extract org/repo from URL
	remote = strings.TrimSuffix(remote, ".git")
	if idx := strings.LastIndex(remote, ":"); idx > 0 {
		return remote[idx+1:]
	}
	parts := strings.Split(remote, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	return remote
}

func detectBranch(workspace string) string {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = workspace
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func detectDeploymentMode() string {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return "container"
	}
	if os.Getenv("REMOTE_WORKSPACE") != "" || os.Getenv("CODESPACE_NAME") != "" {
		return "remote"
	}
	return "host"
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
