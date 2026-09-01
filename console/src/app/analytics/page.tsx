/**
 * Author: Deepankar Das
 */

"use client";

import { useEffect, useState, useCallback } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useAuth } from "@/lib/auth-context";
import { MetricCard } from "@/components/metric-card";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  BarChart3,
  Users,
  Shield,
  TrendingUp,
  TrendingDown,
  Lightbulb,
  AlertTriangle,
  CheckCircle,
} from "lucide-react";
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
} from "recharts";
import {
  getBlockedOperations,
  getDeveloperImpact,
  getAnalyticsGroups,
  getRecommendations,
  applyRecommendation,
  getApprovalMetrics,
} from "@/lib/api";

// ── Types ───────────────────────────────────────────────────────────

interface BlockedOp {
  action_type: string;
  reason_code: string;
  policy_id: string;
  count: number;
  trend: number;
  top_developers: string[];
}

interface DevImpact {
  user_id: string;
  agent_type: string;
  total_actions: number;
  blocked_actions: number;
  block_rate: number;
  group: string;
  top_block_reasons: string[];
}

interface AnalyticsGroup {
  id: string;
  name: string;
  icon: string;
  description: string;
  member_count: number;
  avg_block_rate: number;
  suggested_action: string;
}

interface Recommendation {
  id: string;
  title: string;
  description: string;
  impact: string;
  risk: string;
  target_group: string;
  status: string;
}

// ── Helpers ─────────────────────────────────────────────────────────

const DONUT_COLORS = [
  "#06b6d4", // cyan-500
  "#8b5cf6", // violet-500
  "#f59e0b", // amber-500
  "#10b981", // emerald-500
  "#ef4444", // red-500
  "#3b82f6", // blue-500
  "#ec4899", // pink-500
];

function blockRateColor(rate: number): string {
  if (rate < 2) return "text-emerald-400";
  if (rate <= 5) return "text-amber-400";
  return "text-red-400";
}

function blockRateBg(rate: number): string {
  if (rate < 2) return "bg-emerald-900/40 text-emerald-400";
  if (rate <= 5) return "bg-amber-900/40 text-amber-400";
  return "bg-red-900/40 text-red-400";
}

function heatCellColor(count: number): string {
  if (count === 0) return "bg-emerald-900/40 text-emerald-300";
  if (count <= 10) return "bg-amber-900/40 text-amber-300";
  return "bg-red-900/40 text-red-300";
}

function complianceColor(score: number): string {
  if (score > 95) return "text-emerald-400";
  if (score > 90) return "text-amber-400";
  return "text-red-400";
}

// ── Component ───────────────────────────────────────────────────────

