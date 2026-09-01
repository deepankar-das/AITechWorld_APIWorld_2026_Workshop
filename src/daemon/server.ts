/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Local Daemon HTTP Server
 *
 * Central control-plane component on the developer's machine.
 * Receives action requests from enforcement points, evaluates policy,
 * routes approvals, and buffers audit events.
 *
 * TDD Reference: Section 7
 * PRD Reference: Appendix C, R-1 through R-5
 */

import * as http from "node:http";
import * as path from "node:path";
import { handleEvaluate } from "./routes/evaluate.js";
import {
  handleGetPending,
  handleGetApproval,
  handleResolveApproval,
  handleGetApprovalMetrics,
} from "./routes/approvals.js";
import { calculateMetrics } from "./routes/metrics.js";
import { handleEnrich } from "./routes/enrich.js";
import {
  setPolicyBundleRef,
  handleListRules,
  handleAddRule,
  handleUpdateRule,
  handleDeleteRule,
  handleToggleRule,
  handleGetBundle,
  handleListPacks,
  handleApplyPack,
  handleGetPack,
  handleCreatePack,
} from "./routes/policy.js";
import { isEnforcementEnabled, setEnforcementEnabled, getEnforcementState } from "./enforcement-state.js";
import { isAuthenticated, sendUnauthorized } from "./auth.js";
import { enforcePosture } from "../enforcement/container-posture.js";
import {
  handleQueryEvents,
  handleGetSessions,
  handleGetSession,
  handleExportEvents,
  handleAuditMetrics,
} from "./routes/audit.js";
import { loadPolicyBundle } from "../policy/loader.js";
import { AuditBuffer } from "../audit/buffer.js";
import { AuditStore } from "../audit/store.js";
import { FlushService } from "../audit/flush.js";
import { ApprovalService } from "../approval/service.js";
import type { PolicyBundle } from "../../types/policy.js";

const DAEMON_PORT = parseInt(process.env.DAEMON_PORT || "9100", 10);

// ── State ───────────────────────────────────────────────────────────────────

let policyBundle: PolicyBundle | null = null;
let auditBuffer: AuditBuffer | null = null;
let auditStore: AuditStore | null = null;
let flushService: FlushService | null = null;
let approvalService: ApprovalService | null = null;

// ── Load policy bundle ──────────────────────────────────────────────────────

function loadPolicy(): PolicyBundle {
  const policyPath = path.resolve(process.cwd(), "policies", "default.yaml");
  try {
    const loaded = loadPolicyBundle(policyPath);
    console.log(`[DAEMON] Policy loaded: ${loaded.bundle.bundle_version} (${loaded.bundle.rules.length} rules)`);
    return loaded.bundle;
  } catch (err) {
    console.warn(`[DAEMON] No policy bundle at ${policyPath} — starting with empty bundle`);
    return {
      bundle_version: "v0.0.0",
      scope_level: "organization",
      rules: [],
    };
  }
}

// ── Request handler ─────────────────────────────────────────────────────────

function readBody(req: http.IncomingMessage): Promise<string> {
  return new Promise((resolve, reject) => {
    const chunks: Buffer[] = [];
    req.on("data", (chunk: Buffer) => chunks.push(chunk));
    req.on("end", () => resolve(Buffer.concat(chunks).toString("utf-8")));
    req.on("error", reject);
  });
}

function sendJson(res: http.ServerResponse, status: number, body: unknown): void {
  const json = JSON.stringify(body);
  res.writeHead(status, {
    "Content-Type": "application/json",
    "Content-Length": Buffer.byteLength(json),
    "Access-Control-Allow-Origin": "*",
    "Access-Control-Allow-Methods": "GET, POST, OPTIONS",
    "Access-Control-Allow-Headers": "Content-Type",
  });
  res.end(json);
}

