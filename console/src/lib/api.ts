/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — API Client for Management Console
 *
 * Fetch helpers for daemon endpoints.
 * Authenticated endpoints require an access token via Authorization Bearer
 * and X-AA-Token headers. Legacy X-Admin-Token is still sent for compatibility.
 * Daemon runs on localhost:9100 by default.
 */

import { isHubMode } from "./console-mode";

const DAEMON_URL = process.env.NEXT_PUBLIC_DAEMON_URL || "http://127.0.0.1:9100";
const HUB_API_URL = process.env.NEXT_PUBLIC_HUB_API_URL || "";

function endpoint(path: string): string {
  if (isHubMode()) {
    if (HUB_API_URL) {
      return `${HUB_API_URL}${path}`;
    }
    return `/api${path}`;
  }
  return `${DAEMON_URL}${path}`;
}

function buildHeaders(token?: string | null): Record<string, string> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (token) {
    headers.Authorization = `Bearer ${token}`;
    headers["X-AA-Token"] = token;
    headers["X-Admin-Token"] = token;
  }
  return headers;
}

async function fetchApi<T>(path: string, token?: string | null): Promise<T> {
  const res = await fetch(endpoint(path), {
    cache: "no-store",
    headers: buildHeaders(token),
  });
  if (!res.ok) {
    throw new Error(`API error: ${res.status} ${res.statusText}`);
  }
  return res.json() as Promise<T>;
}

