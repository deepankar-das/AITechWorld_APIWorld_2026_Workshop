/**
 * Author: Deepankar Das
 */

package analytics

import (
	"fmt"
	"sort"
	"time"

	"github.com/anthropics/enforcer/internal/audit"
	"github.com/anthropics/enforcer/internal/types"
)

// Recommendation represents a data-driven policy recommendation.
type Recommendation struct {
	ID           string                 `json:"id"`
	Title        string                 `json:"title"`
	Description  string                 `json:"description"`
	Pattern      string                 `json:"pattern"`
	Impact       string                 `json:"impact"`
	Risk         string                 `json:"risk"`
	TargetGroup  string                 `json:"target_group"`
	PolicyChange map[string]interface{} `json:"policy_change"`
	Status       string                 `json:"status"`
}

// Recommendation status constants.
const (
	RecommendationPending   = "pending"
	RecommendationApplied   = "applied"
	RecommendationDismissed = "dismissed"
)

// GenerateRecommendations analyses audit patterns and developer groups to
// produce actionable policy recommendations.
func GenerateRecommendations(store audit.AuditStore, groups []DeveloperGroup) ([]Recommendation, error) {
	var recs []Recommendation

	// Pattern 1: Auto-approve for frequently approved actions.
	autoApproveRecs, err := detectAutoApprovePattern(store, groups)
	if err != nil {
		return nil, fmt.Errorf("detect auto-approve pattern: %w", err)
	}
	recs = append(recs, autoApproveRecs...)

	// Pattern 2: Frequently blocked host that is not from BoundaryTesters.
	allowlistRecs, err := detectAllowlistPattern(store, groups)
	if err != nil {
		return nil, fmt.Errorf("detect allowlist pattern: %w", err)
	}
	recs = append(recs, allowlistRecs...)

	// Pattern 3: BoundaryTester evasion alerts.
	evasionRecs := detectEvasionPattern(groups)
	recs = append(recs, evasionRecs...)

	// Pattern 4: NewJoiner high block rate.
	onboardingRecs, err := detectOnboardingPattern(store, groups)
	if err != nil {
		return nil, fmt.Errorf("detect onboarding pattern: %w", err)
	}
	recs = append(recs, onboardingRecs...)

	// Pattern 5: Approval bottlenecks.
	bottleneckRecs, err := detectBottleneckPattern(store)
	if err != nil {
		return nil, fmt.Errorf("detect bottleneck pattern: %w", err)
	}
	recs = append(recs, bottleneckRecs...)

	return recs, nil
}

// detectAutoApprovePattern finds actions where >80% of approvals are approved
// in under 10 seconds and recommends auto-approve for PowerBuilder group.
func detectAutoApprovePattern(store audit.AuditStore, groups []DeveloperGroup) ([]Recommendation, error) {
	events, err := store.QueryEvents(audit.AuditQuery{
		Decision: string(types.DecisionRequireApproval),
	})
	if err != nil {
		return nil, err
	}

	type approvalStats struct {
		Total        int
		FastApproved int
	}

	byActionType := make(map[string]*approvalStats)

	for _, e := range events {
		at := e.Action.Type
		stats, exists := byActionType[at]
		if !exists {
			stats = &approvalStats{}
			byActionType[at] = stats
		}
		stats.Total++

		if e.Approval != nil && e.Approval.Status == types.ApprovalApproved {
			if e.Approval.RequestedAt != "" && e.Approval.ResolvedAt != "" {
				requested, err1 := time.Parse(time.RFC3339, e.Approval.RequestedAt)
				resolved, err2 := time.Parse(time.RFC3339, e.Approval.ResolvedAt)
				if err1 == nil && err2 == nil && resolved.Sub(requested).Seconds() < 10 {
					stats.FastApproved++
				}
			}
		}
	}

	var recs []Recommendation
	recIndex := 0

	for actionType, stats := range byActionType {
		if stats.Total == 0 {
			continue
		}
		fastRate := float64(stats.FastApproved) / float64(stats.Total) * 100.0
		if fastRate > 80.0 {
			recIndex++
			weeklyCount := stats.Total // approximation for current data
			recs = append(recs, Recommendation{
				ID:          fmt.Sprintf("rec-auto-approve-%d", recIndex),
				Title:       fmt.Sprintf("Auto-approve %s for Power Builders", actionType),
				Description: fmt.Sprintf("%.0f%% of %s approvals are approved in <10s. Consider auto-approving for trusted developers.", fastRate, actionType),
				Pattern:     "fast_approval",
				Impact:      fmt.Sprintf("Saves ~%d approvals/week", weeklyCount),
				Risk:        "Low -- Power Builders have <2%% block rate historically.",
				TargetGroup: GroupPowerBuilder,
				PolicyChange: map[string]interface{}{
					"action_type": actionType,
					"decision":    "allow",
					"condition":   "group == power_builder",
				},
				Status: RecommendationPending,
			})
		}
	}

	return recs, nil
}

