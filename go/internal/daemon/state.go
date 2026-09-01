/**
 * Author: Deepankar Das
 */

package daemon

import (
	"os"
	"strings"
	"sync"
	"time"
)

// EnforcementState captures the current enforcement toggle state.
type EnforcementState struct {
	Enabled   bool   `json:"enabled"`
	Since     string `json:"since"`
	ChangedBy string `json:"changed_by"`
}

var (
	enforcementMu      sync.RWMutex
	enforcementEnabled = true
	enforcementSince   = time.Now().UTC().Format(time.RFC3339)
	enforcementBy      = "system:init"
)

// IsEnforcementEnabled returns whether policy enforcement is currently active.
func IsEnforcementEnabled() bool {
	enforcementMu.RLock()
	defer enforcementMu.RUnlock()
	return enforcementEnabled
}

// SetEnforcementEnabled toggles enforcement on or off, recording who made
// the change and when.
func SetEnforcementEnabled(enabled bool, changedBy string) {
	enforcementMu.Lock()
	defer enforcementMu.Unlock()
	enforcementEnabled = enabled
	enforcementSince = time.Now().UTC().Format(time.RFC3339)
	enforcementBy = changedBy
}

// GetEnforcementState returns a snapshot of the current enforcement state.
func GetEnforcementState() EnforcementState {
	enforcementMu.RLock()
	defer enforcementMu.RUnlock()
	return EnforcementState{
		Enabled:   enforcementEnabled,
		Since:     enforcementSince,
		ChangedBy: enforcementBy,
	}
}

// IsStrictMode returns whether strict mode is enabled.
// Default is true for enterprise-safe fail-closed behavior.
// Explicit false values ("false", "0", "no", "off") disable strict mode.
func IsStrictMode() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("AA_STRICT_MODE")))
	if v == "" {
		return true
	}
	if v == "false" || v == "0" || v == "no" || v == "off" {
		return false
	}
	return true
}
