/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Admin Authentication
 *
 * Protects management endpoints from unauthorized access.
 * The admin token is generated during service installation and
 * stored in /etc/enforcer/.admin_token (readable only by root).
 *
 * Endpoints that require auth:
 *   - Policy management (CRUD, toggle, packs)
 *   - Enforcement toggle
 *   - Approval resolution
 *   - Audit export
 *   - Metrics
 *
 * Endpoints that do NOT require auth:
 *   - /v1/evaluate (enforcement path — must work from hooks)
 *   - /v1/health (monitoring)
 *   - /v1/audit/enrich (post-tool-call enrichment from hooks)
 */

import * as http from "node:http";
import * as fs from "node:fs";

const ADMIN_TOKEN = loadAdminToken();

function loadAdminToken(): string {
  // Priority: env var > file > default (for development)
  if (process.env.AA_ADMIN_TOKEN) {
    return process.env.AA_ADMIN_TOKEN;
  }

  // Try reading from protected file (production)
  const tokenFile = "/etc/enforcer/.admin_token";
  try {
    const token = fs.readFileSync(tokenFile, "utf-8").trim();
    if (token.length > 0) {
      console.log("[AUTH] Admin token loaded from", tokenFile);
      return token;
    }
  } catch {
    // File not readable — expected in development mode
  }

  // Development fallback — generate a random token for this session
  const devToken = `dev_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 10)}`;
  console.log(`[AUTH] No admin token configured. Development mode token: ${devToken}`);
  console.log("[AUTH] Set AA_ADMIN_TOKEN env var or install as system service for production.");
  return devToken;
}

/**
 * Check if a request has a valid admin token.
 *
 * Accepts the token via:
 *   - Authorization: Bearer <token> header
 *   - X-Admin-Token: <token> header
 *   - ?admin_token=<token> query parameter
 */
export function isAuthenticated(req: http.IncomingMessage): boolean {
  // Bearer token
  const authHeader = req.headers.authorization;
  if (authHeader?.startsWith("Bearer ")) {
    const token = authHeader.slice(7).trim();
    if (token === ADMIN_TOKEN) return true;
  }

  // X-Admin-Token header
  const tokenHeader = req.headers["x-admin-token"];
  if (typeof tokenHeader === "string" && tokenHeader === ADMIN_TOKEN) return true;

  // Query parameter (for browser access)
  const url = req.url || "";
  const queryStart = url.indexOf("?");
  if (queryStart >= 0) {
    const params = new URLSearchParams(url.slice(queryStart));
    const queryToken = params.get("admin_token");
    if (queryToken === ADMIN_TOKEN) return true;
  }

  return false;
}

/**
 * Get the current admin token (for displaying in install scripts).
 */
export function getAdminToken(): string {
  return ADMIN_TOKEN;
}

/**
 * Check if we're in development mode (no explicit admin token configured).
 */
export function isDevelopmentMode(): boolean {
  return ADMIN_TOKEN.startsWith("dev_");
}

/**
 * Send 401 Unauthorized response.
 */
export function sendUnauthorized(res: http.ServerResponse): void {
  const body = JSON.stringify({
    error: "Unauthorized",
    message: "Admin token required. Provide via Authorization: Bearer <token> header, X-Admin-Token header, or ?admin_token=<token> query parameter.",
  });
  res.writeHead(401, {
    "Content-Type": "application/json",
    "Content-Length": Buffer.byteLength(body),
    "Access-Control-Allow-Origin": "*",
    "WWW-Authenticate": "Bearer",
  });
  res.end(body);
}