/**
 * Author: Deepankar Das
 */

package approval

import (
	"sync"
	"testing"
	"time"

	"github.com/anthropics/enforcer/internal/types"
)

func testRequest() types.ActionRequest {
	return types.ActionRequest{
		RequestID: "req-001",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Actor: types.Actor{
			UserID:        "dev_001",
			AgentType:     "claude_code",
			AgentInstance: "inst-1",
			SessionID:     "sess-001",
		},
		Environment: types.Environment{
			Workspace: "/Users/dev/project",
		},
		Action: types.ActionDetail{
			Type:            types.ActionPackageInstall,
			AttemptedAction: "npm install lodash",
		},
		Resource: types.Resource{
			Kind:  types.ResourceCommand,
			Value: "npm install lodash",
		},
	}
}

func testDecision() types.PolicyDecision {
	return types.PolicyDecision{
		RequestID:        "req-001",
		Decision:         types.DecisionRequireApproval,
		ReasonCode:       "PACKAGE_INSTALL",
		ReasonHuman:      "Package install requires approval",
		PolicyID:         "org.pkg_install",
		PolicyVersion:    "v1.0.0",
		ApprovalRequired: true,
	}
}

// 1. Create approval returns request and channel.
func TestCreateApproval_ReturnsRequestAndChannel(t *testing.T) {
	svc := NewApprovalService(5000, types.TimeoutDeny)
	req := testRequest()
	dec := testDecision()

	approvalReq, ch := svc.CreateApproval(req, dec)

	if approvalReq == nil {
		t.Fatal("expected non-nil approval request")
	}
	if approvalReq.ApprovalID == "" {
		t.Error("expected non-empty approval_id")
	}
	if approvalReq.RequestID != req.RequestID {
		t.Errorf("expected request_id=%s, got %s", req.RequestID, approvalReq.RequestID)
	}
	if ch == nil {
		t.Fatal("expected non-nil channel")
	}

	// Clean up: resolve to stop timer.
	_ = svc.ResolveApproval(approvalReq.ApprovalID, types.ApprovalDecision{Decision: "deny"})
}

