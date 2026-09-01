/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Approval Service
 *
 * Manages the lifecycle of approval requests: create, route, resolve, timeout.
 * Stores pending approvals in SQLite. Emits audit events for every state change.
 *
 * TDD Reference: Section 10, Appendix B.4
 * PRD Reference: Appendix C, R-4
 */

import { v4 as uuidv4 } from "uuid";
import Database from "better-sqlite3";
import * as path from "node:path";
import type { ActionRequest } from "../../types/action.js";
import type { PolicyDecision } from "../../types/policy.js";
import type {
  ApprovalRequest,
  ApprovalDecision,
  ApprovalScope,
  ContextBundle,
} from "../../types/approval.js";
import { matchesScope } from "./scope.js";

export type ApprovalStatus = "pending" | "approved" | "denied" | "expired";

export interface PendingApproval {
  approval: ApprovalRequest;
  status: ApprovalStatus;
  resolve: (decision: ApprovalDecision) => void;
  reject: (reason: string) => void;
  timeoutId: ReturnType<typeof setTimeout> | null;
}

export interface ApprovalServiceOptions {
  dbPath?: string;
  defaultTimeoutSeconds?: number;
  defaultTimeoutBehavior?: "deny" | "allow";
}

export class ApprovalService {
  private db: Database.Database;
  private pending = new Map<string, PendingApproval>();
  private activeScopes: Array<{ scope: ApprovalScope; sessionId: string; expiresAt: number }> = [];
  private defaultTimeoutSeconds: number;
  private defaultTimeoutBehavior: "deny" | "allow";

  // Metrics
  private totalCreated = 0;
  private totalApproved = 0;
  private totalDenied = 0;
  private totalExpired = 0;
  private totalScopeMatches = 0;
  private totalBreakGlass = 0;

  // Event listeners for external consumers (daemon, extension)
  private listeners: Array<(event: ApprovalEvent) => void> = [];

  constructor(options: ApprovalServiceOptions = {}) {
    const dbPath = options.dbPath || path.join(process.cwd(), "build", "approvals.sqlite");
    this.db = new Database(dbPath);
    this.db.pragma("journal_mode = WAL");
    this.defaultTimeoutSeconds = options.defaultTimeoutSeconds ?? 300;
    this.defaultTimeoutBehavior = options.defaultTimeoutBehavior ?? "deny";

    this.db.exec(`
      CREATE TABLE IF NOT EXISTS approval_log (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        approval_id TEXT NOT NULL,
        request_id TEXT NOT NULL,
        status TEXT NOT NULL,
        context_json TEXT NOT NULL,
        decision_json TEXT,
        created_at TEXT NOT NULL DEFAULT (datetime('now')),
        resolved_at TEXT
      )
    `);
  }

  /**
   * Register a listener for approval events (used by daemon WebSocket, extension).
   */
  onEvent(listener: (event: ApprovalEvent) => void): void {
    this.listeners.push(listener);
  }

  private emit(event: ApprovalEvent): void {
    for (const listener of this.listeners) {
      try { listener(event); } catch { /* listener errors don't break the service */ }
    }
  }

  /**
   * Check if a new action matches an existing approved scope.
   * If so, return the auto-approval decision without prompting a reviewer.
   */
  checkScope(request: ActionRequest): ApprovalDecision | null {
    const now = Date.now();
    // Clean expired scopes
    this.activeScopes = this.activeScopes.filter(s => s.expiresAt > now);

    for (const entry of this.activeScopes) {
      if (entry.sessionId !== request.actor.session_id) continue;
      if (matchesScope(request, entry.scope)) {
        this.totalScopeMatches++;
        this.emit({
          type: "scope_matched",
          approvalId: "scope_auto",
          requestId: request.request_id,
        });
        return {
          approval_id: `scope_auto_${uuidv4().slice(0, 8)}`,
          decision: "approve",
          approver_id: "scope_auto",
          rationale: `Auto-approved: matches active scope (${entry.scope.type})`,
          scope: entry.scope,
          is_break_glass: false,
        };
      }
    }
    return null;
  }

  /**
   * Create a new approval request and wait for resolution.
   *
   * Returns a Promise that resolves when the reviewer approves/denies
   * or the timeout expires.
   */
  createApproval(
    request: ActionRequest,
    policyDecision: PolicyDecision,
  ): { approval: ApprovalRequest; waitForDecision: Promise<ApprovalDecision> } {
    const approvalId = `apr_${uuidv4().slice(0, 12)}`;
    const contextBundle: ContextBundle = {
      actor: `${request.actor.user_id} via ${request.actor.agent_type}`,
      resource: request.resource.value || request.resource.path || "unknown",
      risk_rationale: policyDecision.reason_human,
      policy_rule: policyDecision.policy_id,
      agent_identity: `${request.actor.agent_type} (${request.actor.agent_instance})`,
      session_summary: `Session ${request.actor.session_id}`,
    };

    const approval: ApprovalRequest = {
      approval_id: approvalId,
      request_id: request.request_id,
      context_bundle: contextBundle,
      timeout_seconds: this.defaultTimeoutSeconds,
      timeout_behavior: this.defaultTimeoutBehavior,
      created_at: new Date().toISOString(),
    };

    // Persist to SQLite
    this.db.prepare(
      "INSERT INTO approval_log (approval_id, request_id, status, context_json) VALUES (?, ?, ?, ?)",
    ).run(approvalId, request.request_id, "pending", JSON.stringify(contextBundle));

    this.totalCreated++;

    // Create promise that resolves on reviewer decision or timeout
    const waitForDecision = new Promise<ApprovalDecision>((resolve, reject) => {
      const timeoutId = setTimeout(() => {
        this.handleTimeout(approvalId);
      }, approval.timeout_seconds * 1000);

      this.pending.set(approvalId, {
        approval,
        status: "pending",
        resolve,
        reject,
        timeoutId,
      });
    });

    this.emit({
      type: "approval_created",
      approvalId,
      requestId: request.request_id,
      contextBundle,
    });

    return { approval, waitForDecision };
  }

