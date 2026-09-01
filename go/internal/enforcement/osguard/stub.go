/**
 * Author: Deepankar Das
 */

package osguard

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// StubEnforcer implements KernelEnforcer by logging every syscall
// evaluation with full context. It proves the integration path works
// end-to-end without requiring actual kernel modules.
//
// When a real eBPF or ESF enforcer is built, it replaces this stub
// by implementing the same KernelEnforcer interface.
type StubEnforcer struct {
	mu            sync.RWMutex
	config        EnforcerConfig
	rules         []KernelRule
	invocations   []InvocationRecord
	startTime     time.Time

	// Atomic counters for metrics
	totalEvaluated atomic.Int64
	totalAllowed   atomic.Int64
	totalDenied    atomic.Int64
	totalLogged    atomic.Int64

	bySyscallType sync.Map // map[string]*atomic.Int64
	byDecision    sync.Map // map[string]*atomic.Int64

	// Log file for invocation records (persistent proof)
	logFile *os.File
}

// InvocationRecord captures every call to the kernel enforcer interface.
// This is the proof that the platform calls into the kernel module.
type InvocationRecord struct {
	Timestamp   string          `json:"timestamp"`
	Method      string          `json:"method"`      // "EvaluateSyscall", "RegisterPolicy", etc.
	Request     *SyscallRequest `json:"request,omitempty"`
	Decision    *SyscallDecision `json:"decision,omitempty"`
	RuleCount   int             `json:"rule_count,omitempty"`
	LatencyUs   int64           `json:"latency_us"`
	StubNote    string          `json:"stub_note"` // Explains what a real kernel module would do
}

// NewStubEnforcer creates a stub enforcer that logs all invocations.
func NewStubEnforcer() *StubEnforcer {
	return &StubEnforcer{
		invocations: make([]InvocationRecord, 0, 1000),
	}
}

// Init initializes the stub enforcer and opens the invocation log.
func (s *StubEnforcer) Init(config EnforcerConfig) error {
	s.mu.Lock()
	s.config = config
	s.startTime = time.Now()

	// Open invocation log file for persistent proof
	logPath := "build/osguard-invocations.jsonl"
	os.MkdirAll("build", 0755)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		slog.Warn("Could not open OS guard invocation log", "path", logPath, "error", err)
	} else {
		s.logFile = f
	}
	s.mu.Unlock()

	s.record(InvocationRecord{
		Method:   "Init",
		StubNote: fmt.Sprintf("[STUB] Kernel enforcer initialized in '%s' mode. A real eBPF module would attach BPF programs to tracepoints (sys_enter_open, sys_enter_execve, sys_enter_connect). A real ESF client would register ES_EVENT_TYPE_AUTH_OPEN, ES_EVENT_TYPE_AUTH_EXEC, ES_EVENT_TYPE_AUTH_CONNECT.", config.Mode),
	})

	slog.Info("[OS Guard] Stub enforcer initialized",
		"mode", config.Mode,
		"workspace", config.WorkspaceRoot,
		"governed_uids", config.GovernedUIDs,
		"denied_paths", len(config.DeniedPaths),
		"denied_execs", len(config.DeniedExecs),
		"allowed_hosts", len(config.AllowedHosts),
	)

	return nil
}

// EvaluateSyscall logs the syscall and evaluates against configured rules.
func (s *StubEnforcer) EvaluateSyscall(req SyscallRequest) SyscallDecision {
	start := time.Now()

	decision := s.evaluate(req)

	latency := time.Since(start).Microseconds()

	// Update metrics
	s.totalEvaluated.Add(1)
	if decision.Allow {
		s.totalAllowed.Add(1)
	} else {
		s.totalDenied.Add(1)
	}
	if decision.LogOnly {
		s.totalLogged.Add(1)
	}
	s.incrementMap(&s.bySyscallType, string(req.Type))
	if decision.Allow {
		s.incrementMap(&s.byDecision, "allow")
	} else {
		s.incrementMap(&s.byDecision, "deny")
	}

	stubNote := s.describeKernelAction(req, decision)

	s.record(InvocationRecord{
		Method:    "EvaluateSyscall",
		Request:   &req,
		Decision:  &decision,
		LatencyUs: latency,
		StubNote:  stubNote,
	})

	return decision
}

