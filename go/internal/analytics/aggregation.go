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

// AnalyticsPeriod defines the time window for analytics queries.
type AnalyticsPeriod string

const (
	PeriodToday AnalyticsPeriod = "today"
	Period7d    AnalyticsPeriod = "7d"
	Period30d   AnalyticsPeriod = "30d"
)

// BlockedOperation represents a stack-ranked blocked action pattern.
type BlockedOperation struct {
	ActionType    string   `json:"action_type"`
	ReasonCode    string   `json:"reason_code"`
	PolicyID      string   `json:"policy_id"`
	Count         int      `json:"count"`
	Trend         float64  `json:"trend"`
	TopDevelopers []string `json:"top_developers"`
}

// ApprovalBottleneck identifies operations with slow or congested approval flows.
type ApprovalBottleneck struct {
	ActionType     string  `json:"action_type"`
	PolicyID       string  `json:"policy_id"`
	AvgWaitSeconds float64 `json:"avg_wait_seconds"`
	PendingCount   int     `json:"pending_count"`
	ReasonCode     string  `json:"reason_code"`
}

// DeveloperImpact summarises per-developer enforcement impact.
type DeveloperImpact struct {
	UserID          string   `json:"user_id"`
	AgentType       string   `json:"agent_type"`
	TotalActions    int      `json:"total_actions"`
	BlockedActions  int      `json:"blocked_actions"`
	BlockRate       float64  `json:"block_rate"`
	Group           string   `json:"group"`
	TopBlockReasons []string `json:"top_block_reasons"`
}

// periodToTimeRange converts a period string to a (from, to) pair in RFC3339.
func periodToTimeRange(period string) (string, string) {
	now := time.Now().UTC()
	to := now.Format(time.RFC3339)
	var from time.Time

	switch AnalyticsPeriod(period) {
	case PeriodToday:
		from = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	case Period7d:
		from = now.AddDate(0, 0, -7)
	case Period30d:
		from = now.AddDate(0, 0, -30)
	default:
		from = now.AddDate(0, 0, -7)
	}

	return from.Format(time.RFC3339), to
}

// priorPeriodRange returns the equivalent time range immediately before the
// given period (e.g. the 7 days before the current 7-day window).
func priorPeriodRange(period string) (string, string) {
	now := time.Now().UTC()
	var dur time.Duration

	switch AnalyticsPeriod(period) {
	case PeriodToday:
		dur = 24 * time.Hour
	case Period7d:
		dur = 7 * 24 * time.Hour
	case Period30d:
		dur = 30 * 24 * time.Hour
	default:
		dur = 7 * 24 * time.Hour
	}

	periodEnd := now.Add(-dur)
	periodStart := periodEnd.Add(-dur)
	return periodStart.Format(time.RFC3339), periodEnd.Format(time.RFC3339)
}

// blockKey creates a composite key for grouping blocked operations.
type blockKey struct {
	ActionType string
	ReasonCode string
	PolicyID   string
}

// GetBlockedOperations queries deny events and returns a stack-ranked list
// grouped by action_type + reason_code, sorted by count descending.
func GetBlockedOperations(store audit.AuditStore, period string) ([]BlockedOperation, error) {
	from, to := periodToTimeRange(period)

	events, err := store.QueryEvents(audit.AuditQuery{
		Decision: string(types.DecisionDeny),
		TimeFrom: from,
		TimeTo:   to,
	})
	if err != nil {
		return nil, fmt.Errorf("query blocked events: %w", err)
	}

	// Also query the prior period for trend calculation.
	priorFrom, priorTo := priorPeriodRange(period)
	priorEvents, err := store.QueryEvents(audit.AuditQuery{
		Decision: string(types.DecisionDeny),
		TimeFrom: priorFrom,
		TimeTo:   priorTo,
	})
	if err != nil {
		return nil, fmt.Errorf("query prior period events: %w", err)
	}

	// Group current period.
	counts := make(map[blockKey]int)
	devSets := make(map[blockKey]map[string]int)
	for _, e := range events {
		k := blockKey{
			ActionType: e.Action.Type,
			ReasonCode: e.PolicyDetail.ReasonCode,
			PolicyID:   e.PolicyDetail.PolicyID,
		}
		counts[k]++
		if devSets[k] == nil {
			devSets[k] = make(map[string]int)
		}
		devSets[k][e.Actor.UserID]++
	}

	// Group prior period.
	priorCounts := make(map[blockKey]int)
	for _, e := range priorEvents {
		k := blockKey{
			ActionType: e.Action.Type,
			ReasonCode: e.PolicyDetail.ReasonCode,
			PolicyID:   e.PolicyDetail.PolicyID,
		}
		priorCounts[k]++
	}

	// Build result.
	result := make([]BlockedOperation, 0, len(counts))
	for k, count := range counts {
		// Compute trend as percentage change vs prior period.
		prior := priorCounts[k]
		var trend float64
		if prior > 0 {
			trend = float64(count-prior) / float64(prior) * 100.0
		} else if count > 0 {
			trend = 100.0 // new pattern, 100% increase
		}

		// Top developers sorted by block count.
		topDevs := topNKeys(devSets[k], 5)

		result = append(result, BlockedOperation{
			ActionType:    k.ActionType,
			ReasonCode:    k.ReasonCode,
			PolicyID:      k.PolicyID,
			Count:         count,
			Trend:         trend,
			TopDevelopers: topDevs,
		})
	}

	// Sort by count descending.
	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})

	return result, nil
}

