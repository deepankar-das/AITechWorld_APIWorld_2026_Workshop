/**
 * Author: Deepankar Das
 */

"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { DecisionBadge } from "@/components/decision-badge";
import { ActionIcon } from "@/components/action-icon";
import { useAuth } from "@/lib/auth-context";
import { getSession } from "@/lib/api";
import type { AuditEvent } from "@/lib/api";

export default function SessionTimelinePage() {
  const { token } = useAuth();
  const params = useParams();
  const sessionId = params.id as string;
  const [events, setEvents] = useState<AuditEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedEvent, setSelectedEvent] = useState<AuditEvent | null>(null);

  useEffect(() => {
    async function fetchSession() {
      try {
        const data = await getSession(sessionId, token);
        setEvents(data.events || []);
      } catch {
        setEvents([]);
      }
      setLoading(false);
    }
    fetchSession();
  }, [sessionId, token]);

  return (
    <div>
      <div className="flex items-center gap-3 mb-6">
        <Link href="/sessions" className="text-slate-500 hover:text-slate-300 transition-colors">
          <ArrowLeft className="h-5 w-5" />
        </Link>
        <div>
          <h1 className="text-xl font-bold text-slate-100">Session Timeline</h1>
          <p className="text-sm text-slate-500 font-mono">{sessionId}</p>
          <p className="text-xs text-slate-600 mt-1">{events.length} events</p>
        </div>
      </div>

      {loading ? (
        <div className="text-slate-500 text-sm">Loading timeline...</div>
      ) : events.length === 0 ? (
        <div className="bg-slate-900 border border-slate-800 rounded-lg p-8 text-center text-slate-500 text-sm">
          No events found for this session.
        </div>
      ) : (
        <div className="space-y-2">
          {events.map((event, index) => (
            <div key={event.event_id}>
              <button
                onClick={() => setSelectedEvent(selectedEvent?.event_id === event.event_id ? null : event)}
                className="w-full text-left bg-slate-900 border border-slate-800 rounded-lg p-3 hover:bg-slate-800/50 transition-colors flex items-start gap-3"
              >
                  {/* Timeline line */}
                  <div className="flex flex-col items-center shrink-0 w-5">
                    <div className={`h-2 w-2 rounded-full mt-1.5 ${
                      event.policy_detail.decision === "allow" ? "bg-emerald-400" :
                      event.policy_detail.decision === "deny" ? "bg-red-400" :
                      "bg-amber-400"
                    }`} />
                    {index < events.length - 1 && <div className="w-px flex-1 bg-slate-700 mt-1" />}
                  </div>

                  {/* Event content */}
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-1">
                      <ActionIcon actionType={event.action.type} className="h-3.5 w-3.5 text-slate-400" />
                      <span className="text-xs text-slate-400">{event.action.type}</span>
                      <DecisionBadge decision={event.policy_detail.decision} />
                      {event.approval && event.approval.status !== "not_required" && (
                        <DecisionBadge decision={event.approval.status} />
                      )}
                    </div>
                    <div className="text-sm text-slate-300 font-mono truncate">
                      {event.resource.value || event.resource.path || event.resource.host || "unknown"}
                    </div>
                    <div className="text-[11px] text-slate-600 mt-0.5">
                      {new Date(event.timestamp).toLocaleTimeString()} — {event.policy_detail.reason_human}
                    </div>
                  </div>
                </button>
              {selectedEvent?.event_id === event.event_id && (
                <div className="bg-slate-950 border border-slate-800 rounded-lg p-4 mt-1 ml-8">
                  <div className="text-xs text-slate-500 mb-2 font-mono">{event.event_id}</div>
                  <pre className="text-xs text-slate-400 font-mono overflow-auto max-h-[400px]">
                    {JSON.stringify(event, null, 2)}
                  </pre>
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}