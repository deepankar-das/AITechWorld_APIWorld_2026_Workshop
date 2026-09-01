/**
 * Author: Deepankar Das
 */

package enforcement

import (
	"fmt"
	"os"
)

// PostureResult holds the result of a container posture check.
type PostureResult struct {
	Valid      bool     `json:"valid"`
	Violations []string `json:"violations"`
	Warnings   []string `json:"warnings"`
}

// IsContainer checks if running inside a Docker container.
func IsContainer() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	// Check cgroup for container indicators
	data, err := os.ReadFile("/proc/1/cgroup")
	if err == nil {
		content := string(data)
		if len(content) > 0 && (contains(content, "docker") || contains(content, "kubepods")) {
			return true
		}
	}
	return false
}

// IsRoot checks if the process is running as root.
func IsRoot() bool {
	return os.Getuid() == 0
}

// HasDockerSocket checks if Docker socket is mounted.
func HasDockerSocket() bool {
	_, err := os.Stat("/var/run/docker.sock")
	return err == nil
}

// HasBroadMounts checks for dangerous host mount paths.
func HasBroadMounts() []string {
	var mounts []string
	dangerPaths := []string{"/host", "/host-root", "/mnt/host"}
	for _, p := range dangerPaths {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			mounts = append(mounts, p)
		}
	}
	return mounts
}

// ValidateContainerPosture checks the container security posture.
func ValidateContainerPosture() PostureResult {
	result := PostureResult{Valid: true}

	if !IsContainer() {
		return result // Not in a container, posture check not applicable
	}

	if HasDockerSocket() {
		result.Valid = false
		result.Violations = append(result.Violations, "Docker socket is mounted (/var/run/docker.sock) — container escape risk")
	}

	if mounts := HasBroadMounts(); len(mounts) > 0 {
		result.Valid = false
		for _, m := range mounts {
			result.Violations = append(result.Violations, fmt.Sprintf("Broad host mount detected: %s", m))
		}
	}

	if IsRoot() {
		result.Warnings = append(result.Warnings, "Running as root user — recommend non-root user for least privilege")
	}

	return result
}

// EnforcePosture validates posture and exits if critical violations found.
func EnforcePosture() {
	result := ValidateContainerPosture()
	if !result.Valid {
		for _, v := range result.Violations {
			fmt.Fprintf(os.Stderr, "[Enforcer] CRITICAL: %s\n", v)
		}
		fmt.Fprintln(os.Stderr, "[Enforcer] Container posture check FAILED — refusing to start")
		os.Exit(1)
	}
	for _, w := range result.Warnings {
		fmt.Fprintf(os.Stderr, "[Enforcer] WARNING: %s\n", w)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
