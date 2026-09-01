/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Audit Pipeline Integration Test
 *
 * Tests the full audit event lifecycle:
 *   1. Action evaluated → audit event created with attempted_action
 *   2. Event passes minimum schema validation gate
 *   3. Event buffered in SQLite
 *   4. Event flushed to central store
 *   5. Event queryable via API
 *   6. Enrichment updates observed_effect
 *
 * PRD Reference: Appendix C, R-3, US-3
 * TDD Reference: Appendix B.7 VP-2
 */

import { describe, it, expect, beforeEach, afterEach } from "vitest";
import { v4 as uuidv4 } from "uuid";
import * as fs from "node:fs";
import * as path from "node:path";
import { AuditBuffer } from "../../src/audit/buffer.js";
import { AuditStore } from "../../src/audit/store.js";
import { FlushService } from "../../src/audit/flush.js";
import { validateAuditEvent, buildGateFields } from "../../src/audit/validate.js";
import type { AuditEvent } from "../../types/audit-event.js";

const TEST_DB_DIR = path.join(process.cwd(), "build", "test");
let buffer: AuditBuffer;
let store: AuditStore;
let flush: FlushService;
let testDbPath: string;

function makeAuditEvent(overrides: Partial<AuditEvent> = {}): AuditEvent {
  const timestamp = new Date().toISOString();
  const sessionId = `sess_test_${uuidv4().slice(0, 8)}`;

  const base = {
    event_id: uuidv4(),
    timestamp,
    session_id: sessionId,
    correlation_id: uuidv4(),
    who: `dev_001|claude_code|${sessionId}`,
    what: "file.write:Write to src/index.ts",
    when: timestamp,
    policy: "org.allow_project_files@v2026.04.27.1",
    decision: "allow:PROJECT_PATH_ALLOWED",
    result: "pending",
    actor: { user_id: "dev_001", agent_type: "claude_code", agent_instance: "vscode_ext_1" },
    environment: { workspace: "/Users/dev/project", repo: "acme/backend", branch: "main", tier: "development", deployment_mode: "host" },
    action: { type: "file.write", attempted_action: "Write to src/index.ts", observed_effect: "pending" },
    resource: { kind: "file", path: "/Users/dev/project/src/index.ts", classification: [] },
    policy_detail: { policy_id: "org.allow_project_files", policy_version: "v2026.04.27.1", decision: "allow", reason_code: "PROJECT_PATH_ALLOWED", reason_human: "File operations within the project directory are allowed." },
  };

  return { ...base, ...overrides } as AuditEvent;
}

beforeEach(() => {
  fs.mkdirSync(TEST_DB_DIR, { recursive: true });
  testDbPath = path.join(TEST_DB_DIR, `audit-${uuidv4().slice(0, 8)}.sqlite`);
  buffer = new AuditBuffer(testDbPath);
  store = new AuditStore();
  flush = new FlushService(buffer, store, { intervalMs: 100, batchSize: 50 });
});

afterEach(() => {
  flush.stop();
  buffer.close();
  try { fs.unlinkSync(testDbPath); } catch { /* ignore */ }
});

