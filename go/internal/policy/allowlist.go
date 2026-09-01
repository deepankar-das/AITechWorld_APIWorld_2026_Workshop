/**
 * Author: Deepankar Das
 */

package policy

import (
	"os"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// NetworkAllowlist holds the parsed allowlist and warning list.
type NetworkAllowlist struct {
	Allowlist   []string `yaml:"allowlist"`
	WarningList []string `yaml:"warning_list"`
}

var (
	loadedAllowlist *NetworkAllowlist
	allowlistOnce   sync.Once
)

// LoadNetworkAllowlist loads the network allowlist from a YAML file.
func LoadNetworkAllowlist(path string) (*NetworkAllowlist, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var al NetworkAllowlist
	if err := yaml.Unmarshal(data, &al); err != nil {
		return nil, err
	}
	return &al, nil
}

// SetGlobalAllowlist sets the globally cached allowlist for the policy engine.
func SetGlobalAllowlist(al *NetworkAllowlist) {
	loadedAllowlist = al
}

// GetGlobalAllowlist returns the cached allowlist (may be nil).
func GetGlobalAllowlist() *NetworkAllowlist {
	return loadedAllowlist
}

// IsHostAllowlisted checks if a host matches the allowlist.
func (al *NetworkAllowlist) IsHostAllowlisted(host string) bool {
	if al == nil {
		return false
	}
	host = strings.ToLower(host)
	for _, entry := range al.Allowlist {
		entry = strings.ToLower(entry)
		if entry == host {
			return true
		}
		// Wildcard match: *.googleapis.com matches storage.googleapis.com
		if strings.HasPrefix(entry, "*.") {
			suffix := entry[1:] // ".googleapis.com"
			if strings.HasSuffix(host, suffix) {
				return true
			}
		}
	}
	return false
}

// IsHostInWarningList checks if a host is on the warning list (requires approval).
func (al *NetworkAllowlist) IsHostInWarningList(host string) bool {
	if al == nil {
		return false
	}
	host = strings.ToLower(host)
	for _, entry := range al.WarningList {
		if strings.ToLower(entry) == host {
			return true
		}
	}
	return false
}
