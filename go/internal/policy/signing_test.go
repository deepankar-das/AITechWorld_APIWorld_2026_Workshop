/**
 * Author: Deepankar Das
 */

package policy

import (
	"testing"
)

func TestSignAndVerify(t *testing.T) {
	pub, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("key generation failed: %v", err)
	}

	signer := NewPolicySigner(pub, priv, "test-signer")
	bundleYAML := []byte("bundle_version: v1.0.0\nrules: []\n")

	signed, err := signer.Sign(bundleYAML, "v1.0.0")
	if err != nil {
		t.Fatalf("signing failed: %v", err)
	}

	if signed.Signature.BundleHash == "" {
		t.Error("expected non-empty bundle hash")
	}
	if signed.Signature.Signature == "" {
		t.Error("expected non-empty signature")
	}
	if signed.Signature.SignerID != "test-signer" {
		t.Errorf("expected signer_id=test-signer, got %s", signed.Signature.SignerID)
	}

	// Verify with same public key
	if err := signer.Verify(signed); err != nil {
		t.Fatalf("verification failed: %v", err)
	}
}

func TestVerifyDetectsTampering(t *testing.T) {
	pub, priv, _ := GenerateKeyPair()
	signer := NewPolicySigner(pub, priv, "signer")

	signed, _ := signer.Sign([]byte("original content"), "v1.0.0")

	// Tamper with bundle
	signed.Bundle = []byte("tampered content")

	err := signer.Verify(signed)
	if err == nil {
		t.Fatal("expected verification to fail on tampered bundle")
	}
}

func TestVerifyRejectsWrongKey(t *testing.T) {
	pub1, priv1, _ := GenerateKeyPair()
	pub2, _, _ := GenerateKeyPair()

	signer := NewPolicySigner(pub1, priv1, "signer1")
	verifier := NewPolicyVerifier(pub2)

	signed, _ := signer.Sign([]byte("some policy"), "v1.0.0")

	err := verifier.Verify(signed)
	if err == nil {
		t.Fatal("expected verification to fail with wrong public key")
	}
}

func TestVersionMonotonicity_AcceptsNewer(t *testing.T) {
	pub, priv, _ := GenerateKeyPair()
	signer := NewPolicySigner(pub, priv, "signer")

	signed1, _ := signer.Sign([]byte("v1"), "v2026.04.27.1")
	signer.AcceptVersion("v2026.04.27.1")

	signed2, _ := signer.Sign([]byte("v2"), "v2026.04.28.1")
	err := signer.VerifyAndCheckMonotonicity(signed2)
	if err != nil {
		t.Fatalf("newer version should be accepted: %v", err)
	}

	_ = signed1 // used above
}

func TestVersionMonotonicity_RejectsOlder(t *testing.T) {
	pub, priv, _ := GenerateKeyPair()
	signer := NewPolicySigner(pub, priv, "signer")

	signer.AcceptVersion("v2026.04.28.1")

	signed, _ := signer.Sign([]byte("old"), "v2026.04.27.1")
	err := signer.VerifyAndCheckMonotonicity(signed)
	if err == nil {
		t.Fatal("expected rejection of older version (rollback attempt)")
	}
}

func TestVersionMonotonicity_AcceptsSameVersion(t *testing.T) {
	pub, priv, _ := GenerateKeyPair()
	signer := NewPolicySigner(pub, priv, "signer")

	signer.AcceptVersion("v1.0.0")

	signed, _ := signer.Sign([]byte("same"), "v1.0.0")
	err := signer.VerifyAndCheckMonotonicity(signed)
	if err != nil {
		t.Fatalf("same version should be accepted (idempotent): %v", err)
	}
}

func TestVerifierCannotSign(t *testing.T) {
	pub, _, _ := GenerateKeyPair()
	verifier := NewPolicyVerifier(pub)

	_, err := verifier.Sign([]byte("data"), "v1.0.0")
	if err == nil {
		t.Fatal("verifier-only instance should not be able to sign")
	}
}

func TestHashBundle(t *testing.T) {
	hash1 := HashBundle([]byte("hello"))
	hash2 := HashBundle([]byte("hello"))
	hash3 := HashBundle([]byte("different"))

	if hash1 != hash2 {
		t.Error("same input should produce same hash")
	}
	if hash1 == hash3 {
		t.Error("different input should produce different hash")
	}
	if len(hash1) != 64 {
		t.Errorf("SHA-256 hex should be 64 chars, got %d", len(hash1))
	}
}
