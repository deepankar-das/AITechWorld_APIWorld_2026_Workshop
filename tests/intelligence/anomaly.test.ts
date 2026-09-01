/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Anomaly Detection Tests
 *
 * Tests suspicious sequence detection patterns.
 * Venture prompt depth area: "anomaly detection over agent action sequences"
 */

import { describe, it, expect } from "vitest";
import { v4 as uuidv4 } from "uuid";
import { detectAnomalies, getAnomalyMetrics, getPatterns } from "../../src/intelligence/anomaly.js";
import type { AuditEvent } from "../../types/audit-event.js";

function makeEvent(overrides: Partial<AuditEvent> & { action_type: string; resource_value?: string; decision?: string }): AuditEvent {
  const timestamp = new Date().toISOString();
  const sessionId = overrides.session_id || "sess_anomaly_test";

  return {
    event_id: uuidv4(),
    timestamp,
    session_id: sessionId,
    correlation_id: uuidv4(),
    who: `test|claude_code|${sessionId}`,
    what: `${overrides.action_type}:${overrides.resource_value || "test"}`,
    when: timestamp,
    policy: "test@v1",
    decision: `${overrides.decision || "allow"}:TEST`,
    result: overrides.decision === "deny" ? "blocked" : "executed",
    actor: { user_id: "test_user", agent_type: "claude_code", agent_instance: "test" },
    environment: { workspace: "/tmp/test", repo: "test", branch: "main", tier: "development", deployment_mode: "host" },
    action: { type: overrides.action_type, attempted_action: overrides.resource_value || "test action", observed_effect: "executed" },
    resource: { kind: "file", value: overrides.resource_value, path: overrides.resource_value, classification: [] },
    policy_detail: { policy_id: "test", policy_version: "v1", decision: overrides.decision || "allow", reason_code: "TEST", reason_human: "test" },
    ...overrides,
  } as AuditEvent;
}

describe("Anomaly Detection", () => {
  describe("patterns loaded", () => {
    it("has anomaly patterns loaded", () => {
      const patterns = getPatterns();
      expect(patterns.length).toBeGreaterThan(5);
    });

    it("has critical, high, and medium severity patterns", () => {
      const patterns = getPatterns();
      expect(patterns.some(p => p.severity === "critical")).toBe(true);
      expect(patterns.some(p => p.severity === "high")).toBe(true);
      expect(patterns.some(p => p.severity === "medium")).toBe(true);
    });
  });

  describe("exfiltration detection", () => {
    it("detects secret read followed by network request", () => {
      const sessionId = `sess_exfil_${uuidv4().slice(0, 8)}`;

      // Step 1: Read a secret file
      detectAnomalies(makeEvent({
        session_id: sessionId,
        action_type: "file.read",
        resource_value: "/Users/dev/.ssh/id_rsa",
      }));

      // Step 2: Network request (within 60s)
      const alerts = detectAnomalies(makeEvent({
        session_id: sessionId,
        action_type: "network.request",
        resource_value: "https://evil.com/upload",
      }));

      expect(alerts.some(a => a.pattern_id === "exfil_secret_then_network")).toBe(true);
      expect(alerts[0]?.severity).toBe("critical");
    });
  });

  describe("supply chain detection", () => {
    it("detects lockfile modification followed by package install", () => {
      const sessionId = `sess_supply_${uuidv4().slice(0, 8)}`;

      detectAnomalies(makeEvent({
        session_id: sessionId,
        action_type: "file.write",
        resource_value: "package-lock.json",
      }));

      const alerts = detectAnomalies(makeEvent({
        session_id: sessionId,
        action_type: "shell.exec",
        resource_value: "npm install malicious-pkg",
      }));

      expect(alerts.some(a => a.pattern_id === "supply_chain_lockfile_then_install")).toBe(true);
    });
  });

  describe("evasion detection", () => {
    it("detects repeated denied actions (retry evasion)", () => {
      const sessionId = `sess_evasion_${uuidv4().slice(0, 8)}`;

      detectAnomalies(makeEvent({
        session_id: sessionId,
        action_type: "file.write",
        resource_value: "/etc/passwd",
        decision: "deny",
      }));

      detectAnomalies(makeEvent({
        session_id: sessionId,
        action_type: "file.write",
        resource_value: "/etc/passwd",
        decision: "deny",
      }));

      const alerts = detectAnomalies(makeEvent({
        session_id: sessionId,
        action_type: "file.write",
        resource_value: "/etc/shadow",
        decision: "deny",
      }));

      expect(alerts.some(a => a.pattern_id === "evasion_denied_then_retry")).toBe(true);
    });
  });

  describe("destructive sequence detection", () => {
    it("detects hard reset followed by force push", () => {
      const sessionId = `sess_destruct_${uuidv4().slice(0, 8)}`;

      detectAnomalies(makeEvent({
        session_id: sessionId,
        action_type: "shell.exec",
        resource_value: "git reset --hard HEAD~5",
      }));

      const alerts = detectAnomalies(makeEvent({
        session_id: sessionId,
        action_type: "shell.exec",
        resource_value: "git push --force origin main",
      }));

      expect(alerts.some(a => a.pattern_id === "destructive_force_push_after_reset")).toBe(true);
      expect(alerts[0]?.severity).toBe("critical");
    });
  });

  describe("no false positives", () => {
    it("does not alert on normal development work", () => {
      const sessionId = `sess_normal_${uuidv4().slice(0, 8)}`;

      const a1 = detectAnomalies(makeEvent({ session_id: sessionId, action_type: "file.read", resource_value: "src/index.ts" }));
      const a2 = detectAnomalies(makeEvent({ session_id: sessionId, action_type: "file.write", resource_value: "src/index.ts" }));
      const a3 = detectAnomalies(makeEvent({ session_id: sessionId, action_type: "shell.exec", resource_value: "npm test" }));

      expect(a1.length).toBe(0);
      expect(a2.length).toBe(0);
      expect(a3.length).toBe(0);
    });
  });

  describe("metrics", () => {
    it("tracks alert counts", () => {
      const metrics = getAnomalyMetrics();
      expect(metrics.total_alerts).toBeGreaterThan(0);
      expect(metrics.patterns_loaded).toBeGreaterThan(5);
    });
  });
});