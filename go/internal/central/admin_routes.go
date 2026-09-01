/**
 * Author: Deepankar Das
 */

package central

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/anthropics/enforcer/internal/audit"
	"github.com/anthropics/enforcer/internal/daemon/routes"
	"github.com/anthropics/enforcer/internal/policy"
	"github.com/anthropics/enforcer/internal/types"
)

// ── Audit detail handlers ──────────────────────────────────────────────────

func (s *CentralServer) handleAdminAuditEvents(w http.ResponseWriter, r *http.Request) {
	status, resp := routes.HandleQueryEvents(r.URL.Query(), s.auditStore)
	writeJSON(w, status, resp)
}

func (s *CentralServer) handleAdminAuditMetrics(w http.ResponseWriter, r *http.Request) {
	status, resp := routes.HandleAuditMetrics(s.auditStore)
	writeJSON(w, status, resp)
}

func (s *CentralServer) handleAdminSessionDetail(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimPrefix(r.URL.Path, "/api/v1/audit/sessions/")
	sessionID = strings.TrimSuffix(sessionID, "/")
	if sessionID == "" {
		writeJSON(w, 400, map[string]string{"error": "missing_session_id"})
		return
	}
	status, resp := routes.HandleGetSession(sessionID, s.auditStore)
	writeJSON(w, status, resp)
}

// ── Approval metrics ───────────────────────────────────────────────────────

func (s *CentralServer) handleAdminApprovalMetrics(w http.ResponseWriter, r *http.Request) {
	s.approvalsMu.Lock()
	pending := 0
	resolved := 0
	approved := 0
	denied := 0
	for _, apr := range s.approvals {
		if apr.Decision == "" {
			pending++
		} else {
			resolved++
			if apr.Decision == "approve" {
				approved++
			} else {
				denied++
			}
		}
	}
	s.approvalsMu.Unlock()

	writeJSON(w, 200, map[string]interface{}{
		"total_created":  pending + resolved,
		"total_approved": approved,
		"total_denied":   denied,
		"total_expired":  0,
		"pending_count":  pending,
	})
}

// ── Policy detail handlers ─────────────────────────────────────────────────

func (s *CentralServer) handleAdminPolicyRules(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	state, err := s.stateStore.GetLatestPolicy(ctx)
	if err != nil {
		writeJSON(w, 200, map[string]interface{}{"rules": []interface{}{}, "count": 0})
		return
	}

	parsed, err := policy.LoadPolicyFromBytes([]byte(state.Bundle))
	if err != nil {
		writeJSON(w, 200, map[string]interface{}{"rules": []interface{}{}, "count": 0})
		return
	}

	writeJSON(w, 200, map[string]interface{}{
		"rules":          parsed.Rules,
		"count":          len(parsed.Rules),
		"bundle_version": state.Version,
	})
}

func (s *CentralServer) handleAdminPolicyPacks(w http.ResponseWriter, r *http.Request) {
	packs := policy.GetAvailablePacks()
	writeJSON(w, 200, map[string]interface{}{
		"packs": packs,
	})
}

func (s *CentralServer) handleAdminPolicyPackDetail(w http.ResponseWriter, r *http.Request) {
	packID := strings.TrimPrefix(r.URL.Path, "/api/v1/policy/packs/")
	packID = strings.TrimSuffix(packID, "/")
	if packID == "" {
		writeJSON(w, 400, map[string]string{"error": "missing_pack_id"})
		return
	}

	pack := policy.GetPack(packID)
	if pack == nil {
		writeJSON(w, 404, map[string]string{"error": "pack_not_found"})
		return
	}
	writeJSON(w, 200, pack)
}

// ── Policy mutation helpers ─────────────────────────────────────────────────

// loadCurrentBundle loads the current policy bundle from PostgreSQL.
func (s *CentralServer) loadCurrentBundle() (*types.PolicyBundle, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	state, err := s.stateStore.GetLatestPolicy(ctx)
	if err != nil {
		return nil, "", err
	}
	parsed, err := policy.LoadPolicyFromBytes([]byte(state.Bundle))
	if err != nil {
		return nil, "", err
	}
	return parsed, state.Version, nil
}

// persistBundle saves the modified bundle back to PostgreSQL with a new version.
func (s *CentralServer) persistBundle(bundle *types.PolicyBundle) error {
	data, err := json.Marshal(bundle)
	if err != nil {
		return err
	}
	// Use YAML-compatible format for the bundle text stored in DB.
	// The policy loader accepts JSON as well.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.persistPolicy(ctx, string(data), "")
}

// ── Policy mutation handlers ───────────────────────────────────────────────