// 2. Resolve with approve sends decision on channel.
func TestResolveApproval_ApproveSendsDecision(t *testing.T) {
	svc := NewApprovalService(5000, types.TimeoutDeny)
	req := testRequest()
	dec := testDecision()

	approvalReq, ch := svc.CreateApproval(req, dec)

	err := svc.ResolveApproval(approvalReq.ApprovalID, types.ApprovalDecision{
		Decision:   "approve",
		ApproverID: "admin_001",
		Rationale:  "Looks safe",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case result := <-ch:
		if result.Decision != "approve" {
			t.Errorf("expected approve, got %s", result.Decision)
		}
		if result.ApproverID != "admin_001" {
			t.Errorf("expected approver_id=admin_001, got %s", result.ApproverID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for decision on channel")
	}
}

// 3. Resolve with deny sends decision on channel.
func TestResolveApproval_DenySendsDecision(t *testing.T) {
	svc := NewApprovalService(5000, types.TimeoutDeny)
	req := testRequest()
	dec := testDecision()

	approvalReq, ch := svc.CreateApproval(req, dec)

	err := svc.ResolveApproval(approvalReq.ApprovalID, types.ApprovalDecision{
		Decision:   "deny",
		ApproverID: "admin_002",
		Rationale:  "Not authorized",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case result := <-ch:
		if result.Decision != "deny" {
			t.Errorf("expected deny, got %s", result.Decision)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for decision on channel")
	}
}

// 4. Auto-deny on timeout.
func TestApproval_AutoDenyOnTimeout(t *testing.T) {
	// Use a very short timeout (100ms stored as the timeout value).
	svc := NewApprovalService(1, types.TimeoutDeny)
	req := testRequest()
	dec := testDecision()

	_, ch := svc.CreateApproval(req, dec)

	select {
	case result := <-ch:
		if result.Decision != "deny" {
			t.Errorf("expected deny on timeout, got %s", result.Decision)
		}
		if result.ApproverID != "system:timeout" {
			t.Errorf("expected system:timeout approver, got %s", result.ApproverID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for timeout decision")
	}
}

// 5. Auto-allow on timeout when configured.
func TestApproval_AutoAllowOnTimeout(t *testing.T) {
	svc := NewApprovalService(1, types.TimeoutAllow)
	req := testRequest()
	dec := testDecision()

	_, ch := svc.CreateApproval(req, dec)

	select {
	case result := <-ch:
		if result.Decision != "approve" {
			t.Errorf("expected approve on timeout, got %s", result.Decision)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for timeout decision")
	}
}

// 6. GetPending returns all pending.
func TestGetPending_ReturnsAllPending(t *testing.T) {
	svc := NewApprovalService(5000, types.TimeoutDeny)
	req1 := testRequest()
	req1.RequestID = "req-a"
	req2 := testRequest()
	req2.RequestID = "req-b"
	dec := testDecision()

	ar1, _ := svc.CreateApproval(req1, dec)
	ar2, _ := svc.CreateApproval(req2, dec)

	pending := svc.GetPending()
	if len(pending) != 2 {
		t.Errorf("expected 2 pending, got %d", len(pending))
	}

	// Clean up.
	_ = svc.ResolveApproval(ar1.ApprovalID, types.ApprovalDecision{Decision: "deny"})
	_ = svc.ResolveApproval(ar2.ApprovalID, types.ApprovalDecision{Decision: "deny"})
}

// 7. GetApproval retrieves by ID.
func TestGetApproval_RetrievesByID(t *testing.T) {
	svc := NewApprovalService(5000, types.TimeoutDeny)
	req := testRequest()
	dec := testDecision()

	approvalReq, _ := svc.CreateApproval(req, dec)

	entry := svc.GetApproval(approvalReq.ApprovalID)
	if entry == nil {
		t.Fatal("expected to find pending entry")
	}
	if entry.Request.ApprovalID != approvalReq.ApprovalID {
		t.Errorf("expected approval_id=%s, got %s", approvalReq.ApprovalID, entry.Request.ApprovalID)
	}

	// Non-existent ID.
	missing := svc.GetApproval("nonexistent")
	if missing != nil {
		t.Error("expected nil for non-existent approval")
	}

	// Clean up.
	_ = svc.ResolveApproval(approvalReq.ApprovalID, types.ApprovalDecision{Decision: "deny"})
}

// 8. Metrics track created/approved/denied/expired.
func TestGetMetrics_TracksCounters(t *testing.T) {
	svc := NewApprovalService(1, types.TimeoutDeny)
	req := testRequest()
	dec := testDecision()

	// Create and approve one.
	ar1, _ := svc.CreateApproval(req, dec)
	_ = svc.ResolveApproval(ar1.ApprovalID, types.ApprovalDecision{Decision: "approve", ApproverID: "a"})

	// Create and deny one.
	req.RequestID = "req-002"
	ar2, _ := svc.CreateApproval(req, dec)
	_ = svc.ResolveApproval(ar2.ApprovalID, types.ApprovalDecision{Decision: "deny", ApproverID: "a"})

	// Create one and let it timeout.
	req.RequestID = "req-003"
	_, ch := svc.CreateApproval(req, dec)
	<-ch // Wait for timeout.

	metrics := svc.GetMetrics()
	if metrics.TotalCreated != 3 {
		t.Errorf("expected total_created=3, got %d", metrics.TotalCreated)
	}
	if metrics.TotalApproved != 1 {
		t.Errorf("expected total_approved=1, got %d", metrics.TotalApproved)
	}
	if metrics.TotalDenied != 1 {
		t.Errorf("expected total_denied=1, got %d", metrics.TotalDenied)
	}
	if metrics.TotalExpired != 1 {
		t.Errorf("expected total_expired=1, got %d", metrics.TotalExpired)
	}
	if metrics.PendingCount != 0 {
		t.Errorf("expected pending_count=0, got %d", metrics.PendingCount)
	}
}

// 9. Event listener receives approval_created.
func TestEventListener_ReceivesCreated(t *testing.T) {
	svc := NewApprovalService(5000, types.TimeoutDeny)

	var mu sync.Mutex
	var events []ApprovalEvent
	svc.OnEvent(func(e ApprovalEvent) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	})

	req := testRequest()
	dec := testDecision()
	ar, _ := svc.CreateApproval(req, dec)

	mu.Lock()
	found := false
	for _, e := range events {
		if e.Type == EventApprovalCreated && e.ApprovalID == ar.ApprovalID {
			found = true
		}
	}
	mu.Unlock()

	if !found {
		t.Error("expected to receive approval_created event")
	}

	// Clean up.
	_ = svc.ResolveApproval(ar.ApprovalID, types.ApprovalDecision{Decision: "deny"})
}

// 10. Event listener receives approval_resolved.
func TestEventListener_ReceivesResolved(t *testing.T) {
	svc := NewApprovalService(5000, types.TimeoutDeny)

	var mu sync.Mutex
	var events []ApprovalEvent
	svc.OnEvent(func(e ApprovalEvent) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	})

	req := testRequest()
	dec := testDecision()
	ar, _ := svc.CreateApproval(req, dec)

	_ = svc.ResolveApproval(ar.ApprovalID, types.ApprovalDecision{
		Decision:   "approve",
		ApproverID: "admin",
	})

	mu.Lock()
	found := false
	for _, e := range events {
		if e.Type == EventApprovalResolved && e.ApprovalID == ar.ApprovalID {
			found = true
		}
	}
	mu.Unlock()

	if !found {
		t.Error("expected to receive approval_resolved event")
	}
}

// 11. Scope matching - session scope auto-approves.
func TestScopeMatching_SessionScopeAutoApproves(t *testing.T) {
	svc := NewApprovalService(5000, types.TimeoutDeny)
	req := testRequest()
	dec := testDecision()

	ar, _ := svc.CreateApproval(req, dec)

	// Resolve with a session scope.
	_ = svc.ResolveApproval(ar.ApprovalID, types.ApprovalDecision{
		Decision:   "approve",
		ApproverID: "admin",
		Scope: &types.ApprovalScope{
			Type:    types.ScopeSession,
			Pattern: string(types.ActionPackageInstall),
		},
	})

	// A new request with the same action type should match the scope.
	req2 := testRequest()
	req2.RequestID = "req-002"
	scopeDecision := svc.CheckScope(req2)
	if scopeDecision == nil {
		t.Fatal("expected scope match to return a decision")
	}
	if scopeDecision.Decision != "approve" {
		t.Errorf("expected approve from scope, got %s", scopeDecision.Decision)
	}
}

// 12. Break-glass creates valid decision.
func TestBreakGlass_CreatesValidDecision(t *testing.T) {
	decision, err := RequestBreakGlass("apr-123", "admin_001", "Production incident requires immediate access")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.ApprovalID != "apr-123" {
		t.Errorf("expected approval_id=apr-123, got %s", decision.ApprovalID)
	}
	if decision.Decision != "approve" {
		t.Errorf("expected approve, got %s", decision.Decision)
	}
	if !decision.IsBreakGlass {
		t.Error("expected is_break_glass=true")
	}
	if decision.ApproverID != "admin_001" {
		t.Errorf("expected approver_id=admin_001, got %s", decision.ApproverID)
	}
}

// 13. Break-glass requires non-empty rationale.
func TestBreakGlass_RequiresRationale(t *testing.T) {
	_, err := RequestBreakGlass("apr-123", "admin_001", "")
	if err == nil {
		t.Fatal("expected error for empty rationale")
	}
}

// 14. Resolve unknown approval returns error.
func TestResolveApproval_UnknownIDReturnsError(t *testing.T) {
	svc := NewApprovalService(5000, types.TimeoutDeny)
	err := svc.ResolveApproval("nonexistent-id", types.ApprovalDecision{Decision: "approve"})
	if err == nil {
		t.Fatal("expected error for unknown approval ID")
	}
}

// 15. Single-use scope matches once in MatchesScope, consumed by CheckScope.
func TestScopeMatching_SingleUseScopeMatchesOnce(t *testing.T) {
	// Single-use scopes match in MatchesScope (pattern check).  They are
	// consumed by CheckScope after first use so they cannot match again.
	req := testRequest()
	scope := types.ApprovalScope{
		Type:    types.ScopeSingle,
		Pattern: string(types.ActionPackageInstall),
	}
	if !MatchesScope(req, scope) {
		t.Error("single-use scope should match in MatchesScope (consumed later by CheckScope)")
	}
}

// 15b. Single-use scope is consumed after first CheckScope use.
func TestScopeMatching_SingleUseScopeConsumedAfterFirstUse(t *testing.T) {
	svc := NewApprovalService(300, types.TimeoutDeny)

	// Create and resolve an approval with a single-use scope.
	req := testRequest()
	decision := types.PolicyDecision{
		RequestID:   "test-req",
		Decision:    types.DecisionRequireApproval,
		ReasonHuman: "test",
		PolicyID:    "test-policy",
	}
	approvalReq, _ := svc.CreateApproval(req, decision)
	svc.ResolveApproval(approvalReq.ApprovalID, types.ApprovalDecision{
		Decision:   "approve",
		ApproverID: "admin",
		Scope: &types.ApprovalScope{
			Type:    types.ScopeSingle,
			Pattern: string(types.ActionPackageInstall),
		},
	})

	// First CheckScope should find the scope.
	firstCheck := svc.CheckScope(req)
	if firstCheck == nil {
		t.Fatal("first CheckScope should return the pre-approval")
	}

	// Second CheckScope should NOT find it (consumed).
	secondCheck := svc.CheckScope(req)
	if secondCheck != nil {
		t.Fatal("second CheckScope should return nil — single-use scope was consumed")
	}
}

// 16. Time-bounded scope expires.
func TestScopeMatching_TimeBoundedScopeExpires(t *testing.T) {
	req := testRequest()

	// Scope that expired in the past.
	expired := types.ApprovalScope{
		Type:    types.ScopeTimeBounded,
		Pattern: string(types.ActionPackageInstall),
		Expiry:  "2020-01-01T00:00:00Z",
	}
	if MatchesScope(req, expired) {
		t.Error("expired scope should not match")
	}

	// Scope that expires in the future.
	future := types.ApprovalScope{
		Type:    types.ScopeTimeBounded,
		Pattern: string(types.ActionPackageInstall),
		Expiry:  "2099-12-31T23:59:59Z",
	}
	if !MatchesScope(req, future) {
		t.Error("future scope should match")
	}
}

// 17. Session scope with empty pattern matches all.
func TestScopeMatching_EmptyPatternMatchesAll(t *testing.T) {
	req := testRequest()
	scope := types.ApprovalScope{
		Type:    types.ScopeSession,
		Pattern: "",
	}
	if !MatchesScope(req, scope) {
		t.Error("empty pattern should match all requests")
	}
}

// 18. CreateApproval increments pending count.
func TestCreateApproval_IncrementsPendingCount(t *testing.T) {
	svc := NewApprovalService(5000, types.TimeoutDeny)
	req := testRequest()
	dec := testDecision()

	ar, _ := svc.CreateApproval(req, dec)

	metrics := svc.GetMetrics()
	if metrics.PendingCount != 1 {
		t.Errorf("expected pending_count=1, got %d", metrics.PendingCount)
	}

	// Clean up.
	_ = svc.ResolveApproval(ar.ApprovalID, types.ApprovalDecision{Decision: "deny"})

	metrics = svc.GetMetrics()
	if metrics.PendingCount != 0 {
		t.Errorf("expected pending_count=0 after resolve, got %d", metrics.PendingCount)
	}
}

// 19. Break-glass requires non-empty approver_id.
func TestBreakGlass_RequiresApproverID(t *testing.T) {
	_, err := RequestBreakGlass("apr-123", "", "some rationale")
	if err == nil {
		t.Fatal("expected error for empty approver_id")
	}
}

// 20. Resolve approval with scope registers scope for future matching.
func TestResolveApproval_WithScopeRegistersScope(t *testing.T) {
	svc := NewApprovalService(5000, types.TimeoutDeny)
	req := testRequest()
	dec := testDecision()

	ar, _ := svc.CreateApproval(req, dec)

	_ = svc.ResolveApproval(ar.ApprovalID, types.ApprovalDecision{
		Decision:   "approve",
		ApproverID: "admin",
		Scope: &types.ApprovalScope{
			Type:    types.ScopeSession,
			Pattern: "package",
		},
	})

	// A request with action type starting with "package" should match.
	req2 := testRequest()
	req2.RequestID = "req-002"
	scopeDec := svc.CheckScope(req2)
	if scopeDec == nil {
		t.Fatal("expected scope match after approval with scope")
	}
	if scopeDec.Decision != "approve" {
		t.Errorf("expected approve from scope, got %s", scopeDec.Decision)
	}
}
