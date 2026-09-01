/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Approval Service Tests
 *
 * Tests approval lifecycle: create, resolve (approve/deny), timeout,
 * reusable scopes, break-glass, and metrics.
 */

import { describe, it, expect, beforeEach, afterEach } from "vitest";
import { v4 as uuidv4 } from "uuid";
import * as fs from "node:fs";
import * as path from "node:path";
import { ApprovalService } from "../src/approval/service.js";
import { matchesScope } from "../src/approval/scope.js";
import { requestBreakGlass } from "../src/approval/break-glass.js";
import type { ActionRequest } from "../types/action.js";
import type { ApprovalScope } from "../types/approval.js";

// ── Helpers ─────────────────────────────────────────────────────────────────

function makeRequest(overrides: Partial<ActionRequest> = {}): ActionRequest {
  return {
    request_id: uuidv4(),
    timestamp: new Date().toISOString(),
    actor: {
      user_id: "dev_001",
      agent_type: "claude_code",
      agent_instance: "vscode_ext_1",
      session_id: "sess_test_001",
    },
    environment: {
      workspace: "/Users/dev/project",
      repo: "acme/backend",
      branch: "main",
      tier: "development",
      deployment_mode: "host",
    },
    action: {
      type: "shell.exec",
      attempted_action: "rm -rf node_modules",
    },
    resource: {
      kind: "command",
      value: "rm -rf node_modules",
      classification: ["destructive"],
    },
    ...overrides,
  };
}

const TEST_DB_DIR = path.join(process.cwd(), "build", "test");

let service: ApprovalService;
let testDbPath: string;

beforeEach(() => {
  fs.mkdirSync(TEST_DB_DIR, { recursive: true });
  testDbPath = path.join(TEST_DB_DIR, `approvals-${uuidv4().slice(0, 8)}.sqlite`);
  service = new ApprovalService({
    dbPath: testDbPath,
    defaultTimeoutSeconds: 2, // Short timeout for tests
    defaultTimeoutBehavior: "deny",
  });
});

afterEach(() => {
  service.close();
  try { fs.unlinkSync(testDbPath); } catch { /* ignore */ }
});

// ── Approval Lifecycle ──────────────────────────────────────────────────────

describe("Approval Service", () => {
  describe("create and resolve", () => {
    it("creates an approval request with context bundle", () => {
      const request = makeRequest();
      const policyDecision = {
        request_id: request.request_id,
        decision: "require_approval" as const,
        reason_code: "DESTRUCTIVE_COMMAND",
        reason_human: "Destructive command pattern",
        policy_id: "org.approve_destructive",
        policy_version: "v1",
        approval_required: true,
      };

      const { approval } = service.createApproval(request, policyDecision);

      expect(approval.approval_id).toBeTruthy();
      expect(approval.request_id).toBe(request.request_id);
      expect(approval.context_bundle.actor).toContain("dev_001");
      expect(approval.context_bundle.risk_rationale).toBe("Destructive command pattern");
      expect(approval.context_bundle.policy_rule).toBe("org.approve_destructive");
    });

    it("resolves with approve decision", async () => {
      const request = makeRequest();
      const { approval, waitForDecision } = service.createApproval(request, {
        request_id: request.request_id,
        decision: "require_approval",
        reason_code: "TEST",
        reason_human: "Test",
        policy_id: "test",
        policy_version: "v1",
        approval_required: true,
      });

      // Resolve immediately
      service.resolveApproval(approval.approval_id, {
        approval_id: approval.approval_id,
        decision: "approve",
        approver_id: "reviewer_001",
        rationale: "Looks safe",
        is_break_glass: false,
      });

      const decision = await waitForDecision;
      expect(decision.decision).toBe("approve");
      expect(decision.approver_id).toBe("reviewer_001");
      expect(decision.rationale).toBe("Looks safe");
    });

    it("resolves with deny decision", async () => {
      const request = makeRequest();
      const { approval, waitForDecision } = service.createApproval(request, {
        request_id: request.request_id,
        decision: "require_approval",
        reason_code: "TEST",
        reason_human: "Test",
        policy_id: "test",
        policy_version: "v1",
        approval_required: true,
      });

      service.resolveApproval(approval.approval_id, {
        approval_id: approval.approval_id,
        decision: "deny",
        approver_id: "reviewer_001",
        rationale: "Use npm ci instead",
        is_break_glass: false,
      });

      const decision = await waitForDecision;
      expect(decision.decision).toBe("deny");
      expect(decision.rationale).toBe("Use npm ci instead");
    });

    it("throws when resolving unknown approval", () => {
      expect(() => {
        service.resolveApproval("nonexistent", {
          approval_id: "nonexistent",
          decision: "approve",
          approver_id: "test",
          is_break_glass: false,
        });
      }).toThrow("No pending approval");
    });
  });

  describe("timeout", () => {
    it("auto-denies on timeout when behavior is deny", async () => {
      const request = makeRequest();
      const { waitForDecision } = service.createApproval(request, {
        request_id: request.request_id,
        decision: "require_approval",
        reason_code: "TEST",
        reason_human: "Test",
        policy_id: "test",
        policy_version: "v1",
        approval_required: true,
      });

      // Wait for timeout (2 seconds)
      const decision = await waitForDecision;
      expect(decision.decision).toBe("deny");
      expect(decision.approver_id).toBe("system_timeout");
      expect(decision.rationale).toContain("timed out");
    }, 5000);

    it("auto-allows on timeout when behavior is allow", async () => {
      service.close();
      const allowService = new ApprovalService({
        dbPath: testDbPath + ".allow",
        defaultTimeoutSeconds: 1,
        defaultTimeoutBehavior: "allow",
      });

      const request = makeRequest();
      const { waitForDecision } = allowService.createApproval(request, {
        request_id: request.request_id,
        decision: "require_approval",
        reason_code: "TEST",
        reason_human: "Test",
        policy_id: "test",
        policy_version: "v1",
        approval_required: true,
      });

      const decision = await waitForDecision;
      expect(decision.decision).toBe("approve");
      expect(decision.approver_id).toBe("system_timeout");

      allowService.close();
      try { fs.unlinkSync(testDbPath + ".allow"); } catch { /* ignore */ }
    }, 5000);
  });

  describe("getPending and getApproval", () => {
    it("lists pending approvals", () => {
      const r1 = makeRequest();
      const r2 = makeRequest();
      const policy = {
        request_id: "",
        decision: "require_approval" as const,
        reason_code: "TEST",
        reason_human: "Test",
        policy_id: "test",
        policy_version: "v1",
        approval_required: true,
      };

      service.createApproval(r1, { ...policy, request_id: r1.request_id });
      service.createApproval(r2, { ...policy, request_id: r2.request_id });

      const pending = service.getPending();
      expect(pending).toHaveLength(2);
    });

    it("retrieves approval by ID", () => {
      const request = makeRequest();
      const { approval } = service.createApproval(request, {
        request_id: request.request_id,
        decision: "require_approval",
        reason_code: "TEST",
        reason_human: "Test",
        policy_id: "test",
        policy_version: "v1",
        approval_required: true,
      });

      const found = service.getApproval(approval.approval_id);
      expect(found).not.toBeNull();
      expect(found!.status).toBe("pending");
    });

    it("returns null for unknown ID", () => {
      expect(service.getApproval("nonexistent")).toBeNull();
    });
  });

  describe("metrics", () => {
    it("tracks approval metrics", async () => {
      const r1 = makeRequest();
      const { approval: a1 } = service.createApproval(r1, {
        request_id: r1.request_id,
        decision: "require_approval",
        reason_code: "TEST",
        reason_human: "Test",
        policy_id: "test",
        policy_version: "v1",
        approval_required: true,
      });

      service.resolveApproval(a1.approval_id, {
        approval_id: a1.approval_id,
        decision: "approve",
        approver_id: "rev1",
        is_break_glass: false,
      });

      const metrics = service.getMetrics();
      expect(metrics.totalCreated).toBe(1);
      expect(metrics.totalApproved).toBe(1);
      expect(metrics.pendingCount).toBe(0);
    });
  });

  describe("events", () => {
    it("emits events for approval lifecycle", () => {
      const events: unknown[] = [];
      service.onEvent(e => events.push(e));

      const request = makeRequest();
      const { approval } = service.createApproval(request, {
        request_id: request.request_id,
        decision: "require_approval",
        reason_code: "TEST",
        reason_human: "Test",
        policy_id: "test",
        policy_version: "v1",
        approval_required: true,
      });

      service.resolveApproval(approval.approval_id, {
        approval_id: approval.approval_id,
        decision: "approve",
        approver_id: "rev1",
        is_break_glass: false,
      });

      expect(events).toHaveLength(2);
      expect((events[0] as { type: string }).type).toBe("approval_created");
      expect((events[1] as { type: string }).type).toBe("approval_resolved");
    });
  });
});

