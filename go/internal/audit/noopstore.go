/**
 * Author: Deepankar Das
 */

package audit

import (
	"fmt"

	"github.com/anthropics/enforcer/internal/types"
)

// NoOpStore is a store that rejects all operations. Used when PostgreSQL is
// unavailable and no in-memory fallback is permitted. Every method returns
// an error telling the operator to configure DATABASE_URL.
type NoOpStore struct{}

// NewNoOpStore creates a store that rejects all operations.
func NewNoOpStore() *NoOpStore {
	return &NoOpStore{}
}

var errNoPersistence = fmt.Errorf("no audit persistence configured — set DATABASE_URL and start PostgreSQL")

func (s *NoOpStore) StoreEvent(_ types.AuditEvent) error {
	return errNoPersistence
}

func (s *NoOpStore) StoreEvents(_ []types.AuditEvent) (int, error) {
	return 0, errNoPersistence
}

func (s *NoOpStore) QueryEvents(_ AuditQuery) ([]types.AuditEvent, error) {
	return nil, errNoPersistence
}

func (s *NoOpStore) GetSession(_ string) ([]types.AuditEvent, error) {
	return nil, errNoPersistence
}

func (s *NoOpStore) GetSessions() ([]types.SessionSummary, error) {
	return nil, errNoPersistence
}

func (s *NoOpStore) ExportEvents(_ AuditQuery) (*ExportResult, error) {
	return nil, errNoPersistence
}

func (s *NoOpStore) GetMetrics() (*StoreMetrics, error) {
	return nil, errNoPersistence
}

func (s *NoOpStore) GetCount() int {
	return 0
}
