/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Policy Engine Tests
 *
 * Validates policy evaluation, schema validation gate, and audit event
 * construction for the Phase 0 end-to-end flow.
 */

import { describe, it, expect } from "vitest";
import { v4 as uuidv4 } from "uuid";
import { evaluatePolicy } from "../src/policy/engine.js";
import { validateAuditEvent, buildGateFields } from "../src/audit/validate.js";
import type { ActionRequest } from "../types/action.js";
import type { PolicyBundle, PolicyRule } from "../types/policy.js";

// ── Test Helpers ────────────────────────────────────────────────────────────

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
      type: "file.write",
      attempted_action: "Write 100 bytes to /Users/dev/project/src/index.ts",
    },
    resource: {
      kind: "file",
      path: "/Users/dev/project/src/index.ts",
      classification: [],
    },
    ...overrides,
  };
}

function makeBundle(rules: PolicyRule[]): PolicyBundle {
  return {
    bundle_version: "v2026.04.27.1",
    scope_level: "organization",
    rules,
  };
}

function makeDenyRule(id: string, actionTypes: string[], resource: Record<string, unknown>): PolicyRule {
  return {
    policy_id: id,
    version: "v2026.04.27.1",
    scope: { level: "organization" },
    subject: { agent_types: ["*"], users: ["*"] },
    action: { types: actionTypes },
    resource,
    conditions: {},
    effect: {
      decision: "deny",
      reason_code: `${id.toUpperCase().replace(/\./g, "_")}`,
      reason_human: `Denied by rule ${id}`,
    },
    logging: { mode: "full" },
    approval: { required: false },
  };
}

function makeAllowRule(id: string, actionTypes: string[], resource: Record<string, unknown>): PolicyRule {
  return {
    ...makeDenyRule(id, actionTypes, resource),
    effect: {
      decision: "allow",
      reason_code: `${id.toUpperCase().replace(/\./g, "_")}`,
      reason_human: `Allowed by rule ${id}`,
    },
  };
}

function makeApprovalRule(id: string, actionTypes: string[], resource: Record<string, unknown>): PolicyRule {
  return {
    ...makeDenyRule(id, actionTypes, resource),
    effect: {
      decision: "require_approval",
      reason_code: `${id.toUpperCase().replace(/\./g, "_")}`,
      reason_human: `Approval required by rule ${id}`,
    },
    approval: { required: true },
  };
}

// ── Policy Engine Tests ─────────────────────────────────────────────────────

describe("Policy Engine", () => {
  describe("evaluation order", () => {
    it("deny rules take priority over allow rules", () => {
      const bundle = makeBundle([
        makeAllowRule("org.allow_all", ["file.write"], {}),
        makeDenyRule("org.deny_outside_project", ["file.write"], { path_outside_project: true }),
      ]);

      const request = makeRequest({
        resource: {
          kind: "file",
          path: "/Users/dev/.config/settings.json",
          classification: [],
        },
        action: {
          type: "file.write",
          attempted_action: "Write to ~/.config/settings.json",
        },
      });

      const decision = evaluatePolicy(request, bundle);
      expect(decision.decision).toBe("deny");
      expect(decision.reason_code).toBe("ORG_DENY_OUTSIDE_PROJECT");
    });

    it("deny rules take priority over require_approval rules", () => {
      const bundle = makeBundle([
        makeApprovalRule("org.approve_writes", ["file.write"], {}),
        makeDenyRule("org.deny_outside_project", ["file.write"], { path_outside_project: true }),
      ]);

      const request = makeRequest({
        resource: {
          kind: "file",
          path: "/etc/passwd",
          classification: [],
        },
        action: {
          type: "file.write",
          attempted_action: "Write to /etc/passwd",
        },
      });

      const decision = evaluatePolicy(request, bundle);
      expect(decision.decision).toBe("deny");
    });

    it("require_approval rules take priority over allow rules", () => {
      const bundle = makeBundle([
        makeAllowRule("org.allow_safe", ["shell.exec"], {}),
        makeApprovalRule("org.approve_destructive", ["shell.exec"], {
          command_patterns: ["rm -rf"],
        }),
      ]);

      const request = makeRequest({
        action: { type: "shell.exec", attempted_action: "rm -rf node_modules" },
        resource: { kind: "command", value: "rm -rf node_modules", classification: ["destructive"] },
      });

      const decision = evaluatePolicy(request, bundle);
      expect(decision.decision).toBe("require_approval");
    });
  });

  describe("default deny (least privilege)", () => {
    it("denies when no rules match", () => {
      const bundle = makeBundle([]);

      const request = makeRequest();
      const decision = evaluatePolicy(request, bundle);

      expect(decision.decision).toBe("deny");
      expect(decision.reason_code).toBe("DEFAULT_DENY");
    });
  });

  describe("path-based rules", () => {
    it("allows writes inside project root", () => {
      const bundle = makeBundle([
        makeDenyRule("org.deny_outside", ["file.write"], { path_outside_project: true }),
        makeAllowRule("org.allow_project", ["file.write", "file.read"], { path_inside_project: true }),
      ]);

      const request = makeRequest({
        resource: {
          kind: "file",
          path: "/Users/dev/project/src/auth.ts",
          classification: [],
        },
      });

      const decision = evaluatePolicy(request, bundle);
      expect(decision.decision).toBe("allow");
    });

    it("denies writes outside project root", () => {
      const bundle = makeBundle([
        makeDenyRule("org.deny_outside", ["file.write"], { path_outside_project: true }),
        makeAllowRule("org.allow_project", ["file.write"], { path_inside_project: true }),
      ]);

      const request = makeRequest({
        resource: {
          kind: "file",
          path: "/Users/dev/.bashrc",
          classification: [],
        },
        action: {
          type: "file.write",
          attempted_action: "Write to ~/.bashrc",
        },
      });

      const decision = evaluatePolicy(request, bundle);
      expect(decision.decision).toBe("deny");
    });
  });

  describe("command pattern rules", () => {
    it("matches destructive command patterns", () => {
      const bundle = makeBundle([
        makeApprovalRule("org.approve_destructive", ["shell.exec"], {
          command_patterns: ["rm -rf", "git push --force", "git reset --hard"],
        }),
      ]);

      const request = makeRequest({
        action: { type: "shell.exec", attempted_action: "rm -rf /tmp/junk" },
        resource: { kind: "command", value: "rm -rf /tmp/junk", classification: ["destructive"] },
      });

      const decision = evaluatePolicy(request, bundle);
      expect(decision.decision).toBe("require_approval");
    });

    it("allows safe commands when no pattern matches", () => {
      const bundle = makeBundle([
        makeApprovalRule("org.approve_destructive", ["shell.exec"], {
          command_patterns: ["rm -rf", "git push --force"],
        }),
        makeAllowRule("org.allow_safe", ["shell.exec"], {}),
      ]);

      const request = makeRequest({
        action: { type: "shell.exec", attempted_action: "npm test" },
        resource: { kind: "command", value: "npm test", classification: ["safe"] },
      });

      const decision = evaluatePolicy(request, bundle);
      expect(decision.decision).toBe("allow");
    });
  });

  describe("reason code and policy version", () => {
    it("includes reason_code in every decision", () => {
      const bundle = makeBundle([
        makeDenyRule("org.test_deny", ["file.write"], { path_outside_project: true }),
      ]);

      const request = makeRequest({
        resource: { kind: "file", path: "/etc/hosts", classification: [] },
        action: { type: "file.write", attempted_action: "Write to /etc/hosts" },
      });

      const decision = evaluatePolicy(request, bundle);
      expect(decision.reason_code).toBeTruthy();
      expect(decision.reason_code.length).toBeGreaterThan(0);
    });

    it("includes policy_version in every decision", () => {
      const bundle = makeBundle([
        makeAllowRule("org.allow_all", ["*"], {}),
      ]);

      const decision = evaluatePolicy(makeRequest(), bundle);
      expect(decision.policy_version).toBe("v2026.04.27.1");
    });
  });
});

