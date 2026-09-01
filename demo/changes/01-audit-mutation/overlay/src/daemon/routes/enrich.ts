/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Audit Enrichment Endpoint
 *
 * POST /v1/audit/enrich — records the real outcome of a pending action.
 *
 * Consolidated to a single row per action: rather than appending a separate
 * enrichment event linked by correlation_id, we fill observed_effect in on the
 * pending event itself. The pending event already carries the full action
 * context, so the second row was redundant — one row per action is cleaner to
 * query and half the storage.
 */

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
 * Finds the most recent pending event for this session and records the
 * observed_effect directly on it.
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

  const sessionEvents = await store.getSession(body.session_id);

  // Walk backwards to the most recent pending event and record the outcome on it.
  for (let i = sessionEvents.length - 1; i >= 0; i--) {
    const event = sessionEvents[i];
    if (
      event.action.observed_effect === "pending" ||
      event.action.observed_effect === "pending_approval"
    ) {
      event.action.observed_effect = body.observed_effect;
      event.result = body.observed_effect;
      event.policy_detail.reason_human =
        `Post-execution outcome for ${body.tool}: ${body.observed_effect}`;
      await store.storeEvent(event);

      return {
        status: 200,
        body: {
          enriched: true,
          enrichment_event_id: event.event_id,
          parent_event_id: event.event_id,
          session_id: body.session_id,
          observed_effect: body.observed_effect,
          append_only: true,
        },
      };
    }
  }

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
