/**
 * Author: Deepankar Das
 */

package policy

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"sync"
	"time"
)

// PolicySignature contains the cryptographic signature for a policy bundle.
type PolicySignature struct {
	BundleHash string `json:"bundle_hash" yaml:"bundle_hash"` // SHA-256 of canonical YAML
	Signature  string `json:"signature" yaml:"signature"`     // Ed25519 signature (hex)
	SignedAt   string `json:"signed_at" yaml:"signed_at"`     // RFC3339 timestamp
	SignerID   string `json:"signer_id" yaml:"signer_id"`     // Identity of the signer
	Version    string `json:"version" yaml:"version"`         // Bundle version for monotonicity
}

// SignedBundle wraps a policy bundle with its signature.
type SignedBundle struct {
	Bundle    []byte          `json:"bundle"`    // Raw YAML bytes
	Signature PolicySignature `json:"signature"` // Cryptographic signature
}

// PolicySigner creates and verifies Ed25519 signatures on policy bundles.
type PolicySigner struct {
	mu         sync.RWMutex
	publicKey  ed25519.PublicKey
	privateKey ed25519.PrivateKey
	signerID   string
	lastVersion string // For monotonicity enforcement
}

// NewPolicySigner creates a signer from an Ed25519 key pair.
func NewPolicySigner(publicKey ed25519.PublicKey, privateKey ed25519.PrivateKey, signerID string) *PolicySigner {
	return &PolicySigner{
		publicKey:  publicKey,
		privateKey: privateKey,
		signerID:   signerID,
	}
}

// NewPolicyVerifier creates a verifier with only the public key (no signing capability).
func NewPolicyVerifier(publicKey ed25519.PublicKey) *PolicySigner {
	return &PolicySigner{
		publicKey: publicKey,
	}
}

// GenerateKeyPair creates a new Ed25519 key pair for policy signing.
func GenerateKeyPair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("key generation failed: %w", err)
	}
	return pub, priv, nil
}

// HashBundle computes the SHA-256 hash of raw policy YAML bytes.
func HashBundle(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// Sign signs a policy bundle and returns the signed bundle.
func (s *PolicySigner) Sign(bundleYAML []byte, version string) (*SignedBundle, error) {
	if s.privateKey == nil {
		return nil, fmt.Errorf("no private key — cannot sign (verifier-only instance)")
	}

	bundleHash := HashBundle(bundleYAML)

	signature := ed25519.Sign(s.privateKey, []byte(bundleHash))

	return &SignedBundle{
		Bundle: bundleYAML,
		Signature: PolicySignature{
			BundleHash: bundleHash,
			Signature:  hex.EncodeToString(signature),
			SignedAt:   time.Now().UTC().Format(time.RFC3339),
			SignerID:   s.signerID,
			Version:    version,
		},
	}, nil
}

// Verify checks the signature of a signed bundle.
// Returns nil if valid, error if invalid.
func (s *PolicySigner) Verify(signed *SignedBundle) error {
	if s.publicKey == nil {
		return fmt.Errorf("no public key — cannot verify")
	}

	// Verify hash matches bundle content
	actualHash := HashBundle(signed.Bundle)
	if actualHash != signed.Signature.BundleHash {
		return fmt.Errorf("bundle hash mismatch: expected %s, got %s (bundle may have been tampered with)",
			signed.Signature.BundleHash, actualHash)
	}

	// Verify Ed25519 signature
	sigBytes, err := hex.DecodeString(signed.Signature.Signature)
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}

	if !ed25519.Verify(s.publicKey, []byte(signed.Signature.BundleHash), sigBytes) {
		return fmt.Errorf("signature verification failed — bundle was not signed by a trusted key")
	}

	return nil
}

// VerifyAndCheckMonotonicity verifies signature and ensures version is newer than last accepted.
func (s *PolicySigner) VerifyAndCheckMonotonicity(signed *SignedBundle) error {
	if err := s.Verify(signed); err != nil {
		return err
	}

	s.mu.RLock()
	lastVersion := s.lastVersion
	s.mu.RUnlock()

	// Version monotonicity: new version must be >= last accepted version (lexicographic)
	if lastVersion != "" && signed.Signature.Version < lastVersion {
		return fmt.Errorf("version monotonicity violation: new version %s < last accepted %s (rollback attempt blocked)",
			signed.Signature.Version, lastVersion)
	}

	return nil
}

// AcceptVersion records a version as accepted (for future monotonicity checks).
func (s *PolicySigner) AcceptVersion(version string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastVersion = version
}

// LoadPublicKeyFromFile loads an Ed25519 public key from a PEM file.
func LoadPublicKeyFromFile(path string) (ed25519.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading public key file: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		// Try as raw hex
		keyBytes, err := hex.DecodeString(string(data))
		if err != nil || len(keyBytes) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("invalid public key format (expected PEM or 32-byte hex)")
		}
		return ed25519.PublicKey(keyBytes), nil
	}
	if len(block.Bytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key size: expected %d bytes, got %d", ed25519.PublicKeySize, len(block.Bytes))
	}
	return ed25519.PublicKey(block.Bytes), nil
}

// LoadPrivateKeyFromFile loads an Ed25519 private key from a PEM file.
func LoadPrivateKeyFromFile(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading private key file: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		keyBytes, err := hex.DecodeString(string(data))
		if err != nil || len(keyBytes) != ed25519.PrivateKeySize {
			return nil, fmt.Errorf("invalid private key format (expected PEM or 64-byte hex)")
		}
		return ed25519.PrivateKey(keyBytes), nil
	}
	if len(block.Bytes) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key size: expected %d bytes, got %d", ed25519.PrivateKeySize, len(block.Bytes))
	}
	return ed25519.PrivateKey(block.Bytes), nil
}

// SavePublicKeyToFile writes an Ed25519 public key to a hex file.
func SavePublicKeyToFile(path string, key ed25519.PublicKey) error {
	return os.WriteFile(path, []byte(hex.EncodeToString(key)), 0644)
}

// SavePrivateKeyToFile writes an Ed25519 private key to a hex file with restricted permissions.
func SavePrivateKeyToFile(path string, key ed25519.PrivateKey) error {
	return os.WriteFile(path, []byte(hex.EncodeToString(key)), 0600)
}
