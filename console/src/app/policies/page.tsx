/**
 * Author: Deepankar Das
 */

"use client";

import { useEffect, useState } from "react";
import { DecisionBadge } from "@/components/decision-badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Shield, Package, Lock } from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { isHubMode } from "../../lib/console-mode";
import { getPolicyRules, getPolicyPacks, getPolicyPack, togglePolicyRule, deletePolicyRule, addPolicyRule, applyPolicyPack } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

interface PolicyRule {
  policy_id: string;
  version: string;
  action: { types: string[] };
  effect: { decision: string; reason_code: string; reason_human: string };
  resource: Record<string, unknown>;
  enabled: boolean;
}

interface PackRule {
  policy_id: string;
  action_types: string[];
  decision: string;
  reason_code: string;
  reason_human: string;
}

interface PolicyPack {
  id: string;
  name: string;
  description: string;
  category: string;
  tags: string[];
  rule_count: number;
}

export default function PoliciesPage() {
  const { token, role } = useAuth();
  const hubMode = isHubMode();
  const [rules, setRules] = useState<PolicyRule[]>([]);
  const [packs, setPacks] = useState<PolicyPack[]>([]);
  const [bundleVersion, setBundleVersion] = useState("");
  const [loading, setLoading] = useState(true);

  const [expandedPack, setExpandedPack] = useState<string | null>(null);
  const [packRules, setPackRules] = useState<Record<string, PackRule[]>>({});
  const [showAddForm, setShowAddForm] = useState(false);
  const [newRuleId, setNewRuleId] = useState("");
  const [newRuleActions, setNewRuleActions] = useState("shell.exec");
  const [newRuleDecision, setNewRuleDecision] = useState("deny");
  const [newRuleReason, setNewRuleReason] = useState("");
  const [applyingPack, setApplyingPack] = useState<string | null>(null);

  async function fetchRules() {
    try {
      const data = (await getPolicyRules(token)) as { rules: PolicyRule[]; count: number; bundle_version?: string };
      setRules(data.rules || []);
      setBundleVersion(data.bundle_version || "");
    } catch {
      setRules([]);
    }
    setLoading(false);
  }

  async function fetchPacks() {
    try {
      const data = (await getPolicyPacks(token)) as { packs: PolicyPack[] };
      setPacks(data.packs || []);
    } catch {
      setPacks([]);
    }
  }

  async function fetchPackRulesData(packID: string) {
    try {
      const data = (await getPolicyPack(packID, token)) as { rules: PackRule[] };
      setPackRules((prev) => ({ ...prev, [packID]: data.rules || [] }));
    } catch {
      // Ignore failed expansion fetch.
    }
  }

  async function togglePackExpand(packID: string) {
    if (expandedPack === packID) {
      setExpandedPack(null);
      return;
    }
    setExpandedPack(packID);
    if (!packRules[packID]) {
      await fetchPackRulesData(packID);
    }
  }

  useEffect(() => {
    if (role === "none") return;
    fetchRules();
    fetchPacks();
  }, [token, role]);

  if (!hubMode) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-center">
          <Shield className="h-12 w-12 text-slate-600 mx-auto mb-3" />
          <h2 className="text-lg font-semibold text-slate-300">Hub Console Access Required</h2>
          <p className="text-sm text-slate-500 mt-1">Policy management is only available on the Management Hub.</p>
          <a href="/" className="text-blue-400 text-sm mt-3 inline-block hover:underline">&larr; Back to my dashboard</a>
        </div>
      </div>
    );
  }

  if (role === "none") {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-center">
          <Shield className="h-12 w-12 text-slate-600 mx-auto mb-3" />
          <h2 className="text-lg font-semibold text-slate-300">Sign In Required</h2>
          <p className="text-sm text-slate-500 mt-1">Sign in to view policy bundle details.</p>
          <a href="/login" className="text-blue-400 text-sm mt-3 inline-block hover:underline">Go to login</a>
        </div>
      </div>
    );
  }

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-xl font-bold text-slate-100">Policies</h1>
        <p className="text-sm text-slate-500">
          Bundle: <span className="font-mono text-blue-400">{bundleVersion || "—"}</span>
          {" — "}{rules.length} rules ({rules.filter((r) => r.enabled !== false).length} active)
        </p>
      </div>

      {role !== "admin" && (
        <Card className="bg-slate-900 border-amber-900/60 mb-6">
          <CardHeader className="pb-2">
            <CardTitle className="text-sm text-amber-300 flex items-center gap-2">
              <Lock className="h-4 w-4" />
              Hub-Managed Policy (Read Only)
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-xs text-slate-400">
              Policy is managed by the Hub admin. Sentinel auto-syncs policy updates.
            </p>
          </CardContent>
        </Card>
      )}

      {/* Add Rule (admin only) */}
      {role === "admin" && (
        <div className="mb-4">
          {!showAddForm ? (
            <Button
              onClick={() => setShowAddForm(true)}
              className="bg-cyan-900 hover:bg-cyan-800 text-cyan-300 text-xs"
            >
              + Add Rule
            </Button>
          ) : (
            <Card className="bg-slate-900 border-cyan-900/60">
              <CardHeader className="pb-2">
                <CardTitle className="text-sm text-cyan-300">Add New Policy Rule</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-1 md:grid-cols-4 gap-2 mb-3">
                  <Input
                    placeholder="Rule ID (e.g. org.block_xyz)"
                    value={newRuleId}
                    onChange={e => setNewRuleId(e.target.value)}
                    className="bg-slate-950 border-slate-700 text-slate-200 text-xs font-mono"
                  />
                  <select
                    value={newRuleActions}
                    onChange={e => setNewRuleActions(e.target.value)}
                    className="bg-slate-950 border border-slate-700 rounded-md px-3 py-2 text-xs text-slate-200"
                  >
                    <option value="file.read">file.read</option>
                    <option value="file.write">file.write</option>
                    <option value="file.delete">file.delete</option>
                    <option value="shell.exec">shell.exec</option>
                    <option value="network.request">network.request</option>
                    <option value="package.install">package.install</option>
                    <option value="credential.access">credential.access</option>
                    <option value="mcp.invoke">mcp.invoke</option>
                  </select>
                  <select
                    value={newRuleDecision}
                    onChange={e => setNewRuleDecision(e.target.value)}
                    className="bg-slate-950 border border-slate-700 rounded-md px-3 py-2 text-xs text-slate-200"
                  >
                    <option value="deny">deny</option>
                    <option value="require_approval">require_approval</option>
                    <option value="allow">allow</option>
                  </select>
                  <Input
                    placeholder="Reason (human-readable)"
                    value={newRuleReason}
                    onChange={e => setNewRuleReason(e.target.value)}
                    className="bg-slate-950 border-slate-700 text-slate-200 text-xs"
                  />
                </div>
                <div className="flex gap-2">
                  <Button
                    onClick={async () => {
                      if (!newRuleId) return;
                      try {
                        await addPolicyRule({
                          policy_id: newRuleId,
                          version: "v1.0.0",
                          scope: { level: "organization" },
                          subject: { agent_types: ["*"], users: ["*"] },
                          action: { types: [newRuleActions] },
                          resource: {},
                          conditions: {},
                          effect: {
                            decision: newRuleDecision,
                            reason_code: newRuleId.toUpperCase().replace(/\./g, "_"),
                            reason_human: newRuleReason || `Rule ${newRuleId}`,
                          },
                          logging: { mode: "full" },
                          approval: { required: newRuleDecision === "require_approval" },
                        }, token);
                        setShowAddForm(false);
                        setNewRuleId("");
                        setNewRuleReason("");
                        fetchRules();
                      } catch (err) {
                        alert(`Failed to add rule: ${err instanceof Error ? err.message : "Unknown error"}`);
                      }
                    }}
                    className="bg-emerald-900 hover:bg-emerald-800 text-emerald-300 text-xs"
                  >
                    Save Rule
                  </Button>
                  <Button
                    onClick={() => setShowAddForm(false)}
                    variant="ghost"
                    className="text-slate-400 text-xs"
                  >
                    Cancel
                  </Button>
                </div>
              </CardContent>
            </Card>
          )}
        </div>
      )}

      <Tabs defaultValue="rules">
        <TabsList className="bg-slate-900 border border-slate-800">
          <TabsTrigger value="rules" className="data-[state=active]:bg-slate-800">
            <Shield className="h-3.5 w-3.5 mr-1" />
            Active Rules ({rules.length})
          </TabsTrigger>
          <TabsTrigger value="packs" className="data-[state=active]:bg-slate-800">
            <Package className="h-3.5 w-3.5 mr-1" />
            Policy Packs ({packs.length})
          </TabsTrigger>
        </TabsList>

        <TabsContent value="rules" className="mt-4">
          {loading ? (
            <div className="text-slate-500 text-sm">Loading rules...</div>
          ) : rules.length === 0 ? (
            <div className="bg-slate-900 border border-slate-800 rounded-lg p-8 text-center text-slate-500 text-sm">
              No rules loaded.
            </div>
          ) : (
            <div className="bg-slate-900 border border-slate-800 rounded-lg overflow-hidden">
              <div className="px-4 py-2 border-b border-slate-800 text-xs text-slate-500">
                Evaluation order: deny → require_approval → allow → default deny (least privilege)
              </div>
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-slate-800">
                    <th className="text-left px-4 py-2 text-[11px] uppercase tracking-wider text-slate-300 font-medium w-8">On</th>
                    <th className="text-left px-4 py-2 text-[11px] uppercase tracking-wider text-slate-300 font-medium">Rule ID</th>
                    <th className="text-left px-4 py-2 text-[11px] uppercase tracking-wider text-slate-300 font-medium">Actions</th>
                    <th className="text-left px-4 py-2 text-[11px] uppercase tracking-wider text-slate-300 font-medium">Effect</th>
                    <th className="text-left px-4 py-2 text-[11px] uppercase tracking-wider text-slate-300 font-medium">Reason</th>
                    {role === "admin" && (
                      <th className="text-left px-4 py-2 text-[11px] uppercase tracking-wider text-slate-300 font-medium">Actions</th>
                    )}
                  </tr>
                </thead>
                <tbody>
                  {rules.map((rule) => (
                    <tr key={rule.policy_id} className={`border-b border-slate-800/50 hover:bg-slate-800/30 ${rule.enabled === false ? "opacity-40" : ""}`}>
                      <td className="px-4 py-2">
                        {role === "admin" ? (
                          <button
                            onClick={async () => {
                              try {
                                await togglePolicyRule(rule.policy_id, token);
                                fetchRules();
                              } catch { /* ignore */ }
                            }}
                            className={`text-xs px-2 py-0.5 rounded cursor-pointer ${rule.enabled !== false ? "bg-emerald-900/50 text-emerald-400 hover:bg-emerald-900" : "bg-slate-800 text-slate-500 hover:bg-slate-700"}`}
                          >
                            {rule.enabled !== false ? "ON" : "OFF"}
                          </button>
                        ) : (
                          <span className={rule.enabled !== false ? "text-emerald-400" : "text-slate-600"}>{rule.enabled !== false ? "ON" : "OFF"}</span>
                        )}
                      </td>
                      <td className="px-4 py-2 font-mono text-xs text-cyan-300 font-semibold">{rule.policy_id}</td>
                      <td className="px-4 py-2 font-mono text-xs text-slate-100">{rule.action.types.join(", ")}</td>
                      <td className="px-4 py-2"><DecisionBadge decision={rule.effect.decision} /></td>
                      <td className="px-4 py-2 text-xs text-slate-200">{rule.effect.reason_human}</td>
                      {role === "admin" && (
                        <td className="px-4 py-2">
                          <button
                            onClick={async () => {
                              if (!confirm(`Delete rule ${rule.policy_id}?`)) return;
                              try {
                                await deletePolicyRule(rule.policy_id, token);
                                fetchRules();
                              } catch { /* ignore */ }
                            }}
                            className="text-[10px] px-2 py-0.5 rounded bg-red-950 text-red-400 hover:bg-red-900 cursor-pointer"
                          >
                            Delete
                          </button>
                        </td>
                      )}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </TabsContent>

        <TabsContent value="packs" className="mt-4">
          <div className="space-y-4">
            {packs.map((pack) => (
              <Card key={pack.id} className="bg-slate-900 border-slate-800">
                <CardHeader className="pb-2">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <CardTitle className="text-sm text-slate-200">{pack.name}</CardTitle>
                      <span className="text-[10px] px-2 py-0.5 rounded-full bg-slate-800 text-slate-400">{pack.category}</span>
                    </div>
                    <div className="flex gap-2">
                      {role === "admin" && (
                        <button
                          className="text-xs border border-emerald-800 rounded px-2 py-1 text-emerald-400 hover:text-emerald-300 hover:bg-emerald-900/30 disabled:opacity-50"
                          disabled={applyingPack === pack.id}
                          onClick={async () => {
                            setApplyingPack(pack.id);
                            try {
                              const result = await applyPolicyPack(pack.id, token) as { added?: string[]; skipped?: string[] };
                              alert(`Applied: ${(result.added || []).length} rules added, ${(result.skipped || []).length} skipped`);
                              fetchRules();
                            } catch (err) {
                              alert(`Failed: ${err instanceof Error ? err.message : "Unknown"}`);
                            }
                            setApplyingPack(null);
                          }}
                        >
                          {applyingPack === pack.id ? "Applying..." : "Apply Pack"}
                        </button>
                      )}
                      <button
                        className="text-xs border border-slate-700 rounded px-2 py-1 text-slate-400 hover:text-slate-200 hover:bg-slate-800"
                        onClick={() => togglePackExpand(pack.id)}
                      >
                        {expandedPack === pack.id ? "Hide Rules" : `Show ${pack.rule_count} Rules`}
                      </button>
                    </div>
                  </div>
                </CardHeader>
                <CardContent>
                  <p className="text-xs text-slate-400 mb-2">{pack.description}</p>
                  <div className="flex gap-1 flex-wrap mb-2">
                    {pack.tags.map((tag) => (
                      <span key={tag} className="text-[10px] px-1.5 py-0.5 rounded bg-slate-800 text-slate-500">{tag}</span>
                    ))}
                  </div>

                  {expandedPack === pack.id && (
                    <div className="mt-3 border-t border-slate-800 pt-3">
                      {!packRules[pack.id] ? (
                        <div className="text-xs text-slate-500">Loading rules...</div>
                      ) : (
                        <table className="w-full text-xs">
                          <thead>
                            <tr className="border-b border-slate-800">
                              <th className="text-left py-1.5 px-2 text-[10px] uppercase text-slate-500 font-medium">Rule ID</th>
                              <th className="text-left py-1.5 px-2 text-[10px] uppercase text-slate-500 font-medium">Actions</th>
                              <th className="text-left py-1.5 px-2 text-[10px] uppercase text-slate-500 font-medium">Effect</th>
                              <th className="text-left py-1.5 px-2 text-[10px] uppercase text-slate-500 font-medium">Description</th>
                            </tr>
                          </thead>
                          <tbody>
                            {packRules[pack.id].map((rule) => (
                              <tr key={rule.policy_id} className="border-b border-slate-800/30">
                                <td className="py-1.5 px-2 font-mono text-cyan-400">{rule.policy_id}</td>
                                <td className="py-1.5 px-2 font-mono text-slate-400">{rule.action_types.join(", ")}</td>
                                <td className="py-1.5 px-2"><DecisionBadge decision={rule.decision} /></td>
                                <td className="py-1.5 px-2 text-slate-400">{rule.reason_human}</td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      )}
                    </div>
                  )}
                </CardContent>
              </Card>
            ))}
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}
