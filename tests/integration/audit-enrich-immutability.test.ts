/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Audit Enrichment Immutability (invariant floor)
 *
 * The enrichment endpoint (POST /v1/audit/enrich) must APPEND a new event
 * linked to the original by correlation_id and must NEVER mutate the original
 * pending event. This is the tamper-evidence guarantee the audit trail rests
 * on: "what was attempted" and "what actually happened" are separate,
 * individually signed rows.
 *
 * This test exercises handleEnrich() directly (the other audit-pipeline test
 * only checks the store-level behaviour). It is a mandatory RADAR floor —
 * the selector may never drop it.
 */

import { describe, it, expect, beforeEach } from "vitest";
import { v4 as uuidv4 } from "uuid";
import { AuditStore } from "../../src/audit/store.js";
import { handleEnrich } from "../../src/daemon/routes/enrich.js";
import type { AuditEvent } from "../../types/audit-event.js";

let store: AuditStore;

function pendingEvent(sessionId: string): AuditEvent {
  const ts = new Date().toISOString();
  return {
    event_id: uuidv4(),
    timestamp: ts,
    session_id: sessionId,
    correlation_id: uuidv4(),
    who: `dev_001|claude_code|${sessionId}`,
    what: "shell.exec:npm test",
    when: ts,
    policy: "org.allow_safe_commands@v1",
    decision: "allow:SAFE_COMMAND",
    result: "pending",
    actor: { user_id: "dev_001", agent_type: "claude_code", agent_instance: "vscode_ext_1" },
    environment: { workspace: "/w", repo: "acme/backend", branch: "main", tier: "development", deployment_mode: "host" },
    action: { type: "shell.exec", attempted_action: "npm test", observed_effect: "pending" },
    resource: { kind: "command", classification: [] },
    policy_detail: {
      policy_id: "org.allow_safe_commands",
      policy_version: "v1",
      decision: "allow",
      reason_code: "SAFE_COMMAND",
      reason_human: "npm test is on the safe-command allowlist.",
    },
  } as AuditEvent;
}

describe("Audit Enrichment Immutability (invariant floor)", () => {
  beforeEach(() => {
    // Unresolvable connection string → AuditStore uses its in-memory fallback,
    // isolated per test. (Same pattern as audit-pipeline.test.ts.)
    store = new AuditStore(`postgresql://enforcer@127.0.0.1:1/nonexistent-${uuidv4().slice(0, 8)}`);
  });

  it("appends an enrichment event and leaves the original pending event byte-identical", async () => {
    const sessionId = `sess_${uuidv4().slice(0, 8)}`;
    const original = pendingEvent(sessionId);
    await store.storeEvent(original);

    const before = await store.getSession(sessionId);
    const originalSnapshot = JSON.stringify(before[0]);

    const res = await handleEnrich(
      JSON.stringify({
        session_id: sessionId,
        tool: "Bash",
        observed_effect: "executed (exit 0)",
        timestamp: new Date().toISOString(),
      }),
      store,
    );

    expect(res.status).toBe(200);
    expect((res.body as Record<string, unknown>).enriched).toBe(true);
    expect((res.body as Record<string, unknown>).append_only).toBe(true);

    const after = await store.getSession(sessionId);

    // 1. The store grew by exactly one row — nothing was replaced.
    expect(after.length).toBe(2);

    // 2. The original pending event is unchanged, field for field.
    expect(JSON.stringify(after[0])).toBe(originalSnapshot);
    expect(after[0].event_id).toBe(original.event_id);
    expect(after[0].action.observed_effect).toBe("pending");

    // 3. The new row is the enrichment, linked back by correlation_id.
    expect(after[1].correlation_id).toBe(original.event_id);
    expect(after[1].action.observed_effect).toBe("executed (exit 0)");
  });

  it("is a no-op (no mutation, no new row) when there is no pending event", async () => {
    const sessionId = `sess_${uuidv4().slice(0, 8)}`;

    const res = await handleEnrich(
      JSON.stringify({
        session_id: sessionId,
        tool: "Bash",
        observed_effect: "executed (exit 0)",
        timestamp: new Date().toISOString(),
      }),
      store,
    );

    expect(res.status).toBe(200);
    expect((res.body as Record<string, unknown>).enriched).toBe(false);
    expect((await store.getSession(sessionId)).length).toBe(0);
  });
});
