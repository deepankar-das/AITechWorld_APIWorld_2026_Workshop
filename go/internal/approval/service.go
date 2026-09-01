/**
 * Author: Deepankar Das
 */

package approval

import (
	"fmt"
	"sync"
	"time"

	"github.com/anthropics/enforcer/internal/types"
)

// ApprovalEventType enumerates the types of approval lifecycle events.
type ApprovalEventType string

const (
	EventApprovalCreated  ApprovalEventType = "approval_created"
	EventApprovalResolved ApprovalEventType = "approval_resolved"
	EventApprovalTimeout  ApprovalEventType = "approval_timeout"
	EventScopeMatched     ApprovalEventType = "scope_matched"
)

// ApprovalEvent is emitted for each lifecycle transition of an approval.
type ApprovalEvent struct {
	Type       ApprovalEventType      `json:"type"`
	ApprovalID string                 `json:"approval_id"`
	Timestamp  string                 `json:"timestamp"`
	Detail     map[string]interface{} `json:"detail,omitempty"`
}

// PendingEntry wraps an approval request with its resolution channel and
// timeout timer so the service can track in-flight approvals.
type PendingEntry struct {
	Request  types.ApprovalRequest  `json:"request"`
	ResultCh chan types.ApprovalDecision
	Timer    *time.Timer
}

// ApprovalMetrics provides aggregate statistics about the approval service.
type ApprovalMetrics struct {
	TotalCreated  int `json:"total_created"`
	TotalApproved int `json:"total_approved"`
	TotalDenied   int `json:"total_denied"`
	TotalExpired  int `json:"total_expired"`
	PendingCount  int `json:"pending_count"`
}

// ApprovalService manages the lifecycle of approval requests: creation,
// timeout, resolution, scope matching, and event emission.
type ApprovalService struct {
	mu              sync.Mutex
	pending         map[string]*PendingEntry
	resolved        map[string]types.ApprovalDecision
	scopes          []scopeEntry
	listeners       []func(ApprovalEvent)
	defaultTimeout  int
	defaultBehavior types.TimeoutBehavior

	// Metrics counters.
	totalCreated  int
	totalApproved int
	totalDenied   int
	totalExpired  int
}

// scopeEntry records an active approval scope together with the action
// type that was approved so we can match future requests.
type scopeEntry struct {
	ActionType string
	Scope      types.ApprovalScope
	Decision   types.ApprovalDecision
}

// NewApprovalService creates a new ApprovalService with the given defaults.
func NewApprovalService(defaultTimeout int, defaultBehavior types.TimeoutBehavior) *ApprovalService {
	return &ApprovalService{
		pending:         make(map[string]*PendingEntry),
		resolved:        make(map[string]types.ApprovalDecision),
		scopes:          make([]scopeEntry, 0),
		listeners:       make([]func(ApprovalEvent), 0),
		defaultTimeout:  defaultTimeout,
		defaultBehavior: defaultBehavior,
	}
}

// OnEvent registers a listener that will be called for every approval
// lifecycle event.
func (s *ApprovalService) OnEvent(listener func(ApprovalEvent)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listeners = append(s.listeners, listener)
}

// emitEvent notifies all registered listeners. Must NOT be called while
// holding s.mu to avoid deadlocks if a listener calls back into the service.
func (s *ApprovalService) emitEvent(event ApprovalEvent) {
	s.mu.Lock()
	listeners := make([]func(ApprovalEvent), len(s.listeners))
	copy(listeners, s.listeners)
	s.mu.Unlock()

	for _, fn := range listeners {
		fn(event)
	}
}

// CheckScope checks whether an incoming action request matches any active
// approval scope. If a match is found the corresponding ApprovalDecision
// is returned; otherwise nil.
func (s *ApprovalService) CheckScope(request types.ActionRequest) *types.ApprovalDecision {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, entry := range s.scopes {
		if MatchesScope(request, entry.Scope) {
			dec := entry.Decision
			// Emit scope_matched event (release lock first).
			s.mu.Unlock()
			s.emitEvent(ApprovalEvent{
				Type:       EventScopeMatched,
				ApprovalID: dec.ApprovalID,
				Timestamp:  time.Now().UTC().Format(time.RFC3339),
				Detail: map[string]interface{}{
					"request_id": request.RequestID,
					"scope_type": string(entry.Scope.Type),
				},
			})
			s.mu.Lock()

			// If single-use, remove the scope so it cannot match again.
			if entry.Scope.Type == types.ScopeSingle {
				s.scopes = append(s.scopes[:i], s.scopes[i+1:]...)
			}
			return &dec
		}
	}
	return nil
}

