/**
 * Author: Deepankar Das
 */

package enforcement

import (
	"testing"
)

func TestDetectPackageInstall_NpmInstall(t *testing.T) {
	r := DetectPackageInstall("npm install express")
	if !r.IsPackageInstall {
		t.Fatal("expected IsPackageInstall=true")
	}
	if r.PackageManager != "npm" {
		t.Fatalf("expected npm, got %s", r.PackageManager)
	}
	if r.PackageName != "express" {
		t.Fatalf("expected express, got %s", r.PackageName)
	}
	if r.Registry != "registry.npmjs.org" {
		t.Fatalf("expected registry.npmjs.org, got %s", r.Registry)
	}
}

func TestDetectPackageInstall_NpmI(t *testing.T) {
	r := DetectPackageInstall("npm i lodash")
	if !r.IsPackageInstall {
		t.Fatal("expected IsPackageInstall=true")
	}
	if r.PackageManager != "npm" {
		t.Fatalf("expected npm, got %s", r.PackageManager)
	}
	if r.PackageName != "lodash" {
		t.Fatalf("expected lodash, got %s", r.PackageName)
	}
}

func TestDetectPackageInstall_NpmInstallWithFlags(t *testing.T) {
	r := DetectPackageInstall("npm install --save-dev typescript")
	if !r.IsPackageInstall {
		t.Fatal("expected IsPackageInstall=true")
	}
	if r.PackageName != "typescript" {
		t.Fatalf("expected typescript, got %s", r.PackageName)
	}
}

func TestDetectPackageInstall_YarnAdd(t *testing.T) {
	r := DetectPackageInstall("yarn add react")
	if !r.IsPackageInstall {
		t.Fatal("expected IsPackageInstall=true")
	}
	if r.PackageManager != "yarn" {
		t.Fatalf("expected yarn, got %s", r.PackageManager)
	}
	if r.PackageName != "react" {
		t.Fatalf("expected react, got %s", r.PackageName)
	}
	if r.Registry != "registry.npmjs.org" {
		t.Fatalf("expected registry.npmjs.org, got %s", r.Registry)
	}
}

func TestDetectPackageInstall_PipInstall(t *testing.T) {
	r := DetectPackageInstall("pip install requests")
	if !r.IsPackageInstall {
		t.Fatal("expected IsPackageInstall=true")
	}
	if r.PackageManager != "pip" {
		t.Fatalf("expected pip, got %s", r.PackageManager)
	}
	if r.PackageName != "requests" {
		t.Fatalf("expected requests, got %s", r.PackageName)
	}
	if r.Registry != "pypi.org" {
		t.Fatalf("expected pypi.org, got %s", r.Registry)
	}
}

func TestDetectPackageInstall_Pip3Install(t *testing.T) {
	r := DetectPackageInstall("pip3 install flask")
	if !r.IsPackageInstall {
		t.Fatal("expected IsPackageInstall=true")
	}
	if r.PackageManager != "pip3" {
		t.Fatalf("expected pip3, got %s", r.PackageManager)
	}
	if r.PackageName != "flask" {
		t.Fatalf("expected flask, got %s", r.PackageName)
	}
}

func TestDetectPackageInstall_BrewInstall(t *testing.T) {
	r := DetectPackageInstall("brew install jq")
	if !r.IsPackageInstall {
		t.Fatal("expected IsPackageInstall=true")
	}
	if r.PackageManager != "brew" {
		t.Fatalf("expected brew, got %s", r.PackageManager)
	}
	if r.PackageName != "jq" {
		t.Fatalf("expected jq, got %s", r.PackageName)
	}
	if r.Registry != "formulae.brew.sh" {
		t.Fatalf("expected formulae.brew.sh, got %s", r.Registry)
	}
}

func TestDetectPackageInstall_CargoInstall(t *testing.T) {
	r := DetectPackageInstall("cargo install ripgrep")
	if !r.IsPackageInstall {
		t.Fatal("expected IsPackageInstall=true")
	}
	if r.PackageManager != "cargo" {
		t.Fatalf("expected cargo, got %s", r.PackageManager)
	}
	if r.PackageName != "ripgrep" {
		t.Fatalf("expected ripgrep, got %s", r.PackageName)
	}
	if r.Registry != "crates.io" {
		t.Fatalf("expected crates.io, got %s", r.Registry)
	}
}

