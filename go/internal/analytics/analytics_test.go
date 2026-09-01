/**
 * Author: Deepankar Das
 */

package analytics

import (
	"fmt"
	"testing"
	"time"

	"github.com/anthropics/enforcer/internal/audit"
	"github.com/anthropics/enforcer/internal/types"
)

// --- Test helpers ---

// makeEvent creates a valid audit event with the given parameters.
func makeEvent(id, userID, agentType, actionType, decision, reasonCode, policyID string) types.AuditEvent {
	now := time.Now().UTC().Format(time.RFC3339)
	return types.AuditEvent{
		EventID:   id,
		Timestamp: now,
		SessionID: "sess-" + userID,
		Who:       fmt.Sprintf("%s|%s|sess-%s", userID, agentType, userID),
		What:      fmt.Sprintf("%s|test-action", actionType),
		When:      now,
		Policy:    fmt.Sprintf("%s@v1", policyID),
		Decision:  decision,
		Result:    "executed",
		Actor: struct {
			UserID        string `json:"user_id"`
			AgentType     string `json:"agent_type"`
			AgentInstance string `json:"agent_instance"`
		}{
			UserID:        userID,
			AgentType:     agentType,
			AgentInstance: "inst-1",
		},
		Action: struct {
			Type            string `json:"type"`
			AttemptedAction string `json:"attempted_action"`
			ObservedEffect  string `json:"observed_effect"`
		}{
			Type:            actionType,
			AttemptedAction: "test-action",
		},
		Resource: struct {
			Kind           string   `json:"kind"`
			Path           string   `json:"path,omitempty"`
			Host           string   `json:"host,omitempty"`
			Value          string   `json:"value,omitempty"`
			Classification []string `json:"classification"`
		}{
			Kind: "file",
		},
		PolicyDetail: struct {
			PolicyID      string `json:"policy_id"`
			PolicyVersion string `json:"policy_version"`
			Decision      string `json:"decision"`
			ReasonCode    string `json:"reason_code"`
			ReasonHuman   string `json:"reason_human"`
		}{
			PolicyID:      policyID,
			PolicyVersion: "v1",
			Decision:      decision,
			ReasonCode:    reasonCode,
			ReasonHuman:   "Test reason",
		},
	}
}

// makeEventAt creates an event with a specific timestamp.
func makeEventAt(id, userID, agentType, actionType, decision, reasonCode, policyID string, ts time.Time) types.AuditEvent {
	e := makeEvent(id, userID, agentType, actionType, decision, reasonCode, policyID)
	tsStr := ts.Format(time.RFC3339)
	e.Timestamp = tsStr
	e.When = tsStr
	return e
}

// makeApprovalEvent creates an event with approval metadata.
func makeApprovalEvent(id, userID, actionType, status string, requestedAt, resolvedAt time.Time) types.AuditEvent {
	e := makeEvent(id, userID, "claude_code", actionType, string(types.DecisionRequireApproval), "REQUIRES_APPROVAL", "policy-approval")

	reqStr := requestedAt.Format(time.RFC3339)
	resStr := resolvedAt.Format(time.RFC3339)

	e.Approval = &types.ApprovalRecord{
		Status:      types.ApprovalStatus(status),
		RequestedAt: reqStr,
		ResolvedAt:  resStr,
	}
	return e
}

// makeEventWithHost creates an event with a network host set.
func makeEventWithHost(id, userID, actionType, decision, reasonCode, host string) types.AuditEvent {
	e := makeEvent(id, userID, "claude_code", actionType, decision, reasonCode, "policy-net")
	e.Resource.Host = host
	e.Resource.Kind = "host"
	return e
}

// makeEventWithClassification creates an event with resource classifications.
func makeEventWithClassification(id, userID, actionType, decision string, classifications []string) types.AuditEvent {
	e := makeEvent(id, userID, "claude_code", actionType, decision, "TEST_REASON", "policy-1")
	e.Resource.Classification = classifications
	return e
}