// GetApprovalBottlenecks finds operations with the most require_approval
// decisions and computes average wait times.
func GetApprovalBottlenecks(store audit.AuditStore) ([]ApprovalBottleneck, error) {
	events, err := store.QueryEvents(audit.AuditQuery{
		Decision: string(types.DecisionRequireApproval),
	})
	if err != nil {
		return nil, fmt.Errorf("query approval events: %w", err)
	}

	type bottleneckAcc struct {
		PolicyID       string
		ReasonCode     string
		TotalWait      float64
		ResolvedCount  int
		PendingCount   int
	}

	groups := make(map[string]*bottleneckAcc)

	for _, e := range events {
		key := e.Action.Type
		acc, exists := groups[key]
		if !exists {
			acc = &bottleneckAcc{
				PolicyID:   e.PolicyDetail.PolicyID,
				ReasonCode: e.PolicyDetail.ReasonCode,
			}
			groups[key] = acc
		}

		if e.Approval != nil {
			if e.Approval.Status == types.ApprovalPending {
				acc.PendingCount++
			} else if e.Approval.RequestedAt != "" && e.Approval.ResolvedAt != "" {
				requested, err1 := time.Parse(time.RFC3339, e.Approval.RequestedAt)
				resolved, err2 := time.Parse(time.RFC3339, e.Approval.ResolvedAt)
				if err1 == nil && err2 == nil {
					acc.TotalWait += resolved.Sub(requested).Seconds()
					acc.ResolvedCount++
				}
			}
		} else {
			// No approval record yet means it is pending.
			acc.PendingCount++
		}
	}

	result := make([]ApprovalBottleneck, 0, len(groups))
	for actionType, acc := range groups {
		var avgWait float64
		if acc.ResolvedCount > 0 {
			avgWait = acc.TotalWait / float64(acc.ResolvedCount)
		}
		result = append(result, ApprovalBottleneck{
			ActionType:     actionType,
			PolicyID:       acc.PolicyID,
			AvgWaitSeconds: avgWait,
			PendingCount:   acc.PendingCount,
			ReasonCode:     acc.ReasonCode,
		})
	}

	// Sort by pending count descending.
	sort.Slice(result, func(i, j int) bool {
		return result[i].PendingCount > result[j].PendingCount
	})

	return result, nil
}

// GetDeveloperImpact computes per-developer block rates and top block reasons.
func GetDeveloperImpact(store audit.AuditStore, period string) ([]DeveloperImpact, error) {
	from, to := periodToTimeRange(period)

	allEvents, err := store.QueryEvents(audit.AuditQuery{
		TimeFrom: from,
		TimeTo:   to,
	})
	if err != nil {
		return nil, fmt.Errorf("query all events: %w", err)
	}

	type devAcc struct {
		AgentType    string
		Total        int
		Blocked      int
		ReasonCounts map[string]int
	}

	devs := make(map[string]*devAcc)

	for _, e := range allEvents {
		uid := e.Actor.UserID
		acc, exists := devs[uid]
		if !exists {
			acc = &devAcc{
				AgentType:    e.Actor.AgentType,
				ReasonCounts: make(map[string]int),
			}
			devs[uid] = acc
		}
		acc.Total++
		if e.Decision == string(types.DecisionDeny) {
			acc.Blocked++
			if e.PolicyDetail.ReasonCode != "" {
				acc.ReasonCounts[e.PolicyDetail.ReasonCode]++
			}
		}
	}

	result := make([]DeveloperImpact, 0, len(devs))
	for uid, acc := range devs {
		var blockRate float64
		if acc.Total > 0 {
			blockRate = float64(acc.Blocked) / float64(acc.Total) * 100.0
		}

		topReasons := topNKeys(acc.ReasonCounts, 3)

		group := ClassifyDeveloper(DeveloperFeatures{
			UserID:    uid,
			BlockRate: blockRate,
		})

		result = append(result, DeveloperImpact{
			UserID:          uid,
			AgentType:       acc.AgentType,
			TotalActions:    acc.Total,
			BlockedActions:  acc.Blocked,
			BlockRate:       blockRate,
			Group:           group,
			TopBlockReasons: topReasons,
		})
	}

	// Sort by block rate descending.
	sort.Slice(result, func(i, j int) bool {
		return result[i].BlockRate > result[j].BlockRate
	})

	return result, nil
}

// topNKeys returns the top N keys from a map[string]int, sorted by value
// descending.
func topNKeys(m map[string]int, n int) []string {
	type kv struct {
		Key   string
		Value int
	}

	pairs := make([]kv, 0, len(m))
	for k, v := range m {
		pairs = append(pairs, kv{k, v})
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].Value > pairs[j].Value
	})

	result := make([]string, 0, n)
	for i := 0; i < n && i < len(pairs); i++ {
		result = append(result, pairs[i].Key)
	}
	return result
}
