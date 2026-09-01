/**
 * Author: Deepankar Das
 */

package policy

import (
	"sync"

	"github.com/anthropics/enforcer/internal/types"
)

// PolicyPack is a pre-built collection of related policy rules.
type PolicyPack struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Category    string             `json:"category"`
	Rules       []types.PolicyRule `json:"rules"`
	Tags        []string           `json:"tags"`
}

var (
	customPacks   []PolicyPack
	customPacksMu sync.RWMutex
)

func makeRule(id, version string, actionTypes []string, decision types.PolicyDecisionType, reasonCode, reasonHuman string, resource map[string]interface{}) types.PolicyRule {
	return types.PolicyRule{
		PolicyID: id,
		Version:  version,
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
		Effect:   types.PolicyEffect{Decision: decision, ReasonCode: reasonCode, ReasonHuman: reasonHuman},
		Logging: struct {
			Mode string `json:"mode" yaml:"mode"`
		}{Mode: "full"},
	}
}

// BuiltinPacks contains the 8 canned policy packs.
var BuiltinPacks = []PolicyPack{
	{
		ID: "source-code-protection", Name: "Source Code Protection",
		Description: "Prevents unauthorized access to and exfiltration of source code",
		Category:    "security", Tags: []string{"source", "exfiltration", "files"},
		Rules: []types.PolicyRule{
			makeRule("pack.scp.block_outside_writes", "v1.0.0", []string{"file.write", "file.delete", "file.move"}, types.DecisionDeny, "PATH_OUTSIDE_PROJECT_ROOT", "Write outside project directory blocked", map[string]interface{}{"path_outside_project": true}),
			makeRule("pack.scp.block_sensitive_reads", "v1.0.0", []string{"file.read"}, types.DecisionDeny, "SENSITIVE_PATH_READ_BLOCKED", "Read of sensitive path blocked", map[string]interface{}{"path_patterns": []interface{}{"~/.ssh/*", "~/.aws/*", "~/.config/gcloud/*"}}),
			makeRule("pack.scp.approve_config_writes", "v1.0.0", []string{"file.write"}, types.DecisionRequireApproval, "CONFIG_WRITE_REQUIRES_APPROVAL", "Config file write requires approval", map[string]interface{}{"command_patterns": []interface{}{".env", "config.json", "settings.json"}}),
		},
	},
	{
		ID: "supply-chain-security", Name: "Supply Chain Security",
		Description: "Gates package installations and dependency changes",
		Category:    "security", Tags: []string{"supply-chain", "packages", "dependencies"},
		Rules: []types.PolicyRule{
			makeRule("pack.scs.approve_package_installs", "v1.0.0", []string{"shell.exec"}, types.DecisionRequireApproval, "PACKAGE_INSTALL_REQUIRES_APPROVAL", "Package installation requires approval", map[string]interface{}{"command_patterns": []interface{}{"npm install", "yarn add", "pip install", "brew install", "cargo install"}}),
			makeRule("pack.scs.approve_lockfile_changes", "v1.0.0", []string{"file.write"}, types.DecisionRequireApproval, "LOCKFILE_CHANGE_REQUIRES_APPROVAL", "Lock file modification requires approval", map[string]interface{}{"command_patterns": []interface{}{"package-lock.json", "yarn.lock", "Pipfile.lock", "Cargo.lock"}}),
		},
	},
	{
		ID: "secrets-hardening", Name: "Secrets & Credentials Hardening",
		Description: "Blocks unauthorized access to credentials and secrets",
		Category:    "security", Tags: []string{"secrets", "credentials", "keys"},
		Rules: []types.PolicyRule{
			makeRule("pack.sec.deny_credential_access", "v1.0.0", []string{"credential.access"}, types.DecisionDeny, "CREDENTIAL_ACCESS_DENIED", "Credential access denied", map[string]interface{}{}),
			makeRule("pack.sec.block_secret_reads", "v1.0.0", []string{"file.read"}, types.DecisionDeny, "SECRET_FILE_READ_BLOCKED", "Secret file read blocked", map[string]interface{}{"path_patterns": []interface{}{"~/.ssh/*", "~/.aws/*", "~/.kube/*"}}),
		},
	},
	{
		ID: "infrastructure-safety", Name: "Infrastructure Safety",
		Description: "Prevents destructive operations on infrastructure",
		Category:    "safety", Tags: []string{"infrastructure", "destructive", "commands"},
		Rules: []types.PolicyRule{
			makeRule("pack.inf.approve_destructive", "v1.0.0", []string{"shell.exec"}, types.DecisionRequireApproval, "DESTRUCTIVE_COMMAND_REQUIRES_APPROVAL", "Destructive command requires approval", map[string]interface{}{"command_patterns": []interface{}{"rm -rf", "git push --force", "git reset --hard", "chmod", "chown"}}),
			makeRule("pack.inf.deny_privilege_escalation", "v1.0.0", []string{"shell.exec"}, types.DecisionDeny, "PRIVILEGE_ESCALATION_DENIED", "Privilege escalation denied", map[string]interface{}{"command_patterns": []interface{}{"sudo ", "su -", "doas "}}),
		},
	},
	{
		ID: "network-egress-control", Name: "Network Egress Control",
		Description: "Controls outbound network traffic",
		Category:    "network", Tags: []string{"network", "egress", "allowlist"},
		Rules: []types.PolicyRule{
			makeRule("pack.net.block_non_allowlisted", "v1.0.0", []string{"network.request"}, types.DecisionDeny, "HOST_NOT_ALLOWLISTED", "Network request to non-allowlisted host blocked", map[string]interface{}{"host_not_in_allowlist": true}),
		},
	},
	{
		ID: "compliance-audit", Name: "Compliance & Audit",
		Description: "Enforces logging and approval requirements for compliance",
		Category:    "compliance", Tags: []string{"compliance", "soc2", "hipaa", "audit"},
		Rules: []types.PolicyRule{
			makeRule("pack.cmp.approve_pii_access", "v1.0.0", []string{"file.read"}, types.DecisionRequireApproval, "PII_ACCESS_REQUIRES_APPROVAL", "Access to PII data requires approval", map[string]interface{}{"command_patterns": []interface{}{"users.csv", "customers.", "personal"}}),
		},
	},
	{
		ID: "dev-best-practices", Name: "Developer Best Practices",
		Description: "Enforces safe development workflow practices",
		Category:    "practices", Tags: []string{"git", "development", "workflow"},
		Rules: []types.PolicyRule{
			makeRule("pack.dev.approve_force_push", "v1.0.0", []string{"shell.exec"}, types.DecisionRequireApproval, "FORCE_PUSH_REQUIRES_APPROVAL", "Force push requires approval", map[string]interface{}{"command_patterns": []interface{}{"git push --force", "git push -f"}}),
			makeRule("pack.dev.deny_direct_prod_write", "v1.0.0", []string{"file.write"}, types.DecisionDeny, "DIRECT_PROD_WRITE_DENIED", "Direct production write denied", map[string]interface{}{"command_patterns": []interface{}{"/prod/", "/production/"}}),
		},
	},
	{
		ID: "mcp-governance", Name: "MCP Tool Governance",
		Description: "Controls agent use of MCP tools and servers",
		Category:    "agent", Tags: []string{"mcp", "tools", "agents"},
		Rules: []types.PolicyRule{
			makeRule("pack.mcp.approve_untrusted", "v1.0.0", []string{"mcp.invoke"}, types.DecisionRequireApproval, "UNTRUSTED_MCP_REQUIRES_APPROVAL", "Untrusted MCP tool invocation requires approval", map[string]interface{}{}),
		},
	},
}