// detectAllowlistPattern finds hosts blocked >100 times per week that are
// never blocked by BoundaryTesters, suggesting they are legitimate.
func detectAllowlistPattern(store audit.AuditStore, groups []DeveloperGroup) ([]Recommendation, error) {
	from, to := periodToTimeRange(string(Period7d))

	blockedEvents, err := store.QueryEvents(audit.AuditQuery{
		Decision: string(types.DecisionDeny),
		TimeFrom: from,
		TimeTo:   to,
	})
	if err != nil {
		return nil, err
	}

	// Get the set of BoundaryTester user IDs.
	features, err := ExtractFeatures(store, string(Period30d))
	if err != nil {
		return nil, err
	}
	boundaryTesters := make(map[string]bool)
	for _, f := range features {
		if ClassifyDeveloper(f) == GroupBoundaryTester {
			boundaryTesters[f.UserID] = true
		}
	}

	// Count blocks per host and track which are from BoundaryTesters.
	type hostStats struct {
		Count               int
		FromBoundaryTesters bool
	}
	hostMap := make(map[string]*hostStats)

	for _, e := range blockedEvents {
		host := e.Resource.Host
		if host == "" {
			continue
		}
		stats, exists := hostMap[host]
		if !exists {
			stats = &hostStats{}
			hostMap[host] = stats
		}
		stats.Count++
		if boundaryTesters[e.Actor.UserID] {
			stats.FromBoundaryTesters = true
		}
	}

	var recs []Recommendation
	recIndex := 0

	for host, stats := range hostMap {
		if stats.Count > 100 && !stats.FromBoundaryTesters {
			recIndex++
			recs = append(recs, Recommendation{
				ID:          fmt.Sprintf("rec-allowlist-host-%d", recIndex),
				Title:       fmt.Sprintf("Add %s to network allowlist", host),
				Description: fmt.Sprintf("Host %s was blocked %d times this week, never by BoundaryTesters. Likely a legitimate development dependency.", host, stats.Count),
				Pattern:     "frequent_block_legitimate",
				Impact:      fmt.Sprintf("Eliminates ~%d blocks/week", stats.Count),
				Risk:        "Medium -- verify the host is a known development dependency before allowlisting.",
				TargetGroup: "",
				PolicyChange: map[string]interface{}{
					"resource.host": host,
					"decision":      "allow",
					"scope":         "network_allowlist",
				},
				Status: RecommendationPending,
			})
		}
	}

	return recs, nil
}

// detectEvasionPattern flags BoundaryTester groups with evasion signals.
func detectEvasionPattern(groups []DeveloperGroup) []Recommendation {
	var recs []Recommendation

	for _, g := range groups {
		if g.ID == GroupBoundaryTester && g.MemberCount > 0 {
			recs = append(recs, Recommendation{
				ID:          "rec-evasion-investigation",
				Title:       "Investigate BoundaryTester evasion alerts",
				Description: fmt.Sprintf("%d developers classified as BoundaryTesters with evasion signals. Review their session logs for policy circumvention attempts.", g.MemberCount),
				Pattern:     "evasion_alert",
				Impact:      "Prevents potential security policy circumvention.",
				Risk:        "High -- unaddressed evasion may indicate compromised agents or malicious intent.",
				TargetGroup: GroupBoundaryTester,
				PolicyChange: map[string]interface{}{
					"action": "investigate",
					"scope":  "boundary_tester_sessions",
				},
				Status: RecommendationPending,
			})
		}
	}

	return recs
}

