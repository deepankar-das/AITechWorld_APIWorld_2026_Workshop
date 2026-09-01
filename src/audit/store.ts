/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Central Audit Store (PostgreSQL)
 *
 * Persistent, append-only event storage using PostgreSQL JSONB.
 * Events are validated via the minimum schema gate before insertion.
 * No UPDATE or DELETE on event rows — enforced at application level.
 *
 * Falls back to in-memory storage if PostgreSQL is unavailable.
 *
 * TDD Reference: Section 9.3
 * PRD Reference: Appendix C, R-3
 */

import pg from "pg";
import { validateAuditEvent } from "./validate.js";
import type { AuditEvent } from "../../types/audit-event.js";

const { Pool } = pg;

export interface AuditQuery {
  session_id?: string;
  actor_user_id?: string;
  action_type?: string;
  decision?: string;
  policy_id?: string;
  reason_code?: string;
  time_from?: string;
  time_to?: string;
  limit?: number;
  offset?: number;
}

export interface AuditStoreMetrics {
  totalStored: number;
  totalRejected: number;
  backend: "postgresql" | "in-memory";
}

export class AuditStore {
  private pool: pg.Pool | null = null;
  private fallbackEvents: AuditEvent[] = []; // In-memory fallback
  private totalStored = 0;
  private totalRejected = 0;
  private usePostgres = false;

  constructor(connectionString?: string) {
    const connStr = connectionString
      || process.env.DATABASE_URL
      || `postgresql://${process.env.USER || "postgres"}@localhost:5432/enforcer`;

    try {
      this.pool = new Pool({ connectionString: connStr, max: 10 });
      // Test connection
      this.pool.query("SELECT 1").then(() => {
        this.usePostgres = true;
        console.log("[STORE] PostgreSQL connected — audit events will persist across restarts");
      }).catch((err) => {
        console.warn(`[STORE] PostgreSQL unavailable (${(err as Error).message}). Using in-memory fallback — data will be lost on restart.`);
        this.pool = null;
        this.usePostgres = false;
      });
    } catch {
      console.warn("[STORE] Could not create PostgreSQL pool. Using in-memory fallback.");
      this.pool = null;
      this.usePostgres = false;
    }
  }

  /**
   * Store an audit event after validation.
   */
  async storeEvent(event: AuditEvent): Promise<boolean> {
    const validation = validateAuditEvent(event);
    if (!validation.valid) {
      this.totalRejected++;
      console.error(`[STORE] Event rejected: ${validation.errors.join(", ")}`);
      return false;
    }

    if (this.pool && this.usePostgres) {
      try {
        await this.pool.query(
          "INSERT INTO audit_events (id, event) VALUES ($1, $2) ON CONFLICT DO NOTHING",
          [event.event_id, JSON.stringify(event)],
        );
        this.totalStored++;
        return true;
      } catch (err) {
        console.error(`[STORE] PostgreSQL insert failed: ${(err as Error).message}. Falling back to memory.`);
        this.fallbackEvents.push(event);
        this.totalStored++;
        return true;
      }
    }

    // In-memory fallback
    this.fallbackEvents.push(event);
    this.totalStored++;
    return true;
  }

  /**
   * Store multiple events (used by flush service).
   */
  async storeEvents(events: AuditEvent[]): Promise<{ stored: number; rejected: number }> {
    let stored = 0;
    let rejected = 0;
    for (const event of events) {
      if (await this.storeEvent(event)) {
        stored++;
      } else {
        rejected++;
      }
    }
    return { stored, rejected };
  }

