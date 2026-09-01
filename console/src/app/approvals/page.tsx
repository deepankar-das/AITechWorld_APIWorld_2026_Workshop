/**
 * Author: Deepankar Das
 */

"use client";

import { useEffect, useState } from "react";
import { DecisionBadge } from "@/components/decision-badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ShieldCheck, Shield, Clock } from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { getPendingApprovals, resolveApproval } from "@/lib/api";
import type { PendingApproval } from "@/lib/api";

export default function ApprovalsPage() {
  const { token, username, role, canApprove } = useAuth();
  const [pending, setPending] = useState<PendingApproval[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    // Skip fetching if user lacks permission
    if (role !== "admin" && role !== "reviewer") return;

    async function fetchPending() {
      try {
        const data = await getPendingApprovals(token);
        const sorted = (data.approvals || []).sort(
          (a: { created_at: string }, b: { created_at: string }) =>
            new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
        );
        setPending(sorted);
      } catch {
        setPending([]);
      }
      setLoading(false);
    }
    fetchPending();
    const interval = setInterval(fetchPending, 3000);
    return () => clearInterval(interval);
  }, [token, role]);

  // Hub Console only — requires reviewer or admin role
  if (role !== "admin" && role !== "reviewer") {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-center">
          <Shield className="h-12 w-12 text-slate-600 mx-auto mb-3" />
          <h2 className="text-lg font-semibold text-slate-300">Hub Console Access Required</h2>
          <p className="text-sm text-slate-500 mt-1">This page is available on the Management Hub. Your Sentinel Console shows your personal activity.</p>
          <a href="/" className="text-blue-400 text-sm mt-3 inline-block hover:underline">&larr; Back to my dashboard</a>
        </div>
      </div>
    );
  }

  async function handleResolve(approvalId: string, decision: "approve" | "deny") {
    if (!canApprove) return;
    try {
      await resolveApproval(approvalId, {
        decision,
        approver_id: username || "console_reviewer",
        rationale: `${decision === "approve" ? "Approved" : "Denied"} via review console`,
      }, token);
      setPending(prev => prev.filter(a => a.approval_id !== approvalId));
    } catch (err) {
      alert(`Failed to ${decision} approval: ${err instanceof Error ? err.message : "Unknown error"}. Check your admin token.`);
    }
  }

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-xl font-bold text-slate-100">Approvals</h1>
        <p className="text-sm text-slate-500">Pending and resolved approval requests</p>
      </div>

      <Tabs defaultValue="pending">
        <TabsList className="bg-slate-900 border border-slate-800">
          <TabsTrigger value="pending" className="data-[state=active]:bg-slate-800">
            Pending ({pending.length})
          </TabsTrigger>
          <TabsTrigger value="resolved" className="data-[state=active]:bg-slate-800">
            Resolved
          </TabsTrigger>
        </TabsList>

        <TabsContent value="pending" className="mt-4">
          {loading ? (
            <div className="text-slate-500 text-sm">Loading...</div>
          ) : pending.length === 0 ? (
            <div className="bg-slate-900 border border-slate-800 rounded-lg p-8 text-center">
              <ShieldCheck className="h-8 w-8 text-slate-600 mx-auto mb-3" />
              <p className="text-slate-500 text-sm">No pending approvals</p>
            </div>
          ) : (
            <div className="space-y-3">
              {pending.map(approval => (
                <Card key={approval.approval_id} className="bg-slate-900 border-amber-900/50">
                  <CardHeader className="pb-2">
                    <div className="flex items-center justify-between">
                      <CardTitle className="text-sm font-mono text-amber-400">
                        {approval.approval_id}
                      </CardTitle>
                      <div className="flex items-center gap-1 text-xs text-slate-500">
                        <Clock className="h-3 w-3" />
                        {new Date(approval.created_at).toLocaleTimeString()}
                      </div>
                    </div>
                  </CardHeader>
                  <CardContent>
                    <div className="space-y-2 text-sm">
                      <div className="flex gap-2">
                        <span className="text-slate-500 w-20 shrink-0">Actor:</span>
                        <span className="text-slate-300">{approval.context_bundle.actor}</span>
                      </div>
                      <div className="flex gap-2">
                        <span className="text-slate-500 w-20 shrink-0">Resource:</span>
                        <span className="text-slate-300 font-mono text-xs">{approval.context_bundle.resource}</span>
                      </div>
                      <div className="flex gap-2">
                        <span className="text-slate-500 w-20 shrink-0">Risk:</span>
                        <span className="text-amber-400 text-xs">{approval.context_bundle.risk_rationale}</span>
                      </div>
                      <div className="flex gap-2">
                        <span className="text-slate-500 w-20 shrink-0">Policy:</span>
                        <span className="text-slate-400 font-mono text-xs">{approval.context_bundle.policy_rule}</span>
                      </div>
                    </div>
                    <div className="flex gap-2 mt-4">
                      <Button
                        size="sm"
                        className="bg-emerald-900 hover:bg-emerald-800 text-emerald-300"
                        disabled={!canApprove}
                        onClick={() => handleResolve(approval.approval_id, "approve")}
                      >
                        Approve
                      </Button>
                      <Button
                        size="sm"
                        variant="destructive"
                        className="bg-red-900 hover:bg-red-800 text-red-300"
                        disabled={!canApprove}
                        onClick={() => handleResolve(approval.approval_id, "deny")}
                      >
                        Deny
                      </Button>
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          )}
        </TabsContent>

        <TabsContent value="resolved" className="mt-4">
          <div className="bg-slate-900 border border-slate-800 rounded-lg p-8 text-center text-slate-500 text-sm">
            Resolved approvals will appear in the audit search. Use the Search page to filter by decision type.
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}
