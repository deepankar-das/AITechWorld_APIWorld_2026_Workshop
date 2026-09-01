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

// Developer group constants.
const (
	GroupPowerBuilder        = "power_builder"
	GroupCautiousContributor = "cautious_contributor"
	GroupToolExplorer        = "tool_explorer"
	GroupBoundaryTester      = "boundary_tester"
	GroupAutomationDriver    = "automation_driver"
	GroupDataAccessor        = "data_accessor"
	GroupNetworkHeavy        = "network_heavy"
	GroupNightOwl            = "night_owl"
	GroupNewJoiner           = "new_joiner"
	GroupDormantAgent        = "dormant_agent"
)

// groupMeta holds display metadata for each group.
type groupMeta struct {
	Name            string
	Icon            string
	Description     string
	SuggestedAction string
}

var groupMetadata = map[string]groupMeta{
	GroupPowerBuilder: {
		Name:            "Power Builder",
		Icon:            "zap",
		Description:     "High-velocity developers with excellent compliance. >200 actions/day, <2% block rate.",
		SuggestedAction: "Consider auto-approving routine operations for this group.",
	},
	GroupCautiousContributor: {
		Name:            "Compliant Developer",
		Icon:            "shield",
		Description:     "Moderate activity with full policy compliance. Zero governance blocks.",
		SuggestedAction: "Consider expanding permissions for this developer profile.",
	},
	GroupToolExplorer: {
		Name:            "Integration-Heavy",
		Icon:            "wrench",
		Description:     "Frequently uses MCP integrations or installs packages.",
		SuggestedAction: "Review integration usage patterns; consider curated tool allowlists.",
	},
	GroupBoundaryTester: {
		Name:            "High-Friction",
		Icon:            "alert-triangle",
		Description:     "Elevated block rate (>5%). May indicate policy misalignment or risk behavior.",
		SuggestedAction: "Review blocked actions for false positives or policy tuning opportunities.",
	},
	GroupAutomationDriver: {
		Name:            "Automation-Intensive",
		Icon:            "cpu",
		Description:     "Heavy shell automation usage. >100 actions/day, primarily shell.exec.",
		SuggestedAction: "Audit shell command patterns; consider command allowlists.",
	},
	GroupDataAccessor: {
		Name:            "Sensitive Data Access",
		Icon:            "database",
		Description:     "Frequently accesses credentials or sensitive data paths.",
		SuggestedAction: "Review credential access patterns and enforce least-privilege.",
	},
	GroupNetworkHeavy: {
		Name:            "High Network Activity",
		Icon:            "globe",
		Description:     "Connects to many distinct external hosts (breadth > 20).",
		SuggestedAction: "Review egress destinations; tighten network allowlists.",
	},
	GroupNightOwl: {
		Name:            "Off-Hours Active",
		Icon:            "moon",
		Description:     "Over 30% of activity occurs outside standard business hours.",
		SuggestedAction: "Consider time-based approval requirements for off-hours work.",
	},
	GroupNewJoiner: {
		Name:            "Recently Onboarded",
		Icon:            "user-plus",
		Description:     "Developer with less than 30 days of governed AI agent activity.",
		SuggestedAction: "Monitor initial compliance patterns and provide policy guidance.",
	},
	GroupDormantAgent: {
		Name:            "Inactive",
		Icon:            "pause",
		Description:     "Very low activity (<1 action/day over last 7 days).",
		SuggestedAction: "Review if agent sessions are still needed.",
	},
}

// DeveloperFeatures holds computed behavioural features for a developer.
type DeveloperFeatures struct {
	UserID          string  `json:"user_id"`
	ActionsPerDay   float64 `json:"actions_per_day"`
	BlockRate       float64 `json:"block_rate"`
	ApprovalRate    float64 `json:"approval_rate"`
	ActionDiversity int     `json:"action_diversity"`
	NetworkBreadth  int     `json:"network_breadth"`
	OffHoursPercent float64 `json:"off_hours_percent"`
	EvasionScore    int     `json:"evasion_score"`
	TenureDays      int     `json:"tenure_days"`
	TopActionType   string  `json:"top_action_type"`
}

// DeveloperGroup describes a behavioural cluster with summary statistics.
type DeveloperGroup struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Icon            string  `json:"icon"`
	Description     string  `json:"description"`
	MemberCount     int     `json:"member_count"`
	AvgBlockRate    float64 `json:"avg_block_rate"`
	SuggestedAction string  `json:"suggested_action"`
}