func (s *CentralServer) handleAdminAddRule(w http.ResponseWriter, r *http.Request) {
	body, err := readRequestBody(r)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid body"})
		return
	}

	var newRule types.PolicyRule
	if err := json.Unmarshal(body, &newRule); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid rule JSON: " + err.Error()})
		return
	}
	if newRule.PolicyID == "" {
		writeJSON(w, 400, map[string]string{"error": "policy_id is required"})
		return
	}

	bundle, _, err := s.loadCurrentBundle()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "failed to load policy: " + err.Error()})
		return
	}

	// Check for duplicate
	for _, r := range bundle.Rules {
		if r.PolicyID == newRule.PolicyID {
			writeJSON(w, 409, map[string]string{"error": "rule already exists: " + newRule.PolicyID})
			return
		}
	}

	bundle.Rules = append(bundle.Rules, newRule)
	if err := s.persistBundle(bundle); err != nil {
		writeJSON(w, 500, map[string]string{"error": "failed to persist: " + err.Error()})
		return
	}

	writeJSON(w, 201, map[string]interface{}{
		"policy_id": newRule.PolicyID,
		"added":     true,
		"total":     len(bundle.Rules),
	})
}

func (s *CentralServer) handleAdminToggleRule(w http.ResponseWriter, r *http.Request) {
	ruleID := strings.TrimPrefix(r.URL.Path, "/api/v1/policy/rules/")
	ruleID = strings.TrimSuffix(ruleID, "/toggle")
	ruleID = strings.TrimSuffix(ruleID, "/")

	bundle, _, err := s.loadCurrentBundle()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "failed to load policy: " + err.Error()})
		return
	}

	found := false
	var newEnabled bool
	for i := range bundle.Rules {
		if bundle.Rules[i].PolicyID == ruleID {
			// Toggle: if Enabled is nil or true, set to false; if false, set to true
			current := bundle.Rules[i].IsEnabled()
			newEnabled = !current
			bundle.Rules[i].Enabled = &newEnabled
			found = true
			break
		}
	}

	if !found {
		writeJSON(w, 404, map[string]string{"error": "rule not found: " + ruleID})
		return
	}

	if err := s.persistBundle(bundle); err != nil {
		writeJSON(w, 500, map[string]string{"error": "failed to persist: " + err.Error()})
		return
	}

	writeJSON(w, 200, map[string]interface{}{
		"policy_id": ruleID,
		"enabled":   newEnabled,
	})
}

func (s *CentralServer) handleAdminDeleteRule(w http.ResponseWriter, r *http.Request) {
	ruleID := strings.TrimPrefix(r.URL.Path, "/api/v1/policy/rules/")
	ruleID = strings.TrimSuffix(ruleID, "/")

	bundle, _, err := s.loadCurrentBundle()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "failed to load policy: " + err.Error()})
		return
	}

	found := false
	newRules := make([]types.PolicyRule, 0, len(bundle.Rules))
	for _, rule := range bundle.Rules {
		if rule.PolicyID == ruleID {
			found = true
			continue
		}
		newRules = append(newRules, rule)
	}

	if !found {
		writeJSON(w, 404, map[string]string{"error": "rule not found: " + ruleID})
		return
	}

	bundle.Rules = newRules
	if err := s.persistBundle(bundle); err != nil {
		writeJSON(w, 500, map[string]string{"error": "failed to persist: " + err.Error()})
		return
	}

	writeJSON(w, 200, map[string]interface{}{
		"policy_id": ruleID,
		"deleted":   true,
		"remaining": len(bundle.Rules),
	})
}

func (s *CentralServer) handleAdminApplyPack(w http.ResponseWriter, r *http.Request) {
	packID := strings.TrimPrefix(r.URL.Path, "/api/v1/policy/packs/")
	packID = strings.TrimSuffix(packID, "/apply")
	packID = strings.TrimSuffix(packID, "/")

	bundle, _, err := s.loadCurrentBundle()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "failed to load policy: " + err.Error()})
		return
	}

	added, skipped := policy.ApplyPack(bundle, packID)
	if len(added) == 0 && len(skipped) == 0 {
		writeJSON(w, 404, map[string]string{"error": "pack not found: " + packID})
		return
	}

	if err := s.persistBundle(bundle); err != nil {
		writeJSON(w, 500, map[string]string{"error": "failed to persist: " + err.Error()})
		return
	}

	writeJSON(w, 200, map[string]interface{}{
		"pack_id": packID,
		"added":   added,
		"skipped": skipped,
		"total":   len(bundle.Rules),
	})
}

// ── Analytics handlers ─────────────────────────────────────────────────────
// These use the same route handlers as the Sentinel daemon, backed by the
// Hub's aggregated PostgreSQL audit store (events from all Sentinels).

func (s *CentralServer) handleAdminBlockedOps(w http.ResponseWriter, r *http.Request) {
	status, resp := routes.HandleBlockedOperations(r.URL.Query(), s.auditStore)
	writeJSON(w, status, resp)
}

func (s *CentralServer) handleAdminApprovalBottlenecks(w http.ResponseWriter, r *http.Request) {
	status, resp := routes.HandleApprovalBottlenecks(s.auditStore)
	writeJSON(w, status, resp)
}

