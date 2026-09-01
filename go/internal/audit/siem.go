/**
 * Author: Deepankar Das
 */

package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/anthropics/enforcer/internal/types"
)

// SiemTransport enumerates the supported SIEM export transports.
type SiemTransport string

const (
	TransportWebhook SiemTransport = "webhook"
	TransportSyslog  SiemTransport = "syslog"
	TransportFile    SiemTransport = "file"
)

// SiemConfig configures the SIEM exporter.
type SiemConfig struct {
	Enabled        bool          `json:"enabled" yaml:"enabled"`
	Transport      SiemTransport `json:"transport" yaml:"transport"`
	WebhookURL     string        `json:"webhook_url,omitempty" yaml:"webhook_url,omitempty"`
	SyslogHost     string        `json:"syslog_host,omitempty" yaml:"syslog_host,omitempty"`
	SyslogPort     int           `json:"syslog_port,omitempty" yaml:"syslog_port,omitempty"`
	FilePath       string        `json:"file_path,omitempty" yaml:"file_path,omitempty"`
	BatchSize      int           `json:"batch_size" yaml:"batch_size"`
	FlushIntervalMs int          `json:"flush_interval_ms" yaml:"flush_interval_ms"`
}

// SiemExporter exports audit events to external SIEM systems.
type SiemExporter struct {
	config     SiemConfig
	mu         sync.Mutex
	queue      []types.AuditEvent
	httpClient *http.Client
}

// NewSiemExporter creates a new SIEM exporter with the given configuration.
func NewSiemExporter(config SiemConfig) *SiemExporter {
	if config.BatchSize <= 0 {
		config.BatchSize = 50
	}
	if config.FlushIntervalMs <= 0 {
		config.FlushIntervalMs = 10000
	}

	return &SiemExporter{
		config: config,
		queue:  make([]types.AuditEvent, 0, config.BatchSize),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Enqueue adds an audit event to the SIEM export queue.
func (s *SiemExporter) Enqueue(event types.AuditEvent) {
	if !s.config.Enabled {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.queue = append(s.queue, event)
}

// Flush sends queued events via the configured transport.
// Returns (sent, failed) counts.
func (s *SiemExporter) Flush() (int, int) {
	if !s.config.Enabled {
		return 0, 0
	}

	s.mu.Lock()
	if len(s.queue) == 0 {
		s.mu.Unlock()
		return 0, 0
	}

	// Drain the queue into a local batch.
	batchSize := s.config.BatchSize
	if batchSize > len(s.queue) {
		batchSize = len(s.queue)
	}
	batch := make([]types.AuditEvent, batchSize)
	copy(batch, s.queue[:batchSize])
	s.queue = s.queue[batchSize:]
	s.mu.Unlock()

	var sent, failed int

	switch s.config.Transport {
	case TransportWebhook:
		sent, failed = s.sendWebhook(batch)
	case TransportSyslog:
		sent, failed = s.sendSyslog(batch)
	case TransportFile:
		sent, failed = s.sendFile(batch)
	default:
		// Unknown transport: all events fail.
		return 0, len(batch)
	}

	return sent, failed
}

// QueueLen returns the current number of events in the export queue.
func (s *SiemExporter) QueueLen() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.queue)
}

// sendWebhook sends events as a JSON array via HTTP POST.
func (s *SiemExporter) sendWebhook(events []types.AuditEvent) (int, int) {
	payload, err := json.Marshal(events)
	if err != nil {
		return 0, len(events)
	}

	req, err := http.NewRequest(http.MethodPost, s.config.WebhookURL, bytes.NewReader(payload))
	if err != nil {
		return 0, len(events)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Enforcer-SIEM-Exporter/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, len(events)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return len(events), 0
	}
	return 0, len(events)
}

// sendSyslog sends events via UDP using RFC 5424 format.
func (s *SiemExporter) sendSyslog(events []types.AuditEvent) (int, int) {
	addr := net.JoinHostPort(s.config.SyslogHost, fmt.Sprintf("%d", s.config.SyslogPort))
	conn, err := net.DialTimeout("udp", addr, 5*time.Second)
	if err != nil {
		return 0, len(events)
	}
	defer conn.Close()

	sent := 0
	failed := 0

	for _, event := range events {
		msg, err := formatSyslogMessage(event)
		if err != nil {
			failed++
			continue
		}

		_, err = conn.Write([]byte(msg))
		if err != nil {
			failed++
			continue
		}
		sent++
	}

	return sent, failed
}

// formatSyslogMessage formats an audit event as an RFC 5424 syslog message.
// Priority 134 = facility 16 (local0) * 8 + severity 6 (informational).
func formatSyslogMessage(event types.AuditEvent) (string, error) {
	data, err := json.Marshal(event)
	if err != nil {
		return "", err
	}

	// RFC 5424: <PRI>VERSION TIMESTAMP HOSTNAME APP-NAME PROCID MSGID SD MSG
	msg := fmt.Sprintf(
		"<134>1 %s enforcer audit-pipeline - - - %s",
		event.Timestamp,
		string(data),
	)

	return msg, nil
}

// sendFile appends events as JSONL (one JSON object per line) to the
// configured file path.
func (s *SiemExporter) sendFile(events []types.AuditEvent) (int, int) {
	f, err := os.OpenFile(s.config.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return 0, len(events)
	}
	defer f.Close()

	sent := 0
	failed := 0

	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			failed++
			continue
		}

		_, err = f.Write(append(data, '\n'))
		if err != nil {
			failed++
			continue
		}
		sent++
	}

	return sent, failed
}
