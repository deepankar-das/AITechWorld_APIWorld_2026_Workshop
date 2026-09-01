/**
 * Author: Deepankar Das
 */

package intelligence

import (
	"testing"
	"time"

	"github.com/anthropics/enforcer/internal/types"
)

func makeEvent(sessionID, actionType, value string, classification []string) types.AuditEvent {
	now := time.Now().UTC().Format(time.RFC3339)
	return types.AuditEvent{
		EventID:   "evt_test",
		Timestamp: now,
		SessionID: sessionID,
		Who:       "test",
		What:      actionType + ":" + value,
		When:      now,
		Policy:    "test@v1",
		Decision:  "allow:TEST",
		Result:    "executed",
		Action: struct {
			Type            string `json:"type"`
			AttemptedAction string `json:"attempted_action"`
			ObservedEffect  string `json:"observed_effect"`
		}{Type: actionType, AttemptedAction: value},
		Resource: struct {
			Kind           string   `json:"kind"`
			Path           string   `json:"path,omitempty"`
			Host           string   `json:"host,omitempty"`
			Value          string   `json:"value,omitempty"`
			Classification []string `json:"classification"`
		}{Value: value, Classification: classification},
		PolicyDetail: struct {
			PolicyID      string `json:"policy_id"`
			PolicyVersion string `json:"policy_version"`
			Decision      string `json:"decision"`
			ReasonCode    string `json:"reason_code"`
			ReasonHuman   string `json:"reason_human"`
		}{Decision: "allow"},
	}
}

func TestExfiltrationPattern(t *testing.T) {
	d := NewAnomalyDetector()
	sid := "sess_exfil_1"

	// Step 1: Read a sensitive file
	e1 := makeEvent(sid, "file.read", "~/.ssh/id_rsa", []string{"sensitive_path"})
	alerts := d.DetectAnomalies(e1)
	if len(alerts) != 0 {
		t.Error("single read should not trigger alert")
	}

	// Step 2: Network request (should trigger exfil pattern)
	e2 := makeEvent(sid, "network.request", "https://evil.com/upload", nil)
	alerts = d.DetectAnomalies(e2)
	found := false
	for _, a := range alerts {
		if a.PatternID == "exfil_secret_then_network" {
			found = true
			if a.Severity != "critical" {
				t.Errorf("expected critical severity, got %s", a.Severity)
			}
		}
	}
	if !found {
		t.Error("expected exfil_secret_then_network alert")
	}
}

func TestSupplyChainPattern(t *testing.T) {
	d := NewAnomalyDetector()
	sid := "sess_supply_1"

	// Step 1: Write to lockfile
	e1 := makeEvent(sid, "file.write", "package-lock.json", nil)
	e1.Resource.Path = "/project/package-lock.json"
	d.DetectAnomalies(e1)

	// Step 2: Package install
	e2 := makeEvent(sid, "shell.exec", "npm install malicious-pkg", nil)
	alerts := d.DetectAnomalies(e2)
	found := false
	for _, a := range alerts {
		if a.PatternID == "supply_chain_lockfile_then_install" {
			found = true
		}
	}
	if !found {
		t.Error("expected supply_chain_lockfile_then_install alert")
	}
}

func TestDestructiveForceAfterReset(t *testing.T) {
	d := NewAnomalyDetector()
	sid := "sess_destruct_1"

	e1 := makeEvent(sid, "shell.exec", "git reset --hard HEAD~3", nil)
	d.DetectAnomalies(e1)

	e2 := makeEvent(sid, "shell.exec", "git push --force origin main", nil)
	alerts := d.DetectAnomalies(e2)
	found := false
	for _, a := range alerts {
		if a.PatternID == "destructive_force_push_after_reset" {
			found = true
		}
	}
	if !found {
		t.Error("expected destructive_force_push_after_reset alert")
	}
}

func TestEvasionDeniedRetry(t *testing.T) {
	d := NewAnomalyDetector()
	sid := "sess_evasion_1"

	// 3+ denied actions of same type
	for i := 0; i < 4; i++ {
		e := makeEvent(sid, "file.write", "/etc/passwd", nil)
		e.PolicyDetail.Decision = "deny"
		alerts := d.DetectAnomalies(e)
		if i >= 3 {
			found := false
			for _, a := range alerts {
				if a.PatternID == "evasion_denied_then_retry" {
					found = true
				}
			}
			if !found {
				t.Error("expected evasion_denied_then_retry alert after 4 denials")
			}
		}
	}
}

func TestNoFalsePositivesNormalWork(t *testing.T) {
	d := NewAnomalyDetector()
	sid := "sess_normal_1"

	// Normal development workflow
	events := []types.AuditEvent{
		makeEvent(sid, "file.read", "src/main.ts", nil),
		makeEvent(sid, "file.write", "src/main.ts", nil),
		makeEvent(sid, "shell.exec", "npm test", nil),
		makeEvent(sid, "file.read", "README.md", nil),
	}

	for _, e := range events {
		alerts := d.DetectAnomalies(e)
		if len(alerts) > 0 {
			t.Errorf("normal development work should not trigger alerts, got %v", alerts)
		}
	}
}

func TestPatternsLoaded(t *testing.T) {
	d := NewAnomalyDetector()
	patterns := d.GetPatterns()
	if len(patterns) < 5 {
		t.Errorf("expected at least 5 patterns, got %d", len(patterns))
	}
}

func TestGetAlerts(t *testing.T) {
	d := NewAnomalyDetector()
	sid := "sess_alerts_1"

	// Trigger an alert
	e1 := makeEvent(sid, "file.read", "~/.ssh/id_rsa", []string{"sensitive_path"})
	d.DetectAnomalies(e1)
	e2 := makeEvent(sid, "network.request", "https://evil.com", nil)
	d.DetectAnomalies(e2)

	alerts := d.GetAlerts(10)
	if len(alerts) == 0 {
		t.Error("expected at least one alert")
	}
}
