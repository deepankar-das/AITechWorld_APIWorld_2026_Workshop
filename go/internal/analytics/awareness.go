/**
 * Author: Deepankar Das
 */

package analytics

import (
	"fmt"
	"strings"

	"github.com/anthropics/enforcer/internal/audit"
	"github.com/anthropics/enforcer/internal/types"
)

// DeveloperScorecard provides a comprehensive compliance summary for one
// developer, including their group, trends, and actionable tips.
type DeveloperScorecard struct {
	UserID           string   `json:"user_id"`
	Group            string   `json:"group"`
	ComplianceScore  float64  `json:"compliance_score"`
	OrgAvgCompliance float64  `json:"org_avg_compliance"`
	TotalActions     int      `json:"total_actions"`
	BlockedActions   int      `json:"blocked_actions"`
	ApprovedActions  int      `json:"approved_actions"`
	BlockRate        float64  `json:"block_rate"`
	Trend            string   `json:"trend"`
	Tips             []string `json:"tips"`
	WeeklySummary    string   `json:"weekly_summary"`
}

// BlockGuidance provides contextual help when a developer encounters a block.
type BlockGuidance struct {
	ReasonCode    string `json:"reason_code"`
	WhyBlocked    string `json:"why_blocked"`
	HowToProceed  string `json:"how_to_proceed"`
	PersonalStats string `json:"personal_stats"`
}

// reasonCodeGuidance maps common reason codes to user-friendly explanations.
var reasonCodeGuidance = map[string]struct {
	Why     string
	HowTo   string
}{
	"PATH_OUTSIDE_PROJECT_ROOT": {
		Why:   "The action attempted to access a file path outside the allowed project directory. Enforcer restricts file operations to the project root to prevent accidental or malicious access to system files, credentials, or other projects.",
		HowTo: "Ensure your file operations target paths within the project root. If you need access to files outside the project, request an exception through your team's policy administrator or use the break-glass approval flow.",
	},
	"HOST_NOT_ALLOWLISTED": {
		Why:   "The network request targeted a host that is not on the approved allowlist. Egress controls prevent data exfiltration and limit agent access to known, trusted endpoints.",
		HowTo: "Check the network allowlist in your policy configuration. If the host is a legitimate development dependency, request it be added to the allowlist through the policy admin console.",
	},
	"PACKAGE_REQUIRES_APPROVAL": {
		Why:   "Package installations require explicit approval to prevent supply-chain attacks and ensure only vetted dependencies are added to the project.",
		HowTo: "Wait for the approval notification in the console. An approver will review the package details and approve or deny the installation. You can also use the break-glass flow for urgent cases.",
	},
	"COMMAND_BLOCKED": {
		Why:   "The shell command was blocked because it matches a restricted pattern. Certain commands are blocked to prevent destructive operations or data exfiltration.",
		HowTo: "Review the command against the shell policy. If you believe this is a false positive, contact your policy administrator with the command details and use case.",
	},
	"CREDENTIAL_ACCESS_DENIED": {
		Why:   "Access to credentials or secrets was denied. Credential access is tightly controlled to prevent unauthorized exposure of sensitive material.",
		HowTo: "Use the approved credential access patterns defined in your team's policy. If you need broader access, request it through the credential broker configuration.",
	},
	"MCP_TOOL_BLOCKED": {
		Why:   "The MCP tool invocation was blocked because the tool or server is not approved in the current policy configuration.",
		HowTo: "Check the MCP gateway configuration for approved tools and servers. Request additions through the policy admin console if the tool is needed for your workflow.",
	},
}

