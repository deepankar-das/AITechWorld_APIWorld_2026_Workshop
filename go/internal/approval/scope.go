/**
 * Author: Deepankar Das
 */

package approval

import (
	"strings"
	"time"

	"github.com/anthropics/enforcer/internal/types"
)

// MatchesScope checks whether an incoming action request matches the given
// approval scope.
//
// Scope semantics:
//   - Single-use scopes (ScopeSingle) NEVER match here; they are consumed on
//     first use and removed by the caller (ApprovalService.CheckScope).
//     Returning false ensures that a single-use scope is never reused.
//   - Time-bounded scopes (ScopeTimeBounded) match only if the current time
//     is before the scope's expiry.
//   - Session scopes (ScopeSession) match as long as the pattern matches.
//
// Pattern matching:
//   - An empty pattern matches everything.
//   - The pattern is compared against the action type (string prefix) and
//     the resource value (wildcard prefix: "foo.*" matches "foo.bar").
func MatchesScope(request types.ActionRequest, scope types.ApprovalScope) bool {
	// Single-use scopes match once and are consumed by CheckScope after use.

	// Time-bounded scopes check expiry.
	if scope.Type == types.ScopeTimeBounded {
		if scope.Expiry != "" {
			expiry, err := time.Parse(time.RFC3339, scope.Expiry)
			if err != nil {
				return false
			}
			if time.Now().UTC().After(expiry) {
				return false
			}
		}
	}

	// Pattern matching.
	if scope.Pattern == "" {
		return true
	}

	actionType := string(request.Action.Type)
	resourceValue := request.Resource.Value
	if resourceValue == "" {
		resourceValue = request.Resource.Path
	}
	if resourceValue == "" {
		resourceValue = request.Resource.Host
	}

	// Exact match on action type prefix.
	if strings.HasPrefix(actionType, scope.Pattern) {
		return true
	}

	// Wildcard prefix match on resource value: "foo.*" matches "foo.bar".
	if strings.HasSuffix(scope.Pattern, ".*") {
		prefix := scope.Pattern[:len(scope.Pattern)-2]
		if strings.HasPrefix(actionType, prefix) {
			return true
		}
		if strings.HasPrefix(resourceValue, prefix) {
			return true
		}
	}

	// Direct resource value match.
	if resourceValue != "" && strings.HasPrefix(resourceValue, scope.Pattern) {
		return true
	}

	return false
}
