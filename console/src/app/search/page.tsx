/**
 * Author: Deepankar Das
 */

"use client";

import { useState, useEffect } from "react";
import { useSearchParams } from "next/navigation";
import { DecisionBadge } from "@/components/decision-badge";
import { ActionIcon } from "@/components/action-icon";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Search, ArrowLeft } from "lucide-react";
import Link from "next/link";
import { useAuth } from "@/lib/auth-context";
import { getDeveloperImpact, queryEvents } from "@/lib/api";
import type { AuditEvent } from "@/lib/api";

export default function SearchPage() {
  const { token } = useAuth();
  const searchParams = useSearchParams();
  const [sessionId, setSessionId] = useState("");
  const [actionType, setActionType] = useState("");
  const [decision, setDecision] = useState("");
  const [actorUserId, setActorUserId] = useState("");
  const [policyId, setPolicyId] = useState("");
  const [reasonCode, setReasonCode] = useState("");
  const [group, setGroup] = useState("");
  const [results, setResults] = useState<AuditEvent[]>([]);
  const [loading, setLoading] = useState(false);
  const [searched, setSearched] = useState(false);

  // Read URL params on mount (for drill-down from dashboard)
  useEffect(() => {
    const urlDecision = searchParams.get("decision");
    const urlAction = searchParams.get("action_type");
    const urlSession = searchParams.get("session_id");
    const urlActor = searchParams.get("actor_user_id");
    const urlPolicy = searchParams.get("policy_id");
    const urlReason = searchParams.get("reason_code");
    const urlGroup = searchParams.get("group");

    setDecision(urlDecision || "");
    setActionType(urlAction || "");
    setSessionId(urlSession || "");
    setActorUserId(urlActor || "");
    setPolicyId(urlPolicy || "");
    setReasonCode(urlReason || "");
    setGroup(urlGroup || "");

    // Always auto-search on page load (show all events by default).
    // URL params pre-fill the filters for drill-down from dashboard.
    executeSearch({
      session_id: urlSession || undefined,
      action_type: urlAction || undefined,
      decision: urlDecision || undefined,
      actor_user_id: urlActor || undefined,
      policy_id: urlPolicy || undefined,
      reason_code: urlReason || undefined,
    });
  }, [searchParams]);

  async function executeSearch(overrides?: {
    session_id?: string;
    action_type?: string;
    decision?: string;
    actor_user_id?: string;
    policy_id?: string;
    reason_code?: string;
  }) {
    setLoading(true);
    setSearched(true);
    try {
      const queryParams: Record<string, string> = {};
      const sid = overrides?.session_id ?? sessionId;
      const act = overrides?.action_type ?? actionType;
      const dec = overrides?.decision ?? decision;
      const uid = overrides?.actor_user_id ?? actorUserId;
      const pid = overrides?.policy_id ?? policyId;
      const reason = overrides?.reason_code ?? reasonCode;
      if (sid) queryParams.session_id = sid;
      if (act) queryParams.action_type = act;
      if (dec) queryParams.decision = dec;
      if (uid) queryParams.actor_user_id = uid;
      if (pid) queryParams.policy_id = pid;
      if (reason) queryParams.reason_code = reason;
      queryParams.limit = "1000";

      const data = await queryEvents(queryParams, token);
      let events = data.events || [];

      // Group filter is context-only in drilldown URLs; map it to users client-side.
      if (group) {
        try {
          const impact = await getDeveloperImpact("7d", token);
          const members = new Set(
            (impact.developers || [])
              .filter((d) => d.group === group)
              .map((d) => d.user_id),
          );
          if (members.size > 0) {
            events = events.filter((event) => members.has(event.actor?.user_id || ""));
          }
        } catch {
          // Ignore group mapping errors and show base query results.
        }
      }

      setResults(events);
    } catch {
      setResults([]);
    }
    setLoading(false);
  }

  function handleSearch() {
    executeSearch();
  }

  return (
    <div>
      <div className="flex items-center gap-3 mb-6">
        <Link href="/" className="text-slate-500 hover:text-slate-300 transition-colors">
          <ArrowLeft className="h-5 w-5" />
        </Link>
        <div>
          <h1 className="text-xl font-bold text-slate-100">Search</h1>
          <p className="text-sm text-slate-500">Search audit events across all sessions</p>
        </div>
      </div>

      {/* Search filters */}
      <div className="bg-slate-900 border border-slate-800 rounded-lg p-4 mb-6">
        <div className="grid grid-cols-1 md:grid-cols-7 gap-3">
          <Input
            placeholder="Session ID"
            value={sessionId}
            onChange={e => setSessionId(e.target.value)}
            className="bg-slate-950 border-slate-700 text-slate-300 placeholder:text-slate-600 text-sm font-mono"
          />
          <Input
            placeholder="User ID"
            value={actorUserId}
            onChange={e => setActorUserId(e.target.value)}
            className="bg-slate-950 border-slate-700 text-slate-300 placeholder:text-slate-600 text-sm font-mono"
          />
          <select
            value={actionType}
            onChange={e => setActionType(e.target.value)}
            className="bg-slate-950 border border-slate-700 rounded-md px-3 py-2 text-sm text-slate-300"
          >
            <option value="">All action types</option>
            <option value="file.read">file.read</option>
            <option value="file.write">file.write</option>
            <option value="file.delete">file.delete</option>
            <option value="file.move">file.move</option>
            <option value="shell.exec">shell.exec</option>
            <option value="network.request">network.request</option>
            <option value="package.install">package.install</option>
            <option value="credential.access">credential.access</option>
            <option value="mcp.invoke">mcp.invoke</option>
            <option value="internal.orchestration">internal.orchestration</option>
          </select>
          <select
            value={decision}
            onChange={e => setDecision(e.target.value)}
            className="bg-slate-950 border border-slate-700 rounded-md px-3 py-2 text-sm text-slate-300"
          >
            <option value="">All decisions</option>
            <option value="allow">allow</option>
            <option value="deny">deny</option>
            <option value="require_approval">require_approval</option>
          </select>
          <Input
            placeholder="Policy ID"
            value={policyId}
            onChange={e => setPolicyId(e.target.value)}
            className="bg-slate-950 border-slate-700 text-slate-300 placeholder:text-slate-600 text-sm font-mono"
          />
          <Input
            placeholder="Reason Code"
            value={reasonCode}
            onChange={e => setReasonCode(e.target.value)}
            className="bg-slate-950 border-slate-700 text-slate-300 placeholder:text-slate-600 text-sm font-mono"
          />
          <Button
            onClick={handleSearch}
            className="bg-cyan-900 hover:bg-cyan-800 text-cyan-300"
          >
            <Search className="h-4 w-4 mr-2" />
            Search
          </Button>
        </div>
        {group && (
          <div className="mt-2 text-xs text-slate-500">
            Group context: <span className="font-mono text-slate-400">{group}</span>
          </div>
        )}
      </div>

      {/* Results */}
      {loading ? (
        <div className="text-slate-500 text-sm">Searching...</div>
      ) : !searched ? (
        <div className="bg-slate-900 border border-slate-800 rounded-lg p-8 text-center text-slate-500 text-sm">
          Enter filters and click Search to find audit events.
        </div>
      ) : results.length === 0 ? (
        <div className="bg-slate-900 border border-slate-800 rounded-lg p-8 text-center text-slate-500 text-sm">
          No events match your search criteria.
        </div>
      ) : (
        <div className="bg-slate-900 border border-slate-800 rounded-lg overflow-hidden">
          <div className="px-4 py-2 border-b border-slate-800 text-xs text-slate-500">
            {results.length} events found
          </div>
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-slate-800">
                <th className="text-left px-4 py-2 text-[11px] uppercase tracking-wider text-slate-500 font-medium">Time</th>
                <th className="text-left px-4 py-2 text-[11px] uppercase tracking-wider text-slate-500 font-medium">User</th>
                <th className="text-left px-4 py-2 text-[11px] uppercase tracking-wider text-slate-500 font-medium">Session</th>
                <th className="text-left px-4 py-2 text-[11px] uppercase tracking-wider text-slate-500 font-medium">Action</th>
                <th className="text-left px-4 py-2 text-[11px] uppercase tracking-wider text-slate-500 font-medium">Resource</th>
                <th className="text-left px-4 py-2 text-[11px] uppercase tracking-wider text-slate-500 font-medium">Decision</th>
                <th className="text-left px-4 py-2 text-[11px] uppercase tracking-wider text-slate-500 font-medium">Policy</th>
                <th className="text-left px-4 py-2 text-[11px] uppercase tracking-wider text-slate-500 font-medium">Effect</th>
              </tr>
            </thead>
            <tbody>
              {results.map(event => (
                <tr key={event.event_id} className="border-b border-slate-800/50 hover:bg-slate-800/30">
                  <td className="px-4 py-2 text-xs text-slate-400 font-mono whitespace-nowrap">{new Date(event.timestamp).toLocaleTimeString()}</td>
                  <td className="px-4 py-2 text-xs text-slate-300">{event.actor?.user_id || "—"}</td>
                  <td className="px-4 py-2">
                    <Link href={`/sessions/${event.session_id}`} className="text-xs text-cyan-400 font-mono hover:underline truncate max-w-[100px] inline-block">
                      {event.session_id || "—"}
                    </Link>
                  </td>
                  <td className="px-4 py-2">
                    <div className="flex items-center gap-1.5">
                      <ActionIcon actionType={event.action.type} className="h-3.5 w-3.5 text-slate-400" />
                      <span className="text-xs text-slate-300">{event.action.type}</span>
                    </div>
                  </td>
                  <td className="px-4 py-2 text-xs text-slate-300 font-mono truncate max-w-[200px]">
                    {event.resource?.value || event.resource?.path || event.resource?.host || "—"}
                  </td>
                  <td className="px-4 py-2"><DecisionBadge decision={event.policy_detail?.decision || event.decision} /></td>
                  <td className="px-4 py-2 text-xs text-slate-500 font-mono truncate max-w-[150px]">{event.policy_detail?.policy_id || "—"}</td>
                  <td className="px-4 py-2 text-xs text-slate-400">{event.action?.observed_effect || "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
