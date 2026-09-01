/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Basic Anomaly Detection
 *
 * Detects suspicious action sequences that may indicate:
 *   - Data exfiltration (read secrets → network call)
 *   - Privilege escalation (read creds → exec with elevated permissions)
 *   - Reconnaissance (rapid file reads across many directories)
 *   - Supply chain attack (modify lockfile → install package)
 *   - Evasion (disable logging → destructive action)
 *
 * Uses sequence pattern matching on the audit event stream.
 * No ML — deterministic rules based on known attack patterns.
 *
 * Venture prompt depth area: "anomaly detection over agent action sequences"
 */

import type { AuditEvent } from "../../types/audit-event.js";

// ── Anomaly Patterns ────────────────────────────────────────────────────────

export interface AnomalyPattern {
  id: string;
  name: string;
  description: string;
  severity: "critical" | "high" | "medium" | "low";
  /** Sequence of action types that trigger this pattern */
  sequence: Array<{
    action_type: string;
    /** Optional: resource must match this pattern */
    resource_pattern?: string;
    /** Optional: decision must be this */
    decision?: string;
    /** Max time window from the previous step (ms) */
    window_ms?: number;
  }>;
  /** Minimum number of matching events in the window */
  min_matches?: number;
}

const ANOMALY_PATTERNS: AnomalyPattern[] = [
  // ── Exfiltration Patterns ───────────────────────────────────────────────
  {
    id: "exfil_secret_then_network",
    name: "Secret Access → Network Exfiltration",
    description: "Agent read a credential or secret file, then made a network request. Possible data exfiltration.",
    severity: "critical",
    sequence: [
      { action_type: "file.read", resource_pattern: "\\.(env|pem|key|ssh|aws|credentials)" },
      { action_type: "network.request", window_ms: 60000 },
    ],
  },
  {
    id: "exfil_read_then_curl",
    name: "File Read → curl/wget Upload",
    description: "Agent read files then used curl/wget to POST data. Possible code exfiltration.",
    severity: "critical",
    sequence: [
      { action_type: "file.read" },
      { action_type: "shell.exec", resource_pattern: "curl.*-X POST|curl.*--data|curl.*-d |wget.*--post", window_ms: 120000 },
    ],
  },

  // ── Privilege Escalation ────────────────────────────────────────────────
  {
    id: "privesc_cred_then_exec",
    name: "Credential Read → Privileged Execution",
    description: "Agent read credentials then ran a privileged command. Possible privilege escalation.",
    severity: "high",
    sequence: [
      { action_type: "file.read", resource_pattern: "credentials|secret|token|password|api.key" },
      { action_type: "shell.exec", resource_pattern: "sudo |ssh |docker |kubectl ", window_ms: 120000 },
    ],
  },

  // ── Reconnaissance ──────────────────────────────────────────────────────
  {
    id: "recon_rapid_reads",
    name: "Rapid File Reads (Reconnaissance)",
    description: "Agent performed an unusually high number of file reads in a short period. Possible reconnaissance.",
    severity: "medium",
    sequence: [
      { action_type: "file.read" },
    ],
    min_matches: 20, // 20+ reads in the window
  },

  // ── Supply Chain ────────────────────────────────────────────────────────
  {
    id: "supply_chain_lockfile_then_install",
    name: "Lockfile Modification → Package Install",
    description: "Agent modified a lockfile then installed packages. Possible supply chain attack.",
    severity: "high",
    sequence: [
      { action_type: "file.write", resource_pattern: "package-lock|yarn\\.lock|pnpm-lock|Pipfile\\.lock|Cargo\\.lock" },
      { action_type: "shell.exec", resource_pattern: "npm install|yarn add|pip install|cargo install", window_ms: 300000 },
    ],
  },

  // ── Destructive Sequences ───────────────────────────────────────────────
  {
    id: "destructive_multi_delete",
    name: "Multiple Rapid Deletions",
    description: "Agent deleted multiple files in quick succession. Possible destructive intent.",
    severity: "high",
    sequence: [
      { action_type: "file.delete" },
    ],
    min_matches: 5,
  },
  {
    id: "destructive_force_push_after_reset",
    name: "Hard Reset → Force Push",
    description: "Agent did a hard reset then force-pushed. Possible history rewriting attack.",
    severity: "critical",
    sequence: [
      { action_type: "shell.exec", resource_pattern: "git reset --hard" },
      { action_type: "shell.exec", resource_pattern: "git push.*--force|git push.*-f", window_ms: 300000 },
    ],
  },

  // ── Evasion ─────────────────────────────────────────────────────────────
  {
    id: "evasion_denied_then_retry",
    name: "Denied Action → Immediate Retry",
    description: "Agent was denied an action and immediately retried it. Possible evasion attempt.",
    severity: "medium",
    sequence: [
      { action_type: "*", decision: "deny" },
      { action_type: "*", decision: "deny", window_ms: 10000 },
      { action_type: "*", decision: "deny", window_ms: 10000 },
    ],
  },
];

// ── Detection Engine ────────────────────────────────────────────────────────

export interface AnomalyAlert {
  pattern_id: string;
  pattern_name: string;
  description: string;
  severity: string;
  triggered_at: string;
  session_id: string;
  user_id: string;
  matching_events: Array<{
    event_id: string;
    action_type: string;
    resource: string;
    timestamp: string;
  }>;
}

// Sliding window of recent events per session
const sessionWindows = new Map<string, AuditEvent[]>();
const MAX_WINDOW_SIZE = 100;
const WINDOW_TTL_MS = 10 * 60 * 1000; // 10 minutes

const alerts: AnomalyAlert[] = [];
const alertListeners: Array<(alert: AnomalyAlert) => void> = [];

/**
 * Register a listener for anomaly alerts.
 */