// GetDeveloperScorecard computes a full compliance scorecard for a developer.
func GetDeveloperScorecard(store audit.AuditStore, userID string) (*DeveloperScorecard, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID is required")
	}

	// Get current period events for this developer.
	// "_all" is a wildcard — query all events (used by Sentinel Console
	// where all events belong to the single developer on this machine).
	from, to := periodToTimeRange(string(Period30d))

	query := audit.AuditQuery{
		TimeFrom: from,
		TimeTo:   to,
	}
	if userID != "_all" {
		query.ActorUserID = userID
	}

	userEvents, err := store.QueryEvents(query)
	if err != nil {
		return nil, fmt.Errorf("query user events: %w", err)
	}

	// Get all events for org-wide average.
	allEvents, err := store.QueryEvents(audit.AuditQuery{
		TimeFrom: from,
		TimeTo:   to,
	})
	if err != nil {
		return nil, fmt.Errorf("query all events: %w", err)
	}

	// Compute user stats.
	totalActions := len(userEvents)
	var blockedActions, approvedActions int
	reasonCounts := make(map[string]int)

	for _, e := range userEvents {
		switch e.Decision {
		case string(types.DecisionDeny):
			blockedActions++
			if e.PolicyDetail.ReasonCode != "" {
				reasonCounts[e.PolicyDetail.ReasonCode]++
			}
		case string(types.DecisionRequireApproval):
			approvedActions++
		}
	}

	var blockRate float64
	if totalActions > 0 {
		blockRate = float64(blockedActions) / float64(totalActions) * 100.0
	}

	// Compliance score: 100 minus block rate, clamped to [0, 100].
	complianceScore := 100.0 - blockRate
	if complianceScore < 0 {
		complianceScore = 0
	}

	// Org-wide average compliance.
	orgTotalActions := len(allEvents)
	var orgBlockedActions int
	for _, e := range allEvents {
		if e.Decision == string(types.DecisionDeny) {
			orgBlockedActions++
		}
	}
	var orgAvgCompliance float64
	if orgTotalActions > 0 {
		orgBlockRate := float64(orgBlockedActions) / float64(orgTotalActions) * 100.0
		orgAvgCompliance = 100.0 - orgBlockRate
	} else {
		orgAvgCompliance = 100.0
	}

	// Compute trend by comparing recent 7d vs prior 7d.
	trend := computeTrend(store, userID)

	// Classify developer.
	features := DeveloperFeatures{
		UserID:    userID,
		BlockRate: blockRate,
	}
	groupID := ClassifyDeveloper(features)
	group := groupID
	if meta, ok := groupMetadata[groupID]; ok {
		group = meta.Name
	}

	// Generate tips based on top block reasons.
	tips := generateTips(reasonCounts)

	// Generate weekly summary.
	summary := fmt.Sprintf("Over the last 30 days you performed %d actions with %d blocked (%.1f%% block rate). Your compliance score is %.1f vs org average of %.1f.",
		totalActions, blockedActions, blockRate, complianceScore, orgAvgCompliance)

	return &DeveloperScorecard{
		UserID:           userID,
		Group:            group,
		ComplianceScore:  complianceScore,
		OrgAvgCompliance: orgAvgCompliance,
		TotalActions:     totalActions,
		BlockedActions:   blockedActions,
		ApprovedActions:  approvedActions,
		BlockRate:        blockRate,
		Trend:            trend,
		Tips:             tips,
		WeeklySummary:    summary,
	}, nil
}

// computeTrend compares the developer's block rate in the recent 7 days vs
// the prior 7 days and returns "improving", "stable", or "declining".
func computeTrend(store audit.AuditStore, userID string) string {
	recentFrom, recentTo := periodToTimeRange(string(Period7d))
	priorFrom, priorTo := priorPeriodRange(string(Period7d))

	recentQuery := audit.AuditQuery{TimeFrom: recentFrom, TimeTo: recentTo}
	priorQuery := audit.AuditQuery{TimeFrom: priorFrom, TimeTo: priorTo}
	if userID != "_all" {
		recentQuery.ActorUserID = userID
		priorQuery.ActorUserID = userID
	}

	recentEvents, err1 := store.QueryEvents(recentQuery)
	priorEvents, err2 := store.QueryEvents(priorQuery)

	if err1 != nil || err2 != nil {
		return "stable"
	}

	recentBlockRate := blockRateOf(recentEvents)
	priorBlockRate := blockRateOf(priorEvents)

	diff := recentBlockRate - priorBlockRate
	if diff < -1.0 {
		return "improving"
	} else if diff > 1.0 {
		return "declining"
	}
	return "stable"
}

// blockRateOf computes the block rate for a set of events.
func blockRateOf(events []types.AuditEvent) float64 {
	if len(events) == 0 {
		return 0
	}
	blocked := 0
	for _, e := range events {
		if e.Decision == string(types.DecisionDeny) {
			blocked++
		}
	}
	return float64(blocked) / float64(len(events)) * 100.0
}

