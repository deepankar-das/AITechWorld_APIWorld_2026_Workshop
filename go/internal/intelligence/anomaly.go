/**
 * Author: Deepankar Das
 */

package intelligence

import (
	"strings"
	"sync"
	"time"

	"github.com/anthropics/enforcer/internal/types"
)

// AnomalyPattern defines a sequence-based detection rule.
type AnomalyPattern struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Severity    string `json:"severity"` // "critical", "high", "medium"
	Description string `json:"description"`
	WindowSecs  int    `json:"window_seconds"`
	MinEvents   int    `json:"min_events"`
	matcher     func(events []types.AuditEvent, latest types.AuditEvent) bool
}

// AnomalyAlert is emitted when a pattern matches.
type AnomalyAlert struct {
	PatternID  string `json:"pattern_id"`
	PatternName string `json:"pattern_name"`
	Severity   string `json:"severity"`
	SessionID  string `json:"session_id"`
	Timestamp  string `json:"timestamp"`
	Description string `json:"description"`
	EventCount int    `json:"event_count"`
}

// AnomalyDetector maintains session state and detects suspicious sequences.
type AnomalyDetector struct {
	mu         sync.RWMutex
	sessions   map[string][]types.AuditEvent
	alerts     []AnomalyAlert
	listeners  []func(AnomalyAlert)
	patterns   []AnomalyPattern
	maxWindow  int // max events per session
	windowTTL  time.Duration
}

// NewAnomalyDetector creates a new detector with built-in patterns.
func NewAnomalyDetector() *AnomalyDetector {
	d := &AnomalyDetector{
		sessions:  make(map[string][]types.AuditEvent),
		maxWindow: 100,
		windowTTL: 10 * time.Minute,
	}
	d.patterns = d.builtinPatterns()
	return d
}

func (d *AnomalyDetector) builtinPatterns() []AnomalyPattern {
	return []AnomalyPattern{
		{
			ID: "exfil_secret_then_network", Name: "Secret Read → Network Request",
			Severity: "critical", Description: "Credential access followed by network request (potential exfiltration)",
			WindowSecs: 60, MinEvents: 2,
			matcher: func(events []types.AuditEvent, latest types.AuditEvent) bool {
				if latest.Action.Type != "network.request" {
					return false
				}
				for _, e := range events {
					if e.Action.Type == "credential.access" || e.Action.Type == "file.read" {
						if containsClassification(e.Resource.Classification, "sensitive_path") {
							return true
						}
					}
				}
				return false
			},
		},
		{
			ID: "exfil_read_then_curl", Name: "File Read → curl/wget POST",
			Severity: "critical", Description: "File read followed by outbound data transfer tool",
			WindowSecs: 60, MinEvents: 2,
			matcher: func(events []types.AuditEvent, latest types.AuditEvent) bool {
				if latest.Action.Type != "shell.exec" {
					return false
				}
				cmd := strings.ToLower(latest.Resource.Value)
				if !strings.Contains(cmd, "curl") && !strings.Contains(cmd, "wget") {
					return false
				}
				for _, e := range events {
					if e.Action.Type == "file.read" {
						return true
					}
				}
				return false
			},
		},
		{
			ID: "privesc_cred_then_exec", Name: "Credential Read → Privileged Exec",
			Severity: "high", Description: "Credential access followed by privileged command execution",
			WindowSecs: 120, MinEvents: 2,
			matcher: func(events []types.AuditEvent, latest types.AuditEvent) bool {
				if latest.Action.Type != "shell.exec" {
					return false
				}
				cmd := strings.ToLower(latest.Resource.Value)
				isPriv := strings.HasPrefix(cmd, "sudo ") || strings.HasPrefix(cmd, "docker ") || strings.HasPrefix(cmd, "kubectl ")
				if !isPriv {
					return false
				}
				for _, e := range events {
					if e.Action.Type == "credential.access" {
						return true
					}
				}
				return false
			},
		},
		{
			ID: "recon_rapid_reads", Name: "Rapid File Reads",
			Severity: "medium", Description: "Unusually high number of file reads in short window",
			WindowSecs: 5, MinEvents: 20,
			matcher: func(events []types.AuditEvent, latest types.AuditEvent) bool {
				if latest.Action.Type != "file.read" {
					return false
				}
				count := 0
				cutoff := time.Now().Add(-5 * time.Second)
				for _, e := range events {
					if e.Action.Type == "file.read" {
						if t, err := time.Parse(time.RFC3339, e.Timestamp); err == nil && t.After(cutoff) {
							count++
						}
					}
				}
				return count >= 20
			},
		},
		{
			ID: "supply_chain_lockfile_then_install", Name: "Lockfile Modify → Package Install",
			Severity: "high", Description: "Lock file modification followed by package installation",
			WindowSecs: 300, MinEvents: 2,
			matcher: func(events []types.AuditEvent, latest types.AuditEvent) bool {
				if latest.Action.Type != "shell.exec" && latest.Action.Type != "package.install" {
					return false
				}
				cmd := strings.ToLower(latest.Resource.Value)
				isInstall := strings.Contains(cmd, "npm install") || strings.Contains(cmd, "yarn add") || strings.Contains(cmd, "pip install")
				if !isInstall && latest.Action.Type != "package.install" {
					return false
				}
				lockFiles := []string{"package-lock.json", "yarn.lock", "Pipfile.lock", "Cargo.lock"}
				for _, e := range events {
					if e.Action.Type == "file.write" {
						for _, lf := range lockFiles {
							if strings.HasSuffix(e.Resource.Path, lf) {
								return true
							}
						}
					}
				}
				return false
			},
		},
		{
			ID: "destructive_multi_delete", Name: "Multiple Rapid Deletes",
			Severity: "high", Description: "Multiple file deletions in rapid succession",
			WindowSecs: 10, MinEvents: 5,
			matcher: func(events []types.AuditEvent, latest types.AuditEvent) bool {
				if latest.Action.Type != "file.delete" {
					return false
				}
				count := 0
				for _, e := range events {
					if e.Action.Type == "file.delete" {
						count++
					}
				}
				return count >= 5
			},
		},
		{
			ID: "destructive_force_push_after_reset", Name: "git reset --hard → git push --force",
			Severity: "critical", Description: "Hard reset followed by force push (history rewrite)",
			WindowSecs: 60, MinEvents: 2,
			matcher: func(events []types.AuditEvent, latest types.AuditEvent) bool {
				if latest.Action.Type != "shell.exec" {
					return false
				}
				cmd := strings.ToLower(latest.Resource.Value)
				if !strings.Contains(cmd, "git push") || (!strings.Contains(cmd, "--force") && !strings.Contains(cmd, "-f")) {
					return false
				}
				for _, e := range events {
					if e.Action.Type == "shell.exec" {
						prev := strings.ToLower(e.Resource.Value)
						if strings.Contains(prev, "git reset --hard") {
							return true
						}
					}
				}
				return false
			},
		},
		{
			ID: "evasion_denied_then_retry", Name: "Denied Action Retry",
			Severity: "medium", Description: "Same action retried after denial (potential evasion attempt)",
			WindowSecs: 10, MinEvents: 3,
			matcher: func(events []types.AuditEvent, latest types.AuditEvent) bool {
				if latest.PolicyDetail.Decision != "deny" {
					return false
				}
				count := 0
				for _, e := range events {
					if e.PolicyDetail.Decision == "deny" && e.Action.Type == latest.Action.Type {
						count++
					}
				}
				return count >= 3
			},
		},
	}
}

