/**
 * Author: Deepankar Das
 */

package audit

import (
	"log/slog"
	"time"

	"github.com/anthropics/enforcer/internal/types"
)

const (
	// DefaultFlushInterval is the default interval between flush cycles.
	DefaultFlushInterval = 5 * time.Second

	// DefaultFlushBatchSize is the default number of events per flush batch.
	DefaultFlushBatchSize = 100
)

// FlushService is a background service that periodically flushes buffered
// audit events to the persistent store.
type FlushService struct {
	buffer    *AuditBuffer
	store     AuditStore
	interval  time.Duration
	batchSize int
	stopCh    chan struct{}
	doneCh    chan struct{}
}

// NewFlushService creates a new flush service with default settings.
func NewFlushService(buffer *AuditBuffer, store AuditStore) *FlushService {
	return &FlushService{
		buffer:    buffer,
		store:     store,
		interval:  DefaultFlushInterval,
		batchSize: DefaultFlushBatchSize,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
}

// NewFlushServiceWithOptions creates a new flush service with custom settings.
func NewFlushServiceWithOptions(buffer *AuditBuffer, store AuditStore, interval time.Duration, batchSize int) *FlushService {
	return &FlushService{
		buffer:    buffer,
		store:     store,
		interval:  interval,
		batchSize: batchSize,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
}

// Start launches the background flush goroutine.
func (fs *FlushService) Start() {
	go func() {
		defer close(fs.doneCh)
		ticker := time.NewTicker(fs.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				fs.Flush()
			case <-fs.stopCh:
				// Final flush before stopping.
				fs.FlushAll()
				return
			}
		}
	}()
}

// Stop signals the background goroutine to stop and waits for it to finish.
func (fs *FlushService) Stop() {
	close(fs.stopCh)
	<-fs.doneCh
}

// Flush flushes one batch of events from the buffer to the store.
// Returns (stored, failed) counts.
func (fs *FlushService) Flush() (int, int) {
	pending := fs.buffer.GetEventsToFlush(fs.batchSize)
	if len(pending) == 0 {
		return 0, 0
	}

	events := make([]types.AuditEvent, len(pending))
	ids := make([]int64, len(pending))
	for i, be := range pending {
		events[i] = be.Event
		ids[i] = be.ID
	}

	stored, err := fs.store.StoreEvents(events)
	failed := len(events) - stored

	if failed > 0 || err != nil {
		slog.Warn("Audit flush: some events failed to store",
			"total", len(events), "stored", stored, "failed", failed, "err", err)
	} else if stored > 0 {
		slog.Info("Audit flush: events stored", "count", stored)
	}

	// Mark successfully stored events as flushed.
	fs.buffer.MarkFlushed(ids)

	return stored, failed
}

// FlushAll flushes all buffered events to the store, processing in batches.
// Returns cumulative (stored, failed) counts.
func (fs *FlushService) FlushAll() (int, int) {
	totalStored := 0
	totalFailed := 0

	for {
		pending := fs.buffer.GetEventsToFlush(fs.batchSize)
		if len(pending) == 0 {
			break
		}

		events := make([]types.AuditEvent, len(pending))
		ids := make([]int64, len(pending))
		for i, be := range pending {
			events[i] = be.Event
			ids[i] = be.ID
		}

		stored, _ := fs.store.StoreEvents(events)
		failed := len(events) - stored
		totalStored += stored
		totalFailed += failed

		fs.buffer.MarkFlushed(ids)
	}

	return totalStored, totalFailed
}
