/**
 * Author: Deepankar Das
 */

package routes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/anthropics/enforcer/internal/approval"
	"github.com/anthropics/enforcer/internal/audit"
	"github.com/anthropics/enforcer/internal/policy"
	"github.com/anthropics/enforcer/internal/types"
)

// HandleEvaluate processes a policy evaluation request. It validates the
// action request, checks enforcement state, checks approval scopes,
// evaluates policy, builds an audit event, and buffers it.
// enforcementEnabled is passed from the daemon to avoid import cycles.
func HandleEvaluate(body []byte, bundle *types.PolicyBundle, buffer *audit.AuditBuffer, approvalSvc *approval.ApprovalService, enforcementEnabled bool) (int, interface{}) {
	// Parse request.
	var req types.ActionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return http.StatusBadRequest, map[string]string{
			"error":   "invalid_json",
			"message": fmt.Sprintf("Failed to parse action request: %s", err.Error()),
		}
	}

	// Validate.
	if errs := types.ValidateActionRequest(&req); len(errs) > 0 {
		return http.StatusBadRequest, map[string]interface{}{
			"error":            "validation_failed",
			"validation_errors": errs,
		}
	}

	// Check enforcement state.
	if !enforcementEnabled {
		return http.StatusOK, map[string]interface{}{
			"request_id":   req.RequestID,
			"decision":     "allow",
			"reason_code":  "ENFORCEMENT_DISABLED",
			"reason_human": "Enforcement is currently disabled. Action allowed.",
			"enforcement":  false,
		}
	}

	// Check approval scopes for pre-approved actions.
	if scopeDecision := approvalSvc.CheckScope(req); scopeDecision != nil {
		decision := types.PolicyDecision{
			RequestID:        req.RequestID,
			Decision:         types.DecisionAllow,
			ReasonCode:       "SCOPE_APPROVED",
			ReasonHuman:      fmt.Sprintf("Action pre-approved by scope (approval %s)", scopeDecision.ApprovalID),
			PolicyID:         "system.scope_approval",
			PolicyVersion:    bundle.BundleVersion,
			ApprovalRequired: false,
		}
		buildAndBufferEvent(req, decision, buffer)
		return http.StatusOK, decision
	}

	// Evaluate policy.
	decision := policy.EvaluatePolicy(req, *bundle)

	// If approval is required, create an approval request.
	if decision.Decision == types.DecisionRequireApproval {
		approvalReq, _ := approvalSvc.CreateApproval(req, decision)
		buildAndBufferEvent(req, decision, buffer)
		return http.StatusOK, map[string]interface{}{
			"request_id":        decision.RequestID,
			"decision":          string(decision.Decision),
			"reason_code":       decision.ReasonCode,
			"reason_human":      decision.ReasonHuman,
			"policy_id":         decision.PolicyID,
			"policy_version":    decision.PolicyVersion,
			"approval_required": true,
			"approval_id":       approvalReq.ApprovalID,
			"timeout_seconds":   approvalReq.TimeoutSeconds,
		}
	}

	buildAndBufferEvent(req, decision, buffer)
	return http.StatusOK, decision
}

// buildAndBufferEvent constructs an audit event from the request and decision,
// then buffers it for later flush.
func buildAndBufferEvent(req types.ActionRequest, decision types.PolicyDecision, buffer *audit.AuditBuffer) {
	now := time.Now().UTC().Format(time.RFC3339)

	gateFields := audit.BuildGateFields(
		req.Actor,
		req.Actor.SessionID,
		req.Action,
		now,
		decision.PolicyID,
		decision.PolicyVersion,
		string(decision.Decision),
		string(decision.Decision),
	)

	classifications := make([]string, len(req.Resource.Classification))
	for i, c := range req.Resource.Classification {
		classifications[i] = string(c)
	}

	event := types.AuditEvent{
		EventID:       fmt.Sprintf("evt_%s_%d", req.RequestID, time.Now().UnixNano()),
		Timestamp:     now,
		SessionID:     req.Actor.SessionID,
		CorrelationID: req.RequestID,

		Who:      gateFields["who"],
		What:     gateFields["what"],
		When:     gateFields["when"],
		Policy:   gateFields["policy"],
		Decision: gateFields["decision"],
		Result:   gateFields["result"],
	}

	event.Actor.UserID = req.Actor.UserID
	event.Actor.AgentType = req.Actor.AgentType
	event.Actor.AgentInstance = req.Actor.AgentInstance

	event.Environment.Workspace = req.Environment.Workspace
	event.Environment.Repo = req.Environment.Repo
	event.Environment.Branch = req.Environment.Branch
	event.Environment.Tier = req.Environment.Tier
	event.Environment.DeploymentMode = req.Environment.DeploymentMode

	event.Action.Type = string(req.Action.Type)
	event.Action.AttemptedAction = req.Action.AttemptedAction
	// Set effect based on the policy decision — the event is created before
	// tool execution, so we know the outcome from the decision.
	switch decision.Decision {
	case types.DecisionAllow, types.DecisionAllowDegraded, types.DecisionSimulate:
		event.Action.ObservedEffect = "executed"
	case types.DecisionDeny:
		event.Action.ObservedEffect = "blocked"
	case types.DecisionRequireApproval:
		event.Action.ObservedEffect = "pending_approval"
	default:
		event.Action.ObservedEffect = "executed"
	}

	event.Resource.Kind = string(req.Resource.Kind)
	event.Resource.Path = req.Resource.Path
	event.Resource.Host = req.Resource.Host
	event.Resource.Value = req.Resource.Value
	event.Resource.Classification = classifications

	event.PolicyDetail.PolicyID = decision.PolicyID
	event.PolicyDetail.PolicyVersion = decision.PolicyVersion
	event.PolicyDetail.Decision = string(decision.Decision)
	event.PolicyDetail.ReasonCode = decision.ReasonCode
	event.PolicyDetail.ReasonHuman = decision.ReasonHuman

	buffer.BufferEvent(event)
}