// DetectAnomalies processes a new audit event and returns any triggered alerts.
func (d *AnomalyDetector) DetectAnomalies(event types.AuditEvent) []AnomalyAlert {
	d.mu.Lock()
	defer d.mu.Unlock()

	sessionID := event.SessionID
	if sessionID == "" {
		return nil
	}

	// Add to session window
	d.sessions[sessionID] = append(d.sessions[sessionID], event)

	// Trim window to max size
	if len(d.sessions[sessionID]) > d.maxWindow {
		d.sessions[sessionID] = d.sessions[sessionID][len(d.sessions[sessionID])-d.maxWindow:]
	}

	window := d.sessions[sessionID]
	// Exclude the latest event from the "prior events" slice
	priorEvents := window[:len(window)-1]

	var newAlerts []AnomalyAlert
	for _, pattern := range d.patterns {
		if pattern.matcher(priorEvents, event) {
			alert := AnomalyAlert{
				PatternID:   pattern.ID,
				PatternName: pattern.Name,
				Severity:    pattern.Severity,
				SessionID:   sessionID,
				Timestamp:   time.Now().UTC().Format(time.RFC3339),
				Description: pattern.Description,
				EventCount:  len(window),
			}
			newAlerts = append(newAlerts, alert)
			d.alerts = append(d.alerts, alert)

			for _, listener := range d.listeners {
				listener(alert)
			}
		}
	}

	return newAlerts
}

// OnAnomalyAlert registers an alert listener.
func (d *AnomalyDetector) OnAnomalyAlert(listener func(AnomalyAlert)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.listeners = append(d.listeners, listener)
}

// GetAlerts returns the most recent alerts.
func (d *AnomalyDetector) GetAlerts(limit int) []AnomalyAlert {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if limit <= 0 || limit > len(d.alerts) {
		limit = len(d.alerts)
	}
	start := len(d.alerts) - limit
	if start < 0 {
		start = 0
	}
	result := make([]AnomalyAlert, limit)
	copy(result, d.alerts[start:])
	return result
}

// GetPatterns returns the loaded anomaly patterns.
func (d *AnomalyDetector) GetPatterns() []AnomalyPattern {
	return d.patterns
}

// GetMetrics returns anomaly detection statistics.
func (d *AnomalyDetector) GetMetrics() map[string]interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()

	byPattern := make(map[string]int)
	bySeverity := make(map[string]int)
	for _, a := range d.alerts {
		byPattern[a.PatternID]++
		bySeverity[a.Severity]++
	}

	return map[string]interface{}{
		"total_alerts":      len(d.alerts),
		"alerts_by_pattern": byPattern,
		"alerts_by_severity": bySeverity,
		"active_sessions":   len(d.sessions),
		"patterns_loaded":   len(d.patterns),
	}
}

func containsClassification(classifications []string, target string) bool {
	for _, c := range classifications {
		if c == target {
			return true
		}
	}
	return false
}
