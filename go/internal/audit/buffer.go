/**
 * Author: Deepankar Das
 */

package audit

import (
	"sync"

	"github.com/anthropics/enforcer/internal/types"
)

const (
	DefaultMaxBufferSize  = 10000
	BackpressureThreshold = 0.80
)

// BufferedEvent wraps an audit event with buffer-specific metadata.
type BufferedEvent struct {
	ID    int64            `json:"id"`
	Event types.AuditEvent `json:"event"`
}

// BufferMetrics tracks buffer operational metrics.
type BufferMetrics struct {
	Accepted           int `json:"accepted"`
	Rejected           int `json:"rejected"`
	BackpressureAlerts int `json:"backpressureAlerts"`
	BufferCount        int `json:"bufferCount"`
}

// AuditBuffer is a thread-safe in-process event queue that feeds the PostgreSQL store.
// It is NOT a persistence layer — PostgreSQL is the sole persistence layer.
// The buffer exists only to decouple event emission from the synchronous INSERT,
// allowing the flush service to batch writes.
// If the daemon crashes before flush, unflushed events are lost — this is acceptable
// because the daemon should fail-closed (no governance without audit).
type AuditBuffer struct {
	mu                 sync.Mutex
	events             []BufferedEvent
	nextID             int64
	maxSize            int
	accepted           int
	rejected           int
	backpressureAlerts int
}

// NewAuditBuffer creates a new in-process event queue.
func NewAuditBuffer() *AuditBuffer {
	return &AuditBuffer{
		maxSize: DefaultMaxBufferSize,
	}
}

// BufferEvent validates and queues an event for flush to PostgreSQL.
func (b *AuditBuffer) BufferEvent(event types.AuditEvent) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	valid, errs := ValidateAuditEvent(event)
	if !valid {
		b.rejected++
		_ = errs
		return false
	}

	if len(b.events) >= b.maxSize {
		b.rejected++
		return false
	}

	if float64(len(b.events)) >= float64(b.maxSize)*BackpressureThreshold {
		b.backpressureAlerts++
	}

	b.nextID++
	b.events = append(b.events, BufferedEvent{ID: b.nextID, Event: event})
	b.accepted++
	return true
}

// GetEventsToFlush returns up to `limit` unflushed events.
func (b *AuditBuffer) GetEventsToFlush(limit int) []BufferedEvent {
	b.mu.Lock()
	defer b.mu.Unlock()

	if limit <= 0 || limit > len(b.events) {
		limit = len(b.events)
	}
	result := make([]BufferedEvent, limit)
	copy(result, b.events[:limit])
	return result
}

// MarkFlushed removes events from the buffer by their IDs.
func (b *AuditBuffer) MarkFlushed(ids []int64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	idSet := make(map[int64]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}
	remaining := make([]BufferedEvent, 0, len(b.events))
	for _, e := range b.events {
		if !idSet[e.ID] {
			remaining = append(remaining, e)
		}
	}
	b.events = remaining
}

// GetMetrics returns buffer operational metrics.
func (b *AuditBuffer) GetMetrics() BufferMetrics {
	b.mu.Lock()
	defer b.mu.Unlock()
	return BufferMetrics{
		Accepted:           b.accepted,
		Rejected:           b.rejected,
		BackpressureAlerts: b.backpressureAlerts,
		BufferCount:        len(b.events),
	}
}

// GetStats returns buffer capacity statistics.
func (b *AuditBuffer) GetStats() map[string]interface{} {
	b.mu.Lock()
	defer b.mu.Unlock()
	count := len(b.events)
	return map[string]interface{}{
		"count":        count,
		"maxSize":      b.maxSize,
		"backpressure": float64(count) >= float64(b.maxSize)*BackpressureThreshold,
		"full":         count >= b.maxSize,
	}
}

// Close is a no-op for the in-process buffer.
func (b *AuditBuffer) Close() {}