async function requestHandler(
  req: http.IncomingMessage,
  res: http.ServerResponse,
): Promise<void> {
  const url = req.url || "/";
  const method = req.method || "GET";

  // CORS preflight
  if (method === "OPTIONS") {
    res.writeHead(204, {
      "Access-Control-Allow-Origin": "*",
      "Access-Control-Allow-Methods": "GET, POST, OPTIONS",
      "Access-Control-Allow-Headers": "Content-Type",
    });
    res.end();
    return;
  }

  // Health check (includes enforcement state)
  if (url === "/v1/health" && method === "GET") {
    const enfState = getEnforcementState();
    sendJson(res, 200, {
      status: "ok",
      enforcement: enfState,
      policy_version: policyBundle?.bundle_version || "none",
      policy_rules: policyBundle?.rules.length || 0,
      buffer_stats: auditBuffer?.getStats() || null,
    });
    return;
  }

  // ── ADMIN-AUTHENTICATED ENDPOINTS BELOW ─────────────────────────────────
  // All endpoints below this line require admin token.
  // /v1/evaluate, /v1/health, /v1/enforcement (GET), and /v1/audit/enrich
  // are unauthenticated (enforcement path + monitoring).

  // Enforcement state (GET is public — hooks need to check it)
  if (url === "/v1/enforcement" && method === "GET") {
    sendJson(res, 200, getEnforcementState());
    return;
  }

  // Enforcement toggle (POST requires admin)
  if (url === "/v1/enforcement/toggle" && method === "POST") {
    if (!isAuthenticated(req)) { sendUnauthorized(res); return; }
    const body = await readBody(req);
    let parsed: { enabled?: boolean } = {};
    try { parsed = JSON.parse(body); } catch { /* use toggle */ }

    if (typeof parsed.enabled === "boolean") {
      setEnforcementEnabled(parsed.enabled, "console");
    } else {
      // Toggle
      setEnforcementEnabled(!isEnforcementEnabled(), "console");
    }
    const state = getEnforcementState();
    console.log(`[DAEMON] Enforcement ${state.enabled ? "ENABLED" : "DISABLED"} by ${state.changed_by} at ${state.since}`);
    sendJson(res, 200, state);
    return;
  }

  // Policy evaluation (respects enforcement state)
  if (url === "/v1/evaluate" && method === "POST") {
    // If enforcement is disabled, allow everything but still log
    if (!isEnforcementEnabled()) {
      const body = await readBody(req);
      let parsed: { request_id?: string } = {};
      try { parsed = JSON.parse(body); } catch { /* ignore */ }
      sendJson(res, 200, {
        request_id: parsed.request_id || "unknown",
        decision: "allow",
        reason_code: "ENFORCEMENT_DISABLED",
        reason_human: "Enforcer enforcement is currently disabled. Action allowed.",
        policy_id: "system.enforcement_disabled",
        policy_version: policyBundle?.bundle_version || "unknown",
        approval_required: false,
      });
      return;
    }

    const body = await readBody(req);
    const result = handleEvaluate(body, policyBundle!, auditBuffer!, approvalService!);
    sendJson(res, result.status, result.body);
    return;
  }

  // ── ALL ROUTES BELOW REQUIRE ADMIN AUTH ────────────────────────────────
  // Approval resolve, audit queries, policy management, exports.
  // If no valid admin token is provided, return 401.
  if (!isAuthenticated(req)) {
    sendUnauthorized(res);
    return;
  }

  // Approval routes (admin authenticated)
  if (url === "/v1/approvals/pending" && method === "GET") {
    const result = handleGetPending(approvalService!);
    sendJson(res, result.status, result.body);
    return;
  }

  if (url?.startsWith("/v1/approvals/") && url.endsWith("/resolve") && method === "POST") {
    const approvalId = url.split("/")[3];
    const body = await readBody(req);
    const result = handleResolveApproval(approvalId, body, approvalService!);
    sendJson(res, result.status, result.body);
    return;
  }

  if (url?.startsWith("/v1/approvals/") && method === "GET") {
    const approvalId = url.split("/")[3];
    if (approvalId === "metrics") {
      const result = handleGetApprovalMetrics(approvalService!);
      sendJson(res, result.status, result.body);
      return;
    }
    const result = handleGetApproval(approvalId, approvalService!);
    sendJson(res, result.status, result.body);
    return;
  }

  // Metrics (readiness gates)
  if (url === "/v1/metrics" && method === "GET") {
    const metrics = await calculateMetrics(policyBundle!, auditBuffer!, auditStore!, approvalService!);
    sendJson(res, 200, metrics);
    return;
  }

  // Audit routes
  if (url === "/v1/audit/sessions" && method === "GET") {
    const result = await handleGetSessions(auditStore!);
    sendJson(res, result.status, result.body);
    return;
  }

  if (url?.startsWith("/v1/audit/sessions/") && method === "GET") {
    const sessionId = url.split("/v1/audit/sessions/")[1]?.split("?")[0];
    if (sessionId) {
      const result = await handleGetSession(sessionId, auditStore!);
      sendJson(res, result.status, result.body);
      return;
    }
  }

  if (url?.startsWith("/v1/audit/export") && method === "GET") {
    const result = await handleExportEvents(url, auditStore!);
    sendJson(res, result.status, result.body);
    return;
  }

  if (url?.startsWith("/v1/audit/events") && method === "GET") {
    const result = await handleQueryEvents(url, auditStore!);
    sendJson(res, result.status, result.body);
    return;
  }

  if (url === "/v1/audit/metrics" && method === "GET") {
    const result = await handleAuditMetrics(auditStore!, flushService!);
    sendJson(res, result.status, result.body);
    return;
  }

  // Audit enrichment (post-tool-call observed_effect)
  if (url === "/v1/audit/enrich" && method === "POST") {
    const body = await readBody(req);
    const result = await handleEnrich(body, auditStore!);
    sendJson(res, result.status, result.body);
    return;
  }

  // Policy management routes
  if (url === "/v1/policy/rules" && method === "GET") {
    const result = handleListRules();
    sendJson(res, result.status, result.body);
    return;
  }

  if (url === "/v1/policy/rules" && method === "POST") {
    const body = await readBody(req);
    const result = handleAddRule(body);
    sendJson(res, result.status, result.body);
    return;
  }

  if (url?.startsWith("/v1/policy/rules/") && url.endsWith("/toggle") && method === "POST") {
    const ruleId = url.replace("/v1/policy/rules/", "").replace("/toggle", "");
    const result = handleToggleRule(ruleId);
    sendJson(res, result.status, result.body);
    return;
  }

  if (url?.startsWith("/v1/policy/rules/") && method === "PUT") {
    const ruleId = url.replace("/v1/policy/rules/", "");
    const body = await readBody(req);
    const result = handleUpdateRule(ruleId, body);
    sendJson(res, result.status, result.body);
    return;
  }

  if (url?.startsWith("/v1/policy/rules/") && method === "DELETE") {
    const ruleId = url.replace("/v1/policy/rules/", "");
    const result = handleDeleteRule(ruleId);
    sendJson(res, result.status, result.body);
    return;
  }

  if (url === "/v1/policy/bundle" && method === "GET") {
    const result = handleGetBundle();
    sendJson(res, result.status, result.body);
    return;
  }

  if (url === "/v1/policy/packs" && method === "GET") {
    const result = handleListPacks();
    sendJson(res, result.status, result.body);
    return;
  }

  if (url === "/v1/policy/packs" && method === "POST") {
    const body = await readBody(req);
    const result = handleCreatePack(body);
    sendJson(res, result.status, result.body);
    return;
  }

  if (url?.startsWith("/v1/policy/packs/") && url.endsWith("/apply") && method === "POST") {
    const packId = url.replace("/v1/policy/packs/", "").replace("/apply", "");
    const result = handleApplyPack(packId);
    sendJson(res, result.status, result.body);
    return;
  }

  if (url?.startsWith("/v1/policy/packs/") && !url.endsWith("/apply") && method === "GET") {
    const packId = url.replace("/v1/policy/packs/", "").split("?")[0];
    const result = handleGetPack(packId);
    sendJson(res, result.status, result.body);
    return;
  }

  // 404
  sendJson(res, 404, { error: "Not found" });
}

