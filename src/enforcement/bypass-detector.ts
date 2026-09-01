/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Bypass Detector
 *
 * Monitors for ungoverned actions that reach the OS without passing
 * through an enforcement point. Emits ungoverned_execution_detected
 * audit events with alert flags.
 *
 * Detection methods:
 *   - File change watcher: detects writes not preceded by fs-guard event
 *   - Process monitor: detects shell spawns not preceded by shell-proxy event
 *
 * Does NOT block actions (they already happened). Surfaces visibility gaps.
 *
 * TDD Reference: Section 6.3
 * PRD Reference: Appendix C, R-1 (interception quality tracking)
 */

import * as fs from "node:fs";
import { v4 as uuidv4 } from "uuid";
import type { AuditEvent } from "../../types/audit-event.js";
import { buildGateFields } from "../audit/validate.js";
import type { AuditBuffer } from "../audit/buffer.js";

export interface BypassDetectorOptions {
  /** Workspace root to monitor */
  workspace: string;
  /** Audit buffer for emitting bypass events */
  auditBuffer: AuditBuffer;
  /** Session ID for audit events */
  sessionId: string;
  /** Window in ms — if a file change occurs without a preceding fs-guard event within this window, it's a bypass */
  detectionWindowMs?: number;
}

/**
 * Tracks recent enforcement events for bypass detection.
 * When a file change is observed via the OS watcher but no enforcement
 * event was logged within the detection window, it's flagged as a bypass.
 */
export class BypassDetector {
  private recentEnforcementPaths = new Set<string>();
  private watcher: fs.FSWatcher | null = null;
  private bypassCount = 0;
  private options: Required<BypassDetectorOptions>;

  constructor(options: BypassDetectorOptions) {
    this.options = {
      detectionWindowMs: 500,
      ...options,
    };
  }

  /**
   * Record that an enforcement point handled a file operation.
   * Called by the fs-guard after evaluating policy.
   */
  recordEnforcementEvent(filePath: string): void {
    this.recentEnforcementPaths.add(filePath);
    // Clear after detection window
    setTimeout(() => {
      this.recentEnforcementPaths.delete(filePath);
    }, this.options.detectionWindowMs);
  }

  /**
   * Start monitoring the workspace for ungoverned file changes.
   */
  start(): void {
    if (this.watcher) return;

    try {
      this.watcher = fs.watch(
        this.options.workspace,
        { recursive: true },
        (eventType, filename) => {
          if (!filename) return;
          if (eventType !== "change" && eventType !== "rename") return;

          // Skip node_modules, .git, build artifacts
          if (
            filename.startsWith("node_modules") ||
            filename.startsWith(".git") ||
            filename.startsWith("dist") ||
            filename.startsWith("build") ||
            filename.startsWith(".test-results")
          ) {
            return;
          }

          const fullPath = `${this.options.workspace}/${filename}`;

          // Check if this file change was preceded by an enforcement event
          if (!this.recentEnforcementPaths.has(fullPath)) {
            this.emitBypassEvent(fullPath, eventType);
          }
        },
      );
    } catch (err) {
      console.warn(`[BYPASS] Could not start file watcher: ${err}`);
    }
  }

  /**
   * Stop monitoring.
   */
  stop(): void {
    if (this.watcher) {
      this.watcher.close();
      this.watcher = null;
    }
  }

  /**
   * Emit an ungoverned_execution_detected audit event.
   */
  private emitBypassEvent(filePath: string, eventType: string): void {
    this.bypassCount++;

    const timestamp = new Date().toISOString();
    const decisionString = "alert:BYPASS_DETECTED";
    const resultString = `ungoverned_${eventType}`;

    const gateFields = buildGateFields({
      actor: { user_id: "unknown", agent_type: "unknown" },
      session_id: this.options.sessionId,
      action: { type: `file.${eventType}`, attempted_action: `Ungoverned ${eventType}: ${filePath}` },
      timestamp,
      policy_detail: { policy_id: "system.bypass_detector", policy_version: "v1" },
      decision: decisionString,
      result: resultString,
    });

    const auditEvent: AuditEvent = {
      event_id: uuidv4(),
      timestamp,
      session_id: this.options.sessionId,
      correlation_id: uuidv4(),
      ...gateFields,
      actor: {
        user_id: "unknown",
        agent_type: "unknown",
        agent_instance: "bypass_detector",
      },
      environment: {
        workspace: this.options.workspace,
        repo: "",
        branch: "",
        tier: "development",
        deployment_mode: "host",
      },
      action: {
        type: `file.${eventType}`,
        attempted_action: `Ungoverned ${eventType}: ${filePath}`,
        observed_effect: resultString,
      },
      resource: {
        kind: "file",
        path: filePath,
        classification: ["bypass_attempt"],
      },
      policy_detail: {
        policy_id: "system.bypass_detector",
        policy_version: "v1",
        decision: "alert",
        reason_code: "BYPASS_DETECTED",
        reason_human: `File ${eventType} detected without preceding enforcement event. Possible bypass.`,
      },
    };

    this.options.auditBuffer.bufferEvent(auditEvent);
    console.warn(
      `[BYPASS] Ungoverned file ${eventType} detected: ${filePath} (total bypasses: ${this.bypassCount})`,
    );
  }

  /**
   * Get bypass detection metrics.
   */
  getMetrics(): { bypassCount: number; monitoring: boolean } {
    return {
      bypassCount: this.bypassCount,
      monitoring: this.watcher !== null,
    };
  }
}