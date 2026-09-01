/**
 * Author: Deepankar Das
 */

"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { DecisionBadge } from "@/components/decision-badge";
import { Monitor, User, Bot } from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { getHealth, getSessions } from "@/lib/api";
import type { SessionSummary } from "@/lib/api";

export default function SessionsPage() {
  const { token, username, role } = useAuth();
  const isHubUser = role === "admin" || role === "reviewer";
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [loading, setLoading] = useState(true);

  const [governedUser, setGovernedUser] = useState("");

  useEffect(() => {
    async function fetchSessions() {
      try {
        const [health, data] = await Promise.all([
          getHealth(),
          getSessions(token),
        ]);
        // On Sentinel, filter to the governed developer only.
        const govUser = (!isHubUser && health.governed_user) ? health.governed_user : "";
        setGovernedUser(govUser);
        const all = data.sessions || [];
        setSessions(govUser ? all.filter(s => s.user_id === govUser) : all);
      } catch {
        setSessions([]);
      }
      setLoading(false);
    }
    fetchSessions();
    const interval = setInterval(fetchSessions, 5000);
    return () => clearInterval(interval);
  }, [token, isHubUser]);

  const displaySessions = sessions;

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-xl font-bold text-slate-100">Sessions</h1>
        <p className="text-sm text-slate-500">
          {isHubUser ? "Agent sessions under governance" : "Your agent sessions under governance"}
        </p>
      </div>

      {!isHubUser && (
        <div className="bg-slate-900 border border-slate-800 rounded-lg p-3 mb-4 text-sm text-slate-400">
          Showing your sessions only. For all developer sessions, access the Hub Console.
        </div>
      )}

      {loading ? (
        <div className="text-slate-500 text-sm">Loading sessions...</div>
      ) : displaySessions.length === 0 ? (
        <div className="bg-slate-900 border border-slate-800 rounded-lg p-8 text-center">
          <Monitor className="h-8 w-8 text-slate-600 mx-auto mb-3" />
          <p className="text-slate-500 text-sm">No sessions yet. Start a governed agent session to see data here.</p>
        </div>
      ) : (
        <div className="bg-slate-900 border border-slate-800 rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-slate-800">
                <th className="text-left px-4 py-3 text-[11px] uppercase tracking-wider text-slate-500 font-medium">User</th>
                <th className="text-left px-4 py-3 text-[11px] uppercase tracking-wider text-slate-500 font-medium">Agent</th>
                <th className="text-left px-4 py-3 text-[11px] uppercase tracking-wider text-slate-500 font-medium">Session ID</th>
                <th className="text-left px-4 py-3 text-[11px] uppercase tracking-wider text-slate-500 font-medium">Events</th>
                <th className="text-left px-4 py-3 text-[11px] uppercase tracking-wider text-slate-500 font-medium">Started</th>
                <th className="text-left px-4 py-3 text-[11px] uppercase tracking-wider text-slate-500 font-medium">Last Activity</th>
                <th className="text-left px-4 py-3 text-[11px] uppercase tracking-wider text-slate-500 font-medium">Decisions</th>
              </tr>
            </thead>
            <tbody>
              {displaySessions.map(session => (
                <tr key={session.session_id} className="border-b border-slate-800/50 hover:bg-slate-800/30 transition-colors">
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-1.5">
                      <User className="h-3.5 w-3.5 text-slate-500" />
                      <span className="text-slate-300 text-xs">{session.user_id || "unknown"}</span>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-1.5">
                      <Bot className="h-3.5 w-3.5 text-cyan-500" />
                      <div>
                        <span className="text-cyan-400 text-xs">{session.agent_type || "unknown"}</span>
                        {session.agent_instance && (
                          <span className="text-slate-600 text-[10px] ml-1">({session.agent_instance})</span>
                        )}
                      </div>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <Link href={`/sessions/${session.session_id}`} className="font-mono text-cyan-400 hover:underline text-xs">
                      {session.session_id}
                    </Link>
                  </td>
                  <td className="px-4 py-3 text-slate-300">{session.event_count}</td>
                  <td className="px-4 py-3 text-slate-400 text-xs font-mono">{new Date(session.first_event).toLocaleString()}</td>
                  <td className="px-4 py-3 text-slate-400 text-xs font-mono">{new Date(session.last_event).toLocaleString()}</td>
                  <td className="px-4 py-3">
                    <div className="flex gap-1 flex-wrap">
                      {Object.entries(session.decisions).map(([decision, count]) => (
                        <span key={decision} className="flex items-center gap-1">
                          <DecisionBadge decision={decision} />
                          <span className="text-xs text-slate-500">{count}</span>
                        </span>
                      ))}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}