// detectOnboardingPattern detects NewJoiners with significantly higher block
// rates than the org average and suggests onboarding mode.
func detectOnboardingPattern(store audit.AuditStore, groups []DeveloperGroup) ([]Recommendation, error) {
	impact, err := GetDeveloperImpact(store, string(Period30d))
	if err != nil {
		return nil, err
	}

	if len(impact) == 0 {
		return nil, nil
	}

	// Compute org-wide average block rate.
	var totalRate float64
	for _, d := range impact {
		totalRate += d.BlockRate
	}
	orgAvg := totalRate / float64(len(impact))

	// Check NewJoiner block rate.
	var newJoinerRate float64
	var newJoinerCount int
	for _, g := range groups {
		if g.ID == GroupNewJoiner && g.MemberCount > 0 {
			newJoinerRate = g.AvgBlockRate
			newJoinerCount = g.MemberCount
		}
	}

	var recs []Recommendation

	if newJoinerCount > 0 && newJoinerRate > 10.0 && orgAvg < 2.0 {
		recs = append(recs, Recommendation{
			ID:          "rec-onboarding-mode",
			Title:       "Enable onboarding mode for new joiners",
			Description: fmt.Sprintf("New joiners have a %.1f%% block rate vs %.1f%% org average. Consider an onboarding policy that provides guided explanations instead of hard blocks.", newJoinerRate, orgAvg),
			Pattern:     "new_joiner_high_block",
			Impact:      fmt.Sprintf("Reduces friction for %d new joiners.", newJoinerCount),
			Risk:        "Low -- onboarding mode still logs all actions; enforcement is softened, not removed.",
			TargetGroup: GroupNewJoiner,
			PolicyChange: map[string]interface{}{
				"mode":      "onboarding",
				"condition": "tenure_days < 30",
				"effect":    "warn_and_log",
			},
			Status: RecommendationPending,
		})
	}

	return recs, nil
}

// detectBottleneckPattern flags approval actions with median wait > 5 minutes.
func detectBottleneckPattern(store audit.AuditStore) ([]Recommendation, error) {
	events, err := store.QueryEvents(audit.AuditQuery{
		Decision: string(types.DecisionRequireApproval),
	})
	if err != nil {
		return nil, err
	}

	// Collect wait times per action type.
	waitTimes := make(map[string][]float64)

	for _, e := range events {
		if e.Approval != nil && e.Approval.RequestedAt != "" && e.Approval.ResolvedAt != "" {
			requested, err1 := time.Parse(time.RFC3339, e.Approval.RequestedAt)
			resolved, err2 := time.Parse(time.RFC3339, e.Approval.ResolvedAt)
			if err1 == nil && err2 == nil {
				waitSec := resolved.Sub(requested).Seconds()
				waitTimes[e.Action.Type] = append(waitTimes[e.Action.Type], waitSec)
			}
		}
	}

	var recs []Recommendation
	recIndex := 0

	for actionType, times := range waitTimes {
		if len(times) == 0 {
			continue
		}

		// Compute median.
		sort.Float64s(times)
		var median float64
		n := len(times)
		if n%2 == 0 {
			median = (times[n/2-1] + times[n/2]) / 2.0
		} else {
			median = times[n/2]
		}

		if median > 300.0 { // 5 minutes = 300 seconds
			recIndex++
			recs = append(recs, Recommendation{
				ID:          fmt.Sprintf("rec-bottleneck-%d", recIndex),
				Title:       fmt.Sprintf("Approval bottleneck for %s", actionType),
				Description: fmt.Sprintf("Median approval wait for %s is %.0f seconds (%.1f minutes). Consider adding more approvers or auto-approve rules.", actionType, median, median/60.0),
				Pattern:     "approval_bottleneck",
				Impact:      fmt.Sprintf("Reduces median wait from %.0fs to near-instant for qualifying cases.", median),
				Risk:        "Medium -- ensure auto-approve criteria are well-defined.",
				TargetGroup: "",
				PolicyChange: map[string]interface{}{
					"action_type":     actionType,
					"current_wait_s":  median,
					"suggested_action": "add_approvers_or_auto_approve",
				},
				Status: RecommendationPending,
			})
		}
	}

	return recs, nil
}

// ApplyRecommendation applies a recommendation to the given policy bundle.
// In a full implementation this would modify rules; here it adds a new
// auto-generated rule or modifies an existing one based on the recommendation's
// PolicyChange map.
func ApplyRecommendation(recID string, bundle *types.PolicyBundle) error {
	if recID == "" {
		return fmt.Errorf("recommendation ID is required")
	}
	if bundle == nil {
		return fmt.Errorf("policy bundle is nil")
	}

	// In the prototype, we mark the recommendation as applied by adding a
	// synthetic policy rule that encodes the recommendation.
	newRule := types.PolicyRule{
		PolicyID: fmt.Sprintf("auto-%s", recID),
		Version:  bundle.BundleVersion,
	}
	newRule.Scope.Level = types.ScopeLocal
	newRule.Effect = types.PolicyEffect{
		Decision:    types.DecisionAllow,
		ReasonCode:  "AUTO_RECOMMENDED",
		ReasonHuman: fmt.Sprintf("Auto-generated from recommendation %s", recID),
	}

	bundle.Rules = append(bundle.Rules, newRule)
	return nil
}
