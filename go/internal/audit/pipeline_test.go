/**
 * Author: Deepankar Das
 */

package audit

import (
	"fmt"
	"testing"
	"time"

	"github.com/anthropics/enforcer/internal/types"
)

// testBuffer creates an audit buffer for testing.
func testBuffer(t *testing.T) *AuditBuffer {
	t.Helper()
	return NewAuditBuffer()
}

// --- Helpers ---

// makeValidEvent creates a fully populated audit event suitable for testing.
func makeValidEvent(eventID, sessionID, decision string) types.AuditEvent {
	return types.AuditEvent{
		EventID:       eventID,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		SessionID:     sessionID,
		CorrelationID: "corr-001",
		Who:           "user-1|claude_code|" + sessionID,
		What:          "file.write|write /src/main.go",
		When:          time.Now().UTC().Format(time.RFC3339),
		Policy:        "fs-write-policy@1.0",
		Decision:      decision,
		Result:        "enforced",
		Actor: struct {
			UserID        string `json:"user_id"`
			AgentType     string `json:"agent_type"`
			AgentInstance string `json:"agent_instance"`
		}{
			UserID:        "user-1",
			AgentType:     "claude_code",
			AgentInstance: "inst-001",
		},
		Environment: struct {
			Workspace      string `json:"workspace"`
			Repo           string `json:"repo"`
			Branch         string `json:"branch"`
			Tier           string `json:"tier"`
			DeploymentMode string `json:"deployment_mode"`
		}{
			Workspace:      "/workspace",
			Repo:           "test-repo",
			Branch:         "main",
			Tier:           "local",
			DeploymentMode: "daemon",
		},
		Action: struct {
			Type            string `json:"type"`
			AttemptedAction string `json:"attempted_action"`
			ObservedEffect  string `json:"observed_effect"`
		}{
			Type:            "file.write",
			AttemptedAction: "write /src/main.go",
			ObservedEffect:  "pending",
		},
		Resource: struct {
			Kind           string   `json:"kind"`
			Path           string   `json:"path,omitempty"`
			Host           string   `json:"host,omitempty"`
			Value          string   `json:"value,omitempty"`
			Classification []string `json:"classification"`
		}{
			Kind:           "file",
			Path:           "/src/main.go",
			Classification: []string{"safe"},
		},
		PolicyDetail: struct {
			PolicyID      string `json:"policy_id"`
			PolicyVersion string `json:"policy_version"`
			Decision      string `json:"decision"`
			ReasonCode    string `json:"reason_code"`
			ReasonHuman   string `json:"reason_human"`
		}{
			PolicyID:      "fs-write-policy",
			PolicyVersion: "1.0",
			Decision:      decision,
			ReasonCode:    "within_project_root",
			ReasonHuman:   "File is within the project root",
		},
	}
}

// makeEventWithTimestamp creates an event with a specific timestamp for ordering tests.
func makeEventWithTimestamp(eventID, sessionID, decision, ts string) types.AuditEvent {
	e := makeValidEvent(eventID, sessionID, decision)
	e.Timestamp = ts
	e.When = ts
	return e
}

// --- Test 1: ValidateAuditEvent accepts valid events ---

func TestValidateAuditEvent_AcceptsValidEvent(t *testing.T) {
	event := makeValidEvent("evt-001", "sess-001", "allow")

	valid, missing := ValidateAuditEvent(event)

	if !valid {
		t.Fatalf("expected valid=true, got valid=false with missing=%v", missing)
	}
	if len(missing) != 0 {
		t.Fatalf("expected no missing fields, got %v", missing)
	}
}

// --- Test 2: ValidateAuditEvent rejects events missing gate fields ---

