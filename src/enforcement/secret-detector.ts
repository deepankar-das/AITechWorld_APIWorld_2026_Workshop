/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Credential / Secret Access Detector
 *
 * Detects when an agent attempts to access credentials, secrets,
 * API keys, or sensitive environment variables. Routes through
 * daemon for policy evaluation.
 *
 * Detection methods:
 *   - File path matching: ~/.ssh/*, ~/.aws/*, .env, *.pem, *.key
 *   - Environment variable access: patterns like API_KEY, SECRET, TOKEN, PASSWORD
 *   - Shell commands that read credentials: cat .env, printenv, echo $SECRET
 *
 * Venture Prompt: "credential or secret access" as interception surface
 */

import * as path from "node:path";

// ── Sensitive File Patterns ─────────────────────────────────────────────────

const SENSITIVE_PATH_PATTERNS: Array<{ pattern: string; description: string }> = [
  { pattern: "~/.ssh/", description: "SSH keys and config" },
  { pattern: "~/.aws/", description: "AWS credentials and config" },
  { pattern: "~/.config/gcloud/", description: "Google Cloud credentials" },
  { pattern: "~/.azure/", description: "Azure credentials" },
  { pattern: "~/.kube/", description: "Kubernetes config and credentials" },
  { pattern: "~/.gnupg/", description: "GPG keys" },
  { pattern: "~/.netrc", description: "Network credentials" },
  { pattern: "~/.npmrc", description: "npm auth tokens" },
  { pattern: "~/.pypirc", description: "PyPI auth tokens" },
  { pattern: "~/.docker/config.json", description: "Docker registry credentials" },
];

const SENSITIVE_FILE_NAMES = [
  ".env",
  ".env.local",
  ".env.production",
  ".env.secret",
  "credentials.json",
  "service-account.json",
  "secrets.yaml",
  "secrets.yml",
  "vault.json",
];

const SENSITIVE_EXTENSIONS = [
  ".pem",
  ".key",
  ".p12",
  ".pfx",
  ".keystore",
  ".jks",
];

// ── Environment Variable Patterns ───────────────────────────────────────────

const SECRET_ENV_PATTERNS = [
  /API[_-]?KEY/i,
  /SECRET[_-]?KEY/i,
  /ACCESS[_-]?KEY/i,
  /AUTH[_-]?TOKEN/i,
  /PASSWORD/i,
  /PRIVATE[_-]?KEY/i,
  /DATABASE[_-]?URL/i,
  /REDIS[_-]?URL/i,
  /MONGODB[_-]?URI/i,
  /JWT[_-]?SECRET/i,
  /ENCRYPTION[_-]?KEY/i,
  /SIGNING[_-]?KEY/i,
];

// ── Shell Commands That Access Secrets ───────────────────────────────────────

const SECRET_COMMAND_PATTERNS = [
  "cat .env",
  "cat ~/.ssh/",
  "cat ~/.aws/",
  "printenv",
  "echo $",        // echo $SECRET_KEY, echo $API_KEY, etc.
  "env | grep",
  "aws configure",
  "gcloud auth",
  "az login",
  "vault read",
  "vault kv get",
  "op read",       // 1Password CLI
  "op item get",
];

// ── Detection Functions ─────────────────────────────────────────────────────

export interface SecretAccessDetection {
  isSensitive: boolean;
  type: "file" | "env_var" | "command" | "none";
  description: string;
  severity: "high" | "medium" | "low";
  /** The specific pattern that matched */
  matchedPattern: string;
}

/**
 * Check if a file path accesses sensitive credentials.
 */
export function detectSensitiveFilePath(filePath: string): SecretAccessDetection {
  const homeDir = process.env.HOME || process.env.USERPROFILE || "";
  // Expand ~ to home directory before resolving
  const expandedPath = filePath.startsWith("~")
    ? filePath.replace(/^~/, homeDir)
    : filePath;
  const normalizedPath = path.resolve(expandedPath);
  const fileName = path.basename(normalizedPath);
  const ext = path.extname(normalizedPath);

  // Check against known sensitive directory patterns
  for (const { pattern, description } of SENSITIVE_PATH_PATTERNS) {
    const expandedPattern = pattern.replace(/^~/, homeDir);
    if (normalizedPath.startsWith(expandedPattern)) {
      return {
        isSensitive: true,
        type: "file",
        description: `Access to ${description}`,
        severity: "high",
        matchedPattern: pattern,
      };
    }
  }

  // Check sensitive file names
  if (SENSITIVE_FILE_NAMES.includes(fileName)) {
    return {
      isSensitive: true,
      type: "file",
      description: `Access to credential file: ${fileName}`,
      severity: "high",
      matchedPattern: fileName,
    };
  }

  // Check sensitive extensions
  if (SENSITIVE_EXTENSIONS.includes(ext)) {
    return {
      isSensitive: true,
      type: "file",
      description: `Access to key/certificate file (${ext})`,
      severity: "high",
      matchedPattern: ext,
    };
  }

  return { isSensitive: false, type: "none", description: "", severity: "low", matchedPattern: "" };
}

/**
 * Check if a shell command accesses credentials or secrets.
 */
export function detectSecretCommandAccess(command: string): SecretAccessDetection {
  const normalized = command.trim().toLowerCase();

  // Check for environment variable expansion patterns FIRST (more specific than echo $)
  const envVarMatch = command.match(/\$\{?([A-Z_]+)\}?/g);
  if (envVarMatch) {
    for (const varRef of envVarMatch) {
      const varName = varRef.replace(/[${}]/g, "");
      for (const pattern of SECRET_ENV_PATTERNS) {
        if (pattern.test(varName)) {
          return {
            isSensitive: true,
            type: "env_var",
            description: `Command references secret environment variable: ${varName}`,
            severity: "medium",
            matchedPattern: varName,
          };
        }
      }
    }
  }

  // Then check command patterns
  for (const pattern of SECRET_COMMAND_PATTERNS) {
    if (normalized.startsWith(pattern) || normalized.includes(pattern)) {
      return {
        isSensitive: true,
        type: "command",
        description: `Command accesses credentials: ${pattern}`,
        severity: "high",
        matchedPattern: pattern,
      };
    }
  }

  return { isSensitive: false, type: "none", description: "", severity: "low", matchedPattern: "" };
}

/**
 * Combined detection: check both file path and command for secret access.
 */
export function detectSecretAccess(
  filePath?: string,
  command?: string,
): SecretAccessDetection {
  if (filePath) {
    const fileResult = detectSensitiveFilePath(filePath);
    if (fileResult.isSensitive) return fileResult;
  }

  if (command) {
    const cmdResult = detectSecretCommandAccess(command);
    if (cmdResult.isSensitive) return cmdResult;
  }

  return { isSensitive: false, type: "none", description: "", severity: "low", matchedPattern: "" };
}