// ── Scope Matching ──────────────────────────────────────────────────────────

describe("Approval Scopes", () => {
  it("single scope never matches a second time", () => {
    const scope: ApprovalScope = { type: "single" };
    const request = makeRequest();
    expect(matchesScope(request, scope)).toBe(false);
  });

  it("session scope matches any action in session", () => {
    const scope: ApprovalScope = { type: "session" };
    const request = makeRequest();
    expect(matchesScope(request, scope)).toBe(true);
  });

  it("session scope with pattern matches matching actions", () => {
    const scope: ApprovalScope = {
      type: "session",
      pattern: "npm install *",
    };
    const request = makeRequest({
      resource: { kind: "command", value: "npm install express", classification: [] },
    });
    expect(matchesScope(request, scope)).toBe(true);
  });

  it("session scope with pattern rejects non-matching actions", () => {
    const scope: ApprovalScope = {
      type: "session",
      pattern: "npm install *",
    };
    const request = makeRequest({
      resource: { kind: "command", value: "rm -rf /tmp", classification: [] },
    });
    expect(matchesScope(request, scope)).toBe(false);
  });

  it("time-bounded scope rejects expired scope", () => {
    const scope: ApprovalScope = {
      type: "time_bounded",
      expiry: new Date(Date.now() - 1000).toISOString(), // Already expired
    };
    const request = makeRequest();
    expect(matchesScope(request, scope)).toBe(false);
  });

  it("time-bounded scope accepts non-expired scope", () => {
    const scope: ApprovalScope = {
      type: "time_bounded",
      expiry: new Date(Date.now() + 60000).toISOString(), // 1 minute from now
    };
    const request = makeRequest();
    expect(matchesScope(request, scope)).toBe(true);
  });
});

// ── Break-Glass ─────────────────────────────────────────────────────────────

describe("Break-Glass", () => {
  it("creates break-glass decision with rationale", () => {
    const decision = requestBreakGlass("apr_123", "admin_001", "Production incident requires immediate fix");
    expect(decision.decision).toBe("approve");
    expect(decision.is_break_glass).toBe(true);
    expect(decision.rationale).toContain("[BREAK-GLASS]");
    expect(decision.approver_id).toBe("admin_001");
  });

  it("rejects break-glass without rationale", () => {
    expect(() => requestBreakGlass("apr_123", "admin_001", "")).toThrow("non-empty rationale");
  });

  it("rejects break-glass with whitespace-only rationale", () => {
    expect(() => requestBreakGlass("apr_123", "admin_001", "   ")).toThrow("non-empty rationale");
  });
});