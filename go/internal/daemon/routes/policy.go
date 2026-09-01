/**
 * Author: Deepankar Das
 */

package routes

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/anthropics/enforcer/internal/policy"
	"github.com/anthropics/enforcer/internal/types"
)

// PolicyRoutes holds a reference to the live policy bundle (protected by a
// read-write mutex) and exposes handler methods for policy CRUD operations.
type PolicyRoutes struct {
	sync.RWMutex
	bundle       *types.PolicyBundle
	saveBundleFn func(types.PolicyBundle) error
	savePacksFn  func([]policy.PolicyPack) error
}

// NewPolicyRoutes creates a PolicyRoutes wrapping the given bundle.
func NewPolicyRoutes(bundle *types.PolicyBundle, saveBundleFn func(types.PolicyBundle) error, savePacksFn func([]policy.PolicyPack) error) *PolicyRoutes {
	return &PolicyRoutes{
		bundle:       bundle,
		saveBundleFn: saveBundleFn,
		savePacksFn:  savePacksFn,
	}
}

// GetBundle returns a copy of the current bundle.
func (pr *PolicyRoutes) GetBundle() types.PolicyBundle {
	return *pr.bundle
}

// --- Rules CRUD ---

// HandleListRules returns all rules in the bundle.
func (pr *PolicyRoutes) HandleListRules() (int, interface{}) {
	pr.RLock()
	defer pr.RUnlock()

	return http.StatusOK, map[string]interface{}{
		"rules": pr.bundle.Rules,
		"count": len(pr.bundle.Rules),
	}
}

// HandleAddRule adds a new rule to the bundle.
func (pr *PolicyRoutes) HandleAddRule(body []byte) (int, interface{}) {
	var rule types.PolicyRule
	if err := json.Unmarshal(body, &rule); err != nil {
		return http.StatusBadRequest, map[string]string{
			"error":   "invalid_json",
			"message": "Failed to parse policy rule: " + err.Error(),
		}
	}

	if rule.PolicyID == "" {
		return http.StatusBadRequest, map[string]string{
			"error":   "missing_policy_id",
			"message": "policy_id is required.",
		}
	}

	pr.Lock()
	defer pr.Unlock()

	// Check for duplicate policy_id.
	for _, existing := range pr.bundle.Rules {
		if existing.PolicyID == rule.PolicyID {
			return http.StatusConflict, map[string]string{
				"error":   "duplicate_policy_id",
				"message": "A rule with policy_id '" + rule.PolicyID + "' already exists.",
			}
		}
	}

	pr.bundle.Rules = append(pr.bundle.Rules, rule)
	if pr.saveBundleFn != nil {
		if err := pr.saveBundleFn(*pr.bundle); err != nil {
			return http.StatusInternalServerError, map[string]string{
				"error":   "persist_failed",
				"message": "Rule added in memory but failed to persist: " + err.Error(),
			}
		}
	}

	return http.StatusCreated, map[string]interface{}{
		"policy_id": rule.PolicyID,
		"added":     true,
		"total":     len(pr.bundle.Rules),
	}
}

// HandleUpdateRule replaces an existing rule by policy_id.
func (pr *PolicyRoutes) HandleUpdateRule(ruleID string, body []byte) (int, interface{}) {
	var rule types.PolicyRule
	if err := json.Unmarshal(body, &rule); err != nil {
		return http.StatusBadRequest, map[string]string{
			"error":   "invalid_json",
			"message": "Failed to parse policy rule: " + err.Error(),
		}
	}

	pr.Lock()
	defer pr.Unlock()

	for i, existing := range pr.bundle.Rules {
		if existing.PolicyID == ruleID {
			rule.PolicyID = ruleID // Preserve original ID.
			pr.bundle.Rules[i] = rule
			if pr.saveBundleFn != nil {
				if err := pr.saveBundleFn(*pr.bundle); err != nil {
					return http.StatusInternalServerError, map[string]string{
						"error":   "persist_failed",
						"message": "Rule updated in memory but failed to persist: " + err.Error(),
					}
				}
			}
			return http.StatusOK, map[string]interface{}{
				"policy_id": ruleID,
				"updated":   true,
			}
		}
	}

	return http.StatusNotFound, map[string]string{
		"error":   "rule_not_found",
		"message": "No rule with policy_id '" + ruleID + "' found.",
	}
}

// HandleDeleteRule removes a rule by policy_id.
func (pr *PolicyRoutes) HandleDeleteRule(ruleID string) (int, interface{}) {
	pr.Lock()
	defer pr.Unlock()

	for i, existing := range pr.bundle.Rules {
		if existing.PolicyID == ruleID {
			pr.bundle.Rules = append(pr.bundle.Rules[:i], pr.bundle.Rules[i+1:]...)
			if pr.saveBundleFn != nil {
				if err := pr.saveBundleFn(*pr.bundle); err != nil {
					return http.StatusInternalServerError, map[string]string{
						"error":   "persist_failed",
						"message": "Rule deleted in memory but failed to persist: " + err.Error(),
					}
				}
			}
			return http.StatusOK, map[string]interface{}{
				"policy_id": ruleID,
				"deleted":   true,
				"remaining": len(pr.bundle.Rules),
			}
		}
	}

	return http.StatusNotFound, map[string]string{
		"error":   "rule_not_found",
		"message": "No rule with policy_id '" + ruleID + "' found.",
	}
}

