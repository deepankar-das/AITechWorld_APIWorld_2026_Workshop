/**
 * Author: Deepankar Das
 */

package osguard

import (
	"testing"
)

func newTestEnforcer(t *testing.T, mode string) *StubEnforcer {
	t.Helper()
	e := NewStubEnforcer()
	err := e.Init(EnforcerConfig{
		Mode:          mode,
		WorkspaceRoot: "/Users/dev/project",
		DeniedPaths:   []string{"~/.ssh/*", "~/.aws/*", "/etc/shadow"},
		DeniedExecs:   []string{"rm", "curl", "nmap"},
		AllowedHosts:  []string{"github.com", "registry.npmjs.org", "*.googleapis.com"},
	})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	t.Cleanup(func() { e.Shutdown() })
	return e
}

// --- File operations ---

func TestFileOpen_InsideWorkspace_Allowed(t *testing.T) {
	e := newTestEnforcer(t, "enforce")
	dec := e.EvaluateSyscall(SyscallRequest{
		Type:        SyscallFileOpen,
		PID:         1234,
		ProcessName: "claude",
		FilePath:    "/Users/dev/project/src/main.go",
	})
	if !dec.Allow {
		t.Errorf("file inside workspace should be allowed, got deny: %s", dec.ReasonCode)
	}
}

func TestFileOpen_OutsideWorkspace_Denied(t *testing.T) {
	e := newTestEnforcer(t, "enforce")
	dec := e.EvaluateSyscall(SyscallRequest{
		Type:        SyscallFileOpen,
		PID:         1234,
		ProcessName: "claude",
		FilePath:    "/etc/passwd",
	})
	if dec.Allow {
		t.Error("file outside workspace should be denied")
	}
	if dec.ReasonCode != "OSGUARD_OUTSIDE_WORKSPACE" {
		t.Errorf("expected OSGUARD_OUTSIDE_WORKSPACE, got %s", dec.ReasonCode)
	}
}

func TestFileOpen_DeniedPath_Blocked(t *testing.T) {
	e := newTestEnforcer(t, "enforce")
	dec := e.EvaluateSyscall(SyscallRequest{
		Type:        SyscallFileOpen,
		PID:         1234,
		ProcessName: "claude",
		FilePath:    "/etc/shadow",
	})
	if dec.Allow {
		t.Error("denied path should be blocked")
	}
	if dec.ReasonCode != "OSGUARD_DENIED_PATH" {
		t.Errorf("expected OSGUARD_DENIED_PATH, got %s", dec.ReasonCode)
	}
}

// --- Process execution ---

func TestExecve_DeniedExec_Blocked(t *testing.T) {
	e := newTestEnforcer(t, "enforce")
	dec := e.EvaluateSyscall(SyscallRequest{
		Type:        SyscallExecve,
		PID:         5678,
		ProcessName: "bash",
		ExecPath:    "/usr/bin/rm",
		ExecArgs:    []string{"-rf", "/"},
	})
	if dec.Allow {
		t.Error("denied exec should be blocked")
	}
	if dec.ReasonCode != "OSGUARD_DENIED_EXEC" {
		t.Errorf("expected OSGUARD_DENIED_EXEC, got %s", dec.ReasonCode)
	}
}

func TestExecve_SafeExec_Allowed(t *testing.T) {
	e := newTestEnforcer(t, "enforce")
	dec := e.EvaluateSyscall(SyscallRequest{
		Type:        SyscallExecve,
		PID:         5678,
		ProcessName: "bash",
		ExecPath:    "/usr/bin/ls",
		ExecArgs:    []string{"-la"},
	})
	if !dec.Allow {
		t.Errorf("safe exec should be allowed, got deny: %s", dec.ReasonCode)
	}
}

func TestExecve_CurlBlocked(t *testing.T) {
	e := newTestEnforcer(t, "enforce")
	dec := e.EvaluateSyscall(SyscallRequest{
		Type:        SyscallExecve,
		PID:         5678,
		ProcessName: "bash",
		ExecPath:    "/usr/bin/curl",
		ExecArgs:    []string{"https://evil.com"},
	})
	if dec.Allow {
		t.Error("curl should be blocked by denied execs")
	}
}

// --- Network operations ---

func TestConnect_AllowlistedHost_Allowed(t *testing.T) {
	e := newTestEnforcer(t, "enforce")
	dec := e.EvaluateSyscall(SyscallRequest{
		Type:        SyscallConnect,
		PID:         9999,
		ProcessName: "node",
		RemoteAddr:  "140.82.121.3:443",
		RemoteHost:  "github.com",
	})
	if !dec.Allow {
		t.Errorf("allowlisted host should be allowed, got: %s", dec.ReasonCode)
	}
}

func TestConnect_UnknownHost_Denied(t *testing.T) {
	e := newTestEnforcer(t, "enforce")
	dec := e.EvaluateSyscall(SyscallRequest{
		Type:        SyscallConnect,
		PID:         9999,
		ProcessName: "curl",
		RemoteAddr:  "1.2.3.4:443",
		RemoteHost:  "evil.com",
	})
	if dec.Allow {
		t.Error("unknown host should be denied")
	}
	if dec.ReasonCode != "OSGUARD_HOST_NOT_ALLOWED" {
		t.Errorf("expected OSGUARD_HOST_NOT_ALLOWED, got %s", dec.ReasonCode)
	}
}