// generateTips produces actionable tips based on the developer's most common
// block reasons.
func generateTips(reasonCounts map[string]int) []string {
	if len(reasonCounts) == 0 {
		return []string{"No blocks recorded. Keep up the good work!"}
	}

	// Sort reasons by count.
	type rc struct {
		Code  string
		Count int
	}
	var sorted []rc
	for code, count := range reasonCounts {
		sorted = append(sorted, rc{code, count})
	}
	// Sort descending by count.
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Count > sorted[i].Count {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	var tips []string
	for i := 0; i < len(sorted) && i < 3; i++ {
		code := sorted[i].Code
		guidance, exists := reasonCodeGuidance[code]
		if exists {
			tips = append(tips, fmt.Sprintf("[%s] %s", code, guidance.HowTo))
		} else {
			tips = append(tips, fmt.Sprintf("[%s] Review the policy documentation for this reason code. Contact your policy administrator if you need clarification.", code))
		}
	}

	return tips
}

// GetBlockGuidance returns contextual guidance for a specific block reason,
// personalised with the developer's statistics for that reason code.
func GetBlockGuidance(reasonCode string, userID string, store audit.AuditStore) (*BlockGuidance, error) {
	if reasonCode == "" {
		return nil, fmt.Errorf("reason code is required")
	}

	guidance, exists := reasonCodeGuidance[reasonCode]
	if !exists {
		guidance = struct {
			Why   string
			HowTo string
		}{
			Why:   fmt.Sprintf("Action was blocked with reason code %s. This reason code enforces an organizational security policy.", reasonCode),
			HowTo: "Review your organization's policy documentation for details on this restriction. Contact your policy administrator if you need an exception.",
		}
	}

	// Compute personal stats for this reason code.
	var personalStats string
	if userID != "" {
		events, err := store.QueryEvents(audit.AuditQuery{
			ActorUserID: userID,
			Decision:    string(types.DecisionDeny),
		})
		if err == nil {
			count := 0
			for _, e := range events {
				if e.PolicyDetail.ReasonCode == reasonCode {
					count++
				}
			}
			if count > 0 {
				personalStats = fmt.Sprintf("You have been blocked by %s %d time(s). Review the guidance above to reduce future blocks.", reasonCode, count)
			} else {
				personalStats = "This is your first encounter with this block reason."
			}
		} else {
			personalStats = "Unable to retrieve personal statistics."
		}
	}

	return &BlockGuidance{
		ReasonCode:    reasonCode,
		WhyBlocked:    guidance.Why,
		HowToProceed:  guidance.HowTo,
		PersonalStats: personalStats,
	}, nil
}

// GenerateWeeklyDigest produces a human-readable weekly summary for a developer.
func GenerateWeeklyDigest(store audit.AuditStore, userID string) (string, error) {
	if userID == "" {
		return "", fmt.Errorf("user ID is required")
	}

	from, to := periodToTimeRange(string(Period7d))

	events, err := store.QueryEvents(audit.AuditQuery{
		ActorUserID: userID,
		TimeFrom:    from,
		TimeTo:      to,
	})
	if err != nil {
		return "", fmt.Errorf("query weekly events: %w", err)
	}

	totalActions := len(events)
	var blocked, approved, allowed int
	reasonCounts := make(map[string]int)
	actionTypeCounts := make(map[string]int)

	for _, e := range events {
		actionTypeCounts[e.Action.Type]++
		switch e.Decision {
		case string(types.DecisionDeny):
			blocked++
			if e.PolicyDetail.ReasonCode != "" {
				reasonCounts[e.PolicyDetail.ReasonCode]++
			}
		case string(types.DecisionRequireApproval):
			approved++
		case string(types.DecisionAllow):
			allowed++
		}
	}

	if totalActions == 0 {
		return fmt.Sprintf("Weekly Digest for %s: No activity recorded this week.", userID), nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Weekly Digest for %s\n", userID))
	b.WriteString(fmt.Sprintf("Period: %s to %s\n\n", from, to))
	b.WriteString(fmt.Sprintf("Total Actions: %d\n", totalActions))
	b.WriteString(fmt.Sprintf("  Allowed: %d\n", allowed))
	b.WriteString(fmt.Sprintf("  Blocked: %d\n", blocked))
	b.WriteString(fmt.Sprintf("  Required Approval: %d\n\n", approved))

	if totalActions > 0 {
		blockRate := float64(blocked) / float64(totalActions) * 100.0
		complianceScore := 100.0 - blockRate
		b.WriteString(fmt.Sprintf("Compliance Score: %.1f%%\n", complianceScore))
		b.WriteString(fmt.Sprintf("Block Rate: %.1f%%\n\n", blockRate))
	}

	if len(reasonCounts) > 0 {
		b.WriteString("Top Block Reasons:\n")
		topReasons := topNKeys(reasonCounts, 3)
		for _, reason := range topReasons {
			b.WriteString(fmt.Sprintf("  - %s (%d occurrences)\n", reason, reasonCounts[reason]))
		}
		b.WriteString("\n")
	}

	if len(actionTypeCounts) > 0 {
		b.WriteString("Activity Breakdown:\n")
		topActions := topNKeys(actionTypeCounts, 5)
		for _, at := range topActions {
			b.WriteString(fmt.Sprintf("  - %s: %d\n", at, actionTypeCounts[at]))
		}
		b.WriteString("\n")
	}

	// Add tips.
	tips := generateTips(reasonCounts)
	if len(tips) > 0 {
		b.WriteString("Tips:\n")
		for _, tip := range tips {
			b.WriteString(fmt.Sprintf("  %s\n", tip))
		}
	}

	return b.String(), nil
}