describe("Audit Pipeline Integration", () => {
  describe("event creation and validation", () => {
    it("creates valid audit events that pass the schema gate", () => {
      const event = makeAuditEvent();
      const validation = validateAuditEvent(event);
      expect(validation.valid).toBe(true);
      expect(validation.errors).toHaveLength(0);
    });

    it("rejects events missing minimum gate fields", () => {
      const badEvent = { ...makeAuditEvent(), who: "" };
      const validation = validateAuditEvent(badEvent);
      expect(validation.valid).toBe(false);
      expect(validation.errors).toContain("Field who is empty");
    });

    it("includes attempted_action and observed_effect in every event", () => {
      const event = makeAuditEvent();
      expect(event.action.attempted_action).toBeTruthy();
      expect(event.action.observed_effect).toBeTruthy();
    });

    it("includes policy_version in every event", () => {
      const event = makeAuditEvent();
      expect(event.policy_detail.policy_version).toBe("v2026.04.27.1");
      expect(event.policy).toContain("@v2026.04.27.1");
    });

    it("includes reason_code in every event", () => {
      const event = makeAuditEvent();
      expect(event.policy_detail.reason_code).toBe("PROJECT_PATH_ALLOWED");
      expect(event.decision).toContain("PROJECT_PATH_ALLOWED");
    });
  });

  describe("buffer → store flow", () => {
    it("buffers valid events and rejects invalid ones", () => {
      const validEvent = makeAuditEvent();
      const invalidEvent = { ...makeAuditEvent(), who: "" } as AuditEvent;

      expect(buffer.bufferEvent(validEvent)).toBe(true);
      expect(buffer.bufferEvent(invalidEvent)).toBe(false);

      const metrics = buffer.getMetrics();
      expect(metrics.accepted).toBe(1);
      expect(metrics.rejected).toBe(1);
    });

    it("flushes events from buffer to central store", async () => {
      const event1 = makeAuditEvent({ session_id: "sess_flush_test" });
      const event2 = makeAuditEvent({ session_id: "sess_flush_test" });

      buffer.bufferEvent(event1);
      buffer.bufferEvent(event2);

      const result = await flush.flush();
      expect(result.flushed).toBe(2);
      expect(result.errors).toBe(0);
      expect(await store.getCount()).toBe(2);
    });

    it("events are queryable after flush", async () => {
      const sessionId = `sess_query_${uuidv4().slice(0, 8)}`;
      const event = makeAuditEvent({ session_id: sessionId });
      buffer.bufferEvent(event);

      await flush.flush();

      const results = await store.queryEvents({ session_id: sessionId });
      expect(results).toHaveLength(1);
      expect(results[0].session_id).toBe(sessionId);
    });
  });

  describe("session replay", () => {
    it("returns events in chronological order for a session", async () => {
      const sessionId = `sess_replay_${uuidv4().slice(0, 8)}`;

      const event1 = makeAuditEvent({
        session_id: sessionId,
        timestamp: "2026-04-27T10:00:00.000Z",
        when: "2026-04-27T10:00:00.000Z",
      });
      const event2 = makeAuditEvent({
        session_id: sessionId,
        timestamp: "2026-04-27T10:00:01.000Z",
        when: "2026-04-27T10:00:01.000Z",
      });
      const event3 = makeAuditEvent({
        session_id: sessionId,
        timestamp: "2026-04-27T10:00:02.000Z",
        when: "2026-04-27T10:00:02.000Z",
      });

      // Buffer in reverse order
      buffer.bufferEvent(event3);
      buffer.bufferEvent(event1);
      buffer.bufferEvent(event2);

      await flush.flush();

      const session = await store.getSession(sessionId);
      expect(session).toHaveLength(3);
      // Should be in chronological order
      expect(session[0].timestamp).toBe("2026-04-27T10:00:00.000Z");
      expect(session[1].timestamp).toBe("2026-04-27T10:00:01.000Z");
      expect(session[2].timestamp).toBe("2026-04-27T10:00:02.000Z");
    });
  });

  describe("enrichment (append-only enrichment event)", () => {
    it("creates a separate enrichment event linked by correlation_id — original is never mutated", async () => {
      const sessionId = `sess_enrich_${uuidv4().slice(0, 8)}`;
      const originalEvent = makeAuditEvent({
        session_id: sessionId,
        action: { type: "shell.exec", attempted_action: "npm test", observed_effect: "pending" },
        result: "pending",
      });

      buffer.bufferEvent(originalEvent);
      await flush.flush();

      // Verify original is pending
      const beforeEvents = await store.getSession(sessionId);
      expect(beforeEvents[0].action.observed_effect).toBe("pending");

      // Create an append-only enrichment event linked by correlation_id
      const enrichmentEvent = makeAuditEvent({
        event_id: `enr_${uuidv4().slice(0, 8)}`,
        session_id: sessionId,
        correlation_id: originalEvent.event_id,
        action: { type: "enrichment", attempted_action: "npm test", observed_effect: "executed (exit 0)" },
        result: "executed (exit 0)",
        who: "system:enrichment",
        what: "enrichment:Bash",
        policy: "system:post_execution",
        decision: "enrichment:observed_effect",
      });

      await store.storeEvent(enrichmentEvent);

      // Verify original is STILL pending (immutable)
      const afterEvents = await store.getSession(sessionId);
      expect(afterEvents.length).toBe(2);
      expect(afterEvents[0].action.observed_effect).toBe("pending");

      // Enrichment event has the actual outcome
      expect(afterEvents[1].action.observed_effect).toBe("executed (exit 0)");
      expect(afterEvents[1].correlation_id).toBe(originalEvent.event_id);
    });
  });

  describe("blocked actions", () => {
    it("records blocked actions with observed_effect = blocked", () => {
      const event = makeAuditEvent({
        action: { type: "file.write", attempted_action: "Write to ~/.bashrc", observed_effect: "blocked" },
        result: "blocked",
        decision: "deny:PATH_OUTSIDE_PROJECT_ROOT",
        policy_detail: {
          policy_id: "org.block_non_project_writes",
          policy_version: "v2026.04.27.1",
          decision: "deny",
          reason_code: "PATH_OUTSIDE_PROJECT_ROOT",
          reason_human: "Write outside project directory blocked.",
        },
      });

      const validation = validateAuditEvent(event);
      expect(validation.valid).toBe(true);

      expect(buffer.bufferEvent(event)).toBe(true);
      expect(event.action.observed_effect).toBe("blocked");
      expect(event.result).toBe("blocked");
    });
  });

  describe("export", () => {
    it("exports events as JSON evidence package with metadata", async () => {
      const sessionId = `sess_export_${uuidv4().slice(0, 8)}`;
      buffer.bufferEvent(makeAuditEvent({ session_id: sessionId }));
      buffer.bufferEvent(makeAuditEvent({ session_id: sessionId }));
      await flush.flush();

      const exported = await store.exportEvents({ session_id: sessionId });
      expect(exported.metadata.total_events).toBe(2);
      expect(exported.metadata.exported_at).toBeTruthy();
      expect(exported.events).toHaveLength(2);
    });
  });

  describe("append-only guarantee", () => {
    it("store count only increases, never decreases", async () => {
      buffer.bufferEvent(makeAuditEvent());
      await flush.flush();
      const count1 = await store.getCount();

      buffer.bufferEvent(makeAuditEvent());
      await flush.flush();
      const count2 = await store.getCount();

      expect(count2).toBeGreaterThan(count1);
      // In-memory store has no delete method — enforced by design
    });
  });

  describe("gate field construction", () => {
    it("buildGateFields produces correct compound fields", () => {
      const fields = buildGateFields({
        actor: { user_id: "dev_001", agent_type: "claude_code" },
        session_id: "sess_001",
        action: { type: "shell.exec", attempted_action: "rm -rf /tmp" },
        timestamp: "2026-04-27T10:00:00Z",
        policy_detail: { policy_id: "org.approve_destructive", policy_version: "v1" },
        decision: "require_approval:DESTRUCTIVE_COMMAND",
        result: "pending_approval",
      });

      expect(fields.who).toBe("dev_001|claude_code|sess_001");
      expect(fields.what).toBe("shell.exec:rm -rf /tmp");
      expect(fields.policy).toBe("org.approve_destructive@v1");
      expect(fields.decision).toBe("require_approval:DESTRUCTIVE_COMMAND");
      expect(fields.result).toBe("pending_approval");
    });
  });
});