// seedStore creates an InMemoryStore and populates it with the given events.
func seedStore(t *testing.T, events []types.AuditEvent) *audit.InMemoryStore {
	t.Helper()
	store := audit.NewInMemoryStore()
	for _, e := range events {
		if err := store.StoreEvent(e); err != nil {
			t.Fatalf("failed to seed event %s: %v", e.EventID, err)
		}
	}
	return store
}

// --- Tests ---

func TestGetBlockedOperations_StackRankedByCount(t *testing.T) {
	now := time.Now().UTC()

	events := []types.AuditEvent{
		// 3 blocks for file.write / PATH_OUTSIDE_PROJECT_ROOT
		makeEventAt("e1", "alice", "claude_code", "file.write", "deny", "PATH_OUTSIDE_PROJECT_ROOT", "pol-fs", now.Add(-1*time.Hour)),
		makeEventAt("e2", "alice", "claude_code", "file.write", "deny", "PATH_OUTSIDE_PROJECT_ROOT", "pol-fs", now.Add(-2*time.Hour)),
		makeEventAt("e3", "bob", "claude_code", "file.write", "deny", "PATH_OUTSIDE_PROJECT_ROOT", "pol-fs", now.Add(-3*time.Hour)),
		// 1 block for network.request / HOST_NOT_ALLOWLISTED
		makeEventAt("e4", "alice", "claude_code", "network.request", "deny", "HOST_NOT_ALLOWLISTED", "pol-net", now.Add(-1*time.Hour)),
		// 2 blocks for shell.exec / COMMAND_BLOCKED
		makeEventAt("e5", "charlie", "cursor", "shell.exec", "deny", "COMMAND_BLOCKED", "pol-sh", now.Add(-1*time.Hour)),
		makeEventAt("e6", "charlie", "cursor", "shell.exec", "deny", "COMMAND_BLOCKED", "pol-sh", now.Add(-2*time.Hour)),
	}

	store := seedStore(t, events)

	result, err := GetBlockedOperations(store, "7d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(result))
	}

	// First should be file.write with count 3.
	if result[0].ActionType != "file.write" {
		t.Errorf("expected first result action_type=file.write, got %s", result[0].ActionType)
	}
	if result[0].Count != 3 {
		t.Errorf("expected first result count=3, got %d", result[0].Count)
	}
	if result[0].ReasonCode != "PATH_OUTSIDE_PROJECT_ROOT" {
		t.Errorf("expected reason_code=PATH_OUTSIDE_PROJECT_ROOT, got %s", result[0].ReasonCode)
	}

	// Second should be shell.exec with count 2.
	if result[1].ActionType != "shell.exec" {
		t.Errorf("expected second result action_type=shell.exec, got %s", result[1].ActionType)
	}
	if result[1].Count != 2 {
		t.Errorf("expected second result count=2, got %d", result[1].Count)
	}

	// Third should be network.request with count 1.
	if result[2].ActionType != "network.request" {
		t.Errorf("expected third result action_type=network.request, got %s", result[2].ActionType)
	}
	if result[2].Count != 1 {
		t.Errorf("expected third result count=1, got %d", result[2].Count)
	}
}

func TestGetBlockedOperations_FiltersByPeriod(t *testing.T) {
	now := time.Now().UTC()

	events := []types.AuditEvent{
		// Event within the last 7 days.
		makeEventAt("e1", "alice", "claude_code", "file.write", "deny", "PATH_OUTSIDE", "pol-1", now.Add(-24*time.Hour)),
		// Event 10 days ago -- outside 7d window.
		makeEventAt("e2", "bob", "claude_code", "file.write", "deny", "PATH_OUTSIDE", "pol-1", now.Add(-10*24*time.Hour)),
	}

	store := seedStore(t, events)

	result, err := GetBlockedOperations(store, "7d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 group for 7d period, got %d", len(result))
	}
	if result[0].Count != 1 {
		t.Errorf("expected count=1 (only recent event), got %d", result[0].Count)
	}
}

