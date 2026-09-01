/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Central Security Server
 *
 * The central server that IT admins use to:
 *   - Author and version policy bundles
 *   - Distribute signed policies to client agents over mTLS
 *   - Aggregate audit events from all client agents
 *   - Serve the management console
 *   - Register and manage client agents
 *
 * Communication: HTTPS with mutual TLS (mTLS)
 *   - Server presents its certificate (clients verify against CA)
 *   - Clients present their certificate (server verifies against CA)
 *   - All traffic encrypted in transit
 *
 * Ports:
 *   - 9200: mTLS API (client agents connect here)
 *   - 9201: HTTPS management console (admins connect here)
 */

import * as https from "node:https";
import * as http from "node:http";
import * as fs from "node:fs";
import * as path from "node:path";
import * as crypto from "node:crypto";

const CENTRAL_PORT = parseInt(process.env.CENTRAL_PORT || "9200", 10);
const CONSOLE_API_PORT = parseInt(process.env.CONSOLE_API_PORT || "9201", 10);
const CERT_DIR = process.env.CERT_DIR || "/etc/enforcer/certs";

// ── State ───────────────────────────────────────────────────────────────────

interface RegisteredClient {
  client_id: string;
  hostname: string;
  ip_address: string;
  registered_at: string;
  last_heartbeat: string;
  policy_version: string;
  status: "active" | "stale" | "offline";
  governed_users: string[];
}

interface PolicyVersion {
  version: string;
  created_at: string;
  created_by: string;
  bundle_hash: string;
  bundle_json: string;
}

const registeredClients = new Map<string, RegisteredClient>();
const policyVersions: PolicyVersion[] = [];
const aggregatedAuditEvents: unknown[] = [];

type UserRole = "none" | "operator" | "reviewer" | "admin";

interface AuthConfig {
  adminToken: string;
  reviewerToken: string;
  operatorToken: string;
}

function tokenFilePath(filename: string): string {
  const configDir = process.env.AA_CONFIG_DIR || "/etc/enforcer";
  return path.join(configDir, filename);
}

function loadToken(envVar: string, filePath: string): string {
  const envToken = process.env[envVar]?.trim();
  if (envToken) return envToken;
  try {
    return fs.readFileSync(filePath, "utf-8").trim();
  } catch {
    return "";
  }
}

const authConfig: AuthConfig = {
  adminToken: loadToken("AA_ADMIN_TOKEN", tokenFilePath(".admin_token")),
  reviewerToken: loadToken("AA_REVIEWER_TOKEN", tokenFilePath(".reviewer_token")),
  operatorToken: loadToken("AA_OPERATOR_TOKEN", tokenFilePath(".operator_token")),
};

function extractAccessToken(req: http.IncomingMessage): string {
  const authHeader = req.headers.authorization;
  if (authHeader?.startsWith("Bearer ")) {
    return authHeader.slice(7).trim();
  }
  const xAAToken = req.headers["x-aa-token"];
  if (typeof xAAToken === "string" && xAAToken.trim()) return xAAToken.trim();
  const xAdminToken = req.headers["x-admin-token"];
  if (typeof xAdminToken === "string" && xAdminToken.trim()) return xAdminToken.trim();
  const reqUrl = req.url || "";
  const queryStart = reqUrl.indexOf("?");
  if (queryStart >= 0) {
    const params = new URLSearchParams(reqUrl.slice(queryStart));
    return params.get("access_token")?.trim() || params.get("admin_token")?.trim() || "";
  }
  return "";
}

function authenticateRole(req: http.IncomingMessage): UserRole {
  const token = extractAccessToken(req);
  if (!token) return "none";
  if (authConfig.adminToken && token === authConfig.adminToken) return "admin";
  if (authConfig.reviewerToken && token === authConfig.reviewerToken) return "reviewer";
  if (authConfig.operatorToken && token === authConfig.operatorToken) return "operator";
  return "none";
}

function roleCapabilities(role: UserRole): Record<string, boolean> {
  return {
    view: role === "operator" || role === "reviewer" || role === "admin",
    approve: role === "reviewer" || role === "admin",
    toggle_enforcement: role === "admin",
    manage_policy: role === "admin",
  };
}

// ── Load initial policy ─────────────────────────────────────────────────────

function loadCurrentPolicy(): string {
  const policyDir = process.env.AA_POLICY_DIR || path.resolve(process.cwd(), "policies");
  try {
    const defaultYaml = fs.readFileSync(path.join(policyDir, "default.yaml"), "utf-8");
    return defaultYaml;
  } catch {
    return "# No policy loaded";
  }
}

let currentPolicyBundle = loadCurrentPolicy();
let currentPolicyVersion = `v${new Date().toISOString().slice(0, 10).replace(/-/g, ".")}.1`;
let currentPolicyHash = crypto.createHash("sha256").update(currentPolicyBundle).digest("hex").slice(0, 16);

// ── TLS Setup ───────────────────────────────────────────────────────────────