// CreateApproval builds an ApprovalRequest, starts the timeout timer, stores
// the pending entry, and returns the request together with a channel that
// will receive exactly one ApprovalDecision (either from a reviewer or from
// the timeout).
func (s *ApprovalService) CreateApproval(
	request types.ActionRequest,
	decision types.PolicyDecision,
) (*types.ApprovalRequest, <-chan types.ApprovalDecision) {
	approvalID := fmt.Sprintf("apr_%s_%d", request.RequestID, time.Now().UnixNano())
	now := time.Now().UTC().Format(time.RFC3339)

	timeout := s.defaultTimeout
	if timeout <= 0 {
		timeout = 300
	}

	approvalReq := types.ApprovalRequest{
		ApprovalID: approvalID,
		RequestID:  request.RequestID,
		ContextBundle: types.ContextBundle{
			Actor:         fmt.Sprintf("%s|%s", request.Actor.UserID, request.Actor.AgentType),
			Resource:      fmt.Sprintf("%s:%s%s", request.Resource.Kind, request.Resource.Path, request.Resource.Value),
			RiskRationale: decision.ReasonHuman,
			PolicyRule:    decision.PolicyID,
			AgentIdentity: request.Actor.AgentInstance,
			SessionSummary: request.Actor.SessionID,
		},
		TimeoutSeconds:  timeout,
		TimeoutBehavior: s.defaultBehavior,
		CreatedAt:       now,
	}

	ch := make(chan types.ApprovalDecision, 1)

	entry := &PendingEntry{
		Request:  approvalReq,
		ResultCh: ch,
	}

	// Start timeout timer.
	entry.Timer = time.AfterFunc(time.Duration(timeout)*time.Second, func() {
		s.handleTimeout(approvalID)
	})

	s.mu.Lock()
	s.pending[approvalID] = entry
	s.totalCreated++
	s.mu.Unlock()

	s.emitEvent(ApprovalEvent{
		Type:       EventApprovalCreated,
		ApprovalID: approvalID,
		Timestamp:  now,
		Detail: map[string]interface{}{
			"request_id":       request.RequestID,
			"timeout_seconds":  timeout,
			"timeout_behavior": string(s.defaultBehavior),
		},
	})

	return &approvalReq, ch
}

// handleTimeout fires when the approval timer expires.
func (s *ApprovalService) handleTimeout(approvalID string) {
	s.mu.Lock()
	entry, exists := s.pending[approvalID]
	if !exists {
		s.mu.Unlock()
		return
	}

	var dec string
	if s.defaultBehavior == types.TimeoutAllow {
		dec = "approve"
	} else {
		dec = "deny"
	}

	decision := types.ApprovalDecision{
		ApprovalID: approvalID,
		Decision:   dec,
		ApproverID: "system:timeout",
		Rationale:  fmt.Sprintf("Approval timed out after %d seconds (behavior: %s)", entry.Request.TimeoutSeconds, s.defaultBehavior),
	}

	delete(s.pending, approvalID)
	s.resolved[approvalID] = decision
	s.totalExpired++
	s.mu.Unlock()

	// Send decision on channel (buffered, won't block).
	entry.ResultCh <- decision
	close(entry.ResultCh)

	s.emitEvent(ApprovalEvent{
		Type:       EventApprovalTimeout,
		ApprovalID: approvalID,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Detail: map[string]interface{}{
			"decision":         dec,
			"timeout_behavior": string(s.defaultBehavior),
		},
	})
}

// ResolveApproval resolves a pending approval with the given decision.
// If the decision includes a scope it is registered for future matching.
func (s *ApprovalService) ResolveApproval(approvalID string, decision types.ApprovalDecision) error {
	s.mu.Lock()
	entry, exists := s.pending[approvalID]
	if !exists {
		s.mu.Unlock()
		return fmt.Errorf("approval not found: %s", approvalID)
	}

	// Stop the timeout timer.
	entry.Timer.Stop()

	decision.ApprovalID = approvalID
	delete(s.pending, approvalID)
	s.resolved[approvalID] = decision

	if decision.Decision == "approve" {
		s.totalApproved++
	} else {
		s.totalDenied++
	}

	// Register scope if provided and approved.
	if decision.Decision == "approve" && decision.Scope != nil {
		s.scopes = append(s.scopes, scopeEntry{
			ActionType: entry.Request.ContextBundle.PolicyRule,
			Scope:      *decision.Scope,
			Decision:   decision,
		})
	}
	s.mu.Unlock()

	// Send decision on channel.
	entry.ResultCh <- decision
	close(entry.ResultCh)

	s.emitEvent(ApprovalEvent{
		Type:       EventApprovalResolved,
		ApprovalID: approvalID,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Detail: map[string]interface{}{
			"decision":    decision.Decision,
			"approver_id": decision.ApproverID,
			"has_scope":   decision.Scope != nil,
		},
	})

	return nil
}

// GetApproval retrieves a pending entry by approval ID. Returns nil if not
// found.
func (s *ApprovalService) GetApproval(approvalID string) *PendingEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pending[approvalID]
}

// ApprovalStatus represents the current state of an approval request.
type ApprovalStatus struct {
	ApprovalID string `json:"approval_id"`
	Status     string `json:"status"` // "pending", "approved", "denied"
	ApproverID string `json:"approver_id,omitempty"`
	Rationale  string `json:"rationale,omitempty"`
}

// GetApprovalStatus returns the status of an approval by ID, checking both
// pending and resolved maps.
func (s *ApprovalService) GetApprovalStatus(approvalID string) *ApprovalStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.pending[approvalID]; exists {
		return &ApprovalStatus{
			ApprovalID: approvalID,
			Status:     "pending",
		}
	}

	if dec, exists := s.resolved[approvalID]; exists {
		status := "denied"
		if dec.Decision == "approve" {
			status = "approved"
		}
		return &ApprovalStatus{
			ApprovalID: approvalID,
			Status:     status,
			ApproverID: dec.ApproverID,
			Rationale:  dec.Rationale,
		}
	}

	return nil
}

// GetPending returns all currently pending approval requests.
func (s *ApprovalService) GetPending() []types.ApprovalRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]types.ApprovalRequest, 0, len(s.pending))
	for _, entry := range s.pending {
		result = append(result, entry.Request)
	}
	return result
}

// GetMetrics returns aggregate approval service metrics.
func (s *ApprovalService) GetMetrics() ApprovalMetrics {
	s.mu.Lock()
	defer s.mu.Unlock()

	return ApprovalMetrics{
		TotalCreated:  s.totalCreated,
		TotalApproved: s.totalApproved,
		TotalDenied:   s.totalDenied,
		TotalExpired:  s.totalExpired,
		PendingCount:  len(s.pending),
	}
}
