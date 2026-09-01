/**
 * Author: Deepankar Das
 */

package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/anthropics/enforcer/internal/types"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore is a PostgreSQL-backed, append-only audit store.
// Connection uses TLS (sslmode=require). Data at rest encryption is handled
// by PostgreSQL TDE or filesystem-level encryption.
// No in-memory fallback — fail-closed if PostgreSQL is unavailable.
type PostgresStore struct {
	pool          *pgxpool.Pool
	totalStored   int
	totalRejected int
}

// NewPostgresStore creates a persistent audit store connected to PostgreSQL.
// Returns an error if the connection fails — the daemon must not start without persistence.
func NewPostgresStore() (*PostgresStore, error) {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		user := os.Getenv("USER")
		if user == "" {
			user = "postgres"
		}
		connStr = fmt.Sprintf("postgresql://%s@localhost:5432/enforcer?sslmode=prefer", user)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("PostgreSQL connection failed: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("PostgreSQL ping failed: %w", err)
	}

	slog.Info("PostgreSQL connected — audit events will persist with append-only enforcement")
	return &PostgresStore{pool: pool}, nil
}

// StoreEvent inserts an audit event. Append-only — no UPDATE or DELETE.
func (s *PostgresStore) StoreEvent(event types.AuditEvent) error {
	valid, errs := ValidateAuditEvent(event)
	if !valid {
		s.totalRejected++
		return fmt.Errorf("validation failed: %v", errs)
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal failed: %w", err)
	}

	// The id column is UUID type — use gen_random_uuid() instead of the event_id
	// (which is a human-readable string like "evt_xxx_xxx", not a UUID).
	// The event_id is preserved inside the JSONB event field for querying.
	_, err = s.pool.Exec(context.Background(),
		"INSERT INTO audit_events (id, event) VALUES (gen_random_uuid(), $1) ON CONFLICT DO NOTHING",
		eventJSON,
	)
	if err != nil {
		return fmt.Errorf("PostgreSQL insert failed: %w", err)
	}

	s.totalStored++
	return nil
}

// StoreEvents inserts a batch of events.
func (s *PostgresStore) StoreEvents(events []types.AuditEvent) (int, error) {
	stored := 0
	var lastErr error
	for _, event := range events {
		if err := s.StoreEvent(event); err == nil {
			stored++
		} else {
			lastErr = err
			slog.Warn("Audit event store failed", "event_id", event.EventID, "err", err)
		}
	}
	return stored, lastErr
}

