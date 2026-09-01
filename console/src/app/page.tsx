/**
 * Author: Deepankar Das
 */

"use client";

import { useEffect, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { MetricCard } from "@/components/metric-card";
import { Activity, Shield, ShieldOff, CheckCircle, Clock, Power, User, Users } from "lucide-react";
import { PieChart, Pie, Cell, ResponsiveContainer, BarChart, Bar, XAxis, YAxis, Tooltip } from "recharts";
import { useAuth } from "@/lib/auth-context";
import { getHealth, getAuditMetrics, getApprovalMetrics, getSessions, toggleEnforcement, getBlockedOperations, queryEvents } from "@/lib/api";

interface GovernedUser {
  user_id: string;
  agent_type: string;
  session_count: number;
  last_active: string;
  total_events: number;
  blocks: number;
}

interface BlockedOp {
  action_type: string;
  reason_code: string;
  policy_id: string;
  count: number;
  top_developers?: string[];
}

interface DashboardData {
  totalEvents: number;
  totalBlocked: number;
  totalApproved: number;
  totalAllowed: number;
  policyVersion: string;
  policyRules: number;
  auditCompleteness: string;
  governedUsers: GovernedUser[];
  blockedOps: BlockedOp[];
}

const DECISION_COLORS: Record<string, string> = {
  Allowed: "#10b981",
  Blocked: "#ef4444",
  Approved: "#f59e0b",
};

const DECISION_FILTER: Record<string, string> = {
  Allowed: "allow",
  Blocked: "deny",
  Approved: "require_approval",
};

const BAR_COLOR = "#06b6d4";

// Compute blocked operations breakdown from individual deny events.
function computeBlockedOpsFromEvents(events: Array<{ actor?: { user_id?: string }; action?: { type?: string }; policy_detail?: { reason_code?: string; policy_id?: string } }>): BlockedOp[] {
  const counts = new Map<string, { reason_code: string; policy_id: string; count: number; developers: Set<string> }>();
  for (const evt of events) {
    const actionType = evt.action?.type || "unknown";
    const userId = evt.actor?.user_id || "unknown";
    const existing = counts.get(actionType);
    if (existing) {
      existing.count++;
      existing.developers.add(userId);
    } else {
      counts.set(actionType, {
        reason_code: evt.policy_detail?.reason_code || "",
        policy_id: evt.policy_detail?.policy_id || "",
        count: 1,
        developers: new Set([userId]),
      });
    }
  }
  return Array.from(counts.entries())
    .map(([action_type, v]) => ({ action_type, reason_code: v.reason_code, policy_id: v.policy_id, count: v.count, top_developers: Array.from(v.developers) }))
    .sort((a, b) => b.count - a.count)
    .slice(0, 5);
}

function AgentRow({ name, supported, connected, sessions }: { name: string; supported: boolean; connected: boolean; sessions: number }) {
  const [showComingSoon, setShowComingSoon] = useState(false);
  const isActive = supported && connected && sessions > 0;
  const isOn = supported && connected;

  return (
    <div className="flex items-center justify-between py-1">
      <div className="flex items-center gap-2">
        <div className={`h-2 w-2 rounded-full ${isActive ? "bg-emerald-400" : isOn ? "bg-amber-400" : "bg-slate-600"}`} />
        <span className={`text-sm ${supported ? "text-slate-300" : "text-slate-500"}`}>{name}</span>
      </div>
      <div className="flex items-center gap-2 relative">
        {sessions > 0 && <span className="text-xs text-slate-500">{sessions} sessions</span>}
        {supported ? (
          <span className={`text-[10px] px-1.5 py-0.5 rounded ${isActive ? "bg-emerald-900/50 text-emerald-400" : isOn ? "bg-amber-900/50 text-amber-400" : "bg-red-900/50 text-red-400"}`}>
            {isActive ? "Active" : isOn ? "On" : "Offline"}
          </span>
        ) : (
          <>
            <button
              onClick={() => { setShowComingSoon(true); setTimeout(() => setShowComingSoon(false), 2000); }}
              className="text-[10px] px-1.5 py-0.5 rounded bg-slate-800 text-slate-500 hover:bg-slate-700 hover:text-slate-400 transition-colors cursor-pointer"
            >
              Off
            </button>
            {showComingSoon && (
              <div className="absolute right-0 top-6 bg-slate-800 border border-slate-700 rounded px-2 py-1 text-[10px] text-amber-400 whitespace-nowrap z-10 shadow-lg">
                Coming soon
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}

export default function DashboardPage() {
  const router = useRouter();
  const { token, role } = useAuth();
  const isHubUser = role === "admin" || role === "reviewer";
  // Restore cached state on mount (fixes browser back button showing empty data)
  const [data, setData] = useState<DashboardData | null>(() => {
    if (typeof window === "undefined") return null;
    try { const cached = sessionStorage.getItem("aa_dashboard"); return cached ? JSON.parse(cached) : null; } catch { return null; }
  });
  const [error, setError] = useState<string | null>(null);
  const [daemonStatus, setDaemonStatus] = useState<"connected" | "disconnected">(() => {
    if (typeof window === "undefined") return "disconnected";
    return (sessionStorage.getItem("aa_daemon_status") as "connected" | "disconnected") || "disconnected";
  });
  const [connectedSince, setConnectedSince] = useState<string | null>(() => {
    if (typeof window === "undefined") return null;
    return sessionStorage.getItem("aa_connected_since");
  });
  const [firewallEnabled, setFirewallEnabled] = useState(true);

  useEffect(() => {
    async function fetchDashboard() {
      try {
        // Fetch health first to get governed_user for Sentinel filtering.
        const health = await getHealth();
        const govUser = (!isHubUser && health.governed_user) ? health.governed_user : "";
        const denyParams: Record<string, string> = { decision: "deny", limit: "500" };
        const approvalParams: Record<string, string> = { decision: "require_approval", limit: "500" };
        if (govUser) {
          denyParams.actor_user_id = govUser;
          approvalParams.actor_user_id = govUser;
        }

        const [auditMetrics, approvalMetrics, sessionsData, blockedData, denyEvents, approvalEvents] = await Promise.all([
          getAuditMetrics(token).catch(() => null),
          getApprovalMetrics(token).catch(() => null),
          getSessions(token).catch(() => ({ sessions: [] as never[], count: 0 })),
          getBlockedOperations("7d", token).catch(() => ({ operations: [] })),
          queryEvents(denyParams, token).catch(() => ({ events: [] })),
          queryEvents(approvalParams, token).catch(() => ({ events: [] })),
        ]);

        setDaemonStatus("connected");
        // Read enforcement state from daemon
        if (health.enforcement) {
          setFirewallEnabled(health.enforcement.enabled);
          setConnectedSince(health.enforcement.since);
        } else if (!connectedSince) {
          setConnectedSince(new Date().toISOString());
        }
        // Audit completeness: events that passed schema validation vs total attempted.
        // The API returns total_events (stored) at the top level, not nested under store.
        const totalStored = auditMetrics?.total_events || 0;
        const totalRejected = 0; // Rejected events never reach PostgreSQL

        // On the Sentinel Console, filter to only the governed developer's data.
        const allSessions = sessionsData.sessions || [];
        const filteredSessions = govUser
          ? allSessions.filter((s: { user_id?: string }) => s.user_id === govUser)
          : allSessions;

        // Build governed users from sessions
        let governedUsers: GovernedUser[] = [];
        {
          const userMap = new Map<string, GovernedUser>();
          for (const s of filteredSessions) {
            const uid = s.user_id || "unknown";
            const existing = userMap.get(uid);
            const blocks = (s.decisions?.deny || 0);
            const totalEvts = s.event_count || 0;
            if (existing) {
              existing.session_count++;
              existing.total_events += totalEvts;
              existing.blocks += blocks;
              if (s.last_event > existing.last_active) {
                existing.last_active = s.last_event;
                existing.agent_type = s.agent_type || existing.agent_type;
              }
            } else {
              userMap.set(uid, {
                user_id: uid,
                agent_type: s.agent_type || "unknown",
                session_count: 1,
                last_active: s.last_event || new Date().toISOString(),
                total_events: totalEvts,
                blocks,
              });
            }
          }
          governedUsers = Array.from(userMap.values());
        }

        // Compute totals. On the Hub, use PostgreSQL-aggregated decision_counts
        // (exact). On the Sentinel (govUser set), use filtered event queries.
        const sessionTotalEvents = filteredSessions.reduce((sum: number, s: { event_count?: number }) => sum + (s.event_count || 0), 0);
        const dc = auditMetrics?.decision_counts || {};
        const totalEvents = govUser ? sessionTotalEvents : (auditMetrics?.total_events || sessionTotalEvents);
        const totalBlocked = govUser ? (denyEvents.events || []).length : (dc.deny || 0);
        const totalApproved = govUser ? (approvalEvents.events || []).length : (dc.require_approval || 0);
        const totalAllowed = govUser
          ? (totalEvents - totalBlocked - totalApproved)
          : (dc.allow || Math.max(totalEvents - totalBlocked - totalApproved, 0));

        // Compute blocked ops. On Sentinel, always use deny events (scoped to
        // governed user). On Hub, prefer analytics endpoint (aggregated).
        const blockedOps = govUser
          ? computeBlockedOpsFromEvents(denyEvents.events || [])
          : (blockedData.operations || []).length > 0
            ? (blockedData.operations || []).slice(0, 5)
            : computeBlockedOpsFromEvents(denyEvents.events || []);

        setData({
          totalEvents,
          totalBlocked,
          totalApproved,
          totalAllowed: Math.max(totalAllowed, 0),
          policyVersion: health.policy_version || "none",
          policyRules: health.policy_rules || 0,
          auditCompleteness: totalRejected === 0 && totalStored === 0 ? "--" : totalRejected === 0 ? "100%" : `${((1 - totalRejected / (totalStored + totalRejected)) * 100).toFixed(1)}%`,
          governedUsers,
          blockedOps,
        });
        setError(null);

        // Cache to sessionStorage so browser back button shows data immediately
        try {
          sessionStorage.setItem("aa_dashboard", JSON.stringify({
            totalEvents,
            totalBlocked,
            totalApproved,
            totalAllowed: Math.max(totalAllowed, 0),
            policyVersion: health.policy_version || "none",
            policyRules: health.policy_rules || 0,
            auditCompleteness: totalRejected === 0 && totalStored === 0 ? "--" : totalRejected === 0 ? "100%" : `${((1 - totalRejected / (totalStored + totalRejected)) * 100).toFixed(1)}%`,
            governedUsers,
            blockedOps,
          }));
          sessionStorage.setItem("aa_daemon_status", "connected");
          if (connectedSince) sessionStorage.setItem("aa_connected_since", connectedSince);
        } catch { /* sessionStorage full or unavailable — ignore */ }
      } catch {
        setDaemonStatus("disconnected");
        setError("Cannot connect to Enforcer daemon. Start it with: ./scripts/deploy.sh");
      }
    }

    fetchDashboard();
    const interval = setInterval(fetchDashboard, 5000);
    return () => clearInterval(interval);
  }, [token]);

  return (
    <div>
      {/* Header with On/Off switch */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-xl font-bold text-slate-100">Dashboard</h1>
          <p className="text-sm text-slate-500">Runtime governance overview</p>
        </div>
        <div className="flex items-center gap-4">
          {isHubUser && (
            <>
              {/* Firewall On/Off toggle — calls daemon to actually toggle enforcement */}
              <div className="flex items-center gap-2">
                <button
                  onClick={async () => {
                    try {
                      const state = await toggleEnforcement(!firewallEnabled, "console_admin", token);
                      setFirewallEnabled(state.enabled);
                    } catch { /* daemon unreachable */ }
                  }}
                  className={`relative w-12 h-6 rounded-full transition-colors ${firewallEnabled && daemonStatus === "connected" ? "bg-emerald-600" : "bg-slate-700"}`}
                >
                  <div className={`absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-white transition-transform ${firewallEnabled && daemonStatus === "connected" ? "translate-x-6" : ""}`} />
                </button>
                <div>
                  <div className={`text-sm font-bold ${firewallEnabled && daemonStatus === "connected" ? "text-emerald-400" : "text-slate-500"}`}>
                    {firewallEnabled && daemonStatus === "connected" ? "ACTIVE" : daemonStatus === "disconnected" ? "OFFLINE" : "PAUSED"}
                  </div>
                  <div className="text-[10px] text-slate-600">
                    {connectedSince
                      ? `Since ${new Date(connectedSince).toLocaleString()}`
                      : "Not connected"}
                  </div>
                </div>
              </div>
            </>
          )}
          {/* Daemon status dot */}
          <div className="flex items-center gap-1.5">
            <div className={`h-2 w-2 rounded-full ${daemonStatus === "connected" ? "bg-emerald-400" : "bg-red-400 animate-pulse"}`} />
            <span className="text-[10px] text-slate-600">
              Daemon {daemonStatus === "connected" ? "connected" : "disconnected"}
            </span>
          </div>
        </div>
      </div>

      {error && (
        <div className="bg-red-950/50 border border-red-900 rounded-lg p-4 mb-6 text-sm text-red-400">
          {error}
        </div>
      )}

      {!isHubUser && (
        <div className="bg-slate-900 border border-slate-800 rounded-lg p-3 mb-4">
          <div className="flex items-center gap-2 text-sm text-slate-400">
            <Shield className="h-4 w-4 text-cyan-400" />
            Viewing your governance activity on this machine.
          </div>
        </div>
      )}

      <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-3 mb-6">
        <Link href={isHubUser ? "/analytics" : "/search"}><MetricCard title="Actions Governed" value={data?.totalEvents ?? "--"} color="text-cyan-400" subtitle={isHubUser ? "View org analytics" : "Click to search all"} /></Link>
        <Link href="/search?decision=allow"><MetricCard title="Allowed" value={data?.totalAllowed ?? "--"} color="text-emerald-400" subtitle={isHubUser ? "View org analytics" : "Click to filter"} /></Link>
        <Link href="/search?decision=deny"><MetricCard title="Blocked" value={data?.totalBlocked ?? "--"} color="text-red-400" subtitle={isHubUser ? "View org analytics" : "Click to filter"} /></Link>
        <Link href={isHubUser ? "/approvals" : "/search?decision=require_approval"}><MetricCard title="Approved" value={data?.totalApproved ?? "--"} color="text-amber-400" subtitle={isHubUser ? "Review approvals" : "Click to filter"} /></Link>
        <Link href={isHubUser ? "/analytics" : "/search"}><MetricCard title="Audit Completeness" value={data?.auditCompleteness ?? "--"} color="text-emerald-400" subtitle="Schema gate pass rate" /></Link>
        <Link href={isHubUser ? "/policies" : "/sessions"}><MetricCard title="Policy" value={data?.policyVersion ?? "--"} color="text-blue-400" subtitle={`${data?.policyRules ?? 0} rules${isHubUser ? " — click to manage" : ""}`} /></Link>
      </div>

      {/* Decision Distribution + Top Blocked Operations Charts */}
      {data && (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-6">
          {/* Decision Distribution Donut */}
          <div className="bg-slate-900 border border-slate-800 rounded-lg p-4">
            <h2 className="text-xs uppercase tracking-wider text-slate-500 font-medium mb-3">Decision Distribution</h2>
            <div className="h-48">
              {data.totalEvents > 0 ? (
                <ResponsiveContainer width="100%" height="100%">
                  <PieChart>
                    <Pie
                      data={[
                        { name: "Allowed", value: Math.max(data.totalAllowed, 0) },
                        { name: "Blocked", value: data.totalBlocked },
                        { name: "Approved", value: data.totalApproved },
                      ].filter(d => d.value > 0)}
                      cx="50%"
                      cy="50%"
                      innerRadius={45}
                      outerRadius={70}
                      paddingAngle={2}
                      dataKey="value"
                      label={({ name, value }) => `${name}: ${value}`}
                      isAnimationActive={false}
                      style={{ cursor: "pointer" }}
                      onClick={(_, index) => {
                        const pieData = [
                          { name: "Allowed", value: Math.max(data.totalAllowed, 0) },
                          { name: "Blocked", value: data.totalBlocked },
                          { name: "Approved", value: data.totalApproved },
                        ].filter(d => d.value > 0);
                        const name = pieData[index]?.name || "";
                        const filter = DECISION_FILTER[name];
                        if (filter) router.push(`/search?decision=${filter}`);
                      }}
                    >
                      {[
                        { name: "Allowed", value: Math.max(data.totalAllowed, 0) },
                        { name: "Blocked", value: data.totalBlocked },
                        { name: "Approved", value: data.totalApproved },
                      ].filter(d => d.value > 0).map((entry) => (
                        <Cell key={entry.name} fill={DECISION_COLORS[entry.name] || "#64748b"} />
                      ))}
                    </Pie>
                    <Tooltip contentStyle={{ backgroundColor: "#1e293b", border: "1px solid #334155", color: "#e2e8f0" }} />
                  </PieChart>
                </ResponsiveContainer>
              ) : (
                <div className="h-full flex items-center justify-center text-sm text-slate-500">No events yet</div>
              )}
            </div>
          </div>

          {/* Top Blocked Operations Bar Chart */}
          <div className="bg-slate-900 border border-slate-800 rounded-lg p-4">
            <h2 className="text-xs uppercase tracking-wider text-slate-500 font-medium mb-3">Top Blocked Operations (Last 7 Days)</h2>
            {data.blockedOps && data.blockedOps.length > 0 ? (
              <div className="h-48">
                <ResponsiveContainer width="100%" height="100%">
                  <BarChart data={data.blockedOps} layout="vertical" margin={{ left: 80 }}>
                    <XAxis type="number" tick={{ fill: "#94a3b8", fontSize: 10 }} />
                    <YAxis type="category" dataKey="action_type" tick={{ fill: "#94a3b8", fontSize: 10 }} width={75} />
                    <Tooltip
                      contentStyle={{ backgroundColor: "#1e293b", border: "1px solid #334155", color: "#e2e8f0" }}
                      formatter={(value, _name, props) => {
                        const op = data.blockedOps[(props as { index?: number })?.index ?? -1];
                        const devs = op?.top_developers?.length ? ` (${op.top_developers.join(", ")})` : "";
                        return [`${value ?? 0} blocks${devs}`, ""];
                      }}
                    />
                    <Bar
                      dataKey="count"
                      fill={BAR_COLOR}
                      radius={[0, 4, 4, 0]}
                      style={{ cursor: "pointer" }}
                      onClick={(entry) => {
                        const actionType = entry?.payload?.action_type;
                        if (typeof actionType === "string" && actionType.length > 0) {
                          router.push(`/search?action_type=${encodeURIComponent(actionType)}&decision=deny`);
                        }
                      }}
                    />
                  </BarChart>
                </ResponsiveContainer>
              </div>
            ) : (
              <div className="h-48 flex items-center justify-center text-sm text-slate-500">No blocked operations in the last 7 days</div>
            )}
          </div>
        </div>
      )}

      {/* Developer: "What's Blocking Me" — top reasons for denials */}
      {!isHubUser && data && data.blockedOps && data.blockedOps.length > 0 && (
        <div className="bg-slate-900 border border-rose-900/30 rounded-lg p-4 mb-4">
          <h2 className="text-xs uppercase tracking-wider text-rose-400 font-medium mb-2">What&apos;s Blocking You</h2>
          <div className="space-y-1.5">
            {data.blockedOps.slice(0, 3).map((op, i) => (
              <div key={i} className="flex items-center justify-between cursor-pointer hover:bg-slate-800/30 rounded px-2 py-1 -mx-2" onClick={() => router.push(`/search?action_type=${op.action_type}&decision=deny`)}>
                <div className="flex items-center gap-2">
                  <ShieldOff className="h-3.5 w-3.5 text-rose-400" />
                  <span className="text-xs text-slate-300">{op.action_type}</span>
                  <span className="text-[10px] text-slate-500 font-mono">{op.reason_code}</span>
                </div>
                <span className="text-xs text-rose-400 font-mono">{op.count}x</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Admin: Governance SLA Targets */}
      {isHubUser && (
        <div className="bg-slate-900 border border-slate-800 rounded-lg p-3 mb-4">
          <h2 className="text-xs uppercase tracking-wider text-slate-500 font-medium mb-2">Governance SLA Targets</h2>
          <div className="grid grid-cols-3 md:grid-cols-6 gap-2">
            {[
              { label: "Mediation", target: ">95%", icon: Activity },
              { label: "Fidelity", target: ">99%", icon: Shield },
              { label: "False Pos.", target: "<5%", icon: ShieldOff },
              { label: "Audit", target: ">99%", icon: CheckCircle },
              { label: "Schema", target: "100%", icon: CheckCircle },
              { label: "Approval", target: "<60s", icon: Clock },
            ].map(({ label, target, icon: Icon }) => (
              <div key={label} className="flex items-center gap-1.5 text-xs">
                <Icon className="h-3 w-3 text-slate-600" />
                <span className="text-slate-500">{label}</span>
                <span className="text-slate-400 font-mono">{target}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Governed Agents & Monitored Surfaces — visible to all roles */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-6">
        {/* Governed AI Agents */}
        <div className="bg-slate-900 border border-slate-800 rounded-lg p-4">
          <h2 className="text-xs uppercase tracking-wider text-slate-500 font-medium mb-3">Governed AI Agents</h2>
          <div className="space-y-2">
            <AgentRow name="Claude Code" supported={true} connected={daemonStatus === "connected"} sessions={data?.governedUsers?.reduce((a, u) => u.agent_type === "claude_code" ? a + u.session_count : a, 0) || 0} />
            <AgentRow name="Cursor" supported={false} connected={false} sessions={0} />
            <AgentRow name="Codex" supported={false} connected={false} sessions={0} />
            <AgentRow name="Copilot" supported={false} connected={false} sessions={0} />
          </div>
        </div>

        {/* Monitored Surfaces */}
        <div className="bg-slate-900 border border-slate-800 rounded-lg p-4">
          <h2 className="text-xs uppercase tracking-wider text-slate-500 font-medium mb-3">Monitored Surfaces</h2>
          <div className="space-y-2">
            {[
              { name: "File System", icon: "📁", actions: ["file.read", "file.write", "file.delete"], status: "active" },
              { name: "Shell Commands", icon: "💻", actions: ["shell.exec"], status: "active" },
              { name: "Network Egress", icon: "🌐", actions: ["network.request"], status: "active" },
              { name: "Package Installs", icon: "📦", actions: ["package.install"], status: "active" },
              { name: "Credentials", icon: "🔑", actions: ["credential.access"], status: "active" },
              { name: "MCP Tools", icon: "🔧", actions: ["mcp.invoke"], status: "active" },
            ].map(surface => (
              <a key={surface.name} href={`/search?action_type=${surface.actions[0]}`} className="flex items-center justify-between py-1 hover:bg-slate-800/30 rounded px-1 -mx-1 transition-colors">
                <div className="flex items-center gap-2">
                  <span className="text-sm">{surface.icon}</span>
                  <span className="text-sm text-slate-300">{surface.name}</span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="text-[10px] text-slate-600 font-mono">{surface.actions.join(", ")}</span>
                  <span className="text-[10px] px-1.5 py-0.5 rounded bg-emerald-900/50 text-emerald-400">Active</span>
                </div>
              </a>
            ))}
          </div>
        </div>
      </div>

      {/* Governed Users — Hub Console only (shows all developers across Sentinels) */}
      {isHubUser && (
      <div className="bg-slate-900 border border-slate-800 rounded-lg p-4 mb-6">
        <h2 className="text-xs uppercase tracking-wider text-slate-500 font-medium mb-3">
          <div className="flex items-center gap-1.5">
            <Users className="h-3.5 w-3.5" />
            Governed Users
          </div>
        </h2>
        {data?.governedUsers && data.governedUsers.length > 0 ? (
          <div className="space-y-2">
            {data.governedUsers.map(user => {
              const isRecent = (Date.now() - new Date(user.last_active).getTime()) < 5 * 60 * 1000;
              return (
                <Link key={user.user_id} href={`/search?actor_user_id=${encodeURIComponent(user.user_id)}`} className="flex items-center justify-between py-1.5 border-b border-slate-800/30 last:border-0 hover:bg-slate-800/30 rounded px-1 -mx-1 transition-colors">
                  <div className="flex items-center gap-2">
                    <div className={`h-2 w-2 rounded-full ${isRecent ? "bg-emerald-400" : "bg-slate-600"}`} />
                    <User className="h-3.5 w-3.5 text-slate-500" />
                    <span className="text-sm text-slate-300">{user.user_id}</span>
                    <span className="text-[10px] text-cyan-500 font-mono">{user.agent_type}</span>
                  </div>
                  <div className="flex items-center gap-3 text-xs">
                    <span className="text-slate-500">{user.session_count} session{user.session_count !== 1 ? "s" : ""}</span>
                    <span className="text-slate-500">{user.total_events} events</span>
                    {user.blocks > 0 && <span className="text-red-400">{user.blocks} blocked</span>}
                    <span className={`px-1.5 py-0.5 rounded text-[10px] ${isRecent ? "bg-emerald-900/50 text-emerald-400" : "bg-slate-800 text-slate-500"}`}>
                      {isRecent ? "Active" : `Last: ${new Date(user.last_active).toLocaleTimeString()}`}
                    </span>
                  </div>
                </Link>
              );
            })}
          </div>
        ) : (
          <div className="text-sm text-slate-500">No governed users yet.</div>
        )}
      </div>
      )}

      {/* Recent Activity */}
      <div className="bg-slate-900 border border-slate-800 rounded-lg p-4">
        <h2 className="text-xs uppercase tracking-wider text-slate-500 font-medium mb-3">Recent Activity</h2>
        {data && data.totalEvents > 0 ? (
          <div className="text-sm text-slate-400">
            {data.totalEvents} events across active sessions.
            <Link href="/sessions" className="text-cyan-400 ml-1 hover:underline">View sessions</Link>
          </div>
        ) : (
          <div className="text-sm text-slate-500">
            No events yet. Start a governed agent session to see activity here.
          </div>
        )}
      </div>
    </div>
  );
}
