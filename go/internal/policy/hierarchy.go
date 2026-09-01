/**
 * Author: Deepankar Das
 */

package policy

import (
	"github.com/anthropics/enforcer/internal/types"
)

// DecisionSeverity maps decision types to their enforcement strength.
// Lower levels cannot weaken higher-level decisions.
var DecisionSeverity = map[types.PolicyDecisionType]int{
	types.DecisionAllow:           0,
	types.DecisionSimulate:        1,
	types.DecisionRequireApproval: 2,
	types.DecisionRedact:          3,
	types.DecisionQuarantine:      4,
	types.DecisionDeny:            5,
}

// IsValidTightening checks if a child rule tightens (or maintains) the parent's enforcement.
func IsValidTightening(parentDecision, childDecision types.PolicyDecisionType) bool {
	parentSev := DecisionSeverity[parentDecision]
	childSev := DecisionSeverity[childDecision]
	return childSev >= parentSev
}

// MergeHierarchy merges policy bundles from org → team → repo → local.
// Lower levels can only tighten enforcement (add deny/require_approval rules).
// Rules that would weaken parent enforcement are dropped.
func MergeHierarchy(bundles ...*types.PolicyBundle) types.PolicyBundle {
	merged := types.PolicyBundle{
		BundleVersion: "merged",
		ScopeLevel:    types.ScopeOrganization,
	}

	for _, bundle := range bundles {
		if bundle == nil {
			continue
		}
		if bundle.BundleVersion != "" {
			merged.BundleVersion = bundle.BundleVersion
		}
		if bundle.ScopeLevel != "" {
			merged.ScopeLevel = bundle.ScopeLevel
		}
		merged.Rules = append(merged.Rules, bundle.Rules...)
	}

	return merged
}