func TestValidateAuditEvent_RejectsMissingGateFields(t *testing.T) {
	tests := []struct {
		name         string
		mutate       func(*types.AuditEvent)
		wantMissing  string
	}{
		{
			name:        "missing who",
			mutate:      func(e *types.AuditEvent) { e.Who = "" },
			wantMissing: "who",
		},
		{
			name:        "missing what",
			mutate:      func(e *types.AuditEvent) { e.What = "" },
			wantMissing: "what",
		},
		{
			name:        "missing when",
			mutate:      func(e *types.AuditEvent) { e.When = "" },
			wantMissing: "when",
		},
		{
			name:        "missing policy",
			mutate:      func(e *types.AuditEvent) { e.Policy = "" },
			wantMissing: "policy",
		},
		{
			name:        "missing decision",
			mutate:      func(e *types.AuditEvent) { e.Decision = "" },
			wantMissing: "decision",
		},
		{
			name:        "missing result",
			mutate:      func(e *types.AuditEvent) { e.Result = "" },
			wantMissing: "result",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := makeValidEvent("evt-miss", "sess-001", "allow")
			tt.mutate(&event)

			valid, missing := ValidateAuditEvent(event)

			if valid {
				t.Fatal("expected valid=false for event missing a gate field")
			}
			found := false
			for _, m := range missing {
				if m == tt.wantMissing {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected %q in missing fields, got %v", tt.wantMissing, missing)
			}
		})
	}

	// All fields empty at once.
	t.Run("all_fields_empty", func(t *testing.T) {
		event := types.AuditEvent{}
		valid, missing := ValidateAuditEvent(event)

		if valid {
			t.Fatal("expected valid=false for completely empty event")
		}
		if len(missing) != 6 {
			t.Fatalf("expected 6 missing fields, got %d: %v", len(missing), missing)
		}
	})
}

// --- Test 3: BuildGateFields constructs correct compound fields ---

func TestBuildGateFields_ConstructsCompoundFields(t *testing.T) {
	actor := types.Actor{
		UserID:        "dev-42",
		AgentType:     "cursor",
		AgentInstance: "inst-7",
		SessionID:     "sess-99",
	}
	action := types.ActionDetail{
		Type:            "shell.exec",
		AttemptedAction: "npm install lodash",
	}

	fields := BuildGateFields(
		actor,
		"sess-99",
		action,
		"2026-04-27T10:00:00Z",
		"pkg-install-policy",
		"2.1",
		"require_approval",
		"pending",
	)

	// Check all 6 fields are present.
	for _, name := range types.MinimumGateFields {
		if fields[name] == "" {
			t.Fatalf("expected field %q to be non-empty", name)
		}
	}

	// Verify compound formats.
	expectedWho := "dev-42|cursor|sess-99"
	if fields["who"] != expectedWho {
		t.Fatalf("who: expected %q, got %q", expectedWho, fields["who"])
	}

	expectedWhat := "shell.exec|npm install lodash"
	if fields["what"] != expectedWhat {
		t.Fatalf("what: expected %q, got %q", expectedWhat, fields["what"])
	}

	expectedWhen := "2026-04-27T10:00:00Z"
	if fields["when"] != expectedWhen {
		t.Fatalf("when: expected %q, got %q", expectedWhen, fields["when"])
	}

	expectedPolicy := "pkg-install-policy@2.1"
	if fields["policy"] != expectedPolicy {
		t.Fatalf("policy: expected %q, got %q", expectedPolicy, fields["policy"])
	}

	if fields["decision"] != "require_approval" {
		t.Fatalf("decision: expected %q, got %q", "require_approval", fields["decision"])
	}

	if fields["result"] != "pending" {
		t.Fatalf("result: expected %q, got %q", "pending", fields["result"])
	}
}

// --- Test 4: Buffer accepts and stores events ---

func TestBuffer_AcceptsAndStoresEvents(t *testing.T) {
	buf := testBuffer(t)

	event := makeValidEvent("evt-buf-1", "sess-001", "allow")
	accepted := buf.BufferEvent(event)

	if !accepted {
		t.Fatal("expected event to be accepted")
	}

	metrics := buf.GetMetrics()
	if metrics.Accepted != 1 {
		t.Fatalf("expected accepted=1, got %d", metrics.Accepted)
	}
	if metrics.BufferCount != 1 {
		t.Fatalf("expected bufferCount=1, got %d", metrics.BufferCount)
	}
	if metrics.Rejected != 0 {
		t.Fatalf("expected rejected=0, got %d", metrics.Rejected)
	}
}

