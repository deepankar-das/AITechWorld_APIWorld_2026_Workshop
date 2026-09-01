/**
 * Author: Deepankar Das
 */

package audit

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/anthropics/enforcer/internal/types"
)

func makeChainEvent(id, sessionID string) types.AuditEvent {
	now := time.Now().UTC().Format(time.RFC3339)
	return types.AuditEvent{
		EventID:   id,
		Timestamp: now,
		SessionID: sessionID,
		Who:       "test|claude_code|" + sessionID,
		What:      "file.write|test",
		When:      now,
		Policy:    "test@v1",
		Decision:  "allow",
		Result:    "executed",
	}
}

func testKeyPair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("key generation failed: %v", err)
	}
	return pub, priv
}

func TestChainEvent_CreatesValidChain(t *testing.T) {
	pub, priv := testKeyPair(t)
	chain := NewAuditChain(pub, priv, "test")

	e1 := chain.ChainEvent(makeChainEvent("evt-1", "sess-1"))
	e2 := chain.ChainEvent(makeChainEvent("evt-2", "sess-1"))
	e3 := chain.ChainEvent(makeChainEvent("evt-3", "sess-1"))

	if e1.PreviousHash != "genesis" {
		t.Errorf("first event should chain from genesis, got %s", e1.PreviousHash)
	}
	if e2.PreviousHash != e1.EventHash {
		t.Error("second event should chain from first event hash")
	}
	if e3.PreviousHash != e2.EventHash {
		t.Error("third event should chain from second event hash")
	}
	if e1.ChainIndex != 1 || e2.ChainIndex != 2 || e3.ChainIndex != 3 {
		t.Error("chain indices should be monotonic 1, 2, 3")
	}
}

func TestVerifyChain_ValidChain(t *testing.T) {
	pub, priv := testKeyPair(t)
	chain := NewAuditChain(pub, priv, "test")

	events := []ChainedEvent{
		chain.ChainEvent(makeChainEvent("evt-1", "sess-1")),
		chain.ChainEvent(makeChainEvent("evt-2", "sess-1")),
		chain.ChainEvent(makeChainEvent("evt-3", "sess-1")),
	}

	if err := VerifyChain(events); err != nil {
		t.Fatalf("valid chain should verify: %v", err)
	}
}

func TestVerifyChain_DetectsTamperedEvent(t *testing.T) {
	pub, priv := testKeyPair(t)
	chain := NewAuditChain(pub, priv, "test")

	events := []ChainedEvent{
		chain.ChainEvent(makeChainEvent("evt-1", "sess-1")),
		chain.ChainEvent(makeChainEvent("evt-2", "sess-1")),
	}

	// Tamper with second event's content
	events[1].Event.Result = "tampered"

	err := VerifyChain(events)
	if err == nil {
		t.Fatal("tampered event should break chain verification")
	}
}

func TestVerifyChain_DetectsReordering(t *testing.T) {
	pub, priv := testKeyPair(t)
	chain := NewAuditChain(pub, priv, "test")

	e1 := chain.ChainEvent(makeChainEvent("evt-1", "sess-1"))
	e2 := chain.ChainEvent(makeChainEvent("evt-2", "sess-1"))

	// Swap order
	err := VerifyChain([]ChainedEvent{e2, e1})
	if err == nil {
		t.Fatal("reordered events should break chain verification")
	}
}

func TestVerifyChain_DetectsDeletion(t *testing.T) {
	pub, priv := testKeyPair(t)
	chain := NewAuditChain(pub, priv, "test")

	e1 := chain.ChainEvent(makeChainEvent("evt-1", "sess-1"))
	chain.ChainEvent(makeChainEvent("evt-2", "sess-1")) // skip this
	e3 := chain.ChainEvent(makeChainEvent("evt-3", "sess-1"))

	// e3's previous_hash points to e2, but e2 is missing
	err := VerifyChain([]ChainedEvent{e1, e3})
	if err == nil {
		t.Fatal("deleted event should break chain verification")
	}
}

func TestSignedExport_RoundTrip(t *testing.T) {
	pub, priv := testKeyPair(t)
	chain := NewAuditChain(pub, priv, "test-signer")

	events := []ChainedEvent{
		chain.ChainEvent(makeChainEvent("evt-1", "sess-1")),
		chain.ChainEvent(makeChainEvent("evt-2", "sess-1")),
		chain.ChainEvent(makeChainEvent("evt-3", "sess-1")),
	}

	export, err := chain.SignExport(events)
	if err != nil {
		t.Fatalf("signing failed: %v", err)
	}

	if export.EventCount != 3 {
		t.Errorf("expected 3 events, got %d", export.EventCount)
	}
	if export.SignerID != "test-signer" {
		t.Errorf("expected signer_id=test-signer, got %s", export.SignerID)
	}

	// Verify the export
	if err := VerifyExport(export, pub); err != nil {
		t.Fatalf("export verification failed: %v", err)
	}
}

func TestSignedExport_DetectsTampering(t *testing.T) {
	pub, priv := testKeyPair(t)
	chain := NewAuditChain(pub, priv, "signer")

	events := []ChainedEvent{
		chain.ChainEvent(makeChainEvent("evt-1", "sess-1")),
		chain.ChainEvent(makeChainEvent("evt-2", "sess-1")),
	}

	export, _ := chain.SignExport(events)

	// Tamper with an event after signing
	export.Events[0].Event.Result = "tampered"

	err := VerifyExport(export, pub)
	if err == nil {
		t.Fatal("tampered export should fail verification")
	}
}

func TestSignedExport_RejectsWrongKey(t *testing.T) {
	pub1, priv1 := testKeyPair(t)
	pub2, _ := testKeyPair(t)
	chain := NewAuditChain(pub1, priv1, "signer")

	events := []ChainedEvent{
		chain.ChainEvent(makeChainEvent("evt-1", "sess-1")),
	}

	export, _ := chain.SignExport(events)

	// Verify with wrong public key
	err := VerifyExport(export, pub2)
	if err == nil {
		t.Fatal("export should fail verification with wrong key")
	}
}

func TestVerifyChain_EmptyChain(t *testing.T) {
	if err := VerifyChain([]ChainedEvent{}); err != nil {
		t.Fatalf("empty chain should verify: %v", err)
	}
}
