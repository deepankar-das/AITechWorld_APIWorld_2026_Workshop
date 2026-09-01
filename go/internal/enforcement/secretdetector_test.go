/**
 * Author: Deepankar Das
 */

package enforcement

import (
	"testing"
)

// --- File path detection ---

func TestDetectSensitiveFilePath_SshKey(t *testing.T) {
	r := DetectSensitiveFilePath("/home/user/.ssh/id_rsa")
	if !r.IsSecret {
		t.Fatal("expected IsSecret=true for SSH key")
	}
	if r.Category != "ssh_key" {
		t.Fatalf("expected category ssh_key, got %s", r.Category)
	}
	if r.Severity != "critical" {
		t.Fatalf("expected severity critical, got %s", r.Severity)
	}
}

func TestDetectSensitiveFilePath_AwsCredentials(t *testing.T) {
	r := DetectSensitiveFilePath("/home/user/.aws/credentials")
	if !r.IsSecret {
		t.Fatal("expected IsSecret=true for AWS credentials")
	}
	if r.Category != "aws_credentials" {
		t.Fatalf("expected category aws_credentials, got %s", r.Category)
	}
}

func TestDetectSensitiveFilePath_KubeConfig(t *testing.T) {
	r := DetectSensitiveFilePath("/home/user/.kube/config")
	if !r.IsSecret {
		t.Fatal("expected IsSecret=true for kube config")
	}
	if r.Category != "kubernetes_config" {
		t.Fatalf("expected category kubernetes_config, got %s", r.Category)
	}
}

func TestDetectSensitiveFilePath_DockerConfig(t *testing.T) {
	r := DetectSensitiveFilePath("/home/user/.docker/config.json")
	if !r.IsSecret {
		t.Fatal("expected IsSecret=true for docker config")
	}
	if r.Category != "docker_credentials" {
		t.Fatalf("expected category docker_credentials, got %s", r.Category)
	}
}

func TestDetectSensitiveFilePath_EnvFile(t *testing.T) {
	r := DetectSensitiveFilePath("/project/.env")
	if !r.IsSecret {
		t.Fatal("expected IsSecret=true for .env file")
	}
	if r.Category != "env_file" {
		t.Fatalf("expected category env_file, got %s", r.Category)
	}
}

func TestDetectSensitiveFilePath_PemFile(t *testing.T) {
	r := DetectSensitiveFilePath("/certs/server.pem")
	if !r.IsSecret {
		t.Fatal("expected IsSecret=true for .pem file")
	}
	if r.Category != "private_key" {
		t.Fatalf("expected category private_key, got %s", r.Category)
	}
}

func TestDetectSensitiveFilePath_KeyFile(t *testing.T) {
	r := DetectSensitiveFilePath("/certs/private.key")
	if !r.IsSecret {
		t.Fatal("expected IsSecret=true for .key file")
	}
}

func TestDetectSensitiveFilePath_SafeFile(t *testing.T) {
	r := DetectSensitiveFilePath("/project/src/main.go")
	if r.IsSecret {
		t.Fatal("expected IsSecret=false for regular source file")
	}
}

func TestDetectSensitiveFilePath_EmptyPath(t *testing.T) {
	r := DetectSensitiveFilePath("")
	if r.IsSecret {
		t.Fatal("expected IsSecret=false for empty path")
	}
}

func TestDetectSensitiveFilePath_GnupgDir(t *testing.T) {
	r := DetectSensitiveFilePath("/home/user/.gnupg/secring.gpg")
	if !r.IsSecret {
		t.Fatal("expected IsSecret=true for GPG key ring")
	}
	if r.Category != "gpg_key" {
		t.Fatalf("expected category gpg_key, got %s", r.Category)
	}
}

func TestDetectSensitiveFilePath_Npmrc(t *testing.T) {
	r := DetectSensitiveFilePath("/home/user/.npmrc")
	if !r.IsSecret {
		t.Fatal("expected IsSecret=true for .npmrc")
	}
	if r.Category != "npm_token" {
		t.Fatalf("expected category npm_token, got %s", r.Category)
	}
}

