/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Network Proxy
 *
 * HTTP CONNECT proxy that intercepts outbound HTTP(S) requests.
 * Evaluates destination host against policy before forwarding.
 *
 * On "deny": returns HTTP 403 with rationale.
 * On "allow": forwards the request and captures response status.
 *
 * TDD Reference: Section 6.1 (network egress interception)
 * PRD Reference: Appendix C, R-1
 */

import * as http from "node:http";
import * as net from "node:net";
import * as url from "node:url";
import { v4 as uuidv4 } from "uuid";
import type { ActionRequest } from "../../types/action.js";
import type { PolicyDecision } from "../../types/policy.js";

const PROXY_PORT = parseInt(process.env.PROXY_PORT || "9101", 10);
const DAEMON_URL = process.env.DAEMON_URL || "http://127.0.0.1:9100";

/**
 * Evaluate a network request against policy via the daemon.
 */
async function evaluateNetworkRequest(
  host: string,
  method: string,
  path: string,
  sessionId: string,
): Promise<PolicyDecision> {
  const request: ActionRequest = {
    request_id: uuidv4(),
    timestamp: new Date().toISOString(),
    actor: {
      user_id: process.env.USER || "unknown",
      agent_type: "claude_code",
      agent_instance: "network_proxy",
      session_id: sessionId,
    },
    environment: {
      workspace: process.env.AA_WORKSPACE || process.cwd(),
      repo: "",
      branch: "",
      tier: "development",
      deployment_mode: "host",
    },
    action: {
      type: "network.request",
      attempted_action: `${method} https://${host}${path}`,
    },
    resource: {
      kind: "host",
      host,
      value: `${method} https://${host}${path}`,
      classification: [],
    },
  };

  try {
    const response = await fetch(`${DAEMON_URL}/v1/evaluate`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(request),
    });

    if (!response.ok) {
      return {
        request_id: request.request_id,
        decision: "deny",
        reason_code: "DAEMON_ERROR",
        reason_human: `Daemon error (${response.status}). Fail-closed.`,
        policy_id: "system.fail_closed",
        policy_version: "unknown",
        approval_required: false,
      };
    }

    return response.json() as Promise<PolicyDecision>;
  } catch {
    return {
      request_id: uuidv4(),
      decision: "deny",
      reason_code: "DAEMON_UNREACHABLE",
      reason_human: "Daemon unreachable. Fail-closed: network request denied.",
      policy_id: "system.fail_closed",
      policy_version: "unknown",
      approval_required: false,
    };
  }
}

/**
 * Start the HTTP CONNECT proxy server.
 */
export function startNetworkProxy(): http.Server {
  const sessionId = `proxy_${uuidv4().slice(0, 8)}`;

  const server = http.createServer(async (req, res) => {
    // Handle regular HTTP requests (non-CONNECT)
    const parsedUrl = url.parse(req.url || "");
    const host = parsedUrl.hostname || req.headers.host || "unknown";

    const decision = await evaluateNetworkRequest(
      host,
      req.method || "GET",
      parsedUrl.path || "/",
      sessionId,
    );

    if (decision.decision === "deny") {
      res.writeHead(403, { "Content-Type": "application/json" });
      res.end(JSON.stringify({
        error: "Blocked by Enforcer",
        reason_code: decision.reason_code,
        reason_human: decision.reason_human,
        policy_id: decision.policy_id,
      }));
      return;
    }

    // Forward the request (simplified for prototype)
    res.writeHead(200, { "Content-Type": "text/plain" });
    res.end("Proxy forwarding not yet implemented for non-CONNECT requests");
  });

  // Handle CONNECT requests (HTTPS tunneling)
  server.on("connect", async (req, clientSocket, head) => {
    const [host, portStr] = (req.url || "").split(":");
    const port = parseInt(portStr || "443", 10);

    const decision = await evaluateNetworkRequest(
      host,
      "CONNECT",
      "/",
      sessionId,
    );

    if (decision.decision === "deny") {
      clientSocket.write(
        "HTTP/1.1 403 Forbidden\r\n" +
        "Content-Type: application/json\r\n\r\n" +
        JSON.stringify({
          error: "Blocked by Enforcer",
          reason_code: decision.reason_code,
          reason_human: decision.reason_human,
        }),
      );
      clientSocket.end();
      return;
    }

    // Forward CONNECT tunnel
    const serverSocket = net.connect(port, host, () => {
      clientSocket.write("HTTP/1.1 200 Connection Established\r\n\r\n");
      serverSocket.write(head);
      serverSocket.pipe(clientSocket);
      clientSocket.pipe(serverSocket);
    });

    serverSocket.on("error", () => {
      clientSocket.write("HTTP/1.1 502 Bad Gateway\r\n\r\n");
      clientSocket.end();
    });

    clientSocket.on("error", () => {
      serverSocket.end();
    });
  });

  server.listen(PROXY_PORT, "127.0.0.1", () => {
    console.log(`[PROXY] Enforcer network proxy listening on http://127.0.0.1:${PROXY_PORT}`);
  });

  return server;
}

// Run if executed directly
const isMain = process.argv[1] && (
  process.argv[1].endsWith("network-proxy.ts") ||
  process.argv[1].endsWith("network-proxy.js")
);
if (isMain) {
  startNetworkProxy();
}