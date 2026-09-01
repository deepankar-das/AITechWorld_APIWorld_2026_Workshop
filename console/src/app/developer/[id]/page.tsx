/**
 * Author: Deepankar Das
 */

"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { useAuth } from "@/lib/auth-context";
import { MetricCard } from "@/components/metric-card";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Lightbulb,
  TrendingUp,
  TrendingDown,
  Minus,
  Shield,
  User,
} from "lucide-react";
import { getDeveloperScorecard, getHealth } from "@/lib/api";

// ── Types ───────────────────────────────────────────────────────────

interface Scorecard {
  user_id: string;
  group: string;
  compliance_score: number;
  org_avg_compliance: number;
  total_actions: number;
  blocked_actions: number;
  approved_actions: number;
  block_rate: number;
  trend: string;
  tips: string[];
  weekly_summary: string;
}

// ── Helpers ─────────────────────────────────────────────────────────

function complianceColor(score: number, orgAvg: number): string {
  if (score >= orgAvg) return "text-emerald-400";
  if (score >= orgAvg - 5) return "text-amber-400";
  return "text-red-400";
}

function trendIcon(trend: string) {
  switch (trend) {
    case "improving":
      return <TrendingUp className="h-5 w-5 text-emerald-400" />;
    case "declining":
      return <TrendingDown className="h-5 w-5 text-red-400" />;
    default:
      return <Minus className="h-5 w-5 text-slate-400" />;
  }
}

function trendLabel(trend: string): { text: string; color: string } {
  switch (trend) {
    case "improving":
      return { text: "Improving", color: "text-emerald-400" };
    case "declining":
      return { text: "Declining", color: "text-red-400" };
    default:
      return { text: "Stable", color: "text-slate-400" };
  }
}

// ── Component ───────────────────────────────────────────────────────

export default function DeveloperScorecardPage() {
  const { token, username, role } = useAuth();
  const params = useParams<{ id: string }>();
  const isHubUser = role === "admin" || role === "reviewer";
  // For Sentinel: "me" queries all events (_all) since the Sentinel is a
  // single-developer machine. The display name comes from governed_user
  // (the OS username from the Claude Code session, e.g. "deepankardas").
  const resolvedUserId = params.id === "me"
    ? (isHubUser ? (username || "_all") : "_all")
    : params.id;
  const userId = resolvedUserId;

  const [data, setData] = useState<Scorecard | null>(null);
  const [governedUser, setGovernedUser] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!token || !userId) return;

    async function load() {
      // Fetch governed_user from health for display name
      try {
        const health = await getHealth();
        if (health.governed_user) setGovernedUser(health.governed_user);
      } catch { /* ignore */ }
      setLoading(true);
      setError(null);
      try {
        const scorecard = await getDeveloperScorecard(userId, token);
        setData(scorecard);
      } catch {
        setError("Failed to load developer scorecard. Ensure the daemon is running and the developer ID is valid.");
      } finally {
        setLoading(false);
      }
    }

    load();
  }, [token, userId]);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-slate-500 text-sm">Loading scorecard...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-red-950/50 border border-red-900 rounded-lg p-4 text-sm text-red-400">
        {error}
      </div>
    );
  }

  if (!data) {
    return (
      <div className="text-sm text-slate-500">No scorecard data available.</div>
    );
  }

  const trend = trendLabel(data.trend);

  return (
    <div>
      {/* Header */}
      <div className="mb-6">
        <div className="flex items-center gap-3 mb-2">
          <div className="h-10 w-10 rounded-full bg-slate-800 flex items-center justify-center">
            <User className="h-5 w-5 text-cyan-400" />
          </div>
          <div>
            <h1 className="text-xl font-bold text-slate-100">{data.user_id === "_all" ? (governedUser || "My Activity") : data.user_id}</h1>
            <div className="flex items-center gap-2">
              <span className="px-2 py-0.5 rounded text-xs bg-cyan-900/40 text-cyan-400">
                {data.group}
              </span>
            </div>
          </div>
        </div>
        <p className="text-sm text-slate-500">
          Developer awareness scorecard and compliance summary
        </p>
      </div>

      {/* Compliance Score */}
      <Card className="bg-slate-900 border-slate-800 mb-6">
        <CardContent className="pt-6">
          <div className="flex items-center justify-between">
            <div>
              <div className="text-xs uppercase tracking-wider text-slate-500 mb-1">
                Compliance Score
              </div>
              <div className={`text-5xl font-bold ${complianceColor(data.compliance_score, data.org_avg_compliance)}`}>
                {data.compliance_score}
              </div>
              <div className="text-xs text-slate-500 mt-1">
                Org average: <span className="text-slate-400">{data.org_avg_compliance}</span>
              </div>
            </div>
            <div className="text-right">
              <div className="text-xs uppercase tracking-wider text-slate-500 mb-2">
                Block Rate Trend
              </div>
              <div className="flex items-center gap-2 justify-end">
                {trendIcon(data.trend)}
                <span className={`text-sm font-medium ${trend.color}`}>
                  {trend.text}
                </span>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Action Metrics (clickable drill-downs) */}
      {(() => {
        const userParam = userId === "_all" ? "" : `&actor_user_id=${userId}`;
        return (
          <div className="grid grid-cols-1 md:grid-cols-3 gap-3 mb-6">
            <Link href={`/search?${userParam ? `actor_user_id=${userId}` : ""}`}>
              <MetricCard
                title="Total Actions"
                value={data.total_actions}
                color="text-cyan-400"
                subtitle="Click to view all events"
              />
            </Link>
            <Link href={`/search?decision=deny${userParam}`}>
              <MetricCard
                title="Blocked"
                value={data.blocked_actions}
                color="text-red-400"
                subtitle={`${data.block_rate.toFixed(1)}% block rate — click to view`}
              />
            </Link>
            <Link href={`/search?decision=require_approval${userParam}`}>
              <MetricCard
                title="Approved"
                value={data.approved_actions}
                color="text-amber-400"
                subtitle="Click to view approval events"
              />
            </Link>
          </div>
        );
      })()}

      {/* Tips */}
      <Card className="bg-slate-900 border-slate-800 mb-6">
        <CardHeader>
          <CardTitle className="text-sm text-slate-300 flex items-center gap-2">
            <Lightbulb className="h-4 w-4 text-amber-400" />
            Tips to Improve
          </CardTitle>
        </CardHeader>
        <CardContent>
          {data.tips.length === 0 ? (
            <div className="text-sm text-slate-500">No tips available. You are fully compliant.</div>
          ) : (
            <ul className="space-y-2">
              {data.tips.map((tip, idx) => (
                <li key={idx} className="flex items-start gap-2 text-sm text-slate-300">
                  <Lightbulb className="h-4 w-4 text-amber-400 shrink-0 mt-0.5" />
                  <span>{tip}</span>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>

      {/* Weekly Summary */}
      <Card className="bg-slate-900 border-slate-800">
        <CardHeader>
          <CardTitle className="text-sm text-slate-300 flex items-center gap-2">
            <Shield className="h-4 w-4 text-cyan-400" />
            Weekly Summary
          </CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-slate-400 leading-relaxed">
            {data.weekly_summary}
          </p>
        </CardContent>
      </Card>
    </div>
  );
}