export function onAnomalyAlert(listener: (alert: AnomalyAlert) => void): void {
  alertListeners.push(listener);
}

/**
 * Process an audit event through the anomaly detection engine.
 * Call this for every audit event as it's created.
 */
export function detectAnomalies(event: AuditEvent): AnomalyAlert[] {
  const sessionId = event.session_id;

  // Maintain sliding window per session
  if (!sessionWindows.has(sessionId)) {
    sessionWindows.set(sessionId, []);
  }
  const window = sessionWindows.get(sessionId)!;
  window.push(event);

  // Trim window by size and TTL
  const cutoff = Date.now() - WINDOW_TTL_MS;
  while (window.length > MAX_WINDOW_SIZE || (window.length > 0 && new Date(window[0].timestamp).getTime() < cutoff)) {
    window.shift();
  }

  // Check each pattern against the window
  const newAlerts: AnomalyAlert[] = [];

  for (const pattern of ANOMALY_PATTERNS) {
    if (matchesPattern(window, pattern)) {
      const alert: AnomalyAlert = {
        pattern_id: pattern.id,
        pattern_name: pattern.name,
        description: pattern.description,
        severity: pattern.severity,
        triggered_at: new Date().toISOString(),
        session_id: sessionId,
        user_id: event.actor.user_id,
        matching_events: window.slice(-pattern.sequence.length).map(e => ({
          event_id: e.event_id,
          action_type: e.action.type,
          resource: e.resource.value || e.resource.path || e.resource.host || "",
          timestamp: e.timestamp,
        })),
      };

      newAlerts.push(alert);
      alerts.push(alert);

      // Notify listeners
      for (const listener of alertListeners) {
        try { listener(alert); } catch { /* ignore listener errors */ }
      }

      console.warn(`[ANOMALY] ${pattern.severity.toUpperCase()}: ${pattern.name} (session: ${sessionId}, user: ${event.actor.user_id})`);
    }
  }

  return newAlerts;
}

/**
 * Check if the event window matches a pattern.
 */
function matchesPattern(window: AuditEvent[], pattern: AnomalyPattern): boolean {
  // For patterns with min_matches (count-based)
  if (pattern.min_matches && pattern.sequence.length === 1) {
    const step = pattern.sequence[0];
    const recentMs = step.window_ms || WINDOW_TTL_MS;
    const cutoff = Date.now() - recentMs;

    const matches = window.filter(e => {
      if (new Date(e.timestamp).getTime() < cutoff) return false;
      return matchesStep(e, step);
    });

    return matches.length >= pattern.min_matches;
  }

  // For sequence patterns — find the sequence in order within time windows
  if (window.length < pattern.sequence.length) return false;

  // Walk backwards from the most recent event
  let seqIdx = pattern.sequence.length - 1;
  let lastTimestamp = Date.now();

  for (let i = window.length - 1; i >= 0 && seqIdx >= 0; i--) {
    const event = window[i];
    const step = pattern.sequence[seqIdx];

    if (matchesStep(event, step)) {
      // Check time window
      if (step.window_ms && seqIdx < pattern.sequence.length - 1) {
        const timeDiff = lastTimestamp - new Date(event.timestamp).getTime();
        if (timeDiff > step.window_ms) continue; // Too old, keep looking
      }

      lastTimestamp = new Date(event.timestamp).getTime();
      seqIdx--;
    }
  }

  return seqIdx < 0; // All sequence steps matched
}

/**
 * Check if a single event matches a pattern step.
 */
function matchesStep(event: AuditEvent, step: AnomalyPattern["sequence"][0]): boolean {
  // Action type match (or wildcard)
  if (step.action_type !== "*" && event.action.type !== step.action_type) {
    return false;
  }

  // Decision match
  if (step.decision && event.policy_detail.decision !== step.decision) {
    return false;
  }

  // Resource pattern match (regex)
  if (step.resource_pattern) {
    const resource = event.resource.value || event.resource.path || event.resource.host || "";
    const attempted = event.action.attempted_action || "";
    const combined = `${resource} ${attempted}`;
    try {
      const regex = new RegExp(step.resource_pattern, "i");
      if (!regex.test(combined)) return false;
    } catch {
      return false;
    }
  }

  return true;
}

// ── API ─────────────────────────────────────────────────────────────────────

/**
 * Get all alerts (most recent first).
 */
export function getAlerts(limit = 50): AnomalyAlert[] {
  return alerts.slice(-limit).reverse();
}

/**
 * Get alerts for a specific session.
 */
export function getSessionAlerts(sessionId: string): AnomalyAlert[] {
  return alerts.filter(a => a.session_id === sessionId);
}

/**
 * Get anomaly detection metrics.
 */
export function getAnomalyMetrics(): {
  total_alerts: number;
  alerts_by_severity: Record<string, number>;
  alerts_by_pattern: Record<string, number>;
  active_sessions: number;
  patterns_loaded: number;
} {
  const bySeverity: Record<string, number> = {};
  const byPattern: Record<string, number> = {};

  for (const alert of alerts) {
    bySeverity[alert.severity] = (bySeverity[alert.severity] || 0) + 1;
    byPattern[alert.pattern_id] = (byPattern[alert.pattern_id] || 0) + 1;
  }

  return {
    total_alerts: alerts.length,
    alerts_by_severity: bySeverity,
    alerts_by_pattern: byPattern,
    active_sessions: sessionWindows.size,
    patterns_loaded: ANOMALY_PATTERNS.length,
  };
}

/**
 * Get the list of loaded anomaly patterns.
 */
export function getPatterns(): Array<{ id: string; name: string; severity: string; description: string }> {
  return ANOMALY_PATTERNS.map(p => ({
    id: p.id,
    name: p.name,
    severity: p.severity,
    description: p.description,
  }));
}