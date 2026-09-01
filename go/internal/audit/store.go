/**
 * Author: Deepankar Das
 */

package audit

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/anthropics/enforcer/internal/types"
)

// AuditQuery defines the filter criteria for querying audit events.
type AuditQuery struct {
	SessionID   string
	ActorUserID string
	ActionType  string
	Decision    string
	PolicyID    string
	ReasonCode  string
	TimeFrom    string
	TimeTo      string
	Limit       int
	Offset      int
}

// ExportMetadata contains metadata about an audit export.
type ExportMetadata struct {
	ExportedAt string `json:"exported_at"`
	EventCount int    `json:"event_count"`
	Query      string `json:"query"`
}

// ExportResult wraps exported events with metadata.
type ExportResult struct {
	Metadata ExportMetadata    `json:"metadata"`
	Events   []types.AuditEvent `json:"events"`
}

// StoreMetrics provides aggregate statistics about the store.
type StoreMetrics struct {
	TotalEvents    int            `json:"total_events"`
	SessionCount   int            `json:"session_count"`
	DecisionCounts map[string]int `json:"decision_counts"`
}

// AuditStore defines the interface for audit event persistence.
type AuditStore interface {
	// StoreEvent persists a single audit event. Returns an error if the event
	// fails validation.
	StoreEvent(event types.AuditEvent) error

	// StoreEvents persists a batch of audit events. Returns the count of
	// successfully stored events and any error.
	StoreEvents(events []types.AuditEvent) (int, error)

	// QueryEvents retrieves events matching the given query.
	QueryEvents(query AuditQuery) ([]types.AuditEvent, error)

	// GetSession returns all events for a given session in chronological order.
	GetSession(sessionID string) ([]types.AuditEvent, error)

	// GetSessions returns summaries for all known sessions.
	GetSessions() ([]types.SessionSummary, error)

	// ExportEvents exports events matching the query with metadata.
	ExportEvents(query AuditQuery) (*ExportResult, error)

	// GetMetrics returns aggregate statistics.
	GetMetrics() (*StoreMetrics, error)

	// GetCount returns the total number of stored events.
	GetCount() int
}

// InMemoryStore is an in-memory implementation of AuditStore.
// It uses a slice for storage and a sync.RWMutex for concurrent access.
// Suitable for prototyping; PostgreSQL-backed store can be added later.
type InMemoryStore struct {
	mu     sync.RWMutex
	events []types.AuditEvent
	index  map[string]int // eventID -> slice index for O(1) lookups
}

// NewInMemoryStore creates a new empty in-memory audit store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		events: make([]types.AuditEvent, 0, 256),
		index:  make(map[string]int),
	}
}

// StoreEvent persists a single audit event after validation.
func (s *InMemoryStore) StoreEvent(event types.AuditEvent) error {
	valid, missing := ValidateAuditEvent(event)
	if !valid {
		return fmt.Errorf("audit event failed gate validation: missing fields %v", missing)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.index[event.EventID] = len(s.events)
	s.events = append(s.events, event)
	return nil
}

// StoreEvents persists a batch of audit events. Events that fail validation
// are skipped. Returns the count of stored events and any error for the
// first failed event.
func (s *InMemoryStore) StoreEvents(events []types.AuditEvent) (int, error) {
	stored := 0
	var firstErr error

	for _, event := range events {
		err := s.StoreEvent(event)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		stored++
	}

	return stored, firstErr
}

// QueryEvents retrieves events matching the given query filters.
func (s *InMemoryStore) QueryEvents(query AuditQuery) ([]types.AuditEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []types.AuditEvent

	for _, event := range s.events {
		if !matchesQuery(event, query) {
			continue
		}
		results = append(results, event)
	}

	// Sort chronologically by timestamp.
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp < results[j].Timestamp
	})

	// Apply offset.
	if query.Offset > 0 {
		if query.Offset >= len(results) {
			return nil, nil
		}
		results = results[query.Offset:]
	}

	// Apply limit.
	if query.Limit > 0 && query.Limit < len(results) {
		results = results[:query.Limit]
	}

	return results, nil
}

// GetSession returns all events for a session in chronological order.
func (s *InMemoryStore) GetSession(sessionID string) ([]types.AuditEvent, error) {
	return s.QueryEvents(AuditQuery{SessionID: sessionID})
}

// GetSessions returns summaries for all known sessions.
func (s *InMemoryStore) GetSessions() ([]types.SessionSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessionMap := make(map[string]*types.SessionSummary)

	for _, event := range s.events {
		sid := event.SessionID
		summary, exists := sessionMap[sid]
		if !exists {
			summary = &types.SessionSummary{
				SessionID:     sid,
				UserID:        event.Actor.UserID,
				AgentType:     event.Actor.AgentType,
				AgentInstance: event.Actor.AgentInstance,
				EventCount:    0,
				FirstEvent:    event.Timestamp,
				LastEvent:     event.Timestamp,
				Decisions:     make(map[string]int),
			}
			sessionMap[sid] = summary
		}

		summary.EventCount++
		summary.Decisions[event.Decision]++

		if event.Timestamp < summary.FirstEvent {
			summary.FirstEvent = event.Timestamp
		}
		if event.Timestamp > summary.LastEvent {
			summary.LastEvent = event.Timestamp
		}
	}

	summaries := make([]types.SessionSummary, 0, len(sessionMap))
	for _, s := range sessionMap {
		summaries = append(summaries, *s)
	}

	// Sort by first event timestamp for deterministic output.
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].FirstEvent < summaries[j].FirstEvent
	})

	return summaries, nil
}

// ExportEvents exports events matching the query, wrapped with metadata.
func (s *InMemoryStore) ExportEvents(query AuditQuery) (*ExportResult, error) {
	events, err := s.QueryEvents(query)
	if err != nil {
		return nil, err
	}

	return &ExportResult{
		Metadata: ExportMetadata{
			ExportedAt: time.Now().UTC().Format(time.RFC3339),
			EventCount: len(events),
			Query:      fmt.Sprintf("session=%s actor=%s action=%s decision=%s", query.SessionID, query.ActorUserID, query.ActionType, query.Decision),
		},
		Events: events,
	}, nil
}

// GetMetrics returns aggregate statistics about the store.
func (s *InMemoryStore) GetMetrics() (*StoreMetrics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions := make(map[string]bool)
	decisions := make(map[string]int)

	for _, event := range s.events {
		sessions[event.SessionID] = true
		decisions[event.Decision]++
	}

	return &StoreMetrics{
		TotalEvents:    len(s.events),
		SessionCount:   len(sessions),
		DecisionCounts: decisions,
	}, nil
}

// GetCount returns the total number of stored events.
func (s *InMemoryStore) GetCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.events)
}

// matchesQuery checks whether an event matches all non-empty query fields.
func matchesQuery(event types.AuditEvent, query AuditQuery) bool {
	if query.SessionID != "" && event.SessionID != query.SessionID {
		return false
	}
	if query.ActorUserID != "" && event.Actor.UserID != query.ActorUserID {
		return false
	}
	if query.ActionType != "" && event.Action.Type != query.ActionType {
		return false
	}
	if query.Decision != "" && event.Decision != query.Decision {
		return false
	}
	if query.PolicyID != "" && event.PolicyDetail.PolicyID != query.PolicyID {
		return false
	}
	if query.ReasonCode != "" && event.PolicyDetail.ReasonCode != query.ReasonCode {
		return false
	}
	if query.TimeFrom != "" && event.Timestamp < query.TimeFrom {
		return false
	}
	if query.TimeTo != "" && event.Timestamp > query.TimeTo {
		return false
	}
	return true
}