// --- Test 5: Buffer rejects invalid events ---

func TestBuffer_RejectsInvalidEvents(t *testing.T) {
	buf := testBuffer(t)

	// Event with empty gate fields.
	invalidEvent := types.AuditEvent{EventID: "evt-invalid"}
	accepted := buf.BufferEvent(invalidEvent)

	if accepted {
		t.Fatal("expected invalid event to be rejected")
	}

	metrics := buf.GetMetrics()
	if metrics.Rejected != 1 {
		t.Fatalf("expected rejected=1, got %d", metrics.Rejected)
	}
	if metrics.Accepted != 0 {
		t.Fatalf("expected accepted=0, got %d", metrics.Accepted)
	}
	if metrics.BufferCount != 0 {
		t.Fatalf("expected bufferCount=0, got %d", metrics.BufferCount)
	}
}

// --- Test 6: Buffer -> Store flush cycle ---

func TestBufferToStoreFlushCycle(t *testing.T) {
	buf := testBuffer(t)
	store := NewInMemoryStore()
	flusher := NewFlushServiceWithOptions(buf, store, 1*time.Hour, 100)

	// Buffer 5 events.
	for i := 0; i < 5; i++ {
		event := makeValidEvent(fmt.Sprintf("evt-flush-%d", i), "sess-flush", "allow")
		accepted := buf.BufferEvent(event)
		if !accepted {
			t.Fatalf("event %d should have been accepted", i)
		}
	}

	// Verify buffer has 5 events.
	metrics := buf.GetMetrics()
	if metrics.BufferCount != 5 {
		t.Fatalf("expected bufferCount=5, got %d", metrics.BufferCount)
	}

	// Flush.
	stored, failed := flusher.Flush()
	if stored != 5 {
		t.Fatalf("expected stored=5, got %d", stored)
	}
	if failed != 0 {
		t.Fatalf("expected failed=0, got %d", failed)
	}

	// Verify store has 5 events.
	storeCount := store.GetCount()
	if storeCount != 5 {
		t.Fatalf("expected store count=5, got %d", storeCount)
	}

	// Verify buffer is drained (unflushed count = 0).
	metrics = buf.GetMetrics()
	if metrics.BufferCount != 0 {
		t.Fatalf("expected bufferCount=0 after flush, got %d", metrics.BufferCount)
	}

	// Second flush should be a no-op.
	stored, failed = flusher.Flush()
	if stored != 0 {
		t.Fatalf("expected stored=0 on second flush, got %d", stored)
	}
	if failed != 0 {
		t.Fatalf("expected failed=0 on second flush, got %d", failed)
	}
}

// --- Test 7: Store query by session_id ---

func TestStoreQueryBySessionID(t *testing.T) {
	store := NewInMemoryStore()

	// Store events across two sessions.
	for i := 0; i < 3; i++ {
		e := makeValidEvent(fmt.Sprintf("evt-s1-%d", i), "session-alpha", "allow")
		if err := store.StoreEvent(e); err != nil {
			t.Fatalf("failed to store event: %v", err)
		}
	}
	for i := 0; i < 2; i++ {
		e := makeValidEvent(fmt.Sprintf("evt-s2-%d", i), "session-beta", "deny")
		if err := store.StoreEvent(e); err != nil {
			t.Fatalf("failed to store event: %v", err)
		}
	}

	// Query session-alpha.
	results, err := store.QueryEvents(AuditQuery{SessionID: "session-alpha"})
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 events for session-alpha, got %d", len(results))
	}
	for _, r := range results {
		if r.SessionID != "session-alpha" {
			t.Fatalf("expected sessionID=session-alpha, got %s", r.SessionID)
		}
	}

	// Query session-beta.
	results, err = store.QueryEvents(AuditQuery{SessionID: "session-beta"})
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 events for session-beta, got %d", len(results))
	}
}