// GroupMember associates a developer with their computed features and group.
type GroupMember struct {
	UserID    string            `json:"user_id"`
	Features  DeveloperFeatures `json:"features"`
	JoinedGroup string          `json:"joined_group"`
}

// ClassifyDeveloper assigns a developer to a group based on threshold rules.
// The order of checks matters: more specific/risky groups are checked first.
func ClassifyDeveloper(features DeveloperFeatures) string {
	// NewJoiner takes priority for new developers.
	if features.TenureDays > 0 && features.TenureDays < 30 {
		return GroupNewJoiner
	}

	// DormantAgent: very low activity.
	if features.ActionsPerDay < 1 && features.ActionsPerDay >= 0 && features.TenureDays >= 30 {
		return GroupDormantAgent
	}

	// BoundaryTester: high block rate + evasion signals.
	if features.BlockRate > 5.0 && features.EvasionScore > 3 {
		return GroupBoundaryTester
	}

	// PowerBuilder: high velocity, low block rate.
	if features.ActionsPerDay > 200 && features.BlockRate < 2.0 {
		return GroupPowerBuilder
	}

	// AutomationDriver: heavy shell usage.
	if features.ActionsPerDay > 100 && features.TopActionType == string(types.ActionShellExec) {
		return GroupAutomationDriver
	}

	// CautiousContributor: moderate activity, zero blocks.
	if features.ActionsPerDay >= 20 && features.ActionsPerDay <= 50 && features.BlockRate == 0 {
		return GroupCautiousContributor
	}

	// NetworkHeavy: many distinct hosts.
	if features.NetworkBreadth > 20 {
		return GroupNetworkHeavy
	}

	// NightOwl: significant off-hours activity.
	if features.OffHoursPercent > 30 {
		return GroupNightOwl
	}

	// ToolExplorer: high MCP or package install activity.
	if features.TopActionType == string(types.ActionMcpInvoke) || features.TopActionType == string(types.ActionPackageInstall) {
		return GroupToolExplorer
	}

	// DataAccessor: high credential access.
	if features.TopActionType == string(types.ActionCredAccess) {
		return GroupDataAccessor
	}

	// Default: classify as CautiousContributor if nothing else matches.
	return GroupCautiousContributor
}

// ExtractFeatures computes per-developer behavioural features from audit events.
func ExtractFeatures(store audit.AuditStore, period string) ([]DeveloperFeatures, error) {
	from, to := periodToTimeRange(period)

	events, err := store.QueryEvents(audit.AuditQuery{
		TimeFrom: from,
		TimeTo:   to,
	})
	if err != nil {
		return nil, fmt.Errorf("query events for features: %w", err)
	}

	type devAccum struct {
		FirstSeen       time.Time
		LastSeen        time.Time
		TotalActions    int
		BlockedActions  int
		ApprovalActions int
		ActionTypes     map[string]int
		Hosts           map[string]bool
		OffHoursCount   int
		EvasionScore    int
	}

	devMap := make(map[string]*devAccum)

	for _, e := range events {
		uid := e.Actor.UserID
		acc, exists := devMap[uid]
		if !exists {
			acc = &devAccum{
				ActionTypes: make(map[string]int),
				Hosts:       make(map[string]bool),
			}
			devMap[uid] = acc
		}

		acc.TotalActions++
		acc.ActionTypes[e.Action.Type]++

		if e.Decision == string(types.DecisionDeny) {
			acc.BlockedActions++
		}
		if e.Decision == string(types.DecisionRequireApproval) {
			acc.ApprovalActions++
		}

		// Track network breadth.
		if e.Resource.Host != "" {
			acc.Hosts[e.Resource.Host] = true
		}

		// Parse timestamp for time-based features.
		ts, parseErr := time.Parse(time.RFC3339, e.Timestamp)
		if parseErr == nil {
			if acc.FirstSeen.IsZero() || ts.Before(acc.FirstSeen) {
				acc.FirstSeen = ts
			}
			if ts.After(acc.LastSeen) {
				acc.LastSeen = ts
			}

			// Off-hours: before 8am or after 6pm UTC.
			hour := ts.Hour()
			if hour < 8 || hour >= 18 {
				acc.OffHoursCount++
			}
		}

		// Evasion score: count bypass_attempt classifications.
		for _, c := range e.Resource.Classification {
			if c == string(types.ClassBypassAttempt) {
				acc.EvasionScore++
			}
		}
	}

	result := make([]DeveloperFeatures, 0, len(devMap))
	for uid, acc := range devMap {
		// Compute days active.
		var days float64
		if !acc.FirstSeen.IsZero() && !acc.LastSeen.IsZero() {
			days = acc.LastSeen.Sub(acc.FirstSeen).Hours() / 24.0
			if days < 1 {
				days = 1
			}
		} else {
			days = 1
		}

		actionsPerDay := float64(acc.TotalActions) / days

		var blockRate float64
		if acc.TotalActions > 0 {
			blockRate = float64(acc.BlockedActions) / float64(acc.TotalActions) * 100.0
		}

		var approvalRate float64
		if acc.TotalActions > 0 {
			approvalRate = float64(acc.ApprovalActions) / float64(acc.TotalActions) * 100.0
		}

		var offHoursPercent float64
		if acc.TotalActions > 0 {
			offHoursPercent = float64(acc.OffHoursCount) / float64(acc.TotalActions) * 100.0
		}

		// Find top action type.
		topAction := ""
		topCount := 0
		for at, c := range acc.ActionTypes {
			if c > topCount {
				topCount = c
				topAction = at
			}
		}

		tenureDays := int(days)
		if tenureDays < 1 {
			tenureDays = 1
		}

		result = append(result, DeveloperFeatures{
			UserID:          uid,
			ActionsPerDay:   actionsPerDay,
			BlockRate:       blockRate,
			ApprovalRate:    approvalRate,
			ActionDiversity: len(acc.ActionTypes),
			NetworkBreadth:  len(acc.Hosts),
			OffHoursPercent: offHoursPercent,
			EvasionScore:    acc.EvasionScore,
			TenureDays:      tenureDays,
			TopActionType:   topAction,
		})
	}

	// Sort by UserID for deterministic output.
	sort.Slice(result, func(i, j int) bool {
		return result[i].UserID < result[j].UserID
	})

	return result, nil
}