func TestConnect_WildcardAllowlist(t *testing.T) {
	e := newTestEnforcer(t, "enforce")
	dec := e.EvaluateSyscall(SyscallRequest{
		Type:        SyscallConnect,
		PID:         9999,
		ProcessName: "gcloud",
		RemoteHost:  "storage.googleapis.com",
	})
	if !dec.Allow {
		t.Errorf("wildcard allowlist should match, got: %s", dec.ReasonCode)
	}
}

// --- Audit mode ---

func TestAuditMode_LogsButAllows(t *testing.T) {
	e := newTestEnforcer(t, "audit")
	dec := e.EvaluateSyscall(SyscallRequest{
		Type:        SyscallExecve,
		PID:         5678,
		ProcessName: "bash",
		ExecPath:    "/usr/bin/rm",
		ExecArgs:    []string{"-rf", "node_modules"},
	})
	// In audit mode, the decision is deny but LogOnly=true
	if dec.Allow {
		t.Error("audit mode should still evaluate as deny for denied execs")
	}
	if !dec.LogOnly {
		t.Error("audit mode should set LogOnly=true")
	}
}

// --- Disabled mode ---

func TestDisabledMode_AllowsEverything(t *testing.T) {
	e := newTestEnforcer(t, "off")
	dec := e.EvaluateSyscall(SyscallRequest{
		Type:        SyscallExecve,
		PID:         5678,
		ExecPath:    "/usr/bin/rm",
	})
	if !dec.Allow {
		t.Error("disabled mode should allow everything")
	}
	if dec.ReasonCode != "OSGUARD_DISABLED" {
		t.Errorf("expected OSGUARD_DISABLED, got %s", dec.ReasonCode)
	}
}

// --- Custom rules ---

func TestRegisterPolicy_CustomRuleDenies(t *testing.T) {
	e := newTestEnforcer(t, "enforce")
	e.RegisterPolicy([]KernelRule{
		{ID: "block-docker", Type: SyscallExecve, Pattern: "docker", Decision: "deny", Priority: 100},
	})
	dec := e.EvaluateSyscall(SyscallRequest{
		Type:     SyscallExecve,
		PID:      1000,
		ExecPath: "/usr/bin/docker",
	})
	if dec.Allow {
		t.Error("custom deny rule should block docker")
	}
}

// --- Metrics ---

func TestMetrics_TracksCounts(t *testing.T) {
	e := newTestEnforcer(t, "enforce")

	// Allowed file operation
	e.EvaluateSyscall(SyscallRequest{Type: SyscallFileOpen, FilePath: "/Users/dev/project/main.go"})
	// Denied exec
	e.EvaluateSyscall(SyscallRequest{Type: SyscallExecve, ExecPath: "/usr/bin/rm"})
	// Allowed exec
	e.EvaluateSyscall(SyscallRequest{Type: SyscallExecve, ExecPath: "/usr/bin/ls"})

	m := e.GetMetrics()
	if m.TotalEvaluated != 3 {
		t.Errorf("expected 3 evaluations, got %d", m.TotalEvaluated)
	}
	if m.TotalAllowed != 2 {
		t.Errorf("expected 2 allowed, got %d", m.TotalAllowed)
	}
	if m.TotalDenied != 1 {
		t.Errorf("expected 1 denied, got %d", m.TotalDenied)
	}
}

// --- Invocation logging (proof of integration) ---

func TestInvocationRecordsAreCreated(t *testing.T) {
	e := newTestEnforcer(t, "enforce")

	e.EvaluateSyscall(SyscallRequest{Type: SyscallFileOpen, FilePath: "/Users/dev/project/main.go", PID: 100, ProcessName: "claude"})
	e.EvaluateSyscall(SyscallRequest{Type: SyscallExecve, ExecPath: "/usr/bin/rm", PID: 200, ProcessName: "bash"})
	e.EvaluateSyscall(SyscallRequest{Type: SyscallConnect, RemoteHost: "evil.com", PID: 300, ProcessName: "curl"})

	records := e.GetInvocations()
	// Should have: Init + 3 EvaluateSyscall = 4 records minimum
	if len(records) < 4 {
		t.Errorf("expected at least 4 invocation records, got %d", len(records))
	}

	// Verify each record has a timestamp and stub note
	for i, rec := range records {
		if rec.Timestamp == "" {
			t.Errorf("record %d missing timestamp", i)
		}
		if rec.StubNote == "" {
			t.Errorf("record %d missing stub note", i)
		}
		if rec.Method == "" {
			t.Errorf("record %d missing method", i)
		}
	}

	// Verify the stub notes describe what a real kernel module would do
	for _, rec := range records {
		if rec.Method == "EvaluateSyscall" && rec.StubNote == "" {
			t.Error("EvaluateSyscall records should have stub notes describing kernel behavior")
		}
	}
}

func TestStubNotes_DescribeKernelBehavior(t *testing.T) {
	e := newTestEnforcer(t, "enforce")

	e.EvaluateSyscall(SyscallRequest{
		Type:        SyscallConnect,
		PID:         300,
		ProcessName: "curl",
		RemoteAddr:  "1.2.3.4:443",
		RemoteHost:  "evil.com",
	})

	records := e.GetInvocations()
	found := false
	for _, rec := range records {
		if rec.Method == "EvaluateSyscall" && rec.Request != nil && rec.Request.Type == SyscallConnect {
			found = true
			// Stub note should mention what the real kernel module would do
			if rec.StubNote == "" {
				t.Error("connect stub note should not be empty")
			}
		}
	}
	if !found {
		t.Error("expected to find a connect invocation record")
	}
}