// --- Test 8: Store query by decision ---

func TestStoreQueryByDecision(t *testing.T) {
	store := NewInMemoryStore()

	decisions := []string{"allow", "deny", "allow", "require_approval", "deny"}
	for i, d := range decisions {
		e := makeValidEvent(fmt.Sprintf("evt-dec-%d", i), "sess-decisions", d)
		if err := store.StoreEvent(e); err != nil {
			t.Fatalf("failed to store event: %v", err)
		}
	}

	tests := []struct {
		decision string
		wantLen  int
	}{
		{"allow", 2},
		{"deny", 2},
		{"require_approval", 1},
	}

	for _, tt := range tests {
		t.Run(tt.decision, func(t *testing.T) {
			results, err := store.QueryEvents(AuditQuery{Decision: tt.decision})
			if err != nil {
				t.Fatalf("query error: %v", err)
			}
			if len(results) != tt.wantLen {
				t.Fatalf("expected %d events for decision=%s, got %d", tt.wantLen, tt.decision, len(results))
			}
			for _, r := range results {
				if r.Decision != tt.decision {
					t.Fatalf("expected decision=%s, got %s", tt.decision, r.Decision)
				}
			}
		})
	}
}

// --- Test 9: Session replay returns chronological events ---

func TestSessionReplayChronological(t *testing.T) {
	store := NewInMemoryStore()

	// Insert events out of chronological order.
	timestamps := []string{
		"2026-04-27T10:03:00Z",
		"2026-04-27T10:01:00Z",
		"2026-04-27T10:05:00Z",
		"2026-04-27T10:02:00Z",
		"2026-04-27T10:04:00Z",
	}

	for i, ts := range timestamps {
		e := makeEventWithTimestamp(fmt.Sprintf("evt-chrono-%d", i), "sess-replay", "allow", ts)
		if err := store.StoreEvent(e); err != nil {
			t.Fatalf("failed to store event: %v", err)
		}
	}

	// GetSession should return events in chronological order.
	events, err := store.GetSession("sess-replay")
	if err != nil {
		t.Fatalf("GetSession error: %v", err)
	}
	if len(events) != 5 {
		t.Fatalf("expected 5 events, got %d", len(events))
	}

	for i := 1; i < len(events); i++ {
		if events[i].Timestamp < events[i-1].Timestamp {
			t.Fatalf("events not in chronological order: events[%d].Timestamp=%s < events[%d].Timestamp=%s",
				i, events[i].Timestamp, i-1, events[i-1].Timestamp)
		}
	}

	// Verify exact expected order.
	expectedOrder := []string{
		"2026-04-27T10:01:00Z",
		"2026-04-27T10:02:00Z",
		"2026-04-27T10:03:00Z",
		"2026-04-27T10:04:00Z",
		"2026-04-27T10:05:00Z",
	}
	for i, ts := range expectedOrder {
		if events[i].Timestamp != ts {
			t.Fatalf("events[%d]: expected timestamp=%s, got %s", i, ts, events[i].Timestamp)
		}
	}
}

// --- Test 10: Export includes metadata ---

func TestExportIncludesMetadata(t *testing.T) {
	store := NewInMemoryStore()

	for i := 0; i < 3; i++ {
		e := makeValidEvent(fmt.Sprintf("evt-export-%d", i), "sess-export", "allow")
		if err := store.StoreEvent(e); err != nil {
			t.Fatalf("failed to store event: %v", err)
		}
	}

	result, err := store.ExportEvents(AuditQuery{SessionID: "sess-export"})
	if err != nil {
		t.Fatalf("export error: %v", err)
	}

	if result.Metadata.EventCount != 3 {
		t.Fatalf("expected metadata.event_count=3, got %d", result.Metadata.EventCount)
	}
	if result.Metadata.ExportedAt == "" {
		t.Fatal("expected metadata.exported_at to be non-empty")
	}
	if result.Metadata.Query == "" {
		t.Fatal("expected metadata.query to be non-empty")
	}
	if len(result.Events) != 3 {
		t.Fatalf("expected 3 exported events, got %d", len(result.Events))
	}
}

