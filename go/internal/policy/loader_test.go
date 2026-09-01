/**
 * Author: Deepankar Das
 */

package policy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPolicyBundle_WithoutSignatureVerification(t *testing.T) {
	t.Setenv("AA_POLICY_PUBLIC_KEY", "")
	t.Setenv("AA_POLICY_SIGNATURE", "")

	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.yaml")
	policyYAML := []byte("bundle_version: v1.0.0\nscope_level: organization\nrules: []\n")
	if err := os.WriteFile(policyPath, policyYAML, 0644); err != nil {
		t.Fatalf("write policy: %v", err)
	}

	loaded, err := LoadPolicyBundle(policyPath)
	if err != nil {
		t.Fatalf("LoadPolicyBundle failed: %v", err)
	}
	if loaded.Bundle.BundleVersion != "v1.0.0" {
		t.Fatalf("bundle version mismatch: got %s", loaded.Bundle.BundleVersion)
	}
}

func TestLoadPolicyBundle_WithValidSignature(t *testing.T) {
	pub, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}
	signer := NewPolicySigner(pub, priv, "test-signer")

	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.yaml")
	signaturePath := filepath.Join(dir, "policy.sig.json")
	pubKeyPath := filepath.Join(dir, "policy.pub")

	policyYAML := []byte("bundle_version: v1.0.0\nscope_level: organization\nrules: []\n")
	if err := os.WriteFile(policyPath, policyYAML, 0644); err != nil {
		t.Fatalf("write policy: %v", err)
	}
	if err := SavePublicKeyToFile(pubKeyPath, pub); err != nil {
		t.Fatalf("save public key: %v", err)
	}

	signed, err := signer.Sign(policyYAML, "v1.0.0")
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}
	sigBytes, err := json.Marshal(signed.Signature)
	if err != nil {
		t.Fatalf("marshal signature: %v", err)
	}
	if err := os.WriteFile(signaturePath, sigBytes, 0644); err != nil {
		t.Fatalf("write signature: %v", err)
	}

	t.Setenv("AA_POLICY_PUBLIC_KEY", pubKeyPath)
	t.Setenv("AA_POLICY_SIGNATURE", signaturePath)
	if _, err := LoadPolicyBundle(policyPath); err != nil {
		t.Fatalf("expected signature verification success, got: %v", err)
	}
}

func TestLoadPolicyBundle_WithTamperedPolicyFails(t *testing.T) {
	pub, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}
	signer := NewPolicySigner(pub, priv, "test-signer")

	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.yaml")
	signaturePath := filepath.Join(dir, "policy.sig.json")
	pubKeyPath := filepath.Join(dir, "policy.pub")

	original := []byte("bundle_version: v1.0.0\nscope_level: organization\nrules: []\n")
	if err := SavePublicKeyToFile(pubKeyPath, pub); err != nil {
		t.Fatalf("save public key: %v", err)
	}
	signed, err := signer.Sign(original, "v1.0.0")
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}
	sigBytes, err := json.Marshal(signed.Signature)
	if err != nil {
		t.Fatalf("marshal signature: %v", err)
	}
	if err := os.WriteFile(signaturePath, sigBytes, 0644); err != nil {
		t.Fatalf("write signature: %v", err)
	}

	// Tamper after signing.
	tampered := []byte("bundle_version: v1.0.0\nscope_level: organization\nrules:\n  - policy_id: org.tampered\n")
	if err := os.WriteFile(policyPath, tampered, 0644); err != nil {
		t.Fatalf("write tampered policy: %v", err)
	}

	t.Setenv("AA_POLICY_PUBLIC_KEY", pubKeyPath)
	t.Setenv("AA_POLICY_SIGNATURE", signaturePath)
	if _, err := LoadPolicyBundle(policyPath); err == nil {
		t.Fatal("expected tampered policy verification failure")
	}
}
