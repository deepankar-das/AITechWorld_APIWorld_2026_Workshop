/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Audit Query API Routes
 *
 * REST endpoints for querying and exporting audit events:
 *   GET /v1/audit/events         — query with filters
 *   GET /v1/audit/sessions       — list sessions with summaries
 *   GET /v1/audit/sessions/:id   — session replay (chronological)
 *   GET /v1/audit/export         — export evidence package (JSON)
 *
 * TDD Reference: Section 16 (Internal API Summary)
 * PRD Reference: Appendix C, R-3, US-3
 */

import { type AuditStore, type AuditQuery } from "../../audit/store.js";
import { type FlushService } from "../../audit/flush.js";

export interface AuditRouteResult {
  status: number;
  body: unknown;
}

/**
 * Parse query string parameters from URL.
 */
function parseQueryParams(url: string): Record<string, string> {
  const params: Record<string, string> = {};
  const queryStart = url.indexOf("?");
  if (queryStart === -1) return params;

  const queryString = url.slice(queryStart + 1);
  for (const pair of queryString.split("&")) {
    const [key, value] = pair.split("=");
    if (key && value) {
      params[decodeURIComponent(key)] = decodeURIComponent(value);
    }
  }
  return params;
}

/**
 * GET /v1/audit/events?session_id=X&action_type=Y&decision=Z&time_from=T&time_to=T&limit=N&offset=N
 */
export async function handleQueryEvents(
  url: string,
  store: AuditStore,
): Promise<AuditRouteResult> {
  const params = parseQueryParams(url);
  const query: AuditQuery = {
    session_id: params.session_id,
    actor_user_id: params.actor_user_id,
    action_type: params.action_type,
    decision: params.decision,
    policy_id: params.policy_id,
    reason_code: params.reason_code,
    time_from: params.time_from,
    time_to: params.time_to,
    limit: params.limit ? parseInt(params.limit, 10) : 100,
    offset: params.offset ? parseInt(params.offset, 10) : 0,
  };

  const events = await store.queryEvents(query);
  return {
    status: 200,
    body: {
      events,
      count: events.length,
      query,
    },
  };
}

/**
 * GET /v1/audit/sessions
 */
export async function handleGetSessions(store: AuditStore): Promise<AuditRouteResult> {
  const sessions = await store.getSessions();
  return {
    status: 200,
    body: { sessions, count: sessions.length },
  };
}

/**
 * GET /v1/audit/sessions/:id
 */
export async function handleGetSession(
  sessionId: string,
  store: AuditStore,
): Promise<AuditRouteResult> {
  const events = await store.getSession(sessionId);
  if (events.length === 0) {
    return { status: 404, body: { error: `Session ${sessionId} not found` } };
  }
  return {
    status: 200,
    body: {
      session_id: sessionId,
      event_count: events.length,
      events,
    },
  };
}

/**
 * GET /v1/audit/export?session_id=X&...
 */
export async function handleExportEvents(
  url: string,
  store: AuditStore,
): Promise<AuditRouteResult> {
  const params = parseQueryParams(url);
  const query: AuditQuery = {
    session_id: params.session_id,
    actor_user_id: params.actor_user_id,
    action_type: params.action_type,
    decision: params.decision,
    policy_id: params.policy_id,
    reason_code: params.reason_code,
    time_from: params.time_from,
    time_to: params.time_to,
  };

  const exported = await store.exportEvents(query);
  return {
    status: 200,
    body: exported,
  };
}

/**
 * GET /v1/audit/metrics
 */
export async function handleAuditMetrics(
  store: AuditStore,
  flushService?: FlushService,
): Promise<AuditRouteResult> {
  return {
    status: 200,
    body: {
      store: store.getMetrics(),
      totalEvents: await store.getCount(),
      flush: flushService?.getMetrics() || null,
    },
  };
}