// --- Test 11: Append-only (count only increases) ---

func TestAppendOnly_CountOnlyIncreases(t *testing.T) {
	store := NewInMemoryStore()

	counts := make([]int, 0, 10)

	for i := 0; i < 10; i++ {
		e := makeValidEvent(fmt.Sprintf("evt-append-%d", i), "sess-append", "allow")
		if err := store.StoreEvent(e); err != nil {
			t.Fatalf("failed to store event %d: %v", i, err)
		}
		count := store.GetCount()
		counts = append(counts, count)
	}

	// Verify count is strictly increasing.
	for i := 1; i < len(counts); i++ {
		if counts[i] <= counts[i-1] {
			t.Fatalf("count did not increase: counts[%d]=%d <= counts[%d]=%d",
				i, counts[i], i-1, counts[i-1])
		}
	}

	// Verify final count matches expected.
	if store.GetCount() != 10 {
		t.Fatalf("expected final count=10, got %d", store.GetCount())
	}

	// Verify that the AuditStore interface has no Delete method by attempting
	// operations: store events and confirm the count never decreases.
	for i := 0; i < 5; i++ {
		e := makeValidEvent(fmt.Sprintf("evt-append-extra-%d", i), "sess-append", "deny")
		if err := store.StoreEvent(e); err != nil {
			t.Fatalf("failed to store extra event %d: %v", i, err)
		}
	}

	finalCount := store.GetCount()
	if finalCount != 15 {
		t.Fatalf("expected final count=15, got %d", finalCount)
	}
}

// --- Test 12: Append-only enrichment (no mutation of stored events) ---

func TestAppendOnlyEnrichment(t *testing.T) {
	store := NewInMemoryStore()

	// Store an event with pending observed_effect
	event := makeValidEvent("evt-pending-1", "sess-enrich", "allow")
	event.Action.ObservedEffect = "pending"
	event.CorrelationID = "evt-pending-1"

	if err := store.StoreEvent(event); err != nil {
		t.Fatalf("failed to store event: %v", err)
	}

	// Create an enrichment event linked by correlation_id (append-only)
	enrichment := makeValidEvent("enr-1", "sess-enrich", "enrichment")
	enrichment.CorrelationID = "evt-pending-1" // Links to original
	enrichment.Action.Type = "enrichment"
	enrichment.Action.ObservedEffect = "file_written_successfully"
	enrichment.What = "enrichment:Write"
	enrichment.Result = "file_written_successfully"

	if err := store.StoreEvent(enrichment); err != nil {
		t.Fatalf("failed to store enrichment event: %v", err)
	}

	// Verify the original event is NOT mutated
	events, err := store.GetSession("sess-enrich")
	if err != nil {
		t.Fatalf("GetSession error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events (original + enrichment), got %d", len(events))
	}

	// Original event still has "pending" — immutable
	if events[0].Action.ObservedEffect != "pending" {
		t.Errorf("original event should still be pending, got %s", events[0].Action.ObservedEffect)
	}

	// Enrichment event has the actual outcome
	if events[1].Action.ObservedEffect != "file_written_successfully" {
		t.Errorf("enrichment event should have actual effect, got %s", events[1].Action.ObservedEffect)
	}

	// Enrichment links to original via correlation_id
	if events[1].CorrelationID != "evt-pending-1" {
		t.Errorf("enrichment correlation_id should link to original, got %s", events[1].CorrelationID)
	}

	// Total count increased (append-only)
	count := store.GetCount()
	if count != 2 {
		t.Errorf("expected count=2, got %d", count)
	}
}