func TestDetectSensitiveFilePath_GitCredentials(t *testing.T) {
	r := DetectSensitiveFilePath("/home/user/.git-credentials")
	if !r.IsSecret {
		t.Fatal("expected IsSecret=true for .git-credentials")
	}
	if r.Category != "git_credentials" {
		t.Fatalf("expected category git_credentials, got %s", r.Category)
	}
}

// --- Command access detection ---

func TestDetectSecretCommandAccess_CatEnv(t *testing.T) {
	r := DetectSecretCommandAccess("cat .env")
	if !r.IsSecret {
		t.Fatal("expected IsSecret=true for cat .env")
	}
	if r.Category != "env_file_read" {
		t.Fatalf("expected category env_file_read, got %s", r.Category)
	}
}

func TestDetectSecretCommandAccess_Printenv(t *testing.T) {
	r := DetectSecretCommandAccess("printenv")
	if !r.IsSecret {
		t.Fatal("expected IsSecret=true for printenv")
	}
	if r.Category != "env_dump" {
		t.Fatalf("expected category env_dump, got %s", r.Category)
	}
}

func TestDetectSecretCommandAccess_AwsConfigure(t *testing.T) {
	r := DetectSecretCommandAccess("aws configure")
	if !r.IsSecret {
		t.Fatal("expected IsSecret=true for aws configure")
	}
	if r.Category != "aws_config" {
		t.Fatalf("expected category aws_config, got %s", r.Category)
	}
}

func TestDetectSecretCommandAccess_EnvVarReference(t *testing.T) {
	r := DetectSecretCommandAccess("echo $AWS_SECRET_ACCESS_KEY")
	if !r.IsSecret {
		t.Fatal("expected IsSecret=true for secret env var reference")
	}
	if r.Category != "env_var_secret" {
		t.Fatalf("expected category env_var_secret, got %s", r.Category)
	}
}

func TestDetectSecretCommandAccess_SafeCommand(t *testing.T) {
	r := DetectSecretCommandAccess("ls -la")
	if r.IsSecret {
		t.Fatal("expected IsSecret=false for ls")
	}
}

func TestDetectSecretCommandAccess_EmptyCommand(t *testing.T) {
	r := DetectSecretCommandAccess("")
	if r.IsSecret {
		t.Fatal("expected IsSecret=false for empty command")
	}
}

func TestDetectSecretCommandAccess_VaultCommand(t *testing.T) {
	r := DetectSecretCommandAccess("vault kv get secret/myapp")
	if !r.IsSecret {
		t.Fatal("expected IsSecret=true for vault command")
	}
	if r.Category != "vault_access" {
		t.Fatalf("expected category vault_access, got %s", r.Category)
	}
}

func TestDetectSecretCommandAccess_GpgExportSecret(t *testing.T) {
	r := DetectSecretCommandAccess("gpg --export-secret-keys user@example.com")
	if !r.IsSecret {
		t.Fatal("expected IsSecret=true for GPG secret export")
	}
	if r.Category != "gpg_export" {
		t.Fatalf("expected category gpg_export, got %s", r.Category)
	}
}

// --- Combined detection ---

func TestDetectSecretAccess_FilePathPriority(t *testing.T) {
	r := DetectSecretAccess("/home/user/.ssh/id_rsa", "ls -la")
	if !r.IsSecret {
		t.Fatal("expected IsSecret=true via file path")
	}
	if r.Category != "ssh_key" {
		t.Fatalf("expected ssh_key from file path, got %s", r.Category)
	}
}

func TestDetectSecretAccess_CommandFallback(t *testing.T) {
	r := DetectSecretAccess("/project/src/main.go", "cat .env")
	if !r.IsSecret {
		t.Fatal("expected IsSecret=true via command")
	}
	if r.Category != "env_file_read" {
		t.Fatalf("expected env_file_read from command, got %s", r.Category)
	}
}

func TestDetectSecretAccess_BothSafe(t *testing.T) {
	r := DetectSecretAccess("/project/src/main.go", "ls -la")
	if r.IsSecret {
		t.Fatal("expected IsSecret=false when both path and command are safe")
	}
}

func TestDetectSecretAccess_BothEmpty(t *testing.T) {
	r := DetectSecretAccess("", "")
	if r.IsSecret {
		t.Fatal("expected IsSecret=false when both are empty")
	}
}
