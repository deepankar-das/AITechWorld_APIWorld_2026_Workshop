package central

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var errPolicyNotFound = errors.New("policy_not_found")
var errEnforcementStateNotFound = errors.New("enforcement_state_not_found")

type policyState struct {
	Version string
	Hash    string
	Bundle  string
}

type clientSnapshot struct {
	ClientID      string
	Hostname      string
	GovernedUsers []string
	RegisteredAt  time.Time
	LastHeartbeat time.Time
	PolicyVersion string
	Status        string
}

type enforcementState struct {
	Enabled   bool
	ChangedBy string
	ChangedAt time.Time
}

type stateStore interface {
	PutPolicyRevision(ctx context.Context, state policyState) error
	GetLatestPolicy(ctx context.Context) (*policyState, error)
	PutEnforcementState(ctx context.Context, state enforcementState) error
	GetLatestEnforcementState(ctx context.Context) (*enforcementState, error)
	RecordClientSnapshot(ctx context.Context, snap clientSnapshot) error
	ListLatestClients(ctx context.Context) ([]*RegisteredClient, error)
	CountClients(ctx context.Context) (int, error)
	Close()
}

type postgresStateStore struct {
	pool *pgxpool.Pool
}

func newPostgresStateStore() (*postgresStateStore, error) {
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
		return nil, fmt.Errorf("postgres pool init failed: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres ping failed: %w", err)
	}

	store := &postgresStateStore{pool: pool}
	if err := store.verifySchema(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return store, nil
}

func (s *postgresStateStore) verifySchema(ctx context.Context) error {
	const query = `
	SELECT
		to_regclass('public.hub_policy_revisions') IS NOT NULL,
		to_regclass('public.hub_client_snapshots') IS NOT NULL,
		to_regclass('public.hub_enforcement_states') IS NOT NULL
	`
	var hasPolicy, hasClients, hasEnforcement bool
	if err := s.pool.QueryRow(ctx, query).Scan(&hasPolicy, &hasClients, &hasEnforcement); err != nil {
		return fmt.Errorf("schema verification failed: %w", err)
	}
	if !hasPolicy || !hasClients || !hasEnforcement {
		return fmt.Errorf(
			"required hub tables missing (hub_policy_revisions=%t hub_client_snapshots=%t hub_enforcement_states=%t); run DB setup/migration",
			hasPolicy, hasClients, hasEnforcement,
		)
	}
	return nil
}

func (s *postgresStateStore) PutPolicyRevision(ctx context.Context, state policyState) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO hub_policy_revisions (version, hash, bundle)
		VALUES ($1, $2, $3)
	`, state.Version, state.Hash, state.Bundle)
	if err != nil {
		return fmt.Errorf("insert policy revision failed: %w", err)
	}
	return nil
}

func (s *postgresStateStore) GetLatestPolicy(ctx context.Context) (*policyState, error) {
	var p policyState
	err := s.pool.QueryRow(ctx, `
		SELECT version, hash, bundle
		FROM hub_policy_revisions
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`).Scan(&p.Version, &p.Hash, &p.Bundle)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errPolicyNotFound
		}
		return nil, fmt.Errorf("query latest policy failed: %w", err)
	}
	return &p, nil
}

func (s *postgresStateStore) PutEnforcementState(ctx context.Context, state enforcementState) error {
	if state.ChangedBy == "" {
		state.ChangedBy = "hub_console"
	}
	if state.ChangedAt.IsZero() {
		state.ChangedAt = time.Now().UTC()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO hub_enforcement_states (enabled, changed_by, changed_at)
		VALUES ($1, $2, $3)
	`, state.Enabled, state.ChangedBy, state.ChangedAt)
	if err != nil {
		return fmt.Errorf("insert enforcement state failed: %w", err)
	}
	return nil
}

func (s *postgresStateStore) GetLatestEnforcementState(ctx context.Context) (*enforcementState, error) {
	var e enforcementState
	err := s.pool.QueryRow(ctx, `
		SELECT enabled, changed_by, changed_at
		FROM hub_enforcement_states
		ORDER BY changed_at DESC, id DESC
		LIMIT 1
	`).Scan(&e.Enabled, &e.ChangedBy, &e.ChangedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errEnforcementStateNotFound
		}
		return nil, fmt.Errorf("query latest enforcement state failed: %w", err)
	}
	return &e, nil
}

