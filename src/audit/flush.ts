/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Buffer Flush Service
 *
 * Background service that flushes events from SQLite local buffer
 * to the central audit store (PostgreSQL or in-memory for prototype).
 *
 * Runs on a configurable interval (default: 5 seconds).
 * Retries on failure with exponential backoff.
 *
 * TDD Reference: Implementation Plan C3
 */

import type { AuditBuffer } from "./buffer.js";
import type { AuditStore } from "./store.js";

export interface FlushServiceOptions {
  intervalMs?: number;
  batchSize?: number;
  maxRetries?: number;
}

export class FlushService {
  private timer: ReturnType<typeof setInterval> | null = null;
  private running = false;
  private retryCount = 0;
  private totalFlushed = 0;
  private totalFlushErrors = 0;

  private buffer: AuditBuffer;
  private store: AuditStore;
  private intervalMs: number;
  private batchSize: number;
  private maxRetries: number;

  constructor(
    buffer: AuditBuffer,
    store: AuditStore,
    options: FlushServiceOptions = {},
  ) {
    this.buffer = buffer;
    this.store = store;
    this.intervalMs = options.intervalMs ?? 5000;
    this.batchSize = options.batchSize ?? 100;
    this.maxRetries = options.maxRetries ?? 5;
  }

  /**
   * Start the flush service.
   */
  start(): void {
    if (this.timer) return;
    this.running = true;
    this.timer = setInterval(() => {
      this.flush().catch(err => {
        console.error("[FLUSH] Error:", err);
      });
    }, this.intervalMs);
    console.log(`[FLUSH] Service started (interval: ${this.intervalMs}ms, batch: ${this.batchSize})`);
  }

  /**
   * Stop the flush service.
   */
  stop(): void {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
    this.running = false;
  }

  /**
   * Flush one batch of events from buffer to central store.
   */
  async flush(): Promise<{ flushed: number; errors: number }> {
    const batch = this.buffer.getEventsToFlush(this.batchSize);
    if (batch.length === 0) return { flushed: 0, errors: 0 };

    try {
      const result = await this.store.storeEvents(batch.map(b => b.event));

      // Mark successfully stored events as flushed
      if (result.stored > 0) {
        const flushedIds = batch.slice(0, result.stored).map(b => b.id);
        this.buffer.markFlushed(flushedIds);
      }

      this.totalFlushed += result.stored;
      this.totalFlushErrors += result.rejected;
      this.retryCount = 0; // Reset retry counter on success

      if (result.stored > 0) {
        console.log(`[FLUSH] Flushed ${result.stored} events to central store`);
      }

      return { flushed: result.stored, errors: result.rejected };
    } catch (err) {
      this.retryCount++;
      this.totalFlushErrors++;
      console.error(`[FLUSH] Failed (retry ${this.retryCount}/${this.maxRetries}):`, err);

      if (this.retryCount >= this.maxRetries) {
        console.error("[FLUSH] Max retries exceeded. Events remain in local buffer.");
      }

      return { flushed: 0, errors: batch.length };
    }
  }

  /**
   * Force an immediate flush (used before shutdown or on demand).
   */
  async flushAll(): Promise<{ flushed: number; errors: number }> {
    let totalFlushed = 0;
    let totalErrors = 0;

    // Flush in batches until buffer is empty
    while (true) {
      const result = await this.flush();
      totalFlushed += result.flushed;
      totalErrors += result.errors;
      if (result.flushed === 0) break;
    }

    return { flushed: totalFlushed, errors: totalErrors };
  }

  /**
   * Get flush service metrics.
   */
  getMetrics() {
    return {
      running: this.running,
      totalFlushed: this.totalFlushed,
      totalFlushErrors: this.totalFlushErrors,
      retryCount: this.retryCount,
      bufferStats: this.buffer.getStats(),
      storeMetrics: this.store.getMetrics(),
    };
  }
}