func TestDetectPackageInstall_GemInstall(t *testing.T) {
	r := DetectPackageInstall("gem install rails")
	if !r.IsPackageInstall {
		t.Fatal("expected IsPackageInstall=true")
	}
	if r.PackageManager != "gem" {
		t.Fatalf("expected gem, got %s", r.PackageManager)
	}
	if r.PackageName != "rails" {
		t.Fatalf("expected rails, got %s", r.PackageName)
	}
	if r.Registry != "rubygems.org" {
		t.Fatalf("expected rubygems.org, got %s", r.Registry)
	}
}

func TestDetectPackageInstall_GoInstall(t *testing.T) {
	r := DetectPackageInstall("go install golang.org/x/tools/gopls@latest")
	if !r.IsPackageInstall {
		t.Fatal("expected IsPackageInstall=true")
	}
	if r.PackageManager != "go" {
		t.Fatalf("expected go, got %s", r.PackageManager)
	}
	if r.PackageName != "golang.org/x/tools/gopls@latest" {
		t.Fatalf("expected golang.org/x/tools/gopls@latest, got %s", r.PackageName)
	}
	if r.Registry != "proxy.golang.org" {
		t.Fatalf("expected proxy.golang.org, got %s", r.Registry)
	}
}

func TestDetectPackageInstall_GoGet(t *testing.T) {
	r := DetectPackageInstall("go get github.com/stretchr/testify")
	if !r.IsPackageInstall {
		t.Fatal("expected IsPackageInstall=true")
	}
	if r.PackageManager != "go" {
		t.Fatalf("expected go, got %s", r.PackageManager)
	}
	if r.PackageName != "github.com/stretchr/testify" {
		t.Fatalf("expected github.com/stretchr/testify, got %s", r.PackageName)
	}
}

func TestDetectPackageInstall_NpmInstallNoPackage(t *testing.T) {
	r := DetectPackageInstall("npm install")
	if !r.IsPackageInstall {
		t.Fatal("expected IsPackageInstall=true for bare npm install")
	}
	if r.PackageManager != "npm" {
		t.Fatalf("expected npm, got %s", r.PackageManager)
	}
	// No specific package name when running bare "npm install".
	if r.PackageName != "" {
		t.Fatalf("expected empty package name for bare npm install, got %s", r.PackageName)
	}
}

func TestDetectPackageInstall_NotPackageCommand_Ls(t *testing.T) {
	r := DetectPackageInstall("ls -la")
	if r.IsPackageInstall {
		t.Fatal("expected IsPackageInstall=false for ls")
	}
}

func TestDetectPackageInstall_NotPackageCommand_NpmTest(t *testing.T) {
	r := DetectPackageInstall("npm test")
	if r.IsPackageInstall {
		t.Fatal("expected IsPackageInstall=false for npm test")
	}
}

func TestDetectPackageInstall_NotPackageCommand_GitPush(t *testing.T) {
	r := DetectPackageInstall("git push origin main")
	if r.IsPackageInstall {
		t.Fatal("expected IsPackageInstall=false for git push")
	}
}

func TestDetectPackageInstall_NotPackageCommand_Echo(t *testing.T) {
	r := DetectPackageInstall("echo hello world")
	if r.IsPackageInstall {
		t.Fatal("expected IsPackageInstall=false for echo")
	}
}

func TestDetectPackageInstall_ComposerRequire(t *testing.T) {
	r := DetectPackageInstall("composer require laravel/framework")
	if !r.IsPackageInstall {
		t.Fatal("expected IsPackageInstall=true")
	}
	if r.PackageManager != "composer" {
		t.Fatalf("expected composer, got %s", r.PackageManager)
	}
	if r.PackageName != "laravel/framework" {
		t.Fatalf("expected laravel/framework, got %s", r.PackageName)
	}
	if r.Registry != "packagist.org" {
		t.Fatalf("expected packagist.org, got %s", r.Registry)
	}
}
