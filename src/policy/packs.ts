/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Canned Policy Packs
 *
 * Pre-built policy bundles for common industry security use cases.
 * Each pack can be applied with one click from the console.
 *
 * Packs are additive — applying a pack adds its rules to the
 * existing bundle without removing existing rules.
 */

import type { PolicyRule, PolicyBundle } from "../../types/policy.js";

export interface PolicyPack {
  id: string;
  name: string;
  description: string;
  category: string;
  rules: PolicyRule[];
  tags: string[];
}

const PACK_VERSION = "v2026.04.27.1";

function makeRule(
  id: string,
  actionTypes: string[],
  decision: "allow" | "deny" | "require_approval",
  reasonCode: string,
  reasonHuman: string,
  resource: Record<string, unknown> = {},
): PolicyRule {
  return {
    policy_id: id,
    version: PACK_VERSION,
    scope: { level: "organization" },
    subject: { agent_types: ["*"], users: ["*"] },
    action: { types: actionTypes },
    resource,
    conditions: {},
    effect: { decision, reason_code: reasonCode, reason_human: reasonHuman },
    logging: { mode: "full" },
    approval: { required: decision === "require_approval" },
  };
}

// ── Packs ───────────────────────────────────────────────────────────────────

export const POLICY_PACKS: PolicyPack[] = [

  // ── Source Code Protection ────────────────────────────────────────────────
  {
    id: "source-code-protection",
    name: "Source Code Protection",
    description: "Prevent source code exfiltration via network, clipboard, or file copy outside project boundaries.",
    category: "Data Protection",
    tags: ["ip-protection", "exfiltration", "enterprise"],
    rules: [
      makeRule("pack.block_code_upload", ["network.request"], "deny", "CODE_UPLOAD_BLOCKED",
        "Uploading code to paste/file-sharing services is blocked.", { command_patterns: ["curl -X POST", "curl --data", "curl -d", "wget --post"] }),
      makeRule("pack.block_tar_exfil", ["shell.exec"], "deny", "ARCHIVE_EXFIL_BLOCKED",
        "Creating archives of project code for external transfer is blocked.", { command_patterns: ["tar czf", "tar -czf", "zip -r", "7z a"] }),
      makeRule("pack.approve_git_push", ["shell.exec"], "require_approval", "GIT_PUSH_REQUIRES_APPROVAL",
        "Git push to remote requires approval to prevent accidental code exposure.", { command_patterns: ["git push"] }),
    ],
  },

  // ── Supply Chain Security ─────────────────────────────────────────────────
  {
    id: "supply-chain-security",
    name: "Supply Chain Security",
    description: "Gate all dependency changes and package installations with approval. Prevent installation from untrusted registries.",
    category: "Supply Chain",
    tags: ["dependencies", "packages", "npm", "pip", "supply-chain"],
    rules: [
      makeRule("pack.approve_all_installs", ["shell.exec", "package.install"], "require_approval", "PACKAGE_INSTALL_APPROVAL",
        "All package installations require security review.", { command_patterns: ["npm install", "npm i ", "yarn add", "pip install", "pip3 install", "brew install", "apt install", "cargo install", "go install"] }),
      makeRule("pack.block_npm_scripts", ["shell.exec"], "require_approval", "NPM_SCRIPT_APPROVAL",
        "Running npm lifecycle scripts requires approval (post-install attacks).", { command_patterns: ["npm run", "npx "] }),
      makeRule("pack.block_lockfile_changes", ["file.write"], "require_approval", "LOCKFILE_CHANGE_APPROVAL",
        "Changes to lockfiles require approval.", { command_patterns: ["package-lock.json", "yarn.lock", "pnpm-lock.yaml", "Pipfile.lock", "Cargo.lock", "go.sum"] }),
    ],
  },

  // ── Secrets & Credentials Hardening ───────────────────────────────────────
  {
    id: "secrets-hardening",
    name: "Secrets & Credentials Hardening",
    description: "Block all access to credential files, environment variables, and cloud CLI auth. Prevent secrets from appearing in logs or prompts.",
    category: "Secrets Management",
    tags: ["secrets", "credentials", "api-keys", "cloud"],
    rules: [
      makeRule("pack.block_env_file_access", ["file.read", "shell.exec"], "deny", "ENV_FILE_BLOCKED",
        "Reading .env files is blocked to prevent secret exposure.", { command_patterns: ["cat .env", "cat .env.", "less .env", "head .env", "tail .env"] }),
      makeRule("pack.block_cloud_cred_access", ["file.read"], "deny", "CLOUD_CREDS_BLOCKED",
        "Reading cloud credential files is blocked.", { path_patterns: ["~/.aws/*", "~/.config/gcloud/*", "~/.azure/*", "~/.kube/*"] }),
      makeRule("pack.block_key_file_access", ["file.read"], "deny", "KEY_FILE_BLOCKED",
        "Reading private key and certificate files is blocked.", { path_patterns: ["*.pem", "*.key", "*.p12", "*.pfx", "*.jks"] }),
      makeRule("pack.block_vault_commands", ["shell.exec"], "deny", "VAULT_ACCESS_BLOCKED",
        "Direct vault/secrets-manager CLI access is blocked.", { command_patterns: ["vault read", "vault kv", "aws secretsmanager", "gcloud secrets", "az keyvault", "op read", "op item"] }),
      makeRule("pack.block_env_echo", ["shell.exec"], "deny", "ENV_ECHO_BLOCKED",
        "Echoing environment variables that may contain secrets is blocked.", { command_patterns: ["echo $", "printenv", "env | grep"] }),
    ],
  },

  // ── Infrastructure Safety ─────────────────────────────────────────────────
  {
    id: "infrastructure-safety",
    name: "Infrastructure Safety",
    description: "Prevent destructive infrastructure operations. Gate deployments, database changes, and cloud resource modifications.",
    category: "Infrastructure",
    tags: ["infrastructure", "deployment", "database", "cloud", "production"],
    rules: [
      makeRule("pack.approve_docker_ops", ["shell.exec"], "require_approval", "DOCKER_OPS_APPROVAL",
        "Docker operations require approval.", { command_patterns: ["docker run", "docker exec", "docker rm", "docker stop", "docker-compose up", "docker compose up"] }),
      makeRule("pack.block_db_destructive", ["shell.exec"], "deny", "DB_DESTRUCTIVE_BLOCKED",
        "Destructive database operations are blocked.", { command_patterns: ["DROP TABLE", "DROP DATABASE", "TRUNCATE", "DELETE FROM", "drop table", "drop database", "truncate"] }),
      makeRule("pack.approve_cloud_changes", ["shell.exec"], "require_approval", "CLOUD_CHANGE_APPROVAL",
        "Cloud infrastructure changes require approval.", { command_patterns: ["terraform apply", "terraform destroy", "pulumi up", "cdk deploy", "aws ", "gcloud ", "az "] }),
      makeRule("pack.approve_k8s_changes", ["shell.exec"], "require_approval", "K8S_CHANGE_APPROVAL",
        "Kubernetes changes require approval.", { command_patterns: ["kubectl apply", "kubectl delete", "kubectl scale", "helm install", "helm upgrade"] }),
      makeRule("pack.block_process_kill", ["shell.exec"], "deny", "PROCESS_KILL_BLOCKED",
        "Killing system processes is blocked.", { command_patterns: ["kill -9", "kill -KILL", "killall", "pkill"] }),
    ],
  },

  // ── Network Egress Control ────────────────────────────────────────────────
  {
    id: "network-egress-control",
    name: "Network Egress Control",
    description: "Strict control over all outbound network traffic. Block unknown destinations. Require approval for data uploads.",
    category: "Network Security",
    tags: ["network", "egress", "exfiltration", "firewall"],
    rules: [
      makeRule("pack.block_raw_network", ["shell.exec"], "deny", "RAW_NETWORK_BLOCKED",
        "Raw network tools (nc, nmap, telnet) are blocked.", { command_patterns: ["nc ", "ncat ", "nmap ", "telnet ", "netcat"] }),
      makeRule("pack.approve_ssh", ["shell.exec"], "require_approval", "SSH_APPROVAL",
        "SSH connections require approval.", { command_patterns: ["ssh ", "scp ", "sftp "] }),
      makeRule("pack.approve_curl_post", ["shell.exec"], "require_approval", "CURL_POST_APPROVAL",
        "HTTP POST/PUT requests via curl/wget require approval.", { command_patterns: ["curl -X POST", "curl -X PUT", "curl --data", "curl -d ", "wget --post"] }),
    ],
  },

  // ── Compliance & Audit ────────────────────────────────────────────────────
  {
    id: "compliance-audit",
    name: "Compliance & Audit Trail",
    description: "Enforce comprehensive audit logging. Require approval for any action that modifies system configuration or access controls.",
    category: "Compliance",
    tags: ["compliance", "audit", "soc2", "governance"],
    rules: [
      makeRule("pack.approve_config_changes", ["file.write"], "require_approval", "CONFIG_CHANGE_APPROVAL",
        "Changes to configuration files require approval.", { command_patterns: [".config", ".conf", ".cfg", ".ini", ".yaml", ".yml", ".toml"] }),
      makeRule("pack.approve_permission_changes", ["shell.exec"], "require_approval", "PERMISSION_CHANGE_APPROVAL",
        "Permission and ownership changes require approval.", { command_patterns: ["chmod", "chown", "chgrp", "setfacl"] }),
      makeRule("pack.approve_service_changes", ["shell.exec"], "require_approval", "SERVICE_CHANGE_APPROVAL",
        "Service management commands require approval.", { command_patterns: ["systemctl", "service ", "launchctl"] }),
    ],
  },

  // ── Development Best Practices ────────────────────────────────────────────
  {
    id: "dev-best-practices",
    name: "Development Best Practices",
    description: "Enforce safe development practices. Gate force-pushes, branch deletions, and direct commits to protected branches.",
    category: "Development",
    tags: ["git", "development", "branching", "code-review"],
    rules: [
      makeRule("pack.block_force_push", ["shell.exec"], "deny", "FORCE_PUSH_BLOCKED",
        "Force push is blocked to prevent history rewriting.", { command_patterns: ["git push --force", "git push -f"] }),
      makeRule("pack.approve_branch_delete", ["shell.exec"], "require_approval", "BRANCH_DELETE_APPROVAL",
        "Branch deletion requires approval.", { command_patterns: ["git branch -D", "git branch -d", "git push origin --delete"] }),
      makeRule("pack.approve_reset_hard", ["shell.exec"], "require_approval", "RESET_HARD_APPROVAL",
        "Git reset --hard requires approval to prevent data loss.", { command_patterns: ["git reset --hard", "git checkout -- ."] }),
      makeRule("pack.approve_stash_drop", ["shell.exec"], "require_approval", "STASH_DROP_APPROVAL",
        "Dropping git stash requires approval.", { command_patterns: ["git stash drop", "git stash clear"] }),
    ],
  },

  // ── MCP Tool Governance ───────────────────────────────────────────────────
  {
    id: "mcp-governance",
    name: "MCP Tool Governance",
    description: "Control which MCP servers and tools agents can invoke. Block untrusted servers. Require approval for data-access tools.",
    category: "Agent Governance",
    tags: ["mcp", "tools", "agents", "protocol"],
    rules: [
      makeRule("pack.block_untrusted_mcp", ["mcp.invoke"], "deny", "UNTRUSTED_MCP_BLOCKED",
        "MCP calls to untrusted servers are blocked by default."),
      makeRule("pack.approve_db_mcp", ["mcp.invoke"], "require_approval", "DB_MCP_APPROVAL",
        "MCP database tool invocations require approval."),
    ],
  },
];

