/**
 * Author: Deepankar Das
 */

package enforcement

import (
	"strings"
)

// PackageInstallResult describes whether a command is a package install and
// which package manager, package name, and registry are involved.
type PackageInstallResult struct {
	IsPackageInstall bool   `json:"is_package_install"`
	PackageManager   string `json:"package_manager,omitempty"`
	PackageName      string `json:"package_name,omitempty"`
	Registry         string `json:"registry,omitempty"`
}

// packageManagerDef maps a command prefix to its manager name and registry.
type packageManagerDef struct {
	prefix   string
	manager  string
	registry string
	// argOffset is how many tokens to skip (from the start of prefix split)
	// before the package name. For "npm install", the package name is the 3rd token
	// in the full command ("npm", "install", "<pkg>").
	argOffset int
}

// packageManagers defines the supported package manager patterns.
// Order matters: more specific patterns (e.g., "npm install") should appear
// before shorter prefixes (e.g., "npm i ") so that the first match wins.
var packageManagers = []packageManagerDef{
	{prefix: "npm install", manager: "npm", registry: "registry.npmjs.org", argOffset: 2},
	{prefix: "npm i ", manager: "npm", registry: "registry.npmjs.org", argOffset: 2},
	{prefix: "yarn add", manager: "yarn", registry: "registry.npmjs.org", argOffset: 2},
	{prefix: "pip install", manager: "pip", registry: "pypi.org", argOffset: 2},
	{prefix: "pip3 install", manager: "pip3", registry: "pypi.org", argOffset: 2},
	{prefix: "brew install", manager: "brew", registry: "formulae.brew.sh", argOffset: 2},
	{prefix: "cargo install", manager: "cargo", registry: "crates.io", argOffset: 2},
	{prefix: "gem install", manager: "gem", registry: "rubygems.org", argOffset: 2},
	{prefix: "composer require", manager: "composer", registry: "packagist.org", argOffset: 2},
	{prefix: "go install", manager: "go", registry: "proxy.golang.org", argOffset: 2},
	{prefix: "go get", manager: "go", registry: "proxy.golang.org", argOffset: 2},
}

// DetectPackageInstall checks whether a shell command is a package install
// operation. If so, it extracts the package manager, the first package name,
// and the registry URL.
func DetectPackageInstall(command string) PackageInstallResult {
	trimmed := strings.TrimSpace(command)

	for _, pm := range packageManagers {
		if !strings.HasPrefix(trimmed, pm.prefix) {
			continue
		}

		// Extract package name: split on whitespace and take the token at argOffset.
		tokens := strings.Fields(trimmed)
		pkgName := ""
		if len(tokens) > pm.argOffset {
			candidate := tokens[pm.argOffset]
			// Skip flags (tokens starting with '-').
			for i := pm.argOffset; i < len(tokens); i++ {
				if !strings.HasPrefix(tokens[i], "-") {
					candidate = tokens[i]
					break
				}
			}
			if !strings.HasPrefix(candidate, "-") {
				pkgName = candidate
			}
		}

		return PackageInstallResult{
			IsPackageInstall: true,
			PackageManager:   pm.manager,
			PackageName:      pkgName,
			Registry:         pm.registry,
		}
	}

	return PackageInstallResult{IsPackageInstall: false}
}
