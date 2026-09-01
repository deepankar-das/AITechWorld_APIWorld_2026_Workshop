/**
 * Author: Deepankar Das
 */

package enforcement

import (
	"strings"
)

// SecretAccessDetection describes whether an action touches sensitive
// credentials, keys, or configuration.
type SecretAccessDetection struct {
	IsSecret    bool   `json:"is_secret"`
	Category    string `json:"category,omitempty"`
	Description string `json:"description,omitempty"`
	Severity    string `json:"severity,omitempty"` // "critical", "high", "medium", "low"
}

// sensitiveFilePattern defines a file path pattern and its associated metadata.
type sensitiveFilePattern struct {
	pattern     string
	matchMode   string // "prefix", "suffix", "contains", "exact"
	category    string
	description string
	severity    string
}

// sensitiveFilePatterns lists file paths known to contain secrets or credentials.
var sensitiveFilePatterns = []sensitiveFilePattern{
	{pattern: "/.ssh/", matchMode: "contains", category: "ssh_key", description: "SSH key or configuration file", severity: "critical"},
	{pattern: "/.aws/", matchMode: "contains", category: "aws_credentials", description: "AWS credentials or configuration", severity: "critical"},
	{pattern: "/.kube/", matchMode: "contains", category: "kubernetes_config", description: "Kubernetes configuration or credentials", severity: "critical"},
	{pattern: "/.docker/config.json", matchMode: "suffix", category: "docker_credentials", description: "Docker registry credentials", severity: "critical"},
	{pattern: ".env", matchMode: "suffix", category: "env_file", description: "Environment variable file (may contain secrets)", severity: "high"},
	{pattern: ".pem", matchMode: "suffix", category: "private_key", description: "PEM-encoded private key or certificate", severity: "critical"},
	{pattern: ".key", matchMode: "suffix", category: "private_key", description: "Private key file", severity: "critical"},
	{pattern: ".p12", matchMode: "suffix", category: "private_key", description: "PKCS#12 key store", severity: "critical"},
	{pattern: ".pfx", matchMode: "suffix", category: "private_key", description: "PFX certificate file", severity: "critical"},
	{pattern: ".keystore", matchMode: "suffix", category: "keystore", description: "Java key store file", severity: "critical"},
	{pattern: "credentials.json", matchMode: "suffix", category: "credentials_file", description: "Credentials configuration file", severity: "critical"},
	{pattern: "id_rsa", matchMode: "suffix", category: "ssh_private_key", description: "RSA private key", severity: "critical"},
	{pattern: "id_ed25519", matchMode: "suffix", category: "ssh_private_key", description: "Ed25519 private key", severity: "critical"},
	{pattern: "id_ecdsa", matchMode: "suffix", category: "ssh_private_key", description: "ECDSA private key", severity: "critical"},
	{pattern: "id_dsa", matchMode: "suffix", category: "ssh_private_key", description: "DSA private key", severity: "critical"},
	{pattern: "/.gnupg/", matchMode: "contains", category: "gpg_key", description: "GPG key ring or configuration", severity: "critical"},
	{pattern: "/.npmrc", matchMode: "suffix", category: "npm_token", description: "NPM configuration (may contain auth tokens)", severity: "high"},
	{pattern: "/.netrc", matchMode: "suffix", category: "netrc_credentials", description: "Netrc credentials file", severity: "high"},
	{pattern: "/.git-credentials", matchMode: "suffix", category: "git_credentials", description: "Git credentials store", severity: "high"},
}

// sensitiveCommandPattern defines a command pattern that accesses secrets.
type sensitiveCommandPattern struct {
	pattern     string
	matchMode   string // "prefix", "contains"
	category    string
	description string
	severity    string
}

// sensitiveCommandPatterns lists command patterns that access credentials or secrets.
var sensitiveCommandPatterns = []sensitiveCommandPattern{
	{pattern: "cat .env", matchMode: "contains", category: "env_file_read", description: "Reading environment file containing secrets", severity: "high"},
	{pattern: "cat ~/.env", matchMode: "contains", category: "env_file_read", description: "Reading home environment file", severity: "high"},
	{pattern: "printenv", matchMode: "contains", category: "env_dump", description: "Dumping all environment variables", severity: "high"},
	{pattern: "aws configure", matchMode: "contains", category: "aws_config", description: "Configuring AWS credentials", severity: "critical"},
	{pattern: "aws sts", matchMode: "contains", category: "aws_sts", description: "AWS STS token operation", severity: "high"},
	{pattern: "cat /etc/shadow", matchMode: "contains", category: "shadow_file", description: "Reading system password hashes", severity: "critical"},
	{pattern: "cat /etc/passwd", matchMode: "contains", category: "passwd_file", description: "Reading system user database", severity: "medium"},
	{pattern: "export ", matchMode: "contains", category: "env_export", description: "Setting environment variable (may contain secrets)", severity: "medium"},
	{pattern: "vault ", matchMode: "prefix", category: "vault_access", description: "HashiCorp Vault secret access", severity: "high"},
	{pattern: "gpg --export-secret", matchMode: "contains", category: "gpg_export", description: "Exporting GPG secret keys", severity: "critical"},
	{pattern: "security find-generic-password", matchMode: "contains", category: "keychain_access", description: "macOS Keychain password access", severity: "critical"},
	{pattern: "security find-internet-password", matchMode: "contains", category: "keychain_access", description: "macOS Keychain internet password access", severity: "critical"},
}