func (s *CentralServer) handleAdminDeveloperImpact(w http.ResponseWriter, r *http.Request) {
	status, resp := routes.HandleDeveloperImpact(r.URL.Query(), s.auditStore)
	writeJSON(w, status, resp)
}

func (s *CentralServer) handleAdminGroups(w http.ResponseWriter, r *http.Request) {
	status, resp := routes.HandleGetGroups(s.auditStore)
	writeJSON(w, status, resp)
}

func (s *CentralServer) handleAdminGroupMembers(w http.ResponseWriter, r *http.Request) {
	groupID := strings.TrimPrefix(r.URL.Path, "/api/v1/analytics/groups/")
	groupID = strings.TrimSuffix(groupID, "/members")
	groupID = strings.TrimSuffix(groupID, "/")
	status, resp := routes.HandleGetGroupMembers(groupID, s.auditStore)
	writeJSON(w, status, resp)
}

func (s *CentralServer) handleAdminRecommendations(w http.ResponseWriter, r *http.Request) {
	status, resp := routes.HandleGetRecommendations(s.auditStore)
	writeJSON(w, status, resp)
}

func (s *CentralServer) handleAdminApplyRecommendation(w http.ResponseWriter, r *http.Request) {
	recID := strings.TrimPrefix(r.URL.Path, "/api/v1/analytics/recommendations/")
	recID = strings.TrimSuffix(recID, "/apply")
	recID = strings.TrimSuffix(recID, "/")

	body, err := readRequestBody(r)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid body"})
		return
	}

	// Load current policy bundle from Hub state.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	state, err := s.stateStore.GetLatestPolicy(ctx)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "policy_lookup_failed"})
		return
	}
	parsed, err := policy.LoadPolicyFromBytes([]byte(state.Bundle))
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "policy_parse_failed"})
		return
	}

	status, resp := routes.HandleApplyRecommendation(recID, body, parsed)
	writeJSON(w, status, resp)
}

func (s *CentralServer) handleAdminDeveloperAnalytics(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	remainder := strings.TrimPrefix(path, "/api/v1/analytics/developer/")
	remainder = strings.TrimSuffix(remainder, "/")

	if strings.HasSuffix(remainder, "/trends") {
		userID := strings.TrimSuffix(remainder, "/trends")
		status, resp := routes.HandleDeveloperTrends(userID, s.auditStore)
		writeJSON(w, status, resp)
		return
	}

	userID := remainder
	status, resp := routes.HandleDeveloperScorecard(userID, s.auditStore)
	writeJSON(w, status, resp)
}

// ── Metrics handler ────────────────────────────────────────────────────────

func (s *CentralServer) handleAdminMetrics(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Policy info
	var policyVersion string
	var ruleCount int
	if state, err := s.stateStore.GetLatestPolicy(ctx); err == nil {
		policyVersion = state.Version
		if parsed, err := policy.LoadPolicyFromBytes([]byte(state.Bundle)); err == nil {
			ruleCount = len(parsed.Rules)
		}
	}

	// Enforcement state
	enf, _ := s.getEffectiveEnforcementState(ctx)

	// Audit metrics
	var auditMetrics interface{}
	if metrics, err := s.auditStore.GetMetrics(); err == nil {
		auditMetrics = metrics
	}

	// Client count
	clients, _ := s.stateStore.ListLatestClients(ctx)

	// Approval metrics
	s.approvalsMu.Lock()
	pendingCount := 0
	for _, apr := range s.approvals {
		if apr.Decision == "" {
			pendingCount++
		}
	}
	s.approvalsMu.Unlock()

	writeJSON(w, 200, map[string]interface{}{
		"policy": map[string]interface{}{
			"bundle_version": policyVersion,
			"rule_count":     ruleCount,
		},
		"enforcement": map[string]interface{}{
			"enabled":    enf.Enabled,
			"since":      enf.ChangedAt,
			"changed_by": enf.ChangedBy,
		},
		"audit_metrics": auditMetrics,
		"buffer_metrics": map[string]interface{}{
			"accepted":           0,
			"rejected":           0,
			"backpressureAlerts": 0,
			"bufferCount":        0,
		},
		"approval_metrics": map[string]interface{}{
			"pending_count": pendingCount,
		},
		"connected_clients": len(clients),
	})
}

// readRequestBody reads and returns the request body bytes.
func readRequestBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	var buf [1 << 20]byte // 1MB max
	n := 0
	for {
		nn, err := r.Body.Read(buf[n:])
		n += nn
		if err != nil {
			break
		}
	}
	return buf[:n], nil
}

// ── Existing handler updates ───────────────────────────────────────────────
// The handleAdminAudit and handleAdminSessions handlers already exist in
// server.go.  The new /api/v1/audit/events route provides the console-
// compatible query interface that the Search page expects.

// Ensure the routes package types are compatible with Hub's audit store.
var _ audit.AuditStore = (*audit.PostgresStore)(nil)
var _ = types.PolicyBundle{}
var _ = json.Marshal
