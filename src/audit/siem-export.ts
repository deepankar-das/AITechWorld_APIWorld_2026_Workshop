/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — SIEM Export
 *
 * Exports audit events to external SIEM systems:
 *   - Webhook (HTTP POST to configurable URL)
 *   - Syslog (RFC 5424 format)
 *   - File (JSONL for Splunk/Elastic ingestion)
 *
 * Events are exported in real-time as they're buffered,
 * with configurable batching and retry.
 */

import * as fs from "node:fs";
import * as dgram from "node:dgram";
import type { AuditEvent } from "../../types/audit-event.js";

export type SiemTransport = "webhook" | "syslog" | "file";

export interface SiemConfig {
  enabled: boolean;
  transport: SiemTransport;
  /** Webhook URL for HTTP POST transport */
  webhook_url?: string;
  /** Webhook headers (e.g., auth token) */
  webhook_headers?: Record<string, string>;
  /** Syslog host:port for UDP transport */
  syslog_host?: string;
  syslog_port?: number;
  /** File path for JSONL output */
  file_path?: string;
  /** Batch size before sending */
  batch_size?: number;
  /** Flush interval in ms */
  flush_interval_ms?: number;
}

export class SiemExporter {
  private config: SiemConfig;
  private buffer: AuditEvent[] = [];
  private timer: ReturnType<typeof setInterval> | null = null;
  private totalExported = 0;
  private totalErrors = 0;
  private syslogSocket: dgram.Socket | null = null;

  constructor(config: SiemConfig) {
    this.config = config;

    if (config.enabled && config.transport === "syslog") {
      this.syslogSocket = dgram.createSocket("udp4");
    }
  }

  /**
   * Queue an event for SIEM export.
   */
  enqueue(event: AuditEvent): void {
    if (!this.config.enabled) return;

    this.buffer.push(event);

    const batchSize = this.config.batch_size || 10;
    if (this.buffer.length >= batchSize) {
      this.flush().catch(err => console.error("[SIEM] Flush error:", err));
    }
  }

  /**
   * Start the periodic flush timer.
   */
  start(): void {
    if (!this.config.enabled) return;

    const interval = this.config.flush_interval_ms || 10000;
    this.timer = setInterval(() => {
      this.flush().catch(err => console.error("[SIEM] Periodic flush error:", err));
    }, interval);

    console.log(`[SIEM] Export started: ${this.config.transport} (batch: ${this.config.batch_size || 10}, interval: ${interval}ms)`);
  }

  /**
   * Stop the exporter.
   */
  stop(): void {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
    if (this.syslogSocket) {
      this.syslogSocket.close();
    }
  }

  /**
   * Flush buffered events to the SIEM.
   */
  async flush(): Promise<{ exported: number; errors: number }> {
    if (this.buffer.length === 0) return { exported: 0, errors: 0 };

    const batch = this.buffer.splice(0, this.config.batch_size || 100);
    let exported = 0;
    let errors = 0;

    try {
      switch (this.config.transport) {
        case "webhook":
          await this.sendWebhook(batch);
          exported = batch.length;
          break;
        case "syslog":
          this.sendSyslog(batch);
          exported = batch.length;
          break;
        case "file":
          this.writeFile(batch);
          exported = batch.length;
          break;
      }
    } catch (err) {
      errors = batch.length;
      // Put events back for retry
      this.buffer.unshift(...batch);
      console.error(`[SIEM] Export failed (${this.config.transport}): ${(err as Error).message}`);
    }

    this.totalExported += exported;
    this.totalErrors += errors;
    if (exported > 0) {
      console.log(`[SIEM] Exported ${exported} events via ${this.config.transport}`);
    }

    return { exported, errors };
  }

  /**
   * Send events via HTTP webhook.
   */
  private async sendWebhook(events: AuditEvent[]): Promise<void> {
    if (!this.config.webhook_url) throw new Error("webhook_url not configured");

    const response = await fetch(this.config.webhook_url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...(this.config.webhook_headers || {}),
      },
      body: JSON.stringify({
        source: "enforcer",
        timestamp: new Date().toISOString(),
        event_count: events.length,
        events: events.map(e => ({
          // SIEM-friendly flat format
          timestamp: e.timestamp,
          event_id: e.event_id,
          session_id: e.session_id,
          user: e.actor.user_id,
          agent: e.actor.agent_type,
          action: e.action.type,
          resource: e.resource.value || e.resource.path || e.resource.host || "",
          decision: e.policy_detail.decision,
          reason_code: e.policy_detail.reason_code,
          policy_id: e.policy_detail.policy_id,
          policy_version: e.policy_detail.policy_version,
          observed_effect: e.action.observed_effect,
          severity: e.policy_detail.decision === "deny" ? "high" : e.policy_detail.decision === "require_approval" ? "medium" : "low",
        })),
      }),
    });

    if (!response.ok) {
      throw new Error(`Webhook returned ${response.status}`);
    }
  }

  /**
   * Send events via syslog (RFC 5424 over UDP).
   */
  private sendSyslog(events: AuditEvent[]): void {
    if (!this.syslogSocket || !this.config.syslog_host) return;

    const host = this.config.syslog_host;
    const port = this.config.syslog_port || 514;

    for (const event of events) {
      // RFC 5424: <priority>version timestamp hostname app-name procid msgid structured-data msg
      const severity = event.policy_detail.decision === "deny" ? 2 : event.policy_detail.decision === "require_approval" ? 4 : 6;
      const facility = 10; // security/auth
      const priority = facility * 8 + severity;

      const msg = `<${priority}>1 ${event.timestamp} ${event.environment.workspace} enforcer ${process.pid} ${event.event_id} - ` +
        `action="${event.action.type}" user="${event.actor.user_id}" agent="${event.actor.agent_type}" ` +
        `decision="${event.policy_detail.decision}" reason="${event.policy_detail.reason_code}" ` +
        `resource="${event.resource.value || event.resource.path || ""}"`;

      const buf = Buffer.from(msg, "utf-8");
      this.syslogSocket.send(buf, 0, buf.length, port, host);
    }
  }

  /**
   * Write events to a JSONL file (one JSON object per line).
   */
  private writeFile(events: AuditEvent[]): void {
    if (!this.config.file_path) throw new Error("file_path not configured");

    const lines = events.map(e => JSON.stringify({
      timestamp: e.timestamp,
      event_id: e.event_id,
      session_id: e.session_id,
      user: e.actor.user_id,
      agent: e.actor.agent_type,
      action: e.action.type,
      attempted: e.action.attempted_action,
      resource: e.resource.value || e.resource.path || e.resource.host || "",
      decision: e.policy_detail.decision,
      reason_code: e.policy_detail.reason_code,
      reason_human: e.policy_detail.reason_human,
      policy_id: e.policy_detail.policy_id,
      observed_effect: e.action.observed_effect,
    })).join("\n") + "\n";

    fs.appendFileSync(this.config.file_path, lines, "utf-8");
  }

  /**
   * Get export metrics.
   */
  getMetrics() {
    return {
      enabled: this.config.enabled,
      transport: this.config.transport,
      total_exported: this.totalExported,
      total_errors: this.totalErrors,
      buffer_size: this.buffer.length,
    };
  }
}