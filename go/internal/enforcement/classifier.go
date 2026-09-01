/**
 * Author: Deepankar Das
 */

package enforcement

import (
	"strings"

	"github.com/anthropics/enforcer/internal/types"
)

// destructivePatterns are commands that modify or destroy system state irreversibly.
// Note: "rm " and "mv " (with trailing space) catch all invocations including
// plain "rm file.txt" and "mv file.txt /tmp/" — a security product should treat
// any file deletion or move as destructive.
var destructivePatterns = []string{
	"rm ",
	"mv ",
	"rename ",
	"git push --force",
	"git push -f",
	"git reset --hard",
	"chmod",
	"chown",
	"kill -9",
	"kill -KILL",
	"pkill",
	"killall",
	"mkfs",
	"dd if=",
	"shred",
}

// networkToolPatterns are commands that initiate or inspect network connections.
var networkToolPatterns = []string{
	"curl",
	"wget",
	"ssh ",
	"scp ",
	"rsync",
	"nmap",
	"nc ",
	"netcat",
	"telnet",
	"dig ",
	"nslookup",
	"traceroute",
	"ping ",
}

// packageManagerPatterns are commands that install external packages.
var packageManagerPatterns = []string{
	"npm install",
	"npm i ",
	"yarn add",
	"pip install",
	"pip3 install",
	"brew install",
	"cargo install",
	"gem install",
	"composer require",
	"go install",
	"go get",
}

// patternClassifications maps patterns to their resource classification.
var patternClassifications = []struct {
	patterns       []string
	classification types.ResourceClassification
}{
	{destructivePatterns, types.ClassDestructive},
	{networkToolPatterns, types.ClassNetworkTool},
	{packageManagerPatterns, types.ClassPackageManager},
}

// splitCompoundCommand splits a command string on shell compound operators
// (&&, ||, ;, |) and returns the individual subcommands trimmed of whitespace.
func splitCompoundCommand(command string) []string {
	// Replace compound operators with a common separator.
	// Order matters: replace && and || before & and | to avoid partial matches.
	normalized := command
	normalized = strings.ReplaceAll(normalized, "&&", "\x00")
	normalized = strings.ReplaceAll(normalized, "||", "\x00")
	normalized = strings.ReplaceAll(normalized, ";", "\x00")
	normalized = strings.ReplaceAll(normalized, "|", "\x00")

	parts := strings.Split(normalized, "\x00")
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// matchesPattern checks whether a subcommand matches a given pattern.
// The match is case-sensitive and checks whether the pattern appears as a
// prefix of the subcommand or anywhere within the subcommand string.
func matchesPattern(subcommand, pattern string) bool {
	return strings.Contains(subcommand, pattern)
}

// ClassifyCommand analyses a shell command string (which may be compound)
// and returns the set of resource classifications that apply. If no risky
// patterns are found, the result contains only types.ClassSafe.
func ClassifyCommand(command string) []types.ResourceClassification {
	subcommands := splitCompoundCommand(command)
	seen := make(map[types.ResourceClassification]bool)

	for _, sub := range subcommands {
		for _, pc := range patternClassifications {
			for _, pattern := range pc.patterns {
				if matchesPattern(sub, pattern) {
					seen[pc.classification] = true
				}
			}
		}
	}

	if len(seen) == 0 {
		return []types.ResourceClassification{types.ClassSafe}
	}

	result := make([]types.ResourceClassification, 0, len(seen))
	// Maintain deterministic order: destructive, network_tool, package_manager.
	order := []types.ResourceClassification{
		types.ClassDestructive,
		types.ClassNetworkTool,
		types.ClassPackageManager,
	}
	for _, c := range order {
		if seen[c] {
			result = append(result, c)
		}
	}
	return result
}

// CommandHasClassification returns true if the command's classifications
// include the specified tag.
func CommandHasClassification(command string, tag types.ResourceClassification) bool {
	classifications := ClassifyCommand(command)
	for _, c := range classifications {
		if c == tag {
			return true
		}
	}
	return false
}
