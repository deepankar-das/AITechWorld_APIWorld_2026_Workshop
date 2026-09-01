/**
 * Author: Deepankar Das
 */

// Package osguard defines the interface between Enforcer and
// OS-level enforcement mechanisms (eBPF on Linux, Endpoint Security
// Framework on macOS).
//
// The KernelEnforcer interface abstracts the kernel module. The stub
// implementation logs every call with full context, proving the
// integration path works end-to-end. When a real eBPF or ESF module
// is built, it implements the same interface — zero changes to callers.
package osguard

// SyscallType classifies the kernel operation being governed.
type SyscallType string

const (
	SyscallFileOpen    SyscallType = "file.open"
	SyscallFileWrite   SyscallType = "file.write"
	SyscallFileDelete  SyscallType = "file.unlink"
	SyscallExecve      SyscallType = "process.execve"
	SyscallConnect     SyscallType = "network.connect"
	SyscallBind        SyscallType = "network.bind"
	SyscallDNSResolve  SyscallType = "network.dns_resolve"
)

// SyscallRequest represents an OS-level action to be evaluated.
type SyscallRequest struct {
	Type        SyscallType `json:"type"`
	PID         int         `json:"pid"`
	UID         int         `json:"uid"`
	ProcessName string      `json:"process_name"`
	ProcessPath string      `json:"process_path"`

	// File operations
	FilePath    string `json:"file_path,omitempty"`
	FileFlags   int    `json:"file_flags,omitempty"` // O_RDONLY, O_WRONLY, O_RDWR

	// Process execution
	ExecPath    string   `json:"exec_path,omitempty"`
	ExecArgs    []string `json:"exec_args,omitempty"`

	// Network operations
	RemoteAddr  string `json:"remote_addr,omitempty"` // IP:port
	RemoteHost  string `json:"remote_host,omitempty"` // DNS name (if resolved)
	Protocol    string `json:"protocol,omitempty"`     // "tcp", "udp"
}

// SyscallDecision is the kernel module's response.
type SyscallDecision struct {
	Allow      bool   `json:"allow"`
	ReasonCode string `json:"reason_code"`
	ReasonHuman string `json:"reason_human"`
	LogOnly    bool   `json:"log_only"` // If true, log but don't enforce (audit mode)
}

// KernelEnforcer is the interface that the OS-level module must implement.
// This is the contract between Enforcer and the kernel enforcement layer.
//
// Implementations:
//   - StubEnforcer: logs all calls, always allows (proof-of-integration)
//   - (future) EbpfEnforcer: Linux eBPF-based enforcement
//   - (future) EsfEnforcer: macOS Endpoint Security Framework enforcement
type KernelEnforcer interface {
	// Init initializes the kernel module. Returns an error if the module
	// cannot be loaded (missing privileges, unsupported platform, etc.).
	Init(config EnforcerConfig) error

	// EvaluateSyscall decides whether to allow or deny a kernel-level action.
	EvaluateSyscall(req SyscallRequest) SyscallDecision

	// RegisterPolicy pushes governance rules to the kernel module.
	// The kernel module should cache these for fast in-kernel evaluation.
	RegisterPolicy(rules []KernelRule) error

	// GetMetrics returns enforcement statistics from the kernel module.
	GetMetrics() KernelMetrics

	// Shutdown cleanly unloads the kernel module.
	Shutdown() error
}

// EnforcerConfig configures the kernel enforcer.
type EnforcerConfig struct {
	// Mode controls enforcement behavior.
	// "enforce": block denied actions
	// "audit":   log but don't block (for safe rollout)
	// "off":     disabled
	Mode string `json:"mode"`

	// WorkspaceRoot is the project directory. File operations inside
	// this path are generally allowed; operations outside are governed.
	WorkspaceRoot string `json:"workspace_root"`

	// GovernedUIDs limits enforcement to specific user IDs.
	// Empty means all users are governed.
	GovernedUIDs []int `json:"governed_uids,omitempty"`

	// AllowedHosts is the network allowlist for connect() governance.
	AllowedHosts []string `json:"allowed_hosts,omitempty"`

	// DeniedPaths are file paths that are always denied (e.g., ~/.ssh/*).
	DeniedPaths []string `json:"denied_paths,omitempty"`

	// DeniedExecs are executables that require approval (e.g., rm, curl).
	DeniedExecs []string `json:"denied_execs,omitempty"`
}

// KernelRule is a governance rule pushed to the kernel module.
type KernelRule struct {
	ID         string      `json:"id"`
	Type       SyscallType `json:"type"`
	Pattern    string      `json:"pattern"`    // Path glob, exec name, or host pattern
	Decision   string      `json:"decision"`   // "allow", "deny", "log"
	Priority   int         `json:"priority"`   // Higher = evaluated first
}

// KernelMetrics tracks enforcement activity in the kernel module.
type KernelMetrics struct {
	TotalEvaluated int64            `json:"total_evaluated"`
	TotalAllowed   int64            `json:"total_allowed"`
	TotalDenied    int64            `json:"total_denied"`
	TotalLogged    int64            `json:"total_logged"`
	BySyscallType  map[string]int64 `json:"by_syscall_type"`
	ByDecision     map[string]int64 `json:"by_decision"`
	UptimeSeconds  int64            `json:"uptime_seconds"`
}