function loadTlsOptions(): { key: Buffer; cert: Buffer; ca: Buffer } | null {
  try {
    return {
      key: fs.readFileSync(path.join(CERT_DIR, "server-key.pem")),
      cert: fs.readFileSync(path.join(CERT_DIR, "server.pem")),
      ca: fs.readFileSync(path.join(CERT_DIR, "ca.pem")),
    };
  } catch (err) {
    console.warn(`[CENTRAL] TLS certificates not found at ${CERT_DIR}. Running in plaintext mode.`);
    console.warn("[CENTRAL] Run: sudo ./scripts/generate-certs.sh to generate certificates.");
    return null;
  }
}

// ── Request Helpers ─────────────────────────────────────────────────────────

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
    "Access-Control-Allow-Headers": "Content-Type, X-Admin-Token, X-Client-Id",
  });
  res.end(json);
}

// ── Client Agent API (mTLS, port 9200) ──────────────────────────────────────

async function handleClientRequest(
  req: http.IncomingMessage,
  res: http.ServerResponse,
): Promise<void> {
  const url = req.url || "/";
  const method = req.method || "GET";

  if (method === "OPTIONS") {
    res.writeHead(204, {
      "Access-Control-Allow-Origin": "*",
      "Access-Control-Allow-Methods": "GET, POST, OPTIONS",
      "Access-Control-Allow-Headers": "Content-Type, X-Client-Id",
    });
    res.end();
    return;
  }

  const clientId = req.headers["x-client-id"] as string || "unknown";

  // Register / heartbeat
  if (url === "/api/v1/register" && method === "POST") {
    const body = await readBody(req);
    let parsed: { hostname?: string; governed_users?: string[] } = {};
    try { parsed = JSON.parse(body); } catch { /* ignore */ }

    const ip = req.socket.remoteAddress || "unknown";
    registeredClients.set(clientId, {
      client_id: clientId,
      hostname: parsed.hostname || "unknown",
      ip_address: ip,
      registered_at: registeredClients.get(clientId)?.registered_at || new Date().toISOString(),
      last_heartbeat: new Date().toISOString(),
      policy_version: currentPolicyVersion,
      status: "active",
      governed_users: parsed.governed_users || [],
    });

    console.log(`[CENTRAL] Client registered: ${clientId} from ${ip} (${parsed.hostname})`);
    sendJson(res, 200, { status: "registered", client_id: clientId });
    return;
  }

  // Pull policy bundle
  if (url === "/api/v1/policy/pull" && method === "GET") {
    const client = registeredClients.get(clientId);
    if (client) {
      client.last_heartbeat = new Date().toISOString();
      client.status = "active";
    }

    sendJson(res, 200, {
      version: currentPolicyVersion,
      hash: currentPolicyHash,
      bundle: currentPolicyBundle,
      signed_at: new Date().toISOString(),
    });
    return;
  }

  // Push audit events
  if (url === "/api/v1/audit/push" && method === "POST") {
    const body = await readBody(req);
    let events: unknown[] = [];
    try {
      const parsed = JSON.parse(body);
      events = Array.isArray(parsed.events) ? parsed.events : [parsed];
    } catch { /* ignore */ }

    aggregatedAuditEvents.push(...events);

    const client = registeredClients.get(clientId);
    if (client) {
      client.last_heartbeat = new Date().toISOString();
      client.status = "active";
    }

    console.log(`[CENTRAL] Received ${events.length} audit events from ${clientId}`);
    sendJson(res, 200, { received: events.length, total: aggregatedAuditEvents.length });
    return;
  }

  // Heartbeat
  if (url === "/api/v1/heartbeat" && method === "POST") {
    const client = registeredClients.get(clientId);
    if (client) {
      client.last_heartbeat = new Date().toISOString();
      client.status = "active";
    }
    sendJson(res, 200, {
      status: "ok",
      policy_version: currentPolicyVersion,
      policy_hash: currentPolicyHash,
      enforcement_enabled: true,
    });
    return;
  }

  sendJson(res, 404, { error: "Not found" });
}

// ── Admin API (port 9201) ───────────────────────────────────────────────────