async function postApi<T>(path: string, body: unknown, token?: string | null): Promise<T> {
  const res = await fetch(endpoint(path), {
    method: "POST",
    headers: buildHeaders(token),
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    throw new Error(`API error: ${res.status} ${res.statusText}`);
  }
  return res.json() as Promise<T>;
}

// ── Health (no auth required) ───────────────────────────────────────────────

export async function getHealth() {
  return fetchApi<{ status: string; policy_version: string; policy_rules: number; enforcement?: { enabled: boolean; since: string; changed_by: string }; governed_user?: string }>("/v1/health");
}

export async function getEnforcementState() {
  return fetchApi<{ enabled: boolean; since: string; changed_by: string }>("/v1/enforcement");
}

// ── Enforcement Toggle (admin auth required) ────────────────────────────────

export async function toggleEnforcement(enabled: boolean, changedBy: string, token: string | null) {
  return postApi<{ enabled: boolean }>("/v1/enforcement/toggle", { enabled, changed_by: changedBy }, token);
}

// ── Audit Events (admin auth required) ──────────────────────────────────────

export interface AuditEvent {
  event_id: string;
  timestamp: string;
  session_id: string;
  who: string;
  what: string;
  when: string;
  policy: string;
  decision: string;
  result: string;
  actor: { user_id: string; agent_type: string; agent_instance: string };
  environment: { workspace: string; repo: string; branch: string; tier: string; deployment_mode: string };
  action: { type: string; attempted_action: string; observed_effect: string };
  resource: { kind: string; path?: string; host?: string; value?: string; classification: string[] };
  policy_detail: { policy_id: string; policy_version: string; decision: string; reason_code: string; reason_human: string };
  approval?: { status: string; approver_id?: string; rationale?: string; is_break_glass?: boolean };
}

export interface SessionSummary {
  session_id: string;
  user_id: string;
  agent_type: string;
  agent_instance: string;
  event_count: number;
  first_event: string;
  last_event: string;
  decisions: Record<string, number>;
}

export async function getSessions(token: string | null) {
  return fetchApi<{ sessions: SessionSummary[]; count: number }>("/v1/audit/sessions", token);
}

export async function getSession(sessionId: string, token: string | null) {
  return fetchApi<{ session_id: string; event_count: number; events: AuditEvent[] }>(`/v1/audit/sessions/${sessionId}`, token);
}

export async function queryEvents(params: Record<string, string> = {}, token: string | null) {
  const query = new URLSearchParams(params).toString();
  return fetchApi<{ events: AuditEvent[]; count: number }>(`/v1/audit/events?${query}`, token);
}

export async function exportEvents(params: Record<string, string> = {}, token: string | null) {
  const query = new URLSearchParams(params).toString();
  return fetchApi<{ metadata: { exported_at: string; total_events: number }; events: AuditEvent[] }>(`/v1/audit/export?${query}`, token);
}

// ── Approvals (operator/reviewer/admin) ─────────────────────────────────────

export interface PendingApproval {
  approval_id: string;
  request_id: string;
  context_bundle: {
    actor: string;
    resource: string;
    risk_rationale: string;
    policy_rule: string;
    agent_identity: string;
    session_summary: string;
  };
  timeout_seconds: number;
  created_at: string;
}

export async function getPendingApprovals(token: string | null) {
  return fetchApi<{ approvals: PendingApproval[]; count: number }>("/v1/approvals/pending", token);
}

export async function resolveApproval(approvalId: string, body: {
  decision: "approve" | "deny";
  approver_id: string;
  rationale?: string;
}, token: string | null) {
  return postApi<unknown>(`/v1/approvals/${approvalId}/resolve`, body, token);
}

// ── Metrics (admin auth required) ───────────────────────────────────────────

export interface DaemonMetrics {
  policy_version: string;
  policy_rules: number;
  buffer: { accepted: number; rejected: number; backpressureAlerts: number; bufferCount: number };
}

export async function getMetrics(token: string | null) {
  return fetchApi<DaemonMetrics>("/v1/metrics", token);
}

export async function getAuditMetrics(token: string | null) {
  return fetchApi<{ total_events: number; session_count: number; decision_counts: Record<string, number> }>("/v1/audit/metrics", token);
}

export async function getApprovalMetrics(token: string | null) {
  return fetchApi<{
    total_created: number;
    total_approved: number;
    total_denied: number;
    total_expired: number;
    pending_count: number;
  }>("/v1/approvals/metrics", token);
}

// ── Policy (reads are public, writes are admin) ────────────────────────────

export async function getPolicyRules(token: string | null) {
  return fetchApi<{ rules: unknown[]; count: number }>("/v1/policy/rules", token);
}

export async function addPolicyRule(rule: unknown, token: string | null) {
  return postApi<unknown>("/v1/policy/rules", rule, token);
}

export async function togglePolicyRule(ruleId: string, token: string | null) {
  return postApi<unknown>(`/v1/policy/rules/${ruleId}/toggle`, {}, token);
}

export async function deletePolicyRule(ruleId: string, token: string | null) {
  const res = await fetch(endpoint(`/v1/policy/rules/${ruleId}`), {
    method: "DELETE",
    headers: buildHeaders(token),
  });
  return res.json();
}

export async function getPolicyPacks(token: string | null) {
  return fetchApi<{ packs: unknown[] }>("/v1/policy/packs", token);
}

export async function getPolicyPack(packId: string, token: string | null) {
  return fetchApi<unknown>(`/v1/policy/packs/${packId}`, token);
}

export async function applyPolicyPack(packId: string, token: string | null) {
  return postApi<unknown>(`/v1/policy/packs/${packId}/apply`, {}, token);
}

export async function createPolicyPack(pack: unknown, token: string | null) {
  return postApi<unknown>("/v1/policy/packs", pack, token);
}

// ── Token Validation (uses an authenticated endpoint to verify) ─────────────

export async function validateAdminToken(adminToken: string): Promise<boolean> {
  try {
    const res = await fetch(endpoint("/v1/auth/me"), {
      headers: buildHeaders(adminToken),
    });
    return res.ok;
  } catch {
    return false;
  }
}

export async function getAuthMe(token: string) {
  return fetchApi<{ role: "admin" | "reviewer" | "operator"; capabilities: Record<string, boolean> }>("/v1/auth/me", token);
}

// ── Analytics (admin auth required) ─────────────────────────────────

export async function getBlockedOperations(period: string, token: string | null) {
  const data = await fetchApi<{
    operations?: Array<{ action_type: string; reason_code: string; policy_id: string; count: number; trend: number; top_developers: string[] }>;
    blocked_operations?: Array<{ action_type: string; reason_code: string; policy_id: string; count: number; trend: number; top_developers: string[] }>;
    period: string;
  }>(`/v1/analytics/blocked-operations?period=${period}`, token);
  return {
    operations: data.operations ?? data.blocked_operations ?? [],
    period: data.period,
  };
}

export async function getApprovalBottlenecks(token: string | null) {
  return fetchApi<{ bottlenecks: Array<{ action_type: string; policy_id: string; avg_wait_seconds: number; pending_count: number; reason_code: string }> }>("/v1/analytics/approval-bottlenecks", token);
}

export async function getDeveloperImpact(period: string, token: string | null) {
  return fetchApi<{ developers: Array<{ user_id: string; agent_type: string; total_actions: number; blocked_actions: number; block_rate: number; group: string; top_block_reasons: string[] }>; period: string }>(`/v1/analytics/developer-impact?period=${period}`, token);
}

export async function getAnalyticsGroups(token: string | null) {
  return fetchApi<{ groups: Array<{ id: string; name: string; icon: string; description: string; member_count: number; avg_block_rate: number; suggested_action: string }> }>("/v1/analytics/groups", token);
}

export async function getGroupMembers(groupId: string, token: string | null) {
  return fetchApi<{ group_id: string; members: Array<{ user_id: string; actions_per_day: number; block_rate: number; top_action_type: string }> }>(`/v1/analytics/groups/${groupId}/members`, token);
}

export async function getRecommendations(token: string | null) {
  return fetchApi<{ recommendations: Array<{ id: string; title: string; description: string; impact: string; risk: string; target_group: string; status: string }> }>("/v1/analytics/recommendations", token);
}

export async function applyRecommendation(recId: string, token: string | null) {
  return postApi<{ applied: boolean }>(`/v1/analytics/recommendations/${recId}/apply`, { confirm: true }, token);
}

export async function getDeveloperScorecard(userId: string, token: string | null) {
  return fetchApi<{ user_id: string; group: string; compliance_score: number; org_avg_compliance: number; total_actions: number; blocked_actions: number; approved_actions: number; block_rate: number; trend: string; tips: string[]; weekly_summary: string }>(`/v1/analytics/developer/${userId}`, token);
}