// ── Schema Validation Gate Tests ────────────────────────────────────────────

describe("Audit Schema Validation Gate", () => {
  it("accepts event with all minimum gate fields", () => {
    const event = {
      who: "dev_001|claude_code|sess_001",
      what: "file.write:Write to src/index.ts",
      when: new Date().toISOString(),
      policy: "org.allow@v1",
      decision: "allow:ALLOWED",
      result: "executed (exit 0)",
    };

    const result = validateAuditEvent(event);
    expect(result.valid).toBe(true);
    expect(result.errors).toHaveLength(0);
  });

  it("rejects event missing 'who' field", () => {
    const event = {
      what: "file.write:Write to src/index.ts",
      when: new Date().toISOString(),
      policy: "org.allow@v1",
      decision: "allow:ALLOWED",
      result: "executed",
    };

    const result = validateAuditEvent(event);
    expect(result.valid).toBe(false);
    expect(result.errors).toContain("Missing required field: who");
  });

  it("rejects event with empty 'decision' field", () => {
    const event = {
      who: "dev_001|claude_code|sess_001",
      what: "file.write:Write",
      when: new Date().toISOString(),
      policy: "org.allow@v1",
      decision: "",
      result: "executed",
    };

    const result = validateAuditEvent(event);
    expect(result.valid).toBe(false);
    expect(result.errors).toContain("Field decision is empty");
  });

  it("rejects null event", () => {
    const result = validateAuditEvent(null);
    expect(result.valid).toBe(false);
  });

  it("reports all missing fields, not just the first", () => {
    const result = validateAuditEvent({});
    expect(result.valid).toBe(false);
    expect(result.errors.length).toBe(6); // All 6 gate fields missing
  });
});

// ── Gate Fields Builder Tests ───────────────────────────────────────────────

describe("buildGateFields", () => {
  it("constructs gate fields from structured input", () => {
    const fields = buildGateFields({
      actor: { user_id: "dev_001", agent_type: "claude_code" },
      session_id: "sess_001",
      action: { type: "file.write", attempted_action: "Write src/index.ts" },
      timestamp: "2026-04-27T10:00:00Z",
      policy_detail: { policy_id: "org.allow_project", policy_version: "v1" },
      decision: "allow:PROJECT_PATH_ALLOWED",
      result: "executed (exit 0)",
    });

    expect(fields.who).toBe("dev_001|claude_code|sess_001");
    expect(fields.what).toBe("file.write:Write src/index.ts");
    expect(fields.when).toBe("2026-04-27T10:00:00Z");
    expect(fields.policy).toBe("org.allow_project@v1");
    expect(fields.decision).toBe("allow:PROJECT_PATH_ALLOWED");
    expect(fields.result).toBe("executed (exit 0)");
  });
});