// GetGroups returns all developer groups with member counts and average block
// rates computed from current audit data.
func GetGroups(store audit.AuditStore) ([]DeveloperGroup, error) {
	features, err := ExtractFeatures(store, string(Period30d))
	if err != nil {
		return nil, fmt.Errorf("extract features: %w", err)
	}

	// Classify each developer and accumulate group stats.
	type groupAcc struct {
		MemberCount    int
		TotalBlockRate float64
	}

	groupStats := make(map[string]*groupAcc)
	// Initialise all groups so they appear even when empty.
	allGroupIDs := []string{
		GroupPowerBuilder, GroupCautiousContributor, GroupToolExplorer,
		GroupBoundaryTester, GroupAutomationDriver, GroupDataAccessor,
		GroupNetworkHeavy, GroupNightOwl, GroupNewJoiner, GroupDormantAgent,
	}
	for _, gid := range allGroupIDs {
		groupStats[gid] = &groupAcc{}
	}

	for _, f := range features {
		gid := ClassifyDeveloper(f)
		acc := groupStats[gid]
		acc.MemberCount++
		acc.TotalBlockRate += f.BlockRate
	}

	result := make([]DeveloperGroup, 0, len(allGroupIDs))
	for _, gid := range allGroupIDs {
		acc := groupStats[gid]
		meta := groupMetadata[gid]

		var avgBlock float64
		if acc.MemberCount > 0 {
			avgBlock = acc.TotalBlockRate / float64(acc.MemberCount)
		}

		result = append(result, DeveloperGroup{
			ID:              gid,
			Name:            meta.Name,
			Icon:            meta.Icon,
			Description:     meta.Description,
			MemberCount:     acc.MemberCount,
			AvgBlockRate:    avgBlock,
			SuggestedAction: meta.SuggestedAction,
		})
	}

	return result, nil
}

// GetGroupMembers returns all members of a specific group.
func GetGroupMembers(store audit.AuditStore, groupID string) ([]GroupMember, error) {
	features, err := ExtractFeatures(store, string(Period30d))
	if err != nil {
		return nil, fmt.Errorf("extract features: %w", err)
	}

	var members []GroupMember
	for _, f := range features {
		gid := ClassifyDeveloper(f)
		if gid == groupID {
			members = append(members, GroupMember{
				UserID:      f.UserID,
				Features:    f,
				JoinedGroup: gid,
			})
		}
	}

	if members == nil {
		members = []GroupMember{}
	}

	return members, nil
}