func (s *postgresStateStore) RecordClientSnapshot(ctx context.Context, snap clientSnapshot) error {
	if snap.ClientID == "" {
		return errors.New("client_id is required")
	}

	if snap.Hostname == "" || len(snap.GovernedUsers) == 0 || snap.RegisteredAt.IsZero() || snap.PolicyVersion == "" {
		prev, err := s.getLatestClientSnapshot(ctx, snap.ClientID)
		if err == nil {
			if snap.Hostname == "" {
				snap.Hostname = prev.Hostname
			}
			if len(snap.GovernedUsers) == 0 {
				snap.GovernedUsers = prev.GovernedUsers
			}
			if snap.RegisteredAt.IsZero() {
				snap.RegisteredAt = prev.RegisteredAt
			}
			if snap.PolicyVersion == "" {
				snap.PolicyVersion = prev.PolicyVersion
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
	}

	now := time.Now().UTC()
	if snap.Hostname == "" {
		snap.Hostname = "unknown"
	}
	if snap.GovernedUsers == nil {
		snap.GovernedUsers = []string{}
	}
	if snap.RegisteredAt.IsZero() {
		snap.RegisteredAt = now
	}
	if snap.LastHeartbeat.IsZero() {
		snap.LastHeartbeat = now
	}
	if snap.Status == "" {
		snap.Status = "online"
	}

	governedUsersJSON, err := json.Marshal(snap.GovernedUsers)
	if err != nil {
		return fmt.Errorf("marshal governed users failed: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO hub_client_snapshots (
			client_id,
			hostname,
			governed_users,
			registered_at,
			last_heartbeat,
			policy_version,
			status
		)
		VALUES ($1, $2, $3::jsonb, $4, $5, $6, $7)
	`,
		snap.ClientID,
		snap.Hostname,
		governedUsersJSON,
		snap.RegisteredAt,
		snap.LastHeartbeat,
		snap.PolicyVersion,
		snap.Status,
	)
	if err != nil {
		return fmt.Errorf("insert client snapshot failed: %w", err)
	}
	return nil
}

func (s *postgresStateStore) getLatestClientSnapshot(ctx context.Context, clientID string) (*clientSnapshot, error) {
	var snap clientSnapshot
	var governedUsersJSON []byte
	err := s.pool.QueryRow(ctx, `
		SELECT client_id, hostname, governed_users, registered_at, last_heartbeat, policy_version, status
		FROM hub_client_snapshots
		WHERE client_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`, clientID).Scan(
		&snap.ClientID,
		&snap.Hostname,
		&governedUsersJSON,
		&snap.RegisteredAt,
		&snap.LastHeartbeat,
		&snap.PolicyVersion,
		&snap.Status,
	)
	if err != nil {
		return nil, err
	}
	if len(governedUsersJSON) > 0 {
		_ = json.Unmarshal(governedUsersJSON, &snap.GovernedUsers)
	}
	if snap.GovernedUsers == nil {
		snap.GovernedUsers = []string{}
	}
	return &snap, nil
}

func (s *postgresStateStore) ListLatestClients(ctx context.Context) ([]*RegisteredClient, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (client_id)
			client_id, hostname, governed_users, registered_at, last_heartbeat, policy_version, status
		FROM hub_client_snapshots
		ORDER BY client_id, created_at DESC, id DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query clients failed: %w", err)
	}
	defer rows.Close()

	clients := make([]*RegisteredClient, 0)
	for rows.Next() {
		var c RegisteredClient
		var governedUsersJSON []byte
		if err := rows.Scan(
			&c.ClientID,
			&c.Hostname,
			&governedUsersJSON,
			&c.RegisteredAt,
			&c.LastHeartbeat,
			&c.PolicyVersion,
			&c.Status,
		); err != nil {
			return nil, err
		}
		if len(governedUsersJSON) > 0 {
			_ = json.Unmarshal(governedUsersJSON, &c.GovernedUsers)
		}
		if c.GovernedUsers == nil {
			c.GovernedUsers = []string{}
		}
		clients = append(clients, &c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return clients, nil
}

func (s *postgresStateStore) CountClients(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(DISTINCT client_id) FROM hub_client_snapshots`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count clients failed: %w", err)
	}
	return count, nil
}

func (s *postgresStateStore) Close() {
	s.pool.Close()
}
