/**
 * Author: Deepankar Das
 */

package audit

import (
	"fmt"

	"github.com/anthropics/enforcer/internal/types"
)

// ValidateAuditEvent checks the 6 minimum gate fields (who, what, when,
// policy, decision, result) are non-empty strings. Returns true with an
// empty slice if valid, or false with a list of missing field names.
func ValidateAuditEvent(event types.AuditEvent) (bool, []string) {
	var missing []string

	fields := map[string]string{
		"who":      event.Who,
		"what":     event.What,
		"when":     event.When,
		"policy":   event.Policy,
		"decision": event.Decision,
		"result":   event.Result,
	}

	for _, name := range types.MinimumGateFields {
		if fields[name] == "" {
			missing = append(missing, name)
		}
	}

	return len(missing) == 0, missing
}

// BuildGateFields constructs the 6 minimum gate fields from structured input.
// The "who" field is a compound of actor user_id, agent_type, and session_id.
// The "what" field combines action type and attempted_action.
// The "when" field is the timestamp.
// The "policy" field combines policy_id and policy_version.
// The "decision" and "result" fields are passed through directly.
func BuildGateFields(
	actor types.Actor,
	sessionID string,
	action types.ActionDetail,
	timestamp string,
	policyID string,
	policyVersion string,
	decision string,
	result string,
) map[string]string {
	who := fmt.Sprintf("%s|%s|%s", actor.UserID, actor.AgentType, sessionID)
	what := fmt.Sprintf("%s|%s", action.Type, action.AttemptedAction)
	policy := fmt.Sprintf("%s@%s", policyID, policyVersion)

	return map[string]string{
		"who":      who,
		"what":     what,
		"when":     timestamp,
		"policy":   policy,
		"decision": decision,
		"result":   result,
	}
}
