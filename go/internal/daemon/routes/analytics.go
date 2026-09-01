/**
 * Author: Deepankar Das
 */

package routes

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/anthropics/enforcer/internal/analytics"
	"github.com/anthropics/enforcer/internal/audit"
	"github.com/anthropics/enforcer/internal/types"
)

// HandleBlockedOperations returns stack-ranked blocked operations for the
// requested period. Query parameter: period (today, 7d, 30d).
func HandleBlockedOperations(query url.Values, store audit.AuditStore) (int, interface{}) {
	period := query.Get("period")
	if period == "" {
		period = "7d"
	}

	ops, err := analytics.GetBlockedOperations(store, period)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{
			"error":   "blocked_operations_failed",
			"message": err.Error(),
		}
	}

	return http.StatusOK, map[string]interface{}{
		"operations":         ops,
		"blocked_operations": ops, // backward compatibility for older clients
		"period":             period,
		"count":              len(ops),
	}
}

// HandleApprovalBottlenecks returns operations with the most pending
// approvals and highest average wait times.
func HandleApprovalBottlenecks(store audit.AuditStore) (int, interface{}) {
	bottlenecks, err := analytics.GetApprovalBottlenecks(store)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{
			"error":   "approval_bottlenecks_failed",
			"message": err.Error(),
		}
	}

	return http.StatusOK, map[string]interface{}{
		"bottlenecks": bottlenecks,
		"count":       len(bottlenecks),
	}
}

// HandleDeveloperImpact returns per-developer block rates and top block
// reasons. Query parameter: period (today, 7d, 30d).
func HandleDeveloperImpact(query url.Values, store audit.AuditStore) (int, interface{}) {
	period := query.Get("period")
	if period == "" {
		period = "7d"
	}

	impact, err := analytics.GetDeveloperImpact(store, period)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{
			"error":   "developer_impact_failed",
			"message": err.Error(),
		}
	}

	return http.StatusOK, map[string]interface{}{
		"developers": impact,
		"period":     period,
		"count":      len(impact),
	}
}

// HandleGetGroups returns all developer behavioural groups with member
// counts and average block rates.
func HandleGetGroups(store audit.AuditStore) (int, interface{}) {
	groups, err := analytics.GetGroups(store)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{
			"error":   "groups_failed",
			"message": err.Error(),
		}
	}

	return http.StatusOK, map[string]interface{}{
		"groups": groups,
		"count":  len(groups),
	}
}

// HandleGetGroupMembers returns the members of a specific developer group.
func HandleGetGroupMembers(groupID string, store audit.AuditStore) (int, interface{}) {
	if groupID == "" {
		return http.StatusBadRequest, map[string]string{
			"error":   "missing_group_id",
			"message": "Group ID is required.",
		}
	}

	members, err := analytics.GetGroupMembers(store, groupID)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{
			"error":   "group_members_failed",
			"message": err.Error(),
		}
	}

	return http.StatusOK, map[string]interface{}{
		"group_id": groupID,
		"members":  members,
		"count":    len(members),
	}
}

// HandleGetRecommendations generates and returns policy recommendations
// based on current audit patterns and developer groups.
func HandleGetRecommendations(store audit.AuditStore) (int, interface{}) {
	groups, err := analytics.GetGroups(store)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{
			"error":   "groups_failed",
			"message": err.Error(),
		}
	}

	recs, err := analytics.GenerateRecommendations(store, groups)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{
			"error":   "recommendations_failed",
			"message": err.Error(),
		}
	}

	return http.StatusOK, map[string]interface{}{
		"recommendations": recs,
		"count":           len(recs),
	}
}

// applyRecommendationRequest is the JSON body for applying a recommendation.
type applyRecommendationRequest struct {
	Confirm bool `json:"confirm"`
}

// HandleApplyRecommendation applies a specific recommendation to the policy
// bundle.
func HandleApplyRecommendation(recID string, body []byte, bundle *types.PolicyBundle) (int, interface{}) {
	if recID == "" {
		return http.StatusBadRequest, map[string]string{
			"error":   "missing_recommendation_id",
			"message": "Recommendation ID is required.",
		}
	}

	var req applyRecommendationRequest
	if len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			return http.StatusBadRequest, map[string]string{
				"error":   "invalid_json",
				"message": "Failed to parse request body: " + err.Error(),
			}
		}
	}

	if !req.Confirm {
		return http.StatusBadRequest, map[string]string{
			"error":   "confirmation_required",
			"message": "Set confirm=true to apply this recommendation.",
		}
	}

	if err := analytics.ApplyRecommendation(recID, bundle); err != nil {
		return http.StatusInternalServerError, map[string]string{
			"error":   "apply_recommendation_failed",
			"message": err.Error(),
		}
	}

	return http.StatusOK, map[string]interface{}{
		"recommendation_id": recID,
		"applied":           true,
		"rule_count":        len(bundle.Rules),
	}
}

// HandleDeveloperScorecard returns a compliance scorecard for a specific
// developer.
func HandleDeveloperScorecard(userID string, store audit.AuditStore) (int, interface{}) {
	if userID == "" {
		return http.StatusBadRequest, map[string]string{
			"error":   "missing_user_id",
			"message": "User ID is required.",
		}
	}

	scorecard, err := analytics.GetDeveloperScorecard(store, userID)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{
			"error":   "scorecard_failed",
			"message": err.Error(),
		}
	}

	return http.StatusOK, scorecard
}

// HandleDeveloperTrends returns trend data for a specific developer,
// including weekly digest and block guidance for recent blocks.
func HandleDeveloperTrends(userID string, store audit.AuditStore) (int, interface{}) {
	if userID == "" {
		return http.StatusBadRequest, map[string]string{
			"error":   "missing_user_id",
			"message": "User ID is required.",
		}
	}

	digest, err := analytics.GenerateWeeklyDigest(store, userID)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{
			"error":   "digest_failed",
			"message": err.Error(),
		}
	}

	// Get recent blocks for guidance.
	recentBlocks, err := store.QueryEvents(audit.AuditQuery{
		ActorUserID: userID,
		Decision:    string(types.DecisionDeny),
		Limit:       5,
	})
	if err != nil {
		return http.StatusInternalServerError, map[string]string{
			"error":   "recent_blocks_failed",
			"message": err.Error(),
		}
	}

	// Collect unique reason codes and get guidance for each.
	seenReasons := make(map[string]bool)
	var guidanceList []interface{}

	for _, e := range recentBlocks {
		rc := e.PolicyDetail.ReasonCode
		if rc == "" || seenReasons[rc] {
			continue
		}
		seenReasons[rc] = true

		guidance, gErr := analytics.GetBlockGuidance(rc, userID, store)
		if gErr == nil {
			guidanceList = append(guidanceList, guidance)
		}
	}

	return http.StatusOK, map[string]interface{}{
		"user_id":        userID,
		"weekly_digest":  digest,
		"block_guidance": guidanceList,
	}
}