// GetAvailablePacks returns all registered policy packs.
func GetAvailablePacks() []PolicyPack {
	customPacksMu.RLock()
	defer customPacksMu.RUnlock()
	all := make([]PolicyPack, 0, len(BuiltinPacks)+len(customPacks))
	all = append(all, BuiltinPacks...)
	all = append(all, customPacks...)
	return all
}

// GetPack returns a policy pack by ID.
func GetPack(packID string) *PolicyPack {
	for i := range BuiltinPacks {
		if BuiltinPacks[i].ID == packID {
			return &BuiltinPacks[i]
		}
	}
	customPacksMu.RLock()
	defer customPacksMu.RUnlock()
	for i := range customPacks {
		if customPacks[i].ID == packID {
			return &customPacks[i]
		}
	}
	return nil
}

// ApplyPack adds a pack's rules to the bundle. Skips rules with existing policy_ids.
func ApplyPack(bundle *types.PolicyBundle, packID string) (added, skipped []string) {
	pack := GetPack(packID)
	if pack == nil {
		return nil, nil
	}
	existing := make(map[string]bool)
	for _, r := range bundle.Rules {
		existing[r.PolicyID] = true
	}
	for _, rule := range pack.Rules {
		if existing[rule.PolicyID] {
			skipped = append(skipped, rule.PolicyID)
		} else {
			bundle.Rules = append(bundle.Rules, rule)
			added = append(added, rule.PolicyID)
		}
	}
	return added, skipped
}

// AddCustomPack registers a custom policy pack.
func AddCustomPack(pack PolicyPack) {
	customPacksMu.Lock()
	defer customPacksMu.Unlock()
	customPacks = append(customPacks, pack)
}

// GetCustomPacks returns a copy of all custom (non-builtin) packs.
func GetCustomPacks() []PolicyPack {
	customPacksMu.RLock()
	defer customPacksMu.RUnlock()
	out := make([]PolicyPack, len(customPacks))
	copy(out, customPacks)
	return out
}

// SetCustomPacks replaces the in-memory custom pack registry.
func SetCustomPacks(packs []PolicyPack) {
	customPacksMu.Lock()
	defer customPacksMu.Unlock()
	customPacks = make([]PolicyPack, len(packs))
	copy(customPacks, packs)
}
