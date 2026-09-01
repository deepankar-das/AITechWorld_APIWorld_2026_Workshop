/**
 * Author: Deepankar Das
 */

package audit

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/anthropics/enforcer/internal/types"
)

// ChainedEvent wraps an audit event with a hash chain link.
// Each event's hash includes the previous event's hash, forming a tamper-evident chain.
type ChainedEvent struct {
	Event        types.AuditEvent `json:"event"`
	PreviousHash string           `json:"previous_hash"` // SHA-256 of previous chained event
	EventHash    string           `json:"event_hash"`    // SHA-256 of (event JSON + previous_hash)
	ChainIndex   int64            `json:"chain_index"`   // Monotonic sequence number
}

// SignedExport is a cryptographically signed evidence package.
type SignedExport struct {
	ExportedAt  string         `json:"exported_at"`
	EventCount  int            `json:"event_count"`
	ChainStart  string         `json:"chain_start"`  // Hash of first event in export
	ChainEnd    string         `json:"chain_end"`    // Hash of last event in export
	Events      []ChainedEvent `json:"events"`
	ExportHash  string         `json:"export_hash"`  // SHA-256 of all event hashes concatenated
	Signature   string         `json:"signature"`    // Ed25519 signature of export_hash
	SignerID    string         `json:"signer_id"`
}

// AuditChain maintains a tamper-evident hash chain of audit events.
type AuditChain struct {
	mu           sync.Mutex
	lastHash     string
	chainIndex   int64
	privateKey   ed25519.PrivateKey // For signing exports (nil if verify-only)
	publicKey    ed25519.PublicKey  // For verifying exports
	signerID     string
}

// NewAuditChain creates a new audit chain.
// If privateKey is nil, the chain can verify but not sign exports.
func NewAuditChain(publicKey ed25519.PublicKey, privateKey ed25519.PrivateKey, signerID string) *AuditChain {
	return &AuditChain{
		lastHash:   "genesis", // The first event chains from "genesis"
		publicKey:  publicKey,
		privateKey: privateKey,
		signerID:   signerID,
	}
}

// ChainEvent adds an event to the chain and returns the chained event.
// The event hash includes the previous hash, making any insertion/deletion/reordering detectable.
func (c *AuditChain) ChainEvent(event types.AuditEvent) ChainedEvent {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.chainIndex++

	// Hash = SHA-256(event_json + previous_hash)
	eventJSON, _ := json.Marshal(event)
	payload := append(eventJSON, []byte(c.lastHash)...)
	hash := sha256.Sum256(payload)
	eventHash := hex.EncodeToString(hash[:])

	chained := ChainedEvent{
		Event:        event,
		PreviousHash: c.lastHash,
		EventHash:    eventHash,
		ChainIndex:   c.chainIndex,
	}

	c.lastHash = eventHash
	return chained
}

// VerifyChain checks that a sequence of chained events forms a valid chain.
// Returns nil if valid, error describing the first detected break.
func VerifyChain(events []ChainedEvent) error {
	if len(events) == 0 {
		return nil
	}

	for i, chained := range events {
		// Recompute hash
		eventJSON, _ := json.Marshal(chained.Event)
		payload := append(eventJSON, []byte(chained.PreviousHash)...)
		hash := sha256.Sum256(payload)
		expectedHash := hex.EncodeToString(hash[:])

		if chained.EventHash != expectedHash {
			return fmt.Errorf("chain break at index %d (event %s): expected hash %s, got %s — event may have been tampered with",
				chained.ChainIndex, chained.Event.EventID, expectedHash, chained.EventHash)
		}

		// Verify chain linkage (except for first event which may chain from any predecessor)
		if i > 0 && chained.PreviousHash != events[i-1].EventHash {
			return fmt.Errorf("chain linkage break at index %d: previous_hash %s does not match prior event hash %s — event may have been inserted or reordered",
				chained.ChainIndex, chained.PreviousHash, events[i-1].EventHash)
		}

		// Verify monotonic index
		if i > 0 && chained.ChainIndex <= events[i-1].ChainIndex {
			return fmt.Errorf("chain index not monotonic at position %d: %d <= %d",
				i, chained.ChainIndex, events[i-1].ChainIndex)
		}
	}

	return nil
}

// SignExport creates a cryptographically signed evidence package from chained events.
func (c *AuditChain) SignExport(events []ChainedEvent) (*SignedExport, error) {
	if c.privateKey == nil {
		return nil, fmt.Errorf("no private key — cannot sign export")
	}

	if len(events) == 0 {
		return nil, fmt.Errorf("cannot sign empty export")
	}

	// Compute export hash = SHA-256(concatenation of all event hashes)
	var allHashes string
	for _, e := range events {
		allHashes += e.EventHash
	}
	exportHashBytes := sha256.Sum256([]byte(allHashes))
	exportHash := hex.EncodeToString(exportHashBytes[:])

	// Sign with Ed25519
	sig := ed25519.Sign(c.privateKey, []byte(exportHash))

	return &SignedExport{
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		EventCount: len(events),
		ChainStart: events[0].EventHash,
		ChainEnd:   events[len(events)-1].EventHash,
		Events:     events,
		ExportHash: exportHash,
		Signature:  hex.EncodeToString(sig),
		SignerID:   c.signerID,
	}, nil
}

// VerifyExport checks the signature and chain integrity of a signed export.
func VerifyExport(export *SignedExport, publicKey ed25519.PublicKey) error {
	// 1. Verify the chain integrity
	if err := VerifyChain(export.Events); err != nil {
		return fmt.Errorf("chain verification failed: %w", err)
	}

	// 2. Recompute export hash
	var allHashes string
	for _, e := range export.Events {
		allHashes += e.EventHash
	}
	exportHashBytes := sha256.Sum256([]byte(allHashes))
	expectedHash := hex.EncodeToString(exportHashBytes[:])

	if export.ExportHash != expectedHash {
		return fmt.Errorf("export hash mismatch: expected %s, got %s", expectedHash, export.ExportHash)
	}

	// 3. Verify Ed25519 signature
	sigBytes, err := hex.DecodeString(export.Signature)
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}

	if !ed25519.Verify(publicKey, []byte(export.ExportHash), sigBytes) {
		return fmt.Errorf("export signature verification failed — export was not signed by a trusted key")
	}

	return nil
}