async function handleAdminRequest(
  req: http.IncomingMessage,
  res: http.ServerResponse,
): Promise<void> {
  const url = req.url || "/";
  const method = req.method || "GET";

  if (method === "OPTIONS") {
    res.writeHead(204, {
      "Access-Control-Allow-Origin": "*",
      "Access-Control-Allow-Methods": "GET, POST, PUT, DELETE, OPTIONS",
      "Access-Control-Allow-Headers": "Content-Type, X-Admin-Token",
    });
    res.end();
    return;
  }

  // Health
  if (url === "/api/v1/health") {
    sendJson(res, 200, {
      status: "ok",
      type: "central_server",
      clients: registeredClients.size,
      policy_version: currentPolicyVersion,
      audit_events: aggregatedAuditEvents.length,
    });
    return;
  }

  // Auth introspection
  if (url === "/api/v1/auth/me" && method === "GET") {
    const role = authenticateRole(req);
    if (role === "none") {
      sendJson(res, 401, {
        error: "unauthorized",
        message: "Valid access token required. Provide via Authorization: Bearer, X-AA-Token/X-Admin-Token header, or access_token query parameter.",
      });
      return;
    }
    sendJson(res, 200, {
      role,
      capabilities: roleCapabilities(role),
    });
    return;
  }

  // List registered clients
  if (url === "/api/v1/clients" && method === "GET") {
    const clients = Array.from(registeredClients.values()).map(c => {
      const staleSecs = (Date.now() - new Date(c.last_heartbeat).getTime()) / 1000;
      return { ...c, status: staleSecs > 120 ? "stale" as const : staleSecs > 300 ? "offline" as const : "active" as const };
    });
    sendJson(res, 200, { clients, count: clients.length });
    return;
  }

  // Get current policy
  if (url === "/api/v1/policy" && method === "GET") {
    sendJson(res, 200, {
      version: currentPolicyVersion,
      hash: currentPolicyHash,
      bundle: currentPolicyBundle,
    });
    return;
  }

  // Update policy (push to all clients on next pull)
  if (url === "/api/v1/policy" && method === "PUT") {
    const body = await readBody(req);
    let parsed: { bundle?: string; version?: string } = {};
    try { parsed = JSON.parse(body); } catch { /* ignore */ }

    if (!parsed.bundle) {
      sendJson(res, 400, { error: "bundle is required" });
      return;
    }

    currentPolicyBundle = parsed.bundle;
    currentPolicyVersion = parsed.version || `v${new Date().toISOString().slice(0, 10).replace(/-/g, ".")}.${policyVersions.length + 1}`;
    currentPolicyHash = crypto.createHash("sha256").update(currentPolicyBundle).digest("hex").slice(0, 16);

    policyVersions.push({
      version: currentPolicyVersion,
      created_at: new Date().toISOString(),
      created_by: "admin",
      bundle_hash: currentPolicyHash,
      bundle_json: currentPolicyBundle,
    });

    console.log(`[CENTRAL] Policy updated to ${currentPolicyVersion} (hash: ${currentPolicyHash})`);
    sendJson(res, 200, {
      message: "Policy updated. Clients will pull on next sync.",
      version: currentPolicyVersion,
      hash: currentPolicyHash,
    });
    return;
  }

  // Get aggregated audit events
  if (url?.startsWith("/api/v1/audit") && method === "GET") {
    const limit = 100;
    const recent = aggregatedAuditEvents.slice(-limit).reverse();
    sendJson(res, 200, { events: recent, total: aggregatedAuditEvents.length });
    return;
  }

  sendJson(res, 404, { error: "Not found" });
}

// ── Start Servers ───────────────────────────────────────────────────────────

export function startCentralServer(): void {
  const tlsOpts = loadTlsOptions();

  // Client API (mTLS if certs available, plain HTTP fallback)
  if (tlsOpts) {
    const clientServer = https.createServer(
      {
        key: tlsOpts.key,
        cert: tlsOpts.cert,
        ca: tlsOpts.ca,
        requestCert: true,          // Require client certificate
        rejectUnauthorized: true,    // Reject clients without valid cert
      },
      (req, res) => {
        handleClientRequest(req, res).catch(err => {
          console.error("[CENTRAL] Client API error:", err);
          sendJson(res, 500, { error: "Internal error" });
        });
      },
    );

    clientServer.listen(CENTRAL_PORT, () => {
      console.log(`[CENTRAL] Client API (mTLS) listening on https://0.0.0.0:${CENTRAL_PORT}`);
      console.log(`[CENTRAL] Clients must present a valid certificate signed by ${CERT_DIR}/ca.pem`);
    });
  } else {
    const clientServer = http.createServer((req, res) => {
      handleClientRequest(req, res).catch(err => {
        console.error("[CENTRAL] Client API error:", err);
        sendJson(res, 500, { error: "Internal error" });
      });
    });

    clientServer.listen(CENTRAL_PORT, () => {
      console.log(`[CENTRAL] Client API (PLAINTEXT — no TLS) listening on http://0.0.0.0:${CENTRAL_PORT}`);
      console.warn("[CENTRAL] WARNING: Running without TLS. Generate certs for production.");
    });
  }

  // Admin API (always HTTP for now — behind reverse proxy in production)
  const adminServer = http.createServer((req, res) => {
    handleAdminRequest(req, res).catch(err => {
      console.error("[CENTRAL] Admin API error:", err);
      sendJson(res, 500, { error: "Internal error" });
    });
  });

  adminServer.listen(CONSOLE_API_PORT, () => {
    console.log(`[CENTRAL] Admin API listening on http://0.0.0.0:${CONSOLE_API_PORT}`);
  });

  console.log(`[CENTRAL] Enforcer Central Security Server started`);
  console.log(`[CENTRAL] Policy: ${currentPolicyVersion} (hash: ${currentPolicyHash})`);
}

// Run if executed directly
const isMain = process.argv[1]?.endsWith("central/server.ts") || process.argv[1]?.endsWith("central/server.js");
if (isMain) {
  startCentralServer();
}