// DetectSensitiveFilePath checks whether a file path points to a known
// sensitive file such as SSH keys, cloud credentials, or environment files.
func DetectSensitiveFilePath(filePath string) SecretAccessDetection {
	if filePath == "" {
		return SecretAccessDetection{IsSecret: false}
	}

	for _, p := range sensitiveFilePatterns {
		matched := false
		switch p.matchMode {
		case "prefix":
			matched = strings.HasPrefix(filePath, p.pattern)
		case "suffix":
			matched = strings.HasSuffix(filePath, p.pattern)
		case "contains":
			matched = strings.Contains(filePath, p.pattern)
		case "exact":
			matched = filePath == p.pattern
		}
		if matched {
			return SecretAccessDetection{
				IsSecret:    true,
				Category:    p.category,
				Description: p.description,
				Severity:    p.severity,
			}
		}
	}

	return SecretAccessDetection{IsSecret: false}
}

// DetectSecretCommandAccess checks whether a shell command accesses secrets
// via environment variables, known credential commands, or sensitive file reads.
func DetectSecretCommandAccess(command string) SecretAccessDetection {
	if command == "" {
		return SecretAccessDetection{IsSecret: false}
	}

	trimmed := strings.TrimSpace(command)

	// Check for environment variable references ($VAR patterns).
	if containsEnvVarReference(trimmed) {
		// Only flag common secret-related env vars.
		secretEnvVars := []string{
			"$AWS_SECRET_ACCESS_KEY", "$AWS_ACCESS_KEY_ID", "$AWS_SESSION_TOKEN",
			"$DATABASE_URL", "$DB_PASSWORD", "$SECRET_KEY", "$API_KEY",
			"$PRIVATE_KEY", "$TOKEN", "$PASSWORD", "$GITHUB_TOKEN",
			"$NPM_TOKEN", "$SLACK_TOKEN", "$STRIPE_SECRET_KEY",
		}
		for _, envVar := range secretEnvVars {
			if strings.Contains(strings.ToUpper(trimmed), strings.TrimPrefix(envVar, "$")) &&
				strings.Contains(trimmed, "$") {
				return SecretAccessDetection{
					IsSecret:    true,
					Category:    "env_var_secret",
					Description: "Command references secret environment variable: " + envVar,
					Severity:    "high",
				}
			}
		}
	}

	// Check known sensitive command patterns.
	for _, p := range sensitiveCommandPatterns {
		matched := false
		switch p.matchMode {
		case "prefix":
			matched = strings.HasPrefix(trimmed, p.pattern)
		case "contains":
			matched = strings.Contains(trimmed, p.pattern)
		}
		if matched {
			return SecretAccessDetection{
				IsSecret:    true,
				Category:    p.category,
				Description: p.description,
				Severity:    p.severity,
			}
		}
	}

	return SecretAccessDetection{IsSecret: false}
}

// DetectSecretAccess is a combined check that evaluates both a file path and a
// command for secret/credential access. It returns the first detection found,
// prioritising file path detection.
func DetectSecretAccess(filePath, command string) SecretAccessDetection {
	if filePath != "" {
		result := DetectSensitiveFilePath(filePath)
		if result.IsSecret {
			return result
		}
	}

	if command != "" {
		result := DetectSecretCommandAccess(command)
		if result.IsSecret {
			return result
		}
	}

	return SecretAccessDetection{IsSecret: false}
}

// containsEnvVarReference checks whether a string contains a shell
// environment variable reference (e.g., $VAR or ${VAR}).
func containsEnvVarReference(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '$' && i+1 < len(s) {
			next := s[i+1]
			if (next >= 'A' && next <= 'Z') || (next >= 'a' && next <= 'z') || next == '_' || next == '{' {
				return true
			}
		}
	}
	return false
}
