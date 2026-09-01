/**
 * Author: Deepankar Das
 */

package routes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/anthropics/enforcer/internal/audit"
	"github.com/anthropics/enforcer/internal/types"
)

// EnrichRequest is the payload for the audit enrich endpoint.
type EnrichRequest struct {
	SessionID      string `json:"session_id"`
	Tool           string `json:"tool"`
	ObservedEffect string `json:"observed_effect"`
	Timestamp      string `json:"timestamp"`
}

// HandleEnrich creates an append-only enrichment event linked to the original
// pending event by correlation_id. The original event is NEVER mutated —
// forensic immutability is preserved.
func HandleEnrich(body []byte, store audit.AuditStore) (int, interface{}) {
	var req EnrichRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return http.StatusBadRequest, map[string]string{
			"error":   "invalid_json",
			"message": "Failed to parse enrich request: " + err.Error(),
		}
	}

	if req.SessionID == "" {
		return http.StatusBadRequest, map[string]string{
			"error":   "missing_session_id",
			"message": "session_id is required.",
		}
	}

	if req.ObservedEffect == "" {
		return http.StatusBadRequest, map[string]string{
			"error":   "missing_observed_effect",
			"message": "observed_effect is required.",
		}
	}

	// Find the most recent pending event for this session to link to
	events, err := store.GetSession(req.SessionID)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{
			"error":   "session_query_failed",
			"message": err.Error(),
		}
	}

	var parentEventID string
	var parentAction string
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Action.ObservedEffect == "pending" {
			parentEventID = events[i].EventID
			parentAction = events[i].Action.AttemptedAction
			break
		}
	}

	if parentEventID == "" {
		return http.StatusNotFound, map[string]string{
			"error":   "no_pending_event",
			"message": "No pending event found for session " + req.SessionID,
		}
	}

	// Create a new append-only enrichment event linked by correlation_id
	now := time.Now().UTC().Format(time.RFC3339)
	enrichmentEvent := types.AuditEvent{
		EventID:       fmt.Sprintf("enr_%s_%d", parentEventID, time.Now().UnixNano()),
		Timestamp:     now,
		SessionID:     req.SessionID,
		CorrelationID: parentEventID, // Links to the original pending event

		Who:      "system:enrichment",
		What:     fmt.Sprintf("enrichment:%s", req.Tool),
		When:     now,
		Policy:   "system:post_execution",
		Decision: "enrichment:observed_effect",
		Result:   req.ObservedEffect,
	}

	enrichmentEvent.Action.Type = "enrichment"
	enrichmentEvent.Action.AttemptedAction = parentAction
	enrichmentEvent.Action.ObservedEffect = req.ObservedEffect

	enrichmentEvent.Resource.Kind = "enrichment"
	enrichmentEvent.Resource.Value = req.Tool

	enrichmentEvent.PolicyDetail.PolicyID = "system.enrichment"
	enrichmentEvent.PolicyDetail.Decision = "enrichment"
	enrichmentEvent.PolicyDetail.ReasonCode = "POST_EXECUTION_ENRICHMENT"
	enrichmentEvent.PolicyDetail.ReasonHuman = fmt.Sprintf("Post-execution enrichment for %s: %s", req.Tool, req.ObservedEffect)

	if err := store.StoreEvent(enrichmentEvent); err != nil {
		return http.StatusInternalServerError, map[string]string{
			"error":   "store_failed",
			"message": err.Error(),
		}
	}

	return http.StatusOK, map[string]interface{}{
		"enrichment_event_id": enrichmentEvent.EventID,
		"parent_event_id":     parentEventID,
		"observed_effect":     req.ObservedEffect,
		"append_only":         true,
	}
}
