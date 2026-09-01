/**
 * Author: Deepankar Das
 */

"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Download, FileJson } from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { exportEvents } from "@/lib/api";

export default function ExportPage() {
  const { token } = useAuth();
  const [sessionId, setSessionId] = useState("");
  const [decision, setDecision] = useState("");
  const [preview, setPreview] = useState<{ total: number } | null>(null);
  const [loading, setLoading] = useState(false);

  async function handlePreview() {
    setLoading(true);
    try {
      const params: Record<string, string> = {};
      if (sessionId) params.session_id = sessionId;
      if (decision) params.decision = decision;

      const data = await exportEvents(params, token);
      setPreview({ total: data.metadata.total_events });
    } catch {
      setPreview(null);
    }
    setLoading(false);
  }

  async function handleDownload() {
    const params: Record<string, string> = {};
    if (sessionId) params.session_id = sessionId;
    if (decision) params.decision = decision;

    const data = await exportEvents(params, token);

    const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `enforcer-evidence-${new Date().toISOString().slice(0, 10)}.json`;
    a.click();
    URL.revokeObjectURL(url);
  }

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-xl font-bold text-slate-100">Export</h1>
        <p className="text-sm text-slate-500">Export audit events as JSON evidence packages</p>
      </div>

      <div className="bg-slate-900 border border-slate-800 rounded-lg p-4 mb-6">
        <h2 className="text-xs uppercase tracking-wider text-slate-500 font-medium mb-3">Filters</h2>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
          <Input
            placeholder="Session ID (optional)"
            value={sessionId}
            onChange={e => setSessionId(e.target.value)}
            className="bg-slate-950 border-slate-700 text-slate-300 placeholder:text-slate-600 text-sm font-mono"
          />
          <select
            value={decision}
            onChange={e => setDecision(e.target.value)}
            className="bg-slate-950 border border-slate-700 rounded-md px-3 py-2 text-sm text-slate-300"
          >
            <option value="">All decisions</option>
            <option value="deny">Denied only</option>
            <option value="require_approval">Approvals only</option>
            <option value="allow">Allowed only</option>
          </select>
          <Button onClick={handlePreview} variant="outline" className="border-slate-700 text-slate-300">
            Preview
          </Button>
        </div>
      </div>

      {preview && (
        <div className="bg-slate-900 border border-slate-800 rounded-lg p-6 text-center">
          <FileJson className="h-10 w-10 text-cyan-400 mx-auto mb-3" />
          <p className="text-slate-300 mb-1">
            <span className="text-2xl font-bold text-cyan-400">{preview.total}</span> events matching filters
          </p>
          <p className="text-xs text-slate-500 mb-4">
            Export includes metadata header (timestamp, filters, event count)
          </p>
          <Button
            onClick={handleDownload}
            className="bg-cyan-900 hover:bg-cyan-800 text-cyan-300"
            disabled={preview.total === 0}
          >
            <Download className="h-4 w-4 mr-2" />
            Download JSON Evidence Package
          </Button>
        </div>
      )}

      {!preview && !loading && (
        <div className="bg-slate-900 border border-slate-800 rounded-lg p-8 text-center text-slate-500 text-sm">
          Set filters and click Preview to see how many events will be exported.
        </div>
      )}
    </div>
  );
}