// ── Start server ────────────────────────────────────────────────────────────

export function startDaemon(): http.Server {
  // Container posture check (exits on critical violations)
  enforcePosture();

  policyBundle = loadPolicy();
  setPolicyBundleRef({ bundle: policyBundle });
  auditBuffer = new AuditBuffer();
  auditStore = new AuditStore();
  approvalService = new ApprovalService();
  flushService = new FlushService(auditBuffer, auditStore, { intervalMs: 5000 });
  flushService.start();

  // Wire approval lifecycle events into audit buffer
  approvalService.onEvent((event) => {
    const timestamp = new Date().toISOString();
    const eventType = event.type;
    const auditEvent = {
      event_id: `approval_${eventType}_${Date.now()}`,
      timestamp,
      session_id: "approval_service",
      correlation_id: event.approvalId,
      who: `system|approval_service|${event.approvalId}`,
      what: `approval.${eventType}:${event.requestId}`,
      when: timestamp,
      policy: "approval.lifecycle@v1",
      decision: `${eventType}:${event.approvalId}`,
      result: eventType,
      actor: { user_id: "system", agent_type: "approval_service", agent_instance: "daemon" },
      environment: { workspace: "", repo: "", branch: "", tier: "development", deployment_mode: "host" },
      action: { type: `approval.${eventType}`, attempted_action: `Approval ${eventType} for ${event.requestId}`, observed_effect: eventType },
      resource: { kind: "approval" as const, value: event.approvalId, classification: [] },
      policy_detail: { policy_id: "approval.lifecycle", policy_version: "v1", decision: eventType, reason_code: eventType.toUpperCase(), reason_human: `Approval ${eventType}` },
    };
    auditBuffer!.bufferEvent(auditEvent as import("../../types/audit-event.js").AuditEvent);
  });

  const server = http.createServer((req, res) => {
    const requestStart = Date.now();
    requestHandler(req, res).then(() => {
      const latencyMs = Date.now() - requestStart;
      if (latencyMs > 100) {
        console.warn(`[DAEMON] Slow request: ${req.method} ${req.url} took ${latencyMs}ms`);
      }
    }).catch(err => {
      console.error("[DAEMON] Request error:", err);
      sendJson(res, 500, { error: "Internal server error" });
    });
  });

  server.listen(DAEMON_PORT, "127.0.0.1", () => {
    console.log(`[DAEMON] Enforcer daemon listening on http://127.0.0.1:${DAEMON_PORT}`);
    console.log(`[DAEMON] Policy: ${policyBundle!.bundle_version} (${policyBundle!.rules.length} rules)`);
    console.log(`[DAEMON] Endpoints: /v1/evaluate, /v1/health, /v1/metrics, /v1/audit/*, /v1/approvals/*`);
  });

  return server;
}

// Run if executed directly
const isMain = process.argv[1] && (
  process.argv[1].endsWith("server.ts") ||
  process.argv[1].endsWith("server.js")
);
if (isMain) {
  startDaemon();
}