// HandleToggleRule toggles the enabled state of a rule by policy_id.
func (pr *PolicyRoutes) HandleToggleRule(ruleID string) (int, interface{}) {
	pr.Lock()
	defer pr.Unlock()

	for i, existing := range pr.bundle.Rules {
		if existing.PolicyID == ruleID {
			currentEnabled := existing.IsEnabled()
			newEnabled := !currentEnabled
			pr.bundle.Rules[i].Enabled = &newEnabled
			if pr.saveBundleFn != nil {
				if err := pr.saveBundleFn(*pr.bundle); err != nil {
					return http.StatusInternalServerError, map[string]string{
						"error":   "persist_failed",
						"message": "Rule toggled in memory but failed to persist: " + err.Error(),
					}
				}
			}
			return http.StatusOK, map[string]interface{}{
				"policy_id": ruleID,
				"enabled":   newEnabled,
			}
		}
	}

	return http.StatusNotFound, map[string]string{
		"error":   "rule_not_found",
		"message": "No rule with policy_id '" + ruleID + "' found.",
	}
}

// --- Bundle ---

// HandleGetBundle returns the full policy bundle.
func (pr *PolicyRoutes) HandleGetBundle() (int, interface{}) {
	pr.RLock()
	defer pr.RUnlock()

	return http.StatusOK, pr.bundle
}

// --- Packs ---

// HandleListPacks returns all available policy packs.
func (pr *PolicyRoutes) HandleListPacks() (int, interface{}) {
	packs := policy.GetAvailablePacks()
	return http.StatusOK, map[string]interface{}{
		"packs": packs,
		"count": len(packs),
	}
}

// HandleGetPack returns a single policy pack by ID.
func (pr *PolicyRoutes) HandleGetPack(packID string) (int, interface{}) {
	if packID == "" {
		return http.StatusBadRequest, map[string]string{
			"error":   "missing_pack_id",
			"message": "Pack ID is required.",
		}
	}

	pack := policy.GetPack(packID)
	if pack == nil {
		return http.StatusNotFound, map[string]string{
			"error":   "pack_not_found",
			"message": "No policy pack with ID '" + packID + "' found.",
		}
	}

	return http.StatusOK, pack
}

// HandleApplyPack applies a policy pack to the live bundle.
func (pr *PolicyRoutes) HandleApplyPack(packID string) (int, interface{}) {
	if packID == "" {
		return http.StatusBadRequest, map[string]string{
			"error":   "missing_pack_id",
			"message": "Pack ID is required.",
		}
	}

	pr.Lock()
	defer pr.Unlock()

	added, skipped := policy.ApplyPack(pr.bundle, packID)
	if added == nil && skipped == nil {
		return http.StatusNotFound, map[string]string{
			"error":   "pack_not_found",
			"message": "No policy pack with ID '" + packID + "' found.",
		}
	}
	if pr.saveBundleFn != nil {
		if err := pr.saveBundleFn(*pr.bundle); err != nil {
			return http.StatusInternalServerError, map[string]string{
				"error":   "persist_failed",
				"message": "Pack applied in memory but failed to persist policy bundle: " + err.Error(),
			}
		}
	}

	return http.StatusOK, map[string]interface{}{
		"pack_id":     packID,
		"added":       added,
		"skipped":     skipped,
		"total_rules": len(pr.bundle.Rules),
	}
}

// HandleCreatePack registers a custom policy pack.
func (pr *PolicyRoutes) HandleCreatePack(body []byte) (int, interface{}) {
	var pack policy.PolicyPack
	if err := json.Unmarshal(body, &pack); err != nil {
		return http.StatusBadRequest, map[string]string{
			"error":   "invalid_json",
			"message": "Failed to parse policy pack: " + err.Error(),
		}
	}

	if pack.ID == "" {
		return http.StatusBadRequest, map[string]string{
			"error":   "missing_pack_id",
			"message": "Pack id is required.",
		}
	}

	// Check for duplicates.
	if existing := policy.GetPack(pack.ID); existing != nil {
		return http.StatusConflict, map[string]string{
			"error":   "duplicate_pack_id",
			"message": "A pack with ID '" + pack.ID + "' already exists.",
		}
	}

	policy.AddCustomPack(pack)
	if pr.savePacksFn != nil {
		if err := pr.savePacksFn(policy.GetCustomPacks()); err != nil {
			return http.StatusInternalServerError, map[string]string{
				"error":   "persist_failed",
				"message": "Pack created in memory but failed to persist: " + err.Error(),
			}
		}
	}

	return http.StatusCreated, map[string]interface{}{
		"pack_id": pack.ID,
		"created": true,
	}
}