export default function AnalyticsPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { token, role } = useAuth();
  const isHubUser = role === "admin" || role === "reviewer";
  // Key metrics
  const [activeDevelopers, setActiveDevelopers] = useState<number | null>(null);
  const [orgCompliance, setOrgCompliance] = useState<number | null>(null);
  const [blocksToday, setBlocksToday] = useState<number | null>(null);
  const [pendingApprovals, setPendingApprovals] = useState<number | null>(null);

  // Section data
  const [blockedOps, setBlockedOps] = useState<BlockedOp[]>([]);
  const [blockedPeriod, setBlockedPeriod] = useState("today");
  const [groups, setGroups] = useState<AnalyticsGroup[]>([]);
  const [recommendations, setRecommendations] = useState<Recommendation[]>([]);
  const [devImpact, setDevImpact] = useState<DevImpact[]>([]);

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // ── Fetch blocked operations by period ────────────────────────────

  const fetchBlocked = useCallback(
    async (period: string) => {
      try {
        const data = await getBlockedOperations(period, token);
        setBlockedOps((data.operations || []).slice(0, 10));
      } catch {
        setBlockedOps([]);
      }
    },
    [token],
  );

  // ── Initial data load ─────────────────────────────────────────────

  useEffect(() => {
    if (!token || (role !== "admin" && role !== "reviewer" && role !== "operator")) return;

    async function load() {
      setLoading(true);
      setError(null);

      try {
        const [
          blockedRes,
          impactRes,
          groupsRes,
          recsRes,
          approvalRes,
        ] = await Promise.all([
          getBlockedOperations("today", token).catch(() => null),
          getDeveloperImpact("7d", token).catch(() => null),
          getAnalyticsGroups(token).catch(() => null),
          getRecommendations(token).catch(() => null),
          getApprovalMetrics(token).catch(() => null),
        ]);

        // Key metrics — all fields may be null from the API
        if (impactRes) {
          const devs = impactRes.developers || [];
          setActiveDevelopers(devs.length);
          const totalActions = devs.reduce((s: number, d: DevImpact) => s + d.total_actions, 0);
          const totalBlocked = devs.reduce((s: number, d: DevImpact) => s + d.blocked_actions, 0);
          setOrgCompliance(totalActions > 0 ? Math.round(((totalActions - totalBlocked) / totalActions) * 100) : 100);
          setDevImpact(devs);
        }

        if (blockedRes) {
          const ops = blockedRes.operations || [];
          setBlockedOps(ops.slice(0, 10));
          setBlocksToday(ops.reduce((s: number, o: BlockedOp) => s + o.count, 0));
        }

        if (groupsRes) setGroups(groupsRes.groups || []);
        if (recsRes) setRecommendations(recsRes.recommendations || []);
        if (approvalRes) setPendingApprovals(approvalRes.pending_count ?? 0);
      } catch {
        setError("Failed to load analytics data. Ensure the daemon is running.");
      } finally {
        setLoading(false);
      }
    }

    load();
  }, [token]);

  // ── Re-fetch blocked ops when period tab changes ──────────────────

  useEffect(() => {
    if (token) fetchBlocked(blockedPeriod);
  }, [blockedPeriod, token, fetchBlocked]);

  // ── Apply recommendation handler ─────────────────────────────────

  async function handleApply(recId: string) {
    try {
      await applyRecommendation(recId, token);
      setRecommendations((prev) =>
        prev.map((r) => (r.id === recId ? { ...r, status: "applied" } : r)),
      );
    } catch {
      // silently ignore; user can retry
    }
  }

  function handleDismiss(recId: string) {
    setRecommendations((prev) =>
      prev.map((r) => (r.id === recId ? { ...r, status: "dismissed" } : r)),
    );
  }

  function searchUrl(params: Record<string, string>): string {
    const query = new URLSearchParams(params).toString();
    return `/search${query ? `?${query}` : ""}`;
  }

  // Navigate to the Search page with filters applied.
  // The Search page has the full event table with drill-down to session detail.
  // Hierarchy: Analytics (aggregated) → Search (filtered events) → Session (detail)
  const openHubDrilldown = useCallback((
    params: Record<string, string>,
    _label: string,
    groupName?: string,
  ) => {
    const next = { ...params };
    if (groupName) next.group = groupName;
    router.push(searchUrl(next));
  }, [router]);

  // If analytics page is loaded with query params, redirect to Search page.
  useEffect(() => {
    if (!token || loading) return;
    const keys = ["session_id", "actor_user_id", "action_type", "decision", "policy_id", "reason_code", "group"];
    const params: Record<string, string> = {};
    for (const key of keys) {
      const value = searchParams.get(key);
      if (value) params[key] = value;
    }
    if (Object.keys(params).length > 0) {
      router.replace(searchUrl(params));
    }
  }, [token, loading, searchParams, router]);

  // ── Build friction heatmap data ───────────────────────────────────

  const policyIds = Array.from(new Set(blockedOps.map((o) => o.policy_id)));
  const groupNames = groups.map((g) => g.name);

  function heatmapCount(policyId: string, _groupName: string): number {
    // Build from blockedOps + devImpact cross-reference
    const op = blockedOps.find((o) => o.policy_id === policyId);
    if (!op) return 0;
    // Distribute proportionally across groups based on dev impact
    const groupDevs = devImpact.filter((d) => d.group === _groupName);
    const groupBlocked = groupDevs.reduce((s, d) => s + d.blocked_actions, 0);
    return groupBlocked > 0 ? Math.round((op.count * groupBlocked) / Math.max(1, devImpact.reduce((s, d) => s + d.blocked_actions, 0))) : 0;
  }

  // ── Render ────────────────────────────────────────────────────────

  // Analytics is available to all authenticated users.
  // On the Hub Console: org-wide analytics across all Sentinels.
  // On the Sentinel Console: personal analytics for this developer.

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-slate-500 text-sm">Loading analytics...</div>
      </div>
    );
  }

  return (
    <div>
      {/* Page header */}
      <div className="mb-3">
        <h1 className="text-lg font-bold text-slate-100 flex items-center gap-2">
          <BarChart3 className="h-4 w-4 text-cyan-400" />
          {isHubUser ? "Enterprise Analytics" : "My Analytics"}
        </h1>
        <p className="text-xs text-slate-500">
          {isHubUser
            ? "Org-wide governance insights across all developers and Sentinels"
            : "Your personal governance activity on this machine"}
        </p>
      </div>

      {error && (
        <div className="bg-red-950/50 border border-red-900 rounded-lg p-4 mb-6 text-sm text-red-400">
          {error}
        </div>
      )}

      {/* ── Section 1: Key Metrics Bar (all clickable) ──────────────── */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-2 mb-3">
        <div
          className="cursor-pointer"
          onClick={() =>
            isHubUser
              ? openHubDrilldown({}, "All governance events")
              : router.push("/sessions")
          }
        >
          <MetricCard
            title={isHubUser ? "Active Developers" : "My Sessions"}
            value={activeDevelopers ?? "--"}
            color="text-cyan-400"
            subtitle={isHubUser ? "Governed this period — click to view" : "Click to view sessions"}
          />
        </div>
        <div
          className="cursor-pointer"
          onClick={() =>
            isHubUser
              ? openHubDrilldown({ decision: "allow" }, "Allowed events (compliance)")
              : router.push("/developer/me")
          }
        >
          <MetricCard
            title={isHubUser ? "Org Compliance Rate" : "My Compliance Rate"}
            value={orgCompliance !== null ? `${orgCompliance}%` : "--"}
            color={orgCompliance !== null ? complianceColor(orgCompliance) : "text-slate-400"}
            subtitle="Click to view scorecard"
          />
        </div>
        <div
          className="cursor-pointer"
          onClick={() =>
            isHubUser
              ? openHubDrilldown({ decision: "deny" }, "Blocked events")
              : router.push("/search?decision=deny")
          }
        >
          <MetricCard
            title={isHubUser ? "Blocks Today (All)" : "My Blocks Today"}
            value={blocksToday ?? "--"}
            color="text-rose-400"
            subtitle="Click to view blocked events"
          />
        </div>
        <div className="cursor-pointer" onClick={() => router.push(isHubUser ? "/approvals" : "/search?decision=require_approval")}>
          <MetricCard
            title="Pending Approvals"
            value={pendingApprovals ?? "--"}
            color="text-amber-400"
            subtitle={isHubUser ? "Click to review" : "Click to view status"}
          />
        </div>
      </div>

      {/* ── Developer Summary (Hub only) ──────────────────────────────── */}
      {isHubUser && devImpact.length > 0 && (
        <div className="bg-slate-900 border border-slate-800 rounded-lg p-3 mb-3">
          <h2 className="text-xs uppercase tracking-wider text-slate-500 font-medium mb-2">Governed Developers</h2>
          <div className="flex flex-wrap gap-3">
            {devImpact.map((d) => (
              <div
                key={d.user_id}
                className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-slate-800/50 hover:bg-slate-800 cursor-pointer transition-colors"
                onClick={() =>
                  openHubDrilldown(
                    { actor_user_id: d.user_id },
                    `Events for ${d.user_id}`,
                  )
                }
              >
                <div className={`h-2 w-2 rounded-full ${d.block_rate > 5 ? "bg-rose-400" : d.block_rate > 0 ? "bg-amber-400" : "bg-emerald-400"}`} />
                <span className="text-sm text-slate-200">{d.user_id}</span>
                <span className="text-[10px] text-cyan-500 font-mono">{d.agent_type}</span>
                <span className="text-[10px] text-slate-500">{d.total_actions} actions</span>
                {d.blocked_actions > 0 && (
                  <span className="text-[10px] text-rose-400">{d.blocked_actions} blocked</span>
                )}
                <span className={`text-[10px] px-1.5 py-0.5 rounded ${d.block_rate > 5 ? "bg-rose-900/40 text-rose-400" : d.block_rate > 0 ? "bg-amber-900/40 text-amber-400" : "bg-emerald-900/40 text-emerald-400"}`}>
                  {d.block_rate.toFixed(1)}%
                </span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* ── Section 2: Blocked Operations + Section 3: Developer Groups (side by side on Hub) */}
      <div className={`grid grid-cols-1 ${isHubUser ? "lg:grid-cols-2" : ""} gap-3 mb-3`}>
      <Card className="bg-slate-900 border-slate-800">
        <CardHeader className="pb-2 pt-3 px-4">
          <CardTitle className="text-xs text-slate-300 flex items-center gap-2">
            <Shield className="h-3.5 w-3.5 text-rose-400" />
            {isHubUser ? "Blocked Operations — All Developers (Top 10)" : "My Blocked Operations (Top 10)"}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <Tabs
            defaultValue="today"
            onValueChange={(val: string | number | null) => {
              if (typeof val === "string") setBlockedPeriod(val);
            }}
          >
            <TabsList className="bg-slate-800 mb-4">
              <TabsTrigger value="today" className="text-xs data-[state=active]:bg-slate-700">
                Today
              </TabsTrigger>
              <TabsTrigger value="7d" className="text-xs data-[state=active]:bg-slate-700">
                7 Days
              </TabsTrigger>
              <TabsTrigger value="30d" className="text-xs data-[state=active]:bg-slate-700">
                30 Days
              </TabsTrigger>
            </TabsList>

            {["today", "7d", "30d"].map((period) => (
              <TabsContent key={period} value={period}>
                {blockedOps.length === 0 ? (
                  <div className="text-sm text-slate-500 py-8 text-center">
                    No blocked operations in this period.
                  </div>
                ) : (
                  <div className="h-44">
                    <ResponsiveContainer width="100%" height="100%">
                      <BarChart
                        layout="vertical"
                        data={blockedOps.map((op) => ({
                          name: `${op.action_type} / ${op.reason_code}`,
                          count: op.count,
                          trend: op.trend,
                          actionType: op.action_type,
                          reasonCode: op.reason_code,
                          policyId: op.policy_id,
                        }))}
                        margin={{ top: 0, right: 20, left: 0, bottom: 0 }}
                      >
                        <XAxis type="number" tick={{ fill: "#64748b", fontSize: 10 }} />
                        <YAxis
                          type="category"
                          dataKey="name"
                          width={180}
                          tick={{ fill: "#94a3b8", fontSize: 10 }}
                        />
                        <Tooltip
                          contentStyle={{
                            backgroundColor: "#1e293b",
                            border: "1px solid #334155",
                            borderRadius: "8px",
                            color: "#e2e8f0",
                            fontSize: 12,
                          }}
                          formatter={(value, _name, props) => {
                            const op = blockedOps[(props as { index?: number })?.index ?? -1];
                            const devs = op?.top_developers?.length ? ` (${op.top_developers.join(", ")})` : "";
                            return [`${value ?? 0} blocks${devs}`, ""];
                          }}
                        />
                        <Bar
                          dataKey="count"
                          radius={[0, 4, 4, 0]}
                          style={{ cursor: "pointer" }}
                          onClick={(entry) => {
                            const actionType = entry?.payload?.actionType;
                            const reasonCode = entry?.payload?.reasonCode;
                            const policyId = entry?.payload?.policyId;
                            if (typeof actionType === "string" && actionType.length > 0) {
                              const params: Record<string, string> = {
                                action_type: actionType,
                                decision: "deny",
                              };
                              if (typeof reasonCode === "string" && reasonCode.length > 0) params.reason_code = reasonCode;
                              if (typeof policyId === "string" && policyId.length > 0) params.policy_id = policyId;
                              void openHubDrilldown(
                                params,
                                `Blocked ${actionType} events`,
                              );
                            }
                          }}
                        >
                          {blockedOps.map((op, idx) => (
                            <Cell
                              key={idx}
                              fill={op.reason_code.includes("approval") ? "#d4915e" : "#be5c6e"}
                            />
                          ))}
                        </Bar>
                      </BarChart>
                    </ResponsiveContainer>
                  </div>
                )}

              </TabsContent>
            ))}
          </Tabs>
        </CardContent>
      </Card>

      {/* ── Section 3: Developer Groups (Hub only — org-wide concept) ── */}
      {isHubUser && <Card className="bg-slate-900 border-slate-800">
        <CardHeader className="pb-2 pt-3 px-4">
          <CardTitle className="text-xs text-slate-300 flex items-center gap-2">
            <Users className="h-3.5 w-3.5 text-cyan-400" />
            Developer Groups
          </CardTitle>
        </CardHeader>
        <CardContent className="px-4 pb-3">
          <div className="flex gap-4">
            {/* Donut chart */}
            <div className="flex items-center justify-center w-1/3">
              {groups.length === 0 ? (
                <div className="text-xs text-slate-500">No group data.</div>
              ) : (
                <ResponsiveContainer width="100%" height={160}>
                  <PieChart>
                    <Pie
                      data={groups.filter(g => g.member_count > 0).map((g) => ({
                        name: g.name,
                        value: g.member_count,
                      }))}
                      dataKey="value"
                      nameKey="name"
                      cx="50%"
                      cy="50%"
                      innerRadius={35}
                      outerRadius={60}
                      paddingAngle={2}
                      isAnimationActive={false}
                      style={{ cursor: "pointer" }}
                      onClick={(entry) => {
                        const groupName = entry?.name;
                        const clicked = groups.find((g) => g.name === groupName && g.member_count > 0);
                        if (clicked) {
                          void openHubDrilldown(
                            { decision: "deny" },
                            `Blocked events for ${clicked.name}`,
                            clicked.name,
                          );
                        }
                      }}
                    >
                      {groups.map((_, idx) => (
                        <Cell
                          key={idx}
                          fill={DONUT_COLORS[idx % DONUT_COLORS.length]}
                        />
                      ))}
                    </Pie>
                    <Tooltip
                      contentStyle={{
                        backgroundColor: "#1e293b",
                        border: "1px solid #334155",
                        borderRadius: "8px",
                        color: "#e2e8f0",
                        fontSize: 12,
                      }}
                      formatter={(value) => [String(value ?? 0), "Members"]}
                    />
                  </PieChart>
                </ResponsiveContainer>
              )}
            </div>

            {/* Groups list */}
            <div className="overflow-y-auto w-2/3 max-h-40">
              {groups.filter(g => g.member_count > 0).length === 0 ? (
                <div className="text-xs text-slate-500">No active groups.</div>
              ) : (
                <div className="space-y-1">
                  {groups.filter(g => g.member_count > 0).map((g) => (
                    <div
                      key={g.id}
                      className="flex items-center justify-between py-1 px-2 rounded hover:bg-slate-800/30 cursor-pointer text-xs"
                      onClick={() =>
                        openHubDrilldown(
                          { decision: "deny" },
                          `Blocked events for ${g.name}`,
                          g.name,
                        )
                      }
                    >
                      <div className="flex items-center gap-2">
                        <span>{g.icon}</span>
                        <span className="text-slate-300">{g.name}</span>
                      </div>
                      <div className="flex items-center gap-2">
                        <span className="text-slate-500">{g.member_count}</span>
                        <span className={`px-1.5 py-0.5 rounded text-[10px] ${blockRateBg(g.avg_block_rate)}`}>
                          {g.avg_block_rate.toFixed(1)}%
                        </span>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </CardContent>
      </Card>}
      </div> {/* Close the side-by-side grid */}

      {/* ── Section 4 + 5: Recommendations + Heatmap (Hub only — org-wide) ── */}
      {isHubUser && (
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-3 mb-3">
      <Card className="bg-slate-900 border-slate-800">
        <CardHeader className="pb-2 pt-3 px-4">
          <CardTitle className="text-xs text-slate-300 flex items-center gap-2">
            <Lightbulb className="h-3.5 w-3.5 text-amber-400" />
            Policy Recommendations
          </CardTitle>
        </CardHeader>
        <CardContent>
          {recommendations.length === 0 ? (
            <div className="text-sm text-slate-500 py-4 text-center">
              No recommendations at this time.
            </div>
          ) : (
            <div className="space-y-2 max-h-48 overflow-y-auto">
              {recommendations.map((rec) => (
                <div
                  key={rec.id}
                  className={`border rounded-lg p-3 transition-colors ${
                    rec.status === "applied"
                      ? "border-emerald-800 bg-emerald-950/20"
                      : rec.status === "dismissed"
                        ? "border-slate-800 bg-slate-950/50 opacity-50"
                        : "border-slate-700 bg-slate-800/30"
                  }`}
                >
                  <div className="flex items-start justify-between mb-2">
                    <h3 className="text-sm font-medium text-slate-200 flex items-center gap-1.5">
                      {rec.status === "applied" ? (
                        <CheckCircle className="h-4 w-4 text-emerald-400 shrink-0" />
                      ) : (
                        <Lightbulb className="h-4 w-4 text-amber-400 shrink-0" />
                      )}
                      {rec.title}
                    </h3>
                  </div>
                  <p className="text-xs text-slate-400 mb-3">{rec.description}</p>
                  <div className="flex flex-wrap gap-2 mb-3">
                    <span className="px-1.5 py-0.5 rounded text-[10px] bg-cyan-900/40 text-cyan-400">
                      Impact: {rec.impact}
                    </span>
                    <span className={`px-1.5 py-0.5 rounded text-[10px] ${
                      rec.risk === "high"
                        ? "bg-red-900/40 text-red-400"
                        : rec.risk === "medium"
                          ? "bg-amber-900/40 text-amber-400"
                          : "bg-emerald-900/40 text-emerald-400"
                    }`}>
                      Risk: {rec.risk}
                    </span>
                    <span className="px-1.5 py-0.5 rounded text-[10px] bg-slate-700 text-slate-400">
                      Group: {rec.target_group}
                    </span>
                  </div>
                  {rec.status !== "applied" && rec.status !== "dismissed" && (
                    <div className="flex gap-2">
                      <button
                        onClick={() => handleApply(rec.id)}
                        className="px-3 py-1 text-xs rounded bg-cyan-600 hover:bg-cyan-500 text-white transition-colors"
                      >
                        Apply
                      </button>
                      <button
                        onClick={() => handleDismiss(rec.id)}
                        className="px-3 py-1 text-xs rounded bg-slate-700 hover:bg-slate-600 text-slate-300 transition-colors"
                      >
                        Dismiss
                      </button>
                    </div>
                  )}
                  {rec.status === "applied" && (
                    <div className="text-xs text-emerald-400 flex items-center gap-1">
                      <CheckCircle className="h-3 w-3" />
                      Applied
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <Card className="bg-slate-900 border-slate-800">
        <CardHeader className="pb-2 pt-3 px-4">
          <CardTitle className="text-xs text-slate-300 flex items-center gap-2">
            <AlertTriangle className="h-3.5 w-3.5 text-amber-400" />
            Friction Heatmap (Policies vs Groups)
          </CardTitle>
        </CardHeader>
        <CardContent>
          {policyIds.length === 0 || groupNames.length === 0 ? (
            <div className="text-sm text-slate-500 py-4 text-center">
              Insufficient data to render heatmap. Requires blocked operations and developer groups.
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-xs border-collapse">
                <thead>
                  <tr>
                    <th className="text-left text-slate-500 uppercase tracking-wider pb-2 pr-3 font-medium">
                      Policy
                    </th>
                    {groupNames.map((gn) => (
                      <th
                        key={gn}
                        className="text-center text-slate-500 uppercase tracking-wider pb-2 px-2 font-medium"
                      >
                        {gn}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {policyIds.map((pid) => (
                    <tr key={pid} className="border-t border-slate-800/50">
                      <td className="py-1.5 pr-3 text-slate-400 font-mono whitespace-nowrap">
                        {pid}
                      </td>
                      {groupNames.map((gn) => {
                        const count = heatmapCount(pid, gn);
                        return (
                          <td key={gn} className="py-1.5 px-2 text-center">
                            <span
                              className={`inline-block px-2 py-0.5 rounded text-[11px] font-mono cursor-pointer hover:ring-1 hover:ring-slate-500 ${heatCellColor(count)}`}
                              onClick={() => {
                                if (count > 0) {
                                  void openHubDrilldown(
                                    { decision: "deny", policy_id: pid },
                                    `Blocked events for ${pid} / ${gn}`,
                                    gn,
                                  );
                                }
                              }}
                            >
                              {count}
                            </span>
                          </td>
                        );
                      })}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>
      </div>
      )}
    </div>
  );
}