func TestClassifyDeveloper_PowerBuilder(t *testing.T) {
	features := DeveloperFeatures{
		UserID:        "power-dev",
		ActionsPerDay: 250,
		BlockRate:     1.5,
		TenureDays:    180,
		TopActionType: "file.write",
	}

	group := ClassifyDeveloper(features)
	if group != GroupPowerBuilder {
		t.Errorf("expected %s, got %s", GroupPowerBuilder, group)
	}
}

func TestClassifyDeveloper_BoundaryTester(t *testing.T) {
	features := DeveloperFeatures{
		UserID:        "boundary-dev",
		ActionsPerDay: 50,
		BlockRate:     8.0,
		EvasionScore:  5,
		TenureDays:    90,
		TopActionType: "shell.exec",
	}

	group := ClassifyDeveloper(features)
	if group != GroupBoundaryTester {
		t.Errorf("expected %s, got %s", GroupBoundaryTester, group)
	}
}

func TestClassifyDeveloper_NewJoiner(t *testing.T) {
	features := DeveloperFeatures{
		UserID:        "new-dev",
		ActionsPerDay: 30,
		BlockRate:     3.0,
		TenureDays:    15,
		TopActionType: "file.read",
	}

	group := ClassifyDeveloper(features)
	if group != GroupNewJoiner {
		t.Errorf("expected %s, got %s", GroupNewJoiner, group)
	}
}

func TestClassifyDeveloper_CautiousContributor(t *testing.T) {
	features := DeveloperFeatures{
		UserID:        "cautious-dev",
		ActionsPerDay: 35,
		BlockRate:     0,
		TenureDays:    120,
		TopActionType: "file.write",
	}

	group := ClassifyDeveloper(features)
	if group != GroupCautiousContributor {
		t.Errorf("expected %s, got %s", GroupCautiousContributor, group)
	}
}

func TestClassifyDeveloper_DormantAgent(t *testing.T) {
	features := DeveloperFeatures{
		UserID:        "dormant-dev",
		ActionsPerDay: 0.5,
		BlockRate:     0,
		TenureDays:    90,
		TopActionType: "file.read",
	}

	group := ClassifyDeveloper(features)
	if group != GroupDormantAgent {
		t.Errorf("expected %s, got %s", GroupDormantAgent, group)
	}
}

