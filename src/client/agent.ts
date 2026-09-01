/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Client Agent
 *
 * Runs on each developer's machine as a privileged system service.
 * Connects to the Central Security Server over mTLS to:
 *   - Register itself on startup
 *   - Pull policy bundles (signed, versioned)
 *   - Forward audit events
 *   - Send heartbeats
 *
 * Locally:
 *   - Runs the enforcement daemon (policy evaluation, hooks, proxies)
 *   - Writes Claude Code config as read-only for the developer
 *   - Stores audit events in local PostgreSQL + forwards to central
 *
 * The developer CANNOT:
 *   - Stop this agent (it's a LaunchDaemon)
 *   - Edit the hooks config (managed settings, read-only)
 *   - Modify policies (pulled from central, written by root)
 *   - Access the admin token or client certificates
 */

import * as https from "node:https";
import * as http from "node:http";
import * as fs from "node:fs";
import * as path from "node:path";
import * as os from "node:os";
import { v4 as uuidv4 } from "uuid";

const CENTRAL_URL = process.env.AA_CENTRAL_URL || "https://localhost:9200";
const CERT_DIR = process.env.CERT_DIR || "/etc/enforcer/certs";
const CLIENT_ID = process.env.AA_CLIENT_ID || `client_${os.hostname()}_${uuidv4().slice(0, 8)}`;
const SYNC_INTERVAL_MS = parseInt(process.env.AA_SYNC_INTERVAL || "30000", 10); // 30 seconds
const CONFIG_DIR = process.env.AA_CONFIG_DIR || "/etc/enforcer";

// ── State ───────────────────────────────────────────────────────────────────

// eslint-disable-next-line prefer-const
let currentPolicyVersion = "";
let currentPolicyHash = "";
let pendingAuditEvents: unknown[] = [];
let syncTimer: ReturnType<typeof setInterval> | null = null;

// ── TLS Client Setup ────────────────────────────────────────────────────────

function loadClientTls(): https.RequestOptions | null {
  try {
    return {
      key: fs.readFileSync(path.join(CERT_DIR, "client-key.pem")),
      cert: fs.readFileSync(path.join(CERT_DIR, "client.pem")),
      ca: fs.readFileSync(path.join(CERT_DIR, "ca.pem")),
      rejectUnauthorized: true, // Verify server certificate against CA
    };
  } catch {
    console.warn(`[CLIENT] TLS certificates not found at ${CERT_DIR}. Using plaintext.`);
    return null;
  }
}

const tlsOptions = loadClientTls();

// ── HTTP Client ─────────────────────────────────────────────────────────────

async function callCentral(
  method: string,
  apiPath: string,
  body?: unknown,
): Promise<{ status: number; data: Record<string, unknown> }> {
  const url = new URL(apiPath, CENTRAL_URL);
  const isHttps = url.protocol === "https:";

  const payload = body ? JSON.stringify(body) : undefined;

  return new Promise((resolve, reject) => {
    const options: https.RequestOptions = {
      hostname: url.hostname,
      port: url.port || (isHttps ? 443 : 80),
      path: url.pathname,
      method,
      headers: {
        "Content-Type": "application/json",
        "X-Client-Id": CLIENT_ID,
        ...(payload ? { "Content-Length": Buffer.byteLength(payload) } : {}),
      },
      ...(isHttps && tlsOptions ? tlsOptions : {}),
    };

    const transport = isHttps ? https : http;
    const req = transport.request(options, (res) => {
      const chunks: Buffer[] = [];
      res.on("data", (chunk: Buffer) => chunks.push(chunk));
      res.on("end", () => {
        const raw = Buffer.concat(chunks).toString("utf-8");
        try {
          resolve({ status: res.statusCode || 0, data: JSON.parse(raw) });
        } catch {
          resolve({ status: res.statusCode || 0, data: { raw } });
        }
      });
    });

    req.on("error", (err) => {
      reject(err);
    });

    if (payload) req.write(payload);
    req.end();
  });
}

// ── Registration ────────────────────────────────────────────────────────────

async function registerWithCentral(): Promise<boolean> {
  try {
    const result = await callCentral("POST", "/api/v1/register", {
      hostname: os.hostname(),
      governed_users: [process.env.USER || os.userInfo().username],
    });

    if (result.status === 200) {
      // Registration successful
      console.log(`[CLIENT] Registered with central server as ${CLIENT_ID}`);
      return true;
    }
    console.error(`[CLIENT] Registration failed: ${result.status}`);
    return false;
  } catch (err) {
    console.error(`[CLIENT] Cannot reach central server at ${CENTRAL_URL}: ${(err as Error).message}`);
    return false;
  }
}

// ── Policy Sync ─────────────────────────────────────────────────────────────

async function syncPolicy(): Promise<void> {
  try {
    const result = await callCentral("GET", "/api/v1/policy/pull");

    if (result.status === 200) {
      const newVersion = result.data.version as string;
      const newHash = result.data.hash as string;
      const newBundle = result.data.bundle as string;

      if (newHash !== currentPolicyHash) {
        // Policy changed — write to local config
        const policyPath = path.join(CONFIG_DIR, "default.yaml");
        fs.writeFileSync(policyPath, newBundle, { mode: 0o644 });

        currentPolicyVersion = newVersion;
        currentPolicyHash = newHash;

        console.log(`[CLIENT] Policy updated to ${newVersion} (hash: ${newHash})`);
        // TODO: Signal the local daemon to reload policy
      }
    }
  } catch (err) {
    console.warn(`[CLIENT] Policy sync failed: ${(err as Error).message}`);
  }
}

// ── Audit Event Forwarding ──────────────────────────────────────────────────

/**
 * Queue an audit event for forwarding to the central server.
 * Called by the local daemon's audit pipeline.
 */
export function queueAuditEvent(event: unknown): void {
  pendingAuditEvents.push(event);
}

async function flushAuditEvents(): Promise<void> {
  if (pendingAuditEvents.length === 0) return;

  const batch = pendingAuditEvents.splice(0, 100); // Send up to 100 at a time

  try {
    const result = await callCentral("POST", "/api/v1/audit/push", {
      events: batch,
    });

    if (result.status === 200) {
      console.log(`[CLIENT] Forwarded ${batch.length} audit events to central`);
    } else {
      // Put them back for retry
      pendingAuditEvents.unshift(...batch);
      console.warn(`[CLIENT] Audit push failed (${result.status}). Will retry.`);
    }
  } catch (err) {
    // Put them back for retry
    pendingAuditEvents.unshift(...batch);
    console.warn(`[CLIENT] Audit push failed: ${(err as Error).message}. Will retry.`);
  }
}

// ── Heartbeat ───────────────────────────────────────────────────────────────

async function sendHeartbeat(): Promise<void> {
  try {
    const result = await callCentral("POST", "/api/v1/heartbeat");
    if (result.status === 200) {
      const serverPolicyHash = result.data.policy_hash as string;

      // Check if policy needs sync
      if (serverPolicyHash && serverPolicyHash !== currentPolicyHash) {
        console.log(`[CLIENT] Policy drift detected (local: ${currentPolicyHash}, central: ${serverPolicyHash}). Syncing...`);
        await syncPolicy();
      }
    }
  } catch {
    // Silent — heartbeat failures are expected during network issues
  }
}

// ── Sync Loop ───────────────────────────────────────────────────────────────

async function syncLoop(): Promise<void> {
  await sendHeartbeat();
  await flushAuditEvents();
}

// ── Start Client Agent ──────────────────────────────────────────────────────

export async function startClientAgent(): Promise<void> {
  console.log(`[CLIENT] Enforcer Client Agent starting`);
  console.log(`[CLIENT] Client ID: ${CLIENT_ID} (policy: ${currentPolicyVersion || "none"})`);
  console.log(`[CLIENT] Central server: ${CENTRAL_URL}`);
  console.log(`[CLIENT] TLS: ${tlsOptions ? "enabled (mTLS)" : "disabled (plaintext)"}`);
  console.log(`[CLIENT] Sync interval: ${SYNC_INTERVAL_MS}ms`);

  // Register
  const registered = await registerWithCentral();
  if (!registered) {
    console.warn("[CLIENT] Running in standalone mode (central server unavailable)");
    console.warn("[CLIENT] Will retry connection on sync interval");
  }

  // Initial policy sync
  await syncPolicy();

  // Start sync loop
  syncTimer = setInterval(() => {
    syncLoop().catch(err => {
      console.error(`[CLIENT] Sync error: ${(err as Error).message}`);
    });
  }, SYNC_INTERVAL_MS);

  console.log(`[CLIENT] Client agent running. Sync every ${SYNC_INTERVAL_MS / 1000}s.`);
}

/**
 * Stop the client agent (for testing).
 */
export function stopClientAgent(): void {
  if (syncTimer) {
    clearInterval(syncTimer);
    syncTimer = null;
  }
}

// Run if executed directly
const isMain = process.argv[1]?.endsWith("client/agent.ts") || process.argv[1]?.endsWith("client/agent.js");
if (isMain) {
  startClientAgent();
}