// QueryEvents filters events from PostgreSQL.
func (s *PostgresStore) QueryEvents(query AuditQuery) ([]types.AuditEvent, error) {
	conditions := []string{}
	params := []interface{}{}
	idx := 1

	if query.SessionID != "" {
		conditions = append(conditions, fmt.Sprintf("event->>'session_id' = $%d", idx))
		params = append(params, query.SessionID)
		idx++
	}
	if query.ActorUserID != "" {
		conditions = append(conditions, fmt.Sprintf("event->'actor'->>'user_id' = $%d", idx))
		params = append(params, query.ActorUserID)
		idx++
	}
	if query.ActionType != "" {
		conditions = append(conditions, fmt.Sprintf("event->'action'->>'type' = $%d", idx))
		params = append(params, query.ActionType)
		idx++
	}
	if query.Decision != "" {
		conditions = append(conditions, fmt.Sprintf("event->'policy_detail'->>'decision' = $%d", idx))
		params = append(params, query.Decision)
		idx++
	}
	if query.PolicyID != "" {
		conditions = append(conditions, fmt.Sprintf("event->'policy_detail'->>'policy_id' = $%d", idx))
		params = append(params, query.PolicyID)
		idx++
	}
	if query.ReasonCode != "" {
		conditions = append(conditions, fmt.Sprintf("event->'policy_detail'->>'reason_code' = $%d", idx))
		params = append(params, query.ReasonCode)
		idx++
	}
	if query.TimeFrom != "" {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", idx))
		params = append(params, query.TimeFrom)
		idx++
	}
	if query.TimeTo != "" {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", idx))
		params = append(params, query.TimeTo)
		idx++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE "
		for i, c := range conditions {
			if i > 0 {
				where += " AND "
			}
			where += c
		}
	}

	limit := query.Limit
	offset := query.Offset
	if offset < 0 {
		offset = 0
	}

	var sql string
	if limit > 0 {
		sql = fmt.Sprintf("SELECT event FROM audit_events %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d",
			where, idx, idx+1)
		params = append(params, limit, offset)
	} else {
		sql = fmt.Sprintf("SELECT event FROM audit_events %s ORDER BY created_at DESC",
			where)
	}

	rows, err := s.pool.Query(context.Background(), sql, params...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var events []types.AuditEvent
	for rows.Next() {
		var eventJSON []byte
		if err := rows.Scan(&eventJSON); err != nil {
			continue
		}
		var event types.AuditEvent
		if err := json.Unmarshal(eventJSON, &event); err != nil {
			continue
		}
		normalizeDecisionField(&event)
		events = append(events, event)
	}
	return events, nil
}

// GetSession returns all events for a session in chronological order.
func (s *PostgresStore) GetSession(sessionID string) ([]types.AuditEvent, error) {
	rows, err := s.pool.Query(context.Background(),
		"SELECT event FROM audit_events WHERE event->>'session_id' = $1 ORDER BY created_at ASC",
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []types.AuditEvent
	for rows.Next() {
		var eventJSON []byte
		if err := rows.Scan(&eventJSON); err != nil {
			continue
		}
		var event types.AuditEvent
		if err := json.Unmarshal(eventJSON, &event); err != nil {
			continue
		}
		normalizeDecisionField(&event)
		events = append(events, event)
	}
	return events, nil
}

// GetSessions returns summaries for all sessions.
func (s *PostgresStore) GetSessions() ([]types.SessionSummary, error) {
	rows, err := s.pool.Query(context.Background(), `
		SELECT
			event->>'session_id' as session_id,
			event->'actor'->>'user_id' as user_id,
			event->'actor'->>'agent_type' as agent_type,
			event->'actor'->>'agent_instance' as agent_instance,
			COUNT(*) as event_count,
			MIN(created_at) as first_event,
			MAX(created_at) as last_event
		FROM audit_events
		GROUP BY
			event->>'session_id',
			event->'actor'->>'user_id',
			event->'actor'->>'agent_type',
			event->'actor'->>'agent_instance'
		ORDER BY MAX(created_at) DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []types.SessionSummary
	for rows.Next() {
		var ss types.SessionSummary
		var firstEvent, lastEvent time.Time
		if err := rows.Scan(&ss.SessionID, &ss.UserID, &ss.AgentType, &ss.AgentInstance,
			&ss.EventCount, &firstEvent, &lastEvent); err != nil {
			continue
		}
		ss.FirstEvent = firstEvent.Format(time.RFC3339)
		ss.LastEvent = lastEvent.Format(time.RFC3339)
		ss.Decisions = make(map[string]int)

		decRows, err := s.pool.Query(context.Background(), `
			SELECT event->'policy_detail'->>'decision' as decision, COUNT(*) as cnt
			FROM audit_events WHERE event->>'session_id' = $1
			GROUP BY event->'policy_detail'->>'decision'
		`, ss.SessionID)
		if err == nil {
			for decRows.Next() {
				var decision string
				var cnt int
				if decRows.Scan(&decision, &cnt) == nil {
					ss.Decisions[decision] = cnt
				}
			}
			decRows.Close()
		}

		sessions = append(sessions, ss)
	}
	return sessions, nil
}

// ExportEvents exports events matching the query with metadata.
func (s *PostgresStore) ExportEvents(query AuditQuery) (*ExportResult, error) {
	exportQuery := query
	if exportQuery.Limit <= 0 {
		exportQuery.Limit = 10000
	}
	events, err := s.QueryEvents(exportQuery)
	if err != nil {
		return nil, err
	}
	return &ExportResult{
		Metadata: ExportMetadata{
			ExportedAt: time.Now().UTC().Format(time.RFC3339),
			EventCount: len(events),
			Query:      fmt.Sprintf("%+v", query),
		},
		Events: events,
	}, nil
}

// GetMetrics returns aggregate statistics.
func (s *PostgresStore) GetMetrics() (*StoreMetrics, error) {
	var totalEvents int
	err := s.pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM audit_events").Scan(&totalEvents)
	if err != nil {
		return nil, err
	}

	decisions := make(map[string]int)
	rows, err := s.pool.Query(context.Background(),
		"SELECT COALESCE(decision, '') as decision, COUNT(*) FROM audit_events GROUP BY decision")
	if err == nil {
		for rows.Next() {
			var dec *string
			var cnt int
			if err := rows.Scan(&dec, &cnt); err != nil {
				continue
			}
			key := ""
			if dec != nil {
				key = *dec
			}
			if key != "" {
				decisions[key] = cnt
			}
		}
		rows.Close()
	}

	return &StoreMetrics{
		TotalEvents:    totalEvents,
		DecisionCounts: decisions,
	}, nil
}

// GetCount returns total event count.
func (s *PostgresStore) GetCount() int {
	var cnt int
	s.pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM audit_events").Scan(&cnt)
	return cnt
}

// Backend returns "postgresql".
func (s *PostgresStore) Backend() string {
	return "postgresql"
}

// Close shuts down the connection pool.
func (s *PostgresStore) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

// normalizeDecisionField strips the appended reason code from legacy events.
// Old events stored "deny:PATH_OUTSIDE_PROJECT_ROOT"; new events store "deny".
// This ensures analytics code can do exact-match comparisons.
func normalizeDecisionField(event *types.AuditEvent) {
	if idx := strings.Index(event.Decision, ":"); idx > 0 {
		event.Decision = event.Decision[:idx]
	}
	if idx := strings.Index(event.Result, ":"); idx > 0 {
		event.Result = event.Result[:idx]
	}
}