func TestGetGroups_ReturnsAll10Groups(t *testing.T) {
	// Seed a minimal store so GetGroups can run.
	now := time.Now().UTC()
	events := []types.AuditEvent{
		makeEventAt("e1", "alice", "claude_code", "file.write", "allow", "", "pol-1", now.Add(-1*time.Hour)),
	}
	store := seedStore(t, events)

	groups, err := GetGroups(store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(groups) != 10 {
		t.Fatalf("expected 10 groups, got %d", len(groups))
	}

	// Verify all expected group IDs are present.
	expectedIDs := map[string]bool{
		GroupPowerBuilder:        true,
		GroupCautiousContributor: true,
		GroupToolExplorer:        true,
		GroupBoundaryTester:      true,
		GroupAutomationDriver:    true,
		GroupDataAccessor:        true,
		GroupNetworkHeavy:        true,
		GroupNightOwl:            true,
		GroupNewJoiner:           true,
		GroupDormantAgent:        true,
	}

	for _, g := range groups {
		if !expectedIDs[g.ID] {
			t.Errorf("unexpected group ID: %s", g.ID)
		}
		delete(expectedIDs, g.ID)
		// Each group must have metadata.
		if g.Name == "" {
			t.Errorf("group %s has empty name", g.ID)
		}
		if g.Description == "" {
			t.Errorf("group %s has empty description", g.ID)
		}
	}

	if len(expectedIDs) > 0 {
		for id := range expectedIDs {
			t.Errorf("missing expected group: %s", id)
		}
	}
}

func TestGenerateRecommendations_AutoApprove(t *testing.T) {
	now := time.Now().UTC()

	// Create events where >80% of approval events for package.install are
	// approved in <10 seconds.
	var events []types.AuditEvent

	for i := 0; i < 10; i++ {
		reqAt := now.Add(-time.Duration(i+1) * time.Hour)
		resAt := reqAt.Add(5 * time.Second) // approved in 5 seconds

		e := makeApprovalEvent(
			fmt.Sprintf("e-fast-%d", i),
			"alice",
			"package.install",
			string(types.ApprovalApproved),
			reqAt,
			resAt,
		)
		e.Timestamp = reqAt.Format(time.RFC3339)
		e.When = reqAt.Format(time.RFC3339)
		events = append(events, e)
	}

	// Add 1 slow approval to keep it below 100% but above 80%.
	slowReq := now.Add(-20 * time.Hour)
	slowRes := slowReq.Add(30 * time.Second)
	slowEvent := makeApprovalEvent("e-slow-1", "bob", "package.install", string(types.ApprovalApproved), slowReq, slowRes)
	slowEvent.Timestamp = slowReq.Format(time.RFC3339)
	slowEvent.When = slowReq.Format(time.RFC3339)
	events = append(events, slowEvent)

	store := seedStore(t, events)

	groups := []DeveloperGroup{
		{ID: GroupPowerBuilder, MemberCount: 5, AvgBlockRate: 1.0},
	}

	recs, err := GenerateRecommendations(store, groups)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should contain at least one auto-approve recommendation.
	found := false
	for _, r := range recs {
		if r.Pattern == "fast_approval" && r.TargetGroup == GroupPowerBuilder {
			found = true
			if r.Status != RecommendationPending {
				t.Errorf("expected status=%s, got %s", RecommendationPending, r.Status)
			}
			if r.PolicyChange["action_type"] != "package.install" {
				t.Errorf("expected policy_change action_type=package.install, got %v", r.PolicyChange["action_type"])
			}
		}
	}

	if !found {
		t.Error("expected auto-approve recommendation for package.install but none found")
	}
}

func TestGetDeveloperScorecard_ReturnsComplianceAndGroup(t *testing.T) {
	now := time.Now().UTC()

	events := []types.AuditEvent{
		makeEventAt("e1", "alice", "claude_code", "file.write", "allow", "", "pol-1", now.Add(-1*time.Hour)),
		makeEventAt("e2", "alice", "claude_code", "file.write", "allow", "", "pol-1", now.Add(-2*time.Hour)),
		makeEventAt("e3", "alice", "claude_code", "file.write", "deny", "PATH_OUTSIDE_PROJECT_ROOT", "pol-fs", now.Add(-3*time.Hour)),
		makeEventAt("e4", "alice", "claude_code", "file.read", "allow", "", "pol-1", now.Add(-4*time.Hour)),
		// Other developer for org average.
		makeEventAt("e5", "bob", "claude_code", "file.write", "allow", "", "pol-1", now.Add(-1*time.Hour)),
	}

	store := seedStore(t, events)

	scorecard, err := GetDeveloperScorecard(store, "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if scorecard.UserID != "alice" {
		t.Errorf("expected user_id=alice, got %s", scorecard.UserID)
	}
	if scorecard.TotalActions != 4 {
		t.Errorf("expected total_actions=4, got %d", scorecard.TotalActions)
	}
	if scorecard.BlockedActions != 1 {
		t.Errorf("expected blocked_actions=1, got %d", scorecard.BlockedActions)
	}

	// Block rate should be 25% (1/4).
	expectedBlockRate := 25.0
	if scorecard.BlockRate != expectedBlockRate {
		t.Errorf("expected block_rate=%.1f, got %.1f", expectedBlockRate, scorecard.BlockRate)
	}

	// Compliance score should be 75% (100 - 25).
	expectedCompliance := 75.0
	if scorecard.ComplianceScore != expectedCompliance {
		t.Errorf("expected compliance_score=%.1f, got %.1f", expectedCompliance, scorecard.ComplianceScore)
	}

	// Group should be assigned.
	if scorecard.Group == "" {
		t.Error("expected a non-empty group assignment")
	}

	// Trend should be one of the valid values.
	validTrends := map[string]bool{"improving": true, "stable": true, "declining": true}
	if !validTrends[scorecard.Trend] {
		t.Errorf("expected trend to be improving/stable/declining, got %s", scorecard.Trend)
	}

	// Tips should be present since there are blocks.
	if len(scorecard.Tips) == 0 {
		t.Error("expected at least one tip for a developer with blocks")
	}
}

func TestGetBlockGuidance_PathOutsideProjectRoot(t *testing.T) {
	now := time.Now().UTC()

	events := []types.AuditEvent{
		makeEventAt("e1", "alice", "claude_code", "file.write", "deny", "PATH_OUTSIDE_PROJECT_ROOT", "pol-fs", now.Add(-1*time.Hour)),
		makeEventAt("e2", "alice", "claude_code", "file.write", "deny", "PATH_OUTSIDE_PROJECT_ROOT", "pol-fs", now.Add(-2*time.Hour)),
	}

	store := seedStore(t, events)

	guidance, err := GetBlockGuidance("PATH_OUTSIDE_PROJECT_ROOT", "alice", store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if guidance.ReasonCode != "PATH_OUTSIDE_PROJECT_ROOT" {
		t.Errorf("expected reason_code=PATH_OUTSIDE_PROJECT_ROOT, got %s", guidance.ReasonCode)
	}

	if guidance.WhyBlocked == "" {
		t.Error("expected non-empty WhyBlocked")
	}

	if guidance.HowToProceed == "" {
		t.Error("expected non-empty HowToProceed")
	}

	if guidance.PersonalStats == "" {
		t.Error("expected non-empty PersonalStats")
	}

	// PersonalStats should mention the count.
	expectedFragment := "2 time(s)"
	if !containsSubstring(guidance.PersonalStats, expectedFragment) {
		t.Errorf("expected PersonalStats to contain %q, got %q", expectedFragment, guidance.PersonalStats)
	}
}

func TestGetBlockGuidance_UnknownReasonCode(t *testing.T) {
	store := seedStore(t, nil)

	guidance, err := GetBlockGuidance("UNKNOWN_REASON", "alice", store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if guidance.ReasonCode != "UNKNOWN_REASON" {
		t.Errorf("expected reason_code=UNKNOWN_REASON, got %s", guidance.ReasonCode)
	}

	// Should still provide generic guidance.
	if guidance.WhyBlocked == "" {
		t.Error("expected non-empty WhyBlocked for unknown reason code")
	}
	if guidance.HowToProceed == "" {
		t.Error("expected non-empty HowToProceed for unknown reason code")
	}
}

func TestGetApprovalBottlenecks_FindsPendingApprovals(t *testing.T) {
	now := time.Now().UTC()

	events := []types.AuditEvent{
		// 2 pending approvals for package.install.
		makeApprovalEvent("e1", "alice", "package.install", string(types.ApprovalPending), now.Add(-10*time.Minute), now),
		makeApprovalEvent("e2", "bob", "package.install", string(types.ApprovalPending), now.Add(-5*time.Minute), now),
		// 1 resolved approval for shell.exec with known wait.
		makeApprovalEvent("e3", "charlie", "shell.exec", string(types.ApprovalApproved), now.Add(-10*time.Minute), now.Add(-5*time.Minute)),
	}

	// Override approval status to pending for the first two.
	events[0].Approval.Status = types.ApprovalPending
	events[1].Approval.Status = types.ApprovalPending
	// Clear resolved_at for pending events.
	events[0].Approval.ResolvedAt = ""
	events[1].Approval.ResolvedAt = ""

	store := seedStore(t, events)

	bottlenecks, err := GetApprovalBottlenecks(store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(bottlenecks) < 1 {
		t.Fatal("expected at least 1 bottleneck")
	}

	// package.install should be first (2 pending).
	if bottlenecks[0].ActionType != "package.install" {
		t.Errorf("expected first bottleneck action_type=package.install, got %s", bottlenecks[0].ActionType)
	}
	if bottlenecks[0].PendingCount != 2 {
		t.Errorf("expected pending_count=2, got %d", bottlenecks[0].PendingCount)
	}
}

func TestGetDeveloperImpact_ComputesBlockRates(t *testing.T) {
	now := time.Now().UTC()

	events := []types.AuditEvent{
		// Alice: 3 allows, 1 block = 25% block rate.
		makeEventAt("e1", "alice", "claude_code", "file.write", "allow", "", "pol-1", now.Add(-1*time.Hour)),
		makeEventAt("e2", "alice", "claude_code", "file.write", "allow", "", "pol-1", now.Add(-2*time.Hour)),
		makeEventAt("e3", "alice", "claude_code", "file.read", "allow", "", "pol-1", now.Add(-3*time.Hour)),
		makeEventAt("e4", "alice", "claude_code", "file.write", "deny", "PATH_OUTSIDE", "pol-fs", now.Add(-4*time.Hour)),
		// Bob: 2 allows, 0 blocks = 0% block rate.
		makeEventAt("e5", "bob", "cursor", "file.write", "allow", "", "pol-1", now.Add(-1*time.Hour)),
		makeEventAt("e6", "bob", "cursor", "file.read", "allow", "", "pol-1", now.Add(-2*time.Hour)),
	}

	store := seedStore(t, events)

	impact, err := GetDeveloperImpact(store, "7d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(impact) != 2 {
		t.Fatalf("expected 2 developers, got %d", len(impact))
	}

	// Results sorted by block rate descending, so Alice first.
	if impact[0].UserID != "alice" {
		t.Errorf("expected first developer=alice, got %s", impact[0].UserID)
	}
	if impact[0].TotalActions != 4 {
		t.Errorf("expected alice total_actions=4, got %d", impact[0].TotalActions)
	}
	if impact[0].BlockedActions != 1 {
		t.Errorf("expected alice blocked_actions=1, got %d", impact[0].BlockedActions)
	}
	expectedAliceRate := 25.0
	if impact[0].BlockRate != expectedAliceRate {
		t.Errorf("expected alice block_rate=%.1f, got %.1f", expectedAliceRate, impact[0].BlockRate)
	}

	if impact[1].UserID != "bob" {
		t.Errorf("expected second developer=bob, got %s", impact[1].UserID)
	}
	if impact[1].BlockRate != 0.0 {
		t.Errorf("expected bob block_rate=0.0, got %.1f", impact[1].BlockRate)
	}
}

func TestGenerateWeeklyDigest_ProducesOutput(t *testing.T) {
	now := time.Now().UTC()

	events := []types.AuditEvent{
		makeEventAt("e1", "alice", "claude_code", "file.write", "allow", "", "pol-1", now.Add(-1*time.Hour)),
		makeEventAt("e2", "alice", "claude_code", "file.write", "deny", "PATH_OUTSIDE_PROJECT_ROOT", "pol-fs", now.Add(-2*time.Hour)),
	}

	store := seedStore(t, events)

	digest, err := GenerateWeeklyDigest(store, "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if digest == "" {
		t.Fatal("expected non-empty digest")
	}

	if !containsSubstring(digest, "alice") {
		t.Error("digest should contain the user ID")
	}
	if !containsSubstring(digest, "Total Actions: 2") {
		t.Error("digest should contain correct total actions")
	}
	if !containsSubstring(digest, "Blocked: 1") {
		t.Error("digest should contain correct blocked count")
	}
}

func TestExtractFeatures_ComputesCorrectly(t *testing.T) {
	now := time.Now().UTC()

	events := []types.AuditEvent{
		makeEventAt("e1", "alice", "claude_code", "file.write", "allow", "", "pol-1", now.Add(-1*time.Hour)),
		makeEventAt("e2", "alice", "claude_code", "file.write", "deny", "PATH_OUTSIDE", "pol-1", now.Add(-2*time.Hour)),
		makeEventWithHost("e3", "alice", "network.request", "allow", "", "api.github.com"),
		makeEventWithHost("e4", "alice", "network.request", "allow", "", "registry.npmjs.org"),
	}
	// Set timestamps on host events within period.
	events[2].Timestamp = now.Add(-3 * time.Hour).Format(time.RFC3339)
	events[2].When = events[2].Timestamp
	events[3].Timestamp = now.Add(-4 * time.Hour).Format(time.RFC3339)
	events[3].When = events[3].Timestamp

	store := seedStore(t, events)

	features, err := ExtractFeatures(store, "7d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(features) != 1 {
		t.Fatalf("expected 1 developer, got %d", len(features))
	}

	f := features[0]
	if f.UserID != "alice" {
		t.Errorf("expected user_id=alice, got %s", f.UserID)
	}
	if f.NetworkBreadth != 2 {
		t.Errorf("expected network_breadth=2, got %d", f.NetworkBreadth)
	}
	if f.ActionDiversity < 2 {
		t.Errorf("expected action_diversity >= 2, got %d", f.ActionDiversity)
	}
}

func TestApplyRecommendation_AddsRule(t *testing.T) {
	bundle := &types.PolicyBundle{
		BundleVersion: "v1.0.0",
		Rules:         []types.PolicyRule{},
	}

	err := ApplyRecommendation("rec-auto-approve-1", bundle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(bundle.Rules) != 1 {
		t.Fatalf("expected 1 rule after applying recommendation, got %d", len(bundle.Rules))
	}

	rule := bundle.Rules[0]
	if rule.PolicyID != "auto-rec-auto-approve-1" {
		t.Errorf("expected policy_id=auto-rec-auto-approve-1, got %s", rule.PolicyID)
	}
	if rule.Effect.Decision != types.DecisionAllow {
		t.Errorf("expected decision=allow, got %s", rule.Effect.Decision)
	}
	if rule.Effect.ReasonCode != "AUTO_RECOMMENDED" {
		t.Errorf("expected reason_code=AUTO_RECOMMENDED, got %s", rule.Effect.ReasonCode)
	}
}

func TestApplyRecommendation_RejectsEmptyID(t *testing.T) {
	bundle := &types.PolicyBundle{}
	err := ApplyRecommendation("", bundle)
	if err == nil {
		t.Fatal("expected error for empty recommendation ID")
	}
}

func TestApplyRecommendation_RejectsNilBundle(t *testing.T) {
	err := ApplyRecommendation("rec-1", nil)
	if err == nil {
		t.Fatal("expected error for nil bundle")
	}
}

func TestGetGroupMembers_ReturnsCorrectGroup(t *testing.T) {
	now := time.Now().UTC()

	// Create events that will classify alice into NewJoiner group. With a
	// single day of activity, TenureDays = 1 which is < 30, triggering the
	// NewJoiner classification.
	events := []types.AuditEvent{
		makeEventAt("e1", "alice", "claude_code", "file.write", "allow", "", "pol-1", now.Add(-1*time.Hour)),
	}

	store := seedStore(t, events)

	members, err := GetGroupMembers(store, GroupNewJoiner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should find alice as a member (TenureDays=1 < 30 => NewJoiner).
	found := false
	for _, m := range members {
		if m.UserID == "alice" {
			found = true
			if m.JoinedGroup != GroupNewJoiner {
				t.Errorf("expected joined_group=%s, got %s", GroupNewJoiner, m.JoinedGroup)
			}
		}
	}

	if !found {
		t.Error("expected alice to be a member of new_joiner group")
	}
}

func TestGetGroupMembers_EmptyForUnmatchedGroup(t *testing.T) {
	now := time.Now().UTC()

	events := []types.AuditEvent{
		makeEventAt("e1", "alice", "claude_code", "file.write", "allow", "", "pol-1", now.Add(-1*time.Hour)),
	}

	store := seedStore(t, events)

	// BoundaryTester should have no members.
	members, err := GetGroupMembers(store, GroupBoundaryTester)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(members) != 0 {
		t.Errorf("expected 0 members for boundary_tester, got %d", len(members))
	}
}

// containsSubstring checks if s contains substr.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