func (s *StubEnforcer) evaluate(req SyscallRequest) SyscallDecision {
	s.mu.RLock()
	mode := s.config.Mode
	s.mu.RUnlock()

	if mode == "off" {
		return SyscallDecision{Allow: true, ReasonCode: "OSGUARD_DISABLED", ReasonHuman: "OS guard is disabled"}
	}

	logOnly := mode == "audit"

	// Check against rules (higher priority first — rules are pre-sorted)
	s.mu.RLock()
	rules := s.rules
	config := s.config
	s.mu.RUnlock()

	for _, rule := range rules {
		if rule.Type != req.Type {
			continue
		}
		if matchesPattern(req, rule.Pattern) {
			switch rule.Decision {
			case "deny":
				return SyscallDecision{
					Allow:       false,
					ReasonCode:  "OSGUARD_RULE_DENY_" + rule.ID,
					ReasonHuman: fmt.Sprintf("OS-level rule '%s' denies %s matching '%s'", rule.ID, req.Type, rule.Pattern),
					LogOnly:     logOnly,
				}
			case "allow":
				return SyscallDecision{
					Allow:       true,
					ReasonCode:  "OSGUARD_RULE_ALLOW_" + rule.ID,
					ReasonHuman: fmt.Sprintf("OS-level rule '%s' allows %s", rule.ID, req.Type),
					LogOnly:     logOnly,
				}
			}
		}
	}

	// Default evaluation based on config
	switch req.Type {
	case SyscallFileOpen, SyscallFileWrite, SyscallFileDelete:
		return s.evaluateFileOp(req, config, logOnly)
	case SyscallExecve:
		return s.evaluateExec(req, config, logOnly)
	case SyscallConnect, SyscallDNSResolve:
		return s.evaluateNetwork(req, config, logOnly)
	}

	return SyscallDecision{Allow: true, ReasonCode: "OSGUARD_DEFAULT_ALLOW", ReasonHuman: "No matching OS-level rule", LogOnly: logOnly}
}

func (s *StubEnforcer) evaluateFileOp(req SyscallRequest, config EnforcerConfig, logOnly bool) SyscallDecision {
	path := req.FilePath

	// Check denied paths
	for _, denied := range config.DeniedPaths {
		if matchGlob(path, denied) {
			return SyscallDecision{
				Allow:       false,
				ReasonCode:  "OSGUARD_DENIED_PATH",
				ReasonHuman: fmt.Sprintf("File path '%s' matches denied pattern '%s'", path, denied),
				LogOnly:     logOnly,
			}
		}
	}

	// Check workspace boundary
	if config.WorkspaceRoot != "" && path != "" {
		absPath, _ := filepath.Abs(path)
		absWorkspace, _ := filepath.Abs(config.WorkspaceRoot)
		if !strings.HasPrefix(absPath, absWorkspace+string(filepath.Separator)) && absPath != absWorkspace {
			return SyscallDecision{
				Allow:       false,
				ReasonCode:  "OSGUARD_OUTSIDE_WORKSPACE",
				ReasonHuman: fmt.Sprintf("File '%s' is outside workspace '%s'", path, config.WorkspaceRoot),
				LogOnly:     logOnly,
			}
		}
	}

	return SyscallDecision{Allow: true, ReasonCode: "OSGUARD_FILE_ALLOWED", ReasonHuman: "File operation within permitted scope"}
}

func (s *StubEnforcer) evaluateExec(req SyscallRequest, config EnforcerConfig, logOnly bool) SyscallDecision {
	execName := filepath.Base(req.ExecPath)

	for _, denied := range config.DeniedExecs {
		if execName == denied || req.ExecPath == denied {
			return SyscallDecision{
				Allow:       false,
				ReasonCode:  "OSGUARD_DENIED_EXEC",
				ReasonHuman: fmt.Sprintf("Execution of '%s' is denied by OS-level policy", execName),
				LogOnly:     logOnly,
			}
		}
	}

	return SyscallDecision{Allow: true, ReasonCode: "OSGUARD_EXEC_ALLOWED", ReasonHuman: "Process execution permitted"}
}

func (s *StubEnforcer) evaluateNetwork(req SyscallRequest, config EnforcerConfig, logOnly bool) SyscallDecision {
	host := req.RemoteHost
	if host == "" {
		// Extract host from addr (strip port)
		host = req.RemoteAddr
		if idx := strings.LastIndex(host, ":"); idx > 0 {
			host = host[:idx]
		}
	}

	if len(config.AllowedHosts) > 0 {
		for _, allowed := range config.AllowedHosts {
			if matchGlob(host, allowed) {
				return SyscallDecision{Allow: true, ReasonCode: "OSGUARD_HOST_ALLOWED", ReasonHuman: fmt.Sprintf("Host '%s' is allowlisted", host)}
			}
		}
		return SyscallDecision{
			Allow:       false,
			ReasonCode:  "OSGUARD_HOST_NOT_ALLOWED",
			ReasonHuman: fmt.Sprintf("Host '%s' is not in OS-level allowlist", host),
			LogOnly:     logOnly,
		}
	}

	return SyscallDecision{Allow: true, ReasonCode: "OSGUARD_NETWORK_ALLOWED", ReasonHuman: "No OS-level network restrictions configured"}
}

// RegisterPolicy pushes rules to the enforcer.
func (s *StubEnforcer) RegisterPolicy(rules []KernelRule) error {
	s.mu.Lock()
	s.rules = rules
	s.mu.Unlock()

	s.record(InvocationRecord{
		Method:    "RegisterPolicy",
		RuleCount: len(rules),
		StubNote:  fmt.Sprintf("[STUB] %d rules registered. A real eBPF module would compile these into BPF maps for O(1) in-kernel lookups. A real ESF client would update its mute sets and decision cache.", len(rules)),
	})

	slog.Info("[OS Guard] Policy registered", "rules", len(rules))
	return nil
}

