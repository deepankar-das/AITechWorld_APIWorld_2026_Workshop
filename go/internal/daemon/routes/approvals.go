/**
 * Author: Deepankar Das
 */

package routes

import (
	"encoding/json"
	"net/http"

	"github.com/anthropics/enforcer/internal/approval"
	"github.com/anthropics/enforcer/internal/types"
)

// HandleGetPending returns all currently pending approval requests.
func HandleGetPending(svc *approval.ApprovalService) (int, interface{}) {
	pending := svc.GetPending()
	return http.StatusOK, map[string]interface{}{
		"approvals": pending,
		"count":     len(pending),
	}
}

// HandleResolveApproval resolves a pending approval with the given decision.
func HandleResolveApproval(approvalID string, body []byte, svc *approval.ApprovalService) (int, interface{}) {
	if approvalID == "" {
		return http.StatusBadRequest, map[string]string{
			"error":   "missing_approval_id",
			"message": "Approval ID is required in the URL path.",
		}
	}

	var decision types.ApprovalDecision
	if err := json.Unmarshal(body, &decision); err != nil {
		return http.StatusBadRequest, map[string]string{
			"error":   "invalid_json",
			"message": "Failed to parse approval decision: " + err.Error(),
		}
	}

	// Validate decision field.
	if decision.Decision != "approve" && decision.Decision != "deny" {
		return http.StatusBadRequest, map[string]string{
			"error":   "invalid_decision",
			"message": "Decision must be 'approve' or 'deny'.",
		}
	}

	if err := svc.ResolveApproval(approvalID, decision); err != nil {
		return http.StatusNotFound, map[string]string{
			"error":   "approval_not_found",
			"message": err.Error(),
		}
	}

	return http.StatusOK, map[string]interface{}{
		"approval_id": approvalID,
		"decision":    decision.Decision,
		"resolved":    true,
	}
}

// HandleGetApprovalStatus returns the status of a single approval by ID.
// The hook handler polls this endpoint while waiting for admin approval.
func HandleGetApprovalStatus(approvalID string, svc *approval.ApprovalService) (int, interface{}) {
	if approvalID == "" {
		return http.StatusBadRequest, map[string]string{
			"error":   "missing_approval_id",
			"message": "Approval ID is required in the URL path.",
		}
	}

	status := svc.GetApprovalStatus(approvalID)
	if status == nil {
		return http.StatusNotFound, map[string]string{
			"error":   "approval_not_found",
			"message": "No approval found with ID: " + approvalID,
		}
	}

	return http.StatusOK, status
}

// HandleGetApprovalMetrics returns aggregate approval service metrics.
func HandleGetApprovalMetrics(svc *approval.ApprovalService) (int, interface{}) {
	metrics := svc.GetMetrics()
	return http.StatusOK, metrics
}
