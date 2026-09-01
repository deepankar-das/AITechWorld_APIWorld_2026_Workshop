/**
 * Author: Deepankar Das
 */

package policy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/anthropics/enforcer/internal/types"
	"gopkg.in/yaml.v3"
)

var (
	verifierMu       sync.Mutex
	cachedVerifier   *PolicySigner
	cachedVerifierPK string
)

func getPolicyVerifier() (*PolicySigner, error) {
	publicKeyPath := strings.TrimSpace(os.Getenv("AA_POLICY_PUBLIC_KEY"))
	if publicKeyPath == "" {
		return nil, nil
	}

	verifierMu.Lock()
	defer verifierMu.Unlock()

	if cachedVerifier != nil && cachedVerifierPK == publicKeyPath {
		return cachedVerifier, nil
	}

	pub, err := LoadPublicKeyFromFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("loading policy public key from %s: %w", publicKeyPath, err)
	}
	cachedVerifier = NewPolicyVerifier(pub)
	cachedVerifierPK = publicKeyPath
	return cachedVerifier, nil
}

func verifyPolicySignature(policyPath string, bundleYAML []byte, bundleVersion string) error {
	verifier, err := getPolicyVerifier()
	if err != nil {
		return err
	}
	if verifier == nil {
		return nil
	}

	signaturePath := strings.TrimSpace(os.Getenv("AA_POLICY_SIGNATURE"))
	if signaturePath == "" {
		signaturePath = policyPath + ".sig.json"
	}

	signatureBytes, err := os.ReadFile(signaturePath)
	if err != nil {
		return fmt.Errorf("reading policy signature file %s: %w", signaturePath, err)
	}

	var sig PolicySignature
	if err := json.Unmarshal(signatureBytes, &sig); err != nil {
		return fmt.Errorf("parsing policy signature JSON %s: %w", signaturePath, err)
	}

	if bundleVersion != "" && sig.Version != "" && sig.Version != bundleVersion {
		return fmt.Errorf("policy version mismatch: bundle=%s signature=%s", bundleVersion, sig.Version)
	}

	signed := &SignedBundle{
		Bundle:    bundleYAML,
		Signature: sig,
	}
	if err := verifier.VerifyAndCheckMonotonicity(signed); err != nil {
		return fmt.Errorf("policy signature verification failed: %w", err)
	}
	verifier.AcceptVersion(sig.Version)
	return nil
}

// LoadedBundle wraps a parsed policy bundle with metadata.
type LoadedBundle struct {
	Bundle   types.PolicyBundle
	LoadedAt string
	FilePath string
}

// LoadPolicyBundle loads a YAML policy bundle from disk.
func LoadPolicyBundle(filePath string) (*LoadedBundle, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading policy file %s: %w", filePath, err)
	}

	var bundle types.PolicyBundle
	if err := yaml.Unmarshal(data, &bundle); err != nil {
		return nil, fmt.Errorf("parsing policy YAML %s: %w", filePath, err)
	}

	if err := verifyPolicySignature(filePath, data, bundle.BundleVersion); err != nil {
		return nil, err
	}

	return &LoadedBundle{
		Bundle:   bundle,
		LoadedAt: time.Now().UTC().Format(time.RFC3339),
		FilePath: filePath,
	}, nil
}

// LoadPolicyDirectory loads all .yaml/.yml files from a directory and merges them.
func LoadPolicyDirectory(dirPath string) (*LoadedBundle, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("reading policy directory %s: %w", dirPath, err)
	}

	merged := types.PolicyBundle{
		BundleVersion: "merged",
		ScopeLevel:    types.ScopeOrganization,
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		loaded, err := LoadPolicyBundle(filepath.Join(dirPath, name))
		if err != nil {
			return nil, err
		}
		if loaded.Bundle.BundleVersion != "" && loaded.Bundle.BundleVersion != "merged" {
			merged.BundleVersion = loaded.Bundle.BundleVersion
		}
		if loaded.Bundle.ScopeLevel != "" {
			merged.ScopeLevel = loaded.Bundle.ScopeLevel
		}
		merged.Rules = append(merged.Rules, loaded.Bundle.Rules...)
	}

	return &LoadedBundle{
		Bundle:   merged,
		LoadedAt: time.Now().UTC().Format(time.RFC3339),
		FilePath: dirPath,
	}, nil
}

// LoadPolicyFromBytes parses a policy bundle from raw YAML bytes.
func LoadPolicyFromBytes(data []byte) (*types.PolicyBundle, error) {
	var bundle types.PolicyBundle
	if err := yaml.Unmarshal(data, &bundle); err != nil {
		return nil, fmt.Errorf("parsing policy YAML: %w", err)
	}
	return &bundle, nil
}