// GetMetrics returns enforcement statistics.
func (s *StubEnforcer) GetMetrics() KernelMetrics {
	bySyscall := make(map[string]int64)
	s.bySyscallType.Range(func(key, value interface{}) bool {
		bySyscall[key.(string)] = value.(*atomic.Int64).Load()
		return true
	})

	byDecision := make(map[string]int64)
	s.byDecision.Range(func(key, value interface{}) bool {
		byDecision[key.(string)] = value.(*atomic.Int64).Load()
		return true
	})

	return KernelMetrics{
		TotalEvaluated: s.totalEvaluated.Load(),
		TotalAllowed:   s.totalAllowed.Load(),
		TotalDenied:    s.totalDenied.Load(),
		TotalLogged:    s.totalLogged.Load(),
		BySyscallType:  bySyscall,
		ByDecision:     byDecision,
		UptimeSeconds:  int64(time.Since(s.startTime).Seconds()),
	}
}

// GetInvocations returns all recorded invocations (proof of integration).
func (s *StubEnforcer) GetInvocations() []InvocationRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]InvocationRecord, len(s.invocations))
	copy(result, s.invocations)
	return result
}

// Shutdown cleans up resources.
func (s *StubEnforcer) Shutdown() error {
	s.record(InvocationRecord{
		Method:   "Shutdown",
		StubNote: "[STUB] Enforcer shutting down. A real eBPF module would detach BPF programs and free maps. A real ESF client would unsubscribe from events.",
	})

	if s.logFile != nil {
		s.logFile.Close()
	}

	slog.Info("[OS Guard] Stub enforcer shut down",
		"total_evaluated", s.totalEvaluated.Load(),
		"total_denied", s.totalDenied.Load(),
	)
	return nil
}

// --- Internal helpers ---

func (s *StubEnforcer) record(rec InvocationRecord) {
	rec.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)

	s.mu.Lock()
	if len(s.invocations) < 10000 {
		s.invocations = append(s.invocations, rec)
	}
	s.mu.Unlock()

	// Persist to log file
	if s.logFile != nil {
		line, _ := json.Marshal(rec)
		s.logFile.Write(append(line, '\n'))
	}
}

func (s *StubEnforcer) incrementMap(m *sync.Map, key string) {
	val, _ := m.LoadOrStore(key, &atomic.Int64{})
	val.(*atomic.Int64).Add(1)
}

func (s *StubEnforcer) describeKernelAction(req SyscallRequest, dec SyscallDecision) string {
	action := "ALLOW"
	if !dec.Allow {
		action = "DENY"
	}
	if dec.LogOnly {
		action = "LOG-ONLY(" + action + ")"
	}

	switch req.Type {
	case SyscallFileOpen, SyscallFileWrite, SyscallFileDelete:
		return fmt.Sprintf("[STUB] %s %s on '%s' by PID %d (%s). Real kernel module: BPF program on tracepoint/sys_enter_openat would intercept before the syscall completes.",
			action, req.Type, req.FilePath, req.PID, req.ProcessName)
	case SyscallExecve:
		return fmt.Sprintf("[STUB] %s execve('%s', %v) by PID %d. Real kernel module: BPF program on tracepoint/sys_enter_execve would block before the new process image loads.",
			action, req.ExecPath, req.ExecArgs, req.PID)
	case SyscallConnect:
		return fmt.Sprintf("[STUB] %s connect() to %s (%s) by PID %d (%s). Real kernel module: BPF program on tracepoint/sys_enter_connect or cgroup/connect4 would reject the socket connection.",
			action, req.RemoteAddr, req.RemoteHost, req.PID, req.ProcessName)
	default:
		return fmt.Sprintf("[STUB] %s %s by PID %d", action, req.Type, req.PID)
	}
}

func matchesPattern(req SyscallRequest, pattern string) bool {
	switch req.Type {
	case SyscallFileOpen, SyscallFileWrite, SyscallFileDelete:
		return matchGlob(req.FilePath, pattern)
	case SyscallExecve:
		return matchGlob(req.ExecPath, pattern) || matchGlob(filepath.Base(req.ExecPath), pattern)
	case SyscallConnect, SyscallDNSResolve:
		return matchGlob(req.RemoteHost, pattern) || matchGlob(req.RemoteAddr, pattern)
	}
	return false
}

func matchGlob(value, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasPrefix(pattern, "*.") {
		return strings.HasSuffix(value, pattern[1:])
	}
	if strings.HasSuffix(pattern, "/*") {
		dir := pattern[:len(pattern)-2]
		return strings.HasPrefix(value, dir+"/")
	}
	return value == pattern
}
