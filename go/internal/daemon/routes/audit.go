/**
 * Author: Deepankar Das
 */

package routes

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/anthropics/enforcer/internal/audit"
	"github.com/anthropics/enforcer/internal/types"
)

// HandleQueryEvents retrieves audit events matching the query parameters.
func HandleQueryEvents(query url.Values, store audit.AuditStore) (int, interface{}) {
	q := audit.AuditQuery{
		SessionID:   query.Get("session_id"),
		ActorUserID: query.Get("actor_user_id"),
		ActionType:  query.Get("action_type"),
		Decision:    query.Get("decision"),
		PolicyID:    query.Get("policy_id"),
		ReasonCode:  query.Get("reason_code"),
		TimeFrom:    query.Get("time_from"),
		TimeTo:      query.Get("time_to"),
	}

	if limitStr := query.Get("limit"); limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil {
			q.Limit = n
		}
	}
	if offsetStr := query.Get("offset"); offsetStr != "" {
		if n, err := strconv.Atoi(offsetStr); err == nil {
			q.Offset = n
		}
	}

	events, err := store.QueryEvents(q)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{
			"error":   "query_failed",
			"message": err.Error(),
		}
	}

	if events == nil {
		events = []types.AuditEvent{}
	}

	return http.StatusOK, map[string]interface{}{
		"events": events,
		"count":  len(events),
	}
}

// HandleGetSessions returns summaries for all known audit sessions.
func HandleGetSessions(store audit.AuditStore) (int, interface{}) {
	sessions, err := store.GetSessions()
	if err != nil {
		return http.StatusInternalServerError, map[string]string{
			"error":   "sessions_query_failed",
			"message": err.Error(),
		}
	}

	return http.StatusOK, map[string]interface{}{
		"sessions": sessions,
		"count":    len(sessions),
	}
}

// HandleGetSession returns all events for a single session.
func HandleGetSession(sessionID string, store audit.AuditStore) (int, interface{}) {
	if sessionID == "" {
		return http.StatusBadRequest, map[string]string{
			"error":   "missing_session_id",
			"message": "Session ID is required.",
		}
	}

	events, err := store.GetSession(sessionID)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{
			"error":   "session_query_failed",
			"message": err.Error(),
		}
	}

	return http.StatusOK, map[string]interface{}{
		"session_id":  sessionID,
		"events":      events,
		"event_count": len(events),
	}
}

// HandleExportEvents exports audit events matching the query with metadata.
func HandleExportEvents(query url.Values, store audit.AuditStore) (int, interface{}) {
	q := audit.AuditQuery{
		SessionID:   query.Get("session_id"),
		ActorUserID: query.Get("actor_user_id"),
		ActionType:  query.Get("action_type"),
		Decision:    query.Get("decision"),
		PolicyID:    query.Get("policy_id"),
		ReasonCode:  query.Get("reason_code"),
		TimeFrom:    query.Get("time_from"),
		TimeTo:      query.Get("time_to"),
	}

	if limitStr := query.Get("limit"); limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil {
			q.Limit = n
		}
	}

	result, err := store.ExportEvents(q)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{
			"error":   "export_failed",
			"message": err.Error(),
		}
	}

	return http.StatusOK, result
}

// HandleAuditMetrics returns aggregate audit store metrics.
func HandleAuditMetrics(store audit.AuditStore) (int, interface{}) {
	metrics, err := store.GetMetrics()
	if err != nil {
		return http.StatusInternalServerError, map[string]string{
			"error":   "metrics_failed",
			"message": err.Error(),
		}
	}

	return http.StatusOK, metrics
}
