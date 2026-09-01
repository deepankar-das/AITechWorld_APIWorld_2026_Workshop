/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Global Enforcement State
 *
 * Controls whether the firewall is actively enforcing policies.
 * When disabled, all actions are allowed (but still logged).
 *
 * This is the state that the dashboard On/Off switch controls.
 */

let enforcementEnabled = true;
let stateChangedAt: string = new Date().toISOString();
let stateChangedBy: string = "system_startup";

export function isEnforcementEnabled(): boolean {
  return enforcementEnabled;
}

export function setEnforcementEnabled(enabled: boolean, changedBy: string = "console"): void {
  enforcementEnabled = enabled;
  stateChangedAt = new Date().toISOString();
  stateChangedBy = changedBy;
}

export function getEnforcementState(): {
  enabled: boolean;
  since: string;
  changed_by: string;
} {
  return {
    enabled: enforcementEnabled,
    since: stateChangedAt,
    changed_by: stateChangedBy,
  };
}