  /**
   * Query events with filters.
   */
  async queryEvents(query: AuditQuery = {}): Promise<AuditEvent[]> {
    if (this.pool && this.usePostgres) {
      try {
        const conditions: string[] = [];
        const params: unknown[] = [];
        let paramIdx = 1;

        if (query.session_id) {
          conditions.push(`event->>'session_id' = $${paramIdx++}`);
          params.push(query.session_id);
        }
        if (query.actor_user_id) {
          conditions.push(`event->'actor'->>'user_id' = $${paramIdx++}`);
          params.push(query.actor_user_id);
        }
        if (query.action_type) {
          conditions.push(`event->'action'->>'type' = $${paramIdx++}`);
          params.push(query.action_type);
        }
        if (query.decision) {
          conditions.push(`event->'policy_detail'->>'decision' = $${paramIdx++}`);
          params.push(query.decision);
        }
        if (query.policy_id) {
          conditions.push(`event->'policy_detail'->>'policy_id' = $${paramIdx++}`);
          params.push(query.policy_id);
        }
        if (query.reason_code) {
          conditions.push(`event->'policy_detail'->>'reason_code' = $${paramIdx++}`);
          params.push(query.reason_code);
        }
        if (query.time_from) {
          conditions.push(`created_at >= $${paramIdx++}`);
          params.push(query.time_from);
        }
        if (query.time_to) {
          conditions.push(`created_at <= $${paramIdx++}`);
          params.push(query.time_to);
        }

        const where = conditions.length > 0 ? `WHERE ${conditions.join(" AND ")}` : "";
        const limit = query.limit || 100;
        const offset = query.offset || 0;

        const result = await this.pool.query(
          `SELECT event FROM audit_events ${where} ORDER BY created_at DESC LIMIT $${paramIdx++} OFFSET $${paramIdx++}`,
          [...params, limit, offset],
        );

        return result.rows.map(row => row.event as AuditEvent);
      } catch (err) {
        console.error(`[STORE] PostgreSQL query failed: ${(err as Error).message}`);
        // Fall through to in-memory
      }
    }

    // In-memory fallback query
    let results = [...this.fallbackEvents];
    if (query.session_id) results = results.filter(e => e.session_id === query.session_id);
    if (query.actor_user_id) results = results.filter(e => e.actor.user_id === query.actor_user_id);
    if (query.action_type) results = results.filter(e => e.action.type === query.action_type);
    if (query.decision) results = results.filter(e => e.policy_detail.decision === query.decision);
    if (query.policy_id) results = results.filter(e => e.policy_detail.policy_id === query.policy_id);
    if (query.reason_code) results = results.filter(e => e.policy_detail.reason_code === query.reason_code);
    if (query.time_from) {
      const from = new Date(query.time_from).getTime();
      results = results.filter(e => new Date(e.timestamp).getTime() >= from);
    }
    if (query.time_to) {
      const to = new Date(query.time_to).getTime();
      results = results.filter(e => new Date(e.timestamp).getTime() <= to);
    }
    results.sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime());
    const offset = query.offset || 0;
    const limit = query.limit || 100;
    return results.slice(offset, offset + limit);
  }

  /**
   * Get all events for a session (chronological order for replay).
   */
  async getSession(sessionId: string): Promise<AuditEvent[]> {
    if (this.pool && this.usePostgres) {
      try {
        const result = await this.pool.query(
          "SELECT event FROM audit_events WHERE event->>'session_id' = $1 ORDER BY created_at ASC",
          [sessionId],
        );
        return result.rows.map(row => row.event as AuditEvent);
      } catch { /* fall through */ }
    }

    return this.fallbackEvents
      .filter(e => e.session_id === sessionId)
      .sort((a, b) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime());
  }

  /**
   * Get unique session IDs with summary info.
   */
  async getSessions(): Promise<Array<{
    session_id: string;
    user_id: string;
    agent_type: string;
    agent_instance: string;
    event_count: number;
    first_event: string;
    last_event: string;
    decisions: Record<string, number>;
  }>> {
    if (this.pool && this.usePostgres) {
      try {
        const result = await this.pool.query(`
          SELECT
            event->>'session_id' as session_id,
            event->'actor'->>'user_id' as user_id,
            event->'actor'->>'agent_type' as agent_type,
            event->'actor'->>'agent_instance' as agent_instance,
            COUNT(*) as event_count,
            MIN(created_at) as first_event,
            MAX(created_at) as last_event,
            jsonb_object_agg(
              COALESCE(event->'policy_detail'->>'decision', 'unknown'),
              1
            ) as decisions_sample
          FROM audit_events
          GROUP BY
            event->>'session_id',
            event->'actor'->>'user_id',
            event->'actor'->>'agent_type',
            event->'actor'->>'agent_instance'
          ORDER BY MAX(created_at) DESC
        `);

        // Need to get actual decision counts per session
        const sessions = [];
        for (const row of result.rows) {
          const decisionResult = await this.pool!.query(`
            SELECT event->'policy_detail'->>'decision' as decision, COUNT(*) as cnt
            FROM audit_events
            WHERE event->>'session_id' = $1
            GROUP BY event->'policy_detail'->>'decision'
          `, [row.session_id]);

          const decisions: Record<string, number> = {};
          for (const dr of decisionResult.rows) {
            decisions[dr.decision] = parseInt(dr.cnt, 10);
          }

          sessions.push({
            session_id: row.session_id,
            user_id: row.user_id || "unknown",
            agent_type: row.agent_type || "unknown",
            agent_instance: row.agent_instance || "unknown",
            event_count: parseInt(row.event_count, 10),
            first_event: row.first_event,
            last_event: row.last_event,
            decisions,
          });
        }
        return sessions;
      } catch (err) {
        console.error(`[STORE] PostgreSQL sessions query failed: ${(err as Error).message}`);
        // Fall through to in-memory
      }
    }

    // In-memory fallback
    const sessionMap = new Map<string, AuditEvent[]>();
    for (const event of this.fallbackEvents) {
      const existing = sessionMap.get(event.session_id) || [];
      existing.push(event);
      sessionMap.set(event.session_id, existing);
    }

    return Array.from(sessionMap.entries()).map(([sessionId, events]) => {
      const sorted = events.sort((a, b) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime());
      const decisions: Record<string, number> = {};
      for (const e of events) {
        const d = e.policy_detail.decision;
        decisions[d] = (decisions[d] || 0) + 1;
      }
      const firstEvent = sorted[0];
      return {
        session_id: sessionId,
        user_id: firstEvent.actor.user_id,
        agent_type: firstEvent.actor.agent_type,
        agent_instance: firstEvent.actor.agent_instance,
        event_count: events.length,
        first_event: sorted[0].timestamp,
        last_event: sorted[sorted.length - 1].timestamp,
        decisions,
      };
    });
  }

  /**
   * Export events as JSON (for evidence packages).
   */
  async exportEvents(query: AuditQuery = {}): Promise<{
    metadata: { exported_at: string; filters: AuditQuery; total_events: number };
    events: AuditEvent[];
  }> {
    const events = await this.queryEvents({ ...query, limit: 10000 });
    return {
      metadata: {
        exported_at: new Date().toISOString(),
        filters: query,
        total_events: events.length,
      },
      events,
    };
  }

  /**
   * Get metrics.
   */
  getMetrics(): AuditStoreMetrics {
    return {
      totalStored: this.totalStored,
      totalRejected: this.totalRejected,
      backend: this.usePostgres ? "postgresql" : "in-memory",
    };
  }

  /**
   * Get total event count.
   */
  async getCount(): Promise<number> {
    if (this.pool && this.usePostgres) {
      try {
        const result = await this.pool.query("SELECT COUNT(*) as cnt FROM audit_events");
        return parseInt(result.rows[0].cnt, 10);
      } catch { /* fall through */ }
    }
    return this.fallbackEvents.length;
  }
}
