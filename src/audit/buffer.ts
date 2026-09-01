/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — SQLite Audit Event Buffer
 *
 * Resilient local buffer for audit events. Events are validated via the
 * minimum schema gate before storage, then flushed to the central
 * PostgreSQL store asynchronously.
 *
 * Bounded queue: max 10,000 events. Backpressure alert at 80% capacity.
 *
 * TDD Reference: Section 9.3, Appendix B.6 (audit pipeline degradation)
 * PRD Reference: Appendix C, R-3
 */

import Database from "better-sqlite3";
import * as path from "node:path";
import { validateAuditEvent } from "./validate.js";
import type { AuditEvent } from "../../types/audit-event.js";

const MAX_BUFFER_SIZE = 10_000;
const BACKPRESSURE_THRESHOLD = 0.8;

export interface BufferStats {
  count: number;
  maxSize: number;
  backpressure: boolean;
  full: boolean;
}

export class AuditBuffer {
  private db: Database.Database;
  private insertStmt: Database.Statement;
  private countStmt: Database.Statement;
  private flushStmt: Database.Statement;
  private deleteStmt: Database.Statement;

  // Metrics
  private acceptedCount = 0;
  private rejectedCount = 0;
  private backpressureAlerts = 0;

  constructor(dbPath?: string) {
    const resolvedPath = dbPath || path.join(process.cwd(), "build", "audit-buffer.sqlite");
    this.db = new Database(resolvedPath);
    this.db.pragma("journal_mode = WAL");
    this.db.pragma("synchronous = NORMAL");

    this.db.exec(`
      CREATE TABLE IF NOT EXISTS event_buffer (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        event_id TEXT NOT NULL UNIQUE,
        event_json TEXT NOT NULL,
        created_at TEXT NOT NULL DEFAULT (datetime('now')),
        flushed INTEGER NOT NULL DEFAULT 0
      )
    `);

    this.insertStmt = this.db.prepare(
      "INSERT OR IGNORE INTO event_buffer (event_id, event_json) VALUES (?, ?)",
    );
    this.countStmt = this.db.prepare(
      "SELECT COUNT(*) as count FROM event_buffer WHERE flushed = 0",
    );
    this.flushStmt = this.db.prepare(
      "SELECT id, event_json FROM event_buffer WHERE flushed = 0 ORDER BY id LIMIT ?",
    );
    this.deleteStmt = this.db.prepare(
      "UPDATE event_buffer SET flushed = 1 WHERE id = ?",
    );
  }

  /**
   * Buffer an audit event after validating it passes the minimum schema gate.
   * Returns true if accepted, false if rejected.
   */
  bufferEvent(event: AuditEvent): boolean {
    // Validate minimum schema gate
    const validation = validateAuditEvent(event);
    if (!validation.valid) {
      this.rejectedCount++;
      console.error(
        `[AUDIT] Event rejected by schema gate: ${validation.errors.join(", ")}`,
      );
      return false;
    }

    // Check buffer capacity
    const stats = this.getStats();
    if (stats.full) {
      this.rejectedCount++;
      console.error(
        `[AUDIT] Buffer full (${stats.count}/${stats.maxSize}). Event dropped.`,
      );
      return false;
    }

    if (stats.backpressure) {
      this.backpressureAlerts++;
      console.warn(
        `[AUDIT] Backpressure: buffer at ${stats.count}/${stats.maxSize} (${(stats.count / stats.maxSize * 100).toFixed(0)}%)`,
      );
    }

    // Store event
    const eventJson = JSON.stringify(event);
    this.insertStmt.run(event.event_id, eventJson);
    this.acceptedCount++;
    return true;
  }

  /**
   * Get events ready to flush to the central store.
   * Returns up to `limit` events (default 100).
   */
  getEventsToFlush(limit = 100): Array<{ id: number; event: AuditEvent }> {
    const rows = this.flushStmt.all(limit) as Array<{
      id: number;
      event_json: string;
    }>;

    return rows.map(row => ({
      id: row.id,
      event: JSON.parse(row.event_json) as AuditEvent,
    }));
  }

  /**
   * Mark events as flushed after successful central store write.
   */
  markFlushed(ids: number[]): void {
    const markMany = this.db.transaction((idList: number[]) => {
      for (const id of idList) {
        this.deleteStmt.run(id);
      }
    });
    markMany(ids);
  }

  /**
   * Get buffer statistics.
   */
  getStats(): BufferStats {
    const row = this.countStmt.get() as { count: number };
    const count = row.count;
    return {
      count,
      maxSize: MAX_BUFFER_SIZE,
      backpressure: count >= MAX_BUFFER_SIZE * BACKPRESSURE_THRESHOLD,
      full: count >= MAX_BUFFER_SIZE,
    };
  }

  /**
   * Get metrics for the readiness gate dashboard.
   */
  getMetrics(): { accepted: number; rejected: number; backpressureAlerts: number; bufferCount: number } {
    return {
      accepted: this.acceptedCount,
      rejected: this.rejectedCount,
      backpressureAlerts: this.backpressureAlerts,
      bufferCount: this.getStats().count,
    };
  }

  /**
   * Close the database connection.
   */
  close(): void {
    this.db.close();
  }
}