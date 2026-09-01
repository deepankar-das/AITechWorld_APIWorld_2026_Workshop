/**
 * Author: Deepankar Das
 */

package approval

import (
	"fmt"
	"time"

	"github.com/anthropics/enforcer/internal/types"
)

// RequestBreakGlass creates an emergency break-glass approval decision.
// A break-glass override bypasses normal approval flow but records full
// audit context for post-incident review.
//
// Validation:
//   - rationale must be non-empty (the reviewer must explain why).
//   - approverID must be non-empty.
func RequestBreakGlass(approvalID, approverID, rationale string) (types.ApprovalDecision, error) {
	if rationale == "" {
		return types.ApprovalDecision{}, fmt.Errorf("break-glass requires a non-empty rationale")
	}
	if approverID == "" {
		return types.ApprovalDecision{}, fmt.Errorf("break-glass requires a non-empty approver_id")
	}

	return types.ApprovalDecision{
		ApprovalID:   approvalID,
		Decision:     "approve",
		ApproverID:   approverID,
		Rationale:    fmt.Sprintf("[BREAK-GLASS %s] %s", time.Now().UTC().Format(time.RFC3339), rationale),
		IsBreakGlass: true,
	}, nil
}