  /**
   * Resolve an approval (approve or deny).
   */
  resolveApproval(approvalId: string, decision: ApprovalDecision): void {
    const entry = this.pending.get(approvalId);
    if (!entry) {
      throw new Error(`No pending approval with id: ${approvalId}`);
    }

    // Clear timeout
    if (entry.timeoutId) {
      clearTimeout(entry.timeoutId);
    }

    // Update status
    entry.status = decision.decision === "approve" ? "approved" : "denied";

    if (decision.decision === "approve") {
      this.totalApproved++;
      // Register scope if provided
      if (decision.scope) {
        const defaultExpiry = Date.now() + 30 * 60 * 1000; // 30 minutes default
        this.activeScopes.push({
          scope: decision.scope,
          sessionId: entry.approval.request_id, // Will be updated with session context
          expiresAt: decision.scope.expiry
            ? new Date(decision.scope.expiry).getTime()
            : defaultExpiry,
        });
      }
    } else {
      this.totalDenied++;
    }

    if (decision.is_break_glass) {
      this.totalBreakGlass++;
    }

    // Persist resolution
    this.db.prepare(
      "UPDATE approval_log SET status = ?, decision_json = ?, resolved_at = datetime('now') WHERE approval_id = ?",
    ).run(entry.status, JSON.stringify(decision), approvalId);

    // Resolve the promise
    entry.resolve(decision);
    this.pending.delete(approvalId);

    this.emit({
      type: "approval_resolved",
      approvalId,
      requestId: entry.approval.request_id,
      decision: decision.decision,
      approver: decision.approver_id,
    });
  }

  /**
   * Handle timeout expiry.
   */
  private handleTimeout(approvalId: string): void {
    const entry = this.pending.get(approvalId);
    if (!entry || entry.status !== "pending") return;

    entry.status = "expired";
    this.totalExpired++;

    const timeoutDecision: ApprovalDecision = {
      approval_id: approvalId,
      decision: entry.approval.timeout_behavior === "allow" ? "approve" : "deny",
      approver_id: "system_timeout",
      rationale: `Approval timed out after ${entry.approval.timeout_seconds}s. Timeout behavior: ${entry.approval.timeout_behavior}.`,
      is_break_glass: false,
    };

    // Persist
    this.db.prepare(
      "UPDATE approval_log SET status = 'expired', decision_json = ?, resolved_at = datetime('now') WHERE approval_id = ?",
    ).run(JSON.stringify(timeoutDecision), approvalId);

    entry.resolve(timeoutDecision);
    this.pending.delete(approvalId);

    this.emit({
      type: "approval_timeout",
      approvalId,
      requestId: entry.approval.request_id,
      behavior: entry.approval.timeout_behavior,
    });
  }

  /**
   * Get approval by ID.
   */
  getApproval(approvalId: string): { approval: ApprovalRequest; status: ApprovalStatus } | null {
    const entry = this.pending.get(approvalId);
    if (entry) {
      return { approval: entry.approval, status: entry.status };
    }
    // Check persisted log
    const row = this.db.prepare(
      "SELECT * FROM approval_log WHERE approval_id = ?",
    ).get(approvalId) as { status: string; context_json: string; approval_id: string; request_id: string; created_at: string } | undefined;

    if (!row) return null;

    return {
      approval: {
        approval_id: row.approval_id,
        request_id: row.request_id,
        context_bundle: JSON.parse(row.context_json),
        timeout_seconds: this.defaultTimeoutSeconds,
        timeout_behavior: this.defaultTimeoutBehavior,
        created_at: row.created_at,
      },
      status: row.status as ApprovalStatus,
    };
  }

  /**
   * List all pending approvals.
   */
  getPending(): ApprovalRequest[] {
    return Array.from(this.pending.values()).map(e => e.approval);
  }

  /**
   * Get metrics.
   */
  getMetrics() {
    return {
      totalCreated: this.totalCreated,
      totalApproved: this.totalApproved,
      totalDenied: this.totalDenied,
      totalExpired: this.totalExpired,
      totalScopeMatches: this.totalScopeMatches,
      totalBreakGlass: this.totalBreakGlass,
      pendingCount: this.pending.size,
      activeScopeCount: this.activeScopes.length,
    };
  }

  /**
   * Close the database.
   */
  close(): void {
    // Clear all pending timeouts
    for (const entry of this.pending.values()) {
      if (entry.timeoutId) clearTimeout(entry.timeoutId);
    }
    this.pending.clear();
    this.db.close();
  }
}

// ── Event Types ─────────────────────────────────────────────────────────────

export type ApprovalEvent =
  | { type: "approval_created"; approvalId: string; requestId: string; contextBundle: ContextBundle }
  | { type: "approval_resolved"; approvalId: string; requestId: string; decision: string; approver: string }
  | { type: "approval_timeout"; approvalId: string; requestId: string; behavior: string }
  | { type: "scope_matched"; approvalId: string; requestId: string };