/**
 * Get all available policy packs.
 */
export function getAvailablePacks(): PolicyPack[] {
  return POLICY_PACKS;
}

/**
 * Get a specific pack by ID.
 */
export function getPack(packId: string): PolicyPack | undefined {
  return POLICY_PACKS.find(p => p.id === packId);
}

/**
 * Apply a pack to a bundle (additive — doesn't remove existing rules).
 * Skips rules whose policy_id already exists in the bundle.
 */
export function applyPack(bundle: PolicyBundle, packId: string): { added: string[]; skipped: string[] } {
  const pack = getPack(packId);
  if (!pack) throw new Error(`Pack ${packId} not found`);

  const existingIds = new Set(bundle.rules.map(r => r.policy_id));
  const added: string[] = [];
  const skipped: string[] = [];

  for (const rule of pack.rules) {
    if (existingIds.has(rule.policy_id)) {
      skipped.push(rule.policy_id);
    } else {
      bundle.rules.push(rule);
      added.push(rule.policy_id);
    }
  }

  return { added, skipped };
}

/**
 * Add a custom pack to the registry.
 * Throws if a pack with the same ID already exists.
 */
export function addCustomPack(pack: PolicyPack): void {
  if (POLICY_PACKS.some(p => p.id === pack.id)) {
    throw new Error(`Pack ${pack.id} already exists`);
  }
  POLICY_PACKS.push(pack);
}