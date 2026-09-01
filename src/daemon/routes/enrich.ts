/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Audit Enrichment Endpoint (Append-Only)
 *
 * POST /v1/audit/enrich — Creates a new enrichment event linked to the
 * original pending event by correlation_id. The original event is NEVER
 * mutated — forensic immutability is preserved.
 *
 * This closes the gap between "what was attempted" and "what actually happened"
 * while maintaining strict append-only audit semantics.
 */

import { v4 as uuidv4 } from "uuid";
import type { AuditEvent } from "../../../types/audit-event.js";
import type { AuditStore } from "../../audit/store.js";

export interface EnrichRequest {
  session_id: string;
  tool: string;
  observed_effect: string;
  timestamp: string;
}

export interface EnrichResult {
  status: number;
  body: unknown;
}

/**
 * Handle POST /v1/audit/enrich
 *
 * Finds the most recent pending event for this session and creates a new
 * enrichment event linked by correlation_id. The original event is never mutated.
 */
export async function handleEnrich(
  rawBody: string,
  store: AuditStore,
): Promise<EnrichResult> {
  let body: EnrichRequest;
  try {
    body = JSON.parse(rawBody);
  } catch {
    return { status: 400, body: { error: "Invalid JSON" } };
  }

  if (!body.session_id || !body.observed_effect) {
    return { status: 400, body: { error: "session_id and observed_effect are required" } };
  }

  // Find the most recent event for this session with a pending observed_effect
  const sessionEvents = await store.getSession(body.session_id);
  let parentEventId: string | null = null;
  let parentAction = "";

  // Walk backwards to find the most recent pending event
  for (let i = sessionEvents.length - 1; i >= 0; i--) {
    const event = sessionEvents[i];
    if (
      event.action.observed_effect === "pending" ||
      event.action.observed_effect === "pending_approval"
    ) {
      parentEventId = event.event_id;
      parentAction = event.action.attempted_action;
      break;
    }
  }

  if (!parentEventId) {
    return {
      status: 200,
      body: {
        enriched: false,
        session_id: body.session_id,
        observed_effect: body.observed_effect,
        reason: "No pending event found",
      },
    };
  }

  // Create a new append-only enrichment event linked by correlation_id
  const now = new Date().toISOString();
  const enrichmentEvent: AuditEvent = {
    event_id: `enr_${uuidv4()}`,
    timestamp: now,
    session_id: body.session_id,
    correlation_id: parentEventId, // Links to the original pending event

    who: "system:enrichment",
    what: `enrichment:${body.tool}`,
    when: now,
    policy: "system:post_execution",
    decision: "enrichment:observed_effect",
    result: body.observed_effect,

    actor: { user_id: "system", agent_type: "enrichment", agent_instance: "daemon" },
    environment: { workspace: "", repo: "", branch: "", tier: "", deployment_mode: "" },
    action: {
      type: "enrichment",
      attempted_action: parentAction,
      observed_effect: body.observed_effect,
    },
    resource: { kind: "enrichment", classification: [] },
    policy_detail: {
      policy_id: "system.enrichment",
      policy_version: "",
      decision: "enrichment",
      reason_code: "POST_EXECUTION_ENRICHMENT",
      reason_human: `Post-execution enrichment for ${body.tool}: ${body.observed_effect}`,
    },
  };

  await store.storeEvent(enrichmentEvent);

  return {
    status: 200,
    body: {
      enriched: true,
      enrichment_event_id: enrichmentEvent.event_id,
      parent_event_id: parentEventId,
      session_id: body.session_id,
      observed_effect: body.observed_effect,
      append_only: true,
    },
  };
}
