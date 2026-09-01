/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Policy Management API
 *
 * REST endpoints for policy CRUD and hot reload:
 *   GET    /v1/policy/rules           — list all active rules
 *   POST   /v1/policy/rules           — add a new rule
 *   PUT    /v1/policy/rules/:id       — update a rule
 *   DELETE /v1/policy/rules/:id       — delete a rule
 *   POST   /v1/policy/rules/:id/toggle — enable/disable a rule
 *   GET    /v1/policy/bundle          — get current bundle metadata
 *   POST   /v1/policy/reload          — reload from disk
 *   GET    /v1/policy/packs           — list available canned packs
 *   POST   /v1/policy/packs/:id/apply — apply a canned pack
 */

import type { PolicyBundle, PolicyRule } from "../../../types/policy.js";
import { getAvailablePacks, getPack, applyPack, addCustomPack } from "../../policy/packs.js";

export interface PolicyRouteResult {
  status: number;
  body: unknown;
}

// ── In-memory state for active policy bundle ────────────────────────────────
// The daemon passes its policyBundle reference here. Mutations are live.

let activeBundleRef: { bundle: PolicyBundle } | null = null;

export function setPolicyBundleRef(ref: { bundle: PolicyBundle }): void {
  activeBundleRef = ref;
}

// ── Track disabled rules ────────────────────────────────────────────────────

const disabledRules = new Set<string>();

export function isRuleEnabled(ruleId: string): boolean {
  return !disabledRules.has(ruleId);
}

// ── GET /v1/policy/rules ────────────────────────────────────────────────────

export function handleListRules(): PolicyRouteResult {
  if (!activeBundleRef) {
    return { status: 500, body: { error: "No policy bundle loaded" } };
  }

  const rules = activeBundleRef.bundle.rules.map(rule => ({
    ...rule,
    enabled: isRuleEnabled(rule.policy_id),
  }));

  return {
    status: 200,
    body: {
      bundle_version: activeBundleRef.bundle.bundle_version,
      rules,
      count: rules.length,
      enabled_count: rules.filter(r => r.enabled).length,
    },
  };
}

// ── POST /v1/policy/rules ───────────────────────────────────────────────────

export function handleAddRule(rawBody: string): PolicyRouteResult {
  if (!activeBundleRef) {
    return { status: 500, body: { error: "No policy bundle loaded" } };
  }

  let body: Record<string, unknown>;
  try {
    body = JSON.parse(rawBody);
  } catch {
    return { status: 400, body: { error: "Invalid JSON" } };
  }

  if (!body.policy_id || !body.action || !body.effect) {
    return { status: 400, body: { error: "policy_id, action, and effect are required" } };
  }

  // Build rule from input
  const rule: PolicyRule = {
    policy_id: body.policy_id as string,
    version: activeBundleRef.bundle.bundle_version,
    scope: { level: (body.scope_level as "organization" | "team" | "repository" | "local") || "organization" },
    subject: {
      agent_types: (body.agent_types as string[]) || ["*"],
      users: (body.users as string[]) || ["*"],
    },
    action: { types: (body.action as { types: string[] }).types || [] },
    resource: (body.resource as Record<string, unknown>) || {},
    conditions: (body.conditions as Record<string, unknown>) || {},
    effect: body.effect as { decision: "allow" | "deny" | "require_approval"; reason_code: string; reason_human: string },
    logging: { mode: "full" },
    approval: { required: (body.effect as { decision: string }).decision === "require_approval" },
  };

  // Check for duplicate policy_id
  if (activeBundleRef.bundle.rules.some(r => r.policy_id === rule.policy_id)) {
    return { status: 409, body: { error: `Rule ${rule.policy_id} already exists. Use PUT to update.` } };
  }

  activeBundleRef.bundle.rules.push(rule);

  return {
    status: 201,
    body: { message: `Rule ${rule.policy_id} added`, rule },
  };
}

// ── PUT /v1/policy/rules/:id ────────────────────────────────────────────────

export function handleUpdateRule(ruleId: string, rawBody: string): PolicyRouteResult {
  if (!activeBundleRef) {
    return { status: 500, body: { error: "No policy bundle loaded" } };
  }

  const index = activeBundleRef.bundle.rules.findIndex(r => r.policy_id === ruleId);
  if (index === -1) {
    return { status: 404, body: { error: `Rule ${ruleId} not found` } };
  }

  let body: Record<string, unknown>;
  try {
    body = JSON.parse(rawBody);
  } catch {
    return { status: 400, body: { error: "Invalid JSON" } };
  }

  const existing = activeBundleRef.bundle.rules[index];

  // Merge updates
  if (body.action) existing.action = body.action as { types: string[] };
  if (body.resource) existing.resource = body.resource as Record<string, unknown>;
  if (body.conditions) existing.conditions = body.conditions as Record<string, unknown>;
  if (body.effect) {
    existing.effect = body.effect as { decision: "allow" | "deny" | "require_approval"; reason_code: string; reason_human: string };
    existing.approval = { required: existing.effect.decision === "require_approval" };
  }
  if (body.subject) existing.subject = body.subject as { agent_types: string[]; users: string[] };

  return {
    status: 200,
    body: { message: `Rule ${ruleId} updated`, rule: existing },
  };
}

// ── DELETE /v1/policy/rules/:id ─────────────────────────────────────────────

export function handleDeleteRule(ruleId: string): PolicyRouteResult {
  if (!activeBundleRef) {
    return { status: 500, body: { error: "No policy bundle loaded" } };
  }

  const index = activeBundleRef.bundle.rules.findIndex(r => r.policy_id === ruleId);
  if (index === -1) {
    return { status: 404, body: { error: `Rule ${ruleId} not found` } };
  }

  const removed = activeBundleRef.bundle.rules.splice(index, 1)[0];
  disabledRules.delete(ruleId);

  return {
    status: 200,
    body: { message: `Rule ${ruleId} deleted`, rule: removed },
  };
}

// ── POST /v1/policy/rules/:id/toggle ────────────────────────────────────────

export function handleToggleRule(ruleId: string): PolicyRouteResult {
  if (!activeBundleRef) {
    return { status: 500, body: { error: "No policy bundle loaded" } };
  }

  const rule = activeBundleRef.bundle.rules.find(r => r.policy_id === ruleId);
  if (!rule) {
    return { status: 404, body: { error: `Rule ${ruleId} not found` } };
  }

  if (disabledRules.has(ruleId)) {
    disabledRules.delete(ruleId);
    return { status: 200, body: { policy_id: ruleId, enabled: true, message: `Rule ${ruleId} enabled` } };
  } else {
    disabledRules.add(ruleId);
    return { status: 200, body: { policy_id: ruleId, enabled: false, message: `Rule ${ruleId} disabled` } };
  }
}

// ── GET /v1/policy/bundle ───────────────────────────────────────────────────

export function handleGetBundle(): PolicyRouteResult {
  if (!activeBundleRef) {
    return { status: 500, body: { error: "No policy bundle loaded" } };
  }

  return {
    status: 200,
    body: {
      bundle_version: activeBundleRef.bundle.bundle_version,
      scope_level: activeBundleRef.bundle.scope_level,
      rule_count: activeBundleRef.bundle.rules.length,
      enabled_count: activeBundleRef.bundle.rules.filter(r => isRuleEnabled(r.policy_id)).length,
      disabled_count: disabledRules.size,
    },
  };
}

// ── GET /v1/policy/packs ────────────────────────────────────────────────────

export function handleListPacks(): PolicyRouteResult {
  const packs = getAvailablePacks().map(pack => ({
    id: pack.id,
    name: pack.name,
    description: pack.description,
    category: pack.category,
    tags: pack.tags,
    rule_count: pack.rules.length,
  }));

  return {
    status: 200,
    body: { packs, count: packs.length },
  };
}

// ── POST /v1/policy/packs/:id/apply ─────────────────────────────────────────

export function handleApplyPack(packId: string): PolicyRouteResult {
  if (!activeBundleRef) {
    return { status: 500, body: { error: "No policy bundle loaded" } };
  }

  try {
    const result = applyPack(activeBundleRef.bundle, packId);
    return {
      status: 200,
      body: {
        message: `Pack ${packId} applied`,
        added: result.added,
        skipped: result.skipped,
        total_rules: activeBundleRef.bundle.rules.length,
      },
    };
  } catch (err) {
    const message = err instanceof Error ? err.message : "Unknown error";
    return { status: 404, body: { error: message } };
  }
}

// ── GET /v1/policy/packs/:id ────────────────────────────────────────────────

export function handleGetPack(packId: string): PolicyRouteResult {
  const pack = getPack(packId);
  if (!pack) {
    return { status: 404, body: { error: `Pack ${packId} not found` } };
  }

  return {
    status: 200,
    body: {
      id: pack.id,
      name: pack.name,
      description: pack.description,
      category: pack.category,
      tags: pack.tags,
      rules: pack.rules.map(r => ({
        policy_id: r.policy_id,
        action_types: r.action.types,
        decision: r.effect.decision,
        reason_code: r.effect.reason_code,
        reason_human: r.effect.reason_human,
      })),
      rule_count: pack.rules.length,
    },
  };
}

// ── POST /v1/policy/packs ───────────────────────────────────────────────────
// Create a custom pack from a list of rule IDs (copies from active bundle)
// or from provided rules directly.

export function handleCreatePack(rawBody: string): PolicyRouteResult {
  let body: Record<string, unknown>;
  try {
    body = JSON.parse(rawBody);
  } catch {
    return { status: 400, body: { error: "Invalid JSON" } };
  }

  if (!body.id || !body.name) {
    return { status: 400, body: { error: "id and name are required" } };
  }

  const packId = body.id as string;
  const packName = body.name as string;
  const description = (body.description as string) || "";
  const category = (body.category as string) || "Custom";
  const tags = (body.tags as string[]) || ["custom"];

  // Option 1: Copy rules from active bundle by rule IDs
  const ruleIds = body.rule_ids as string[] | undefined;
  // Option 2: Use a template pack and customize
  const templatePackId = body.template_pack_id as string | undefined;
  // Option 3: Provide rules directly
  const directRules = body.rules as Array<{
    policy_id: string;
    action_types: string[];
    decision: string;
    reason_code: string;
    reason_human: string;
  }> | undefined;

  let rules: PolicyRule[] = [];

  if (templatePackId) {
    const template = getPack(templatePackId);
    if (!template) {
      return { status: 404, body: { error: `Template pack ${templatePackId} not found` } };
    }
    // Clone template rules with new pack prefix
    rules = template.rules.map(r => ({
      ...r,
      policy_id: r.policy_id.replace(/^pack\./, `custom.${packId}.`),
    }));
  } else if (ruleIds && activeBundleRef) {
    for (const id of ruleIds) {
      const rule = activeBundleRef.bundle.rules.find(r => r.policy_id === id);
      if (rule) {
        rules.push({ ...rule, policy_id: `custom.${packId}.${rule.policy_id}` });
      }
    }
  } else if (directRules) {
    rules = directRules.map(r => ({
      policy_id: r.policy_id,
      version: activeBundleRef?.bundle.bundle_version || "v1",
      scope: { level: "organization" as const },
      subject: { agent_types: ["*"], users: ["*"] },
      action: { types: r.action_types },
      resource: {},
      conditions: {},
      effect: {
        decision: r.decision as "allow" | "deny" | "require_approval",
        reason_code: r.reason_code,
        reason_human: r.reason_human,
      },
      logging: { mode: "full" as const },
      approval: { required: r.decision === "require_approval" },
    }));
  }

  if (rules.length === 0) {
    return { status: 400, body: { error: "No rules provided. Use rule_ids, template_pack_id, or rules." } };
  }

  try {
    addCustomPack({
      id: packId,
      name: packName,
      description,
      category,
      tags,
      rules,
    });

    return {
      status: 201,
      body: {
        message: `Pack ${packId} created with ${rules.length} rules`,
        id: packId,
        rule_count: rules.length,
      },
    };
  } catch (err) {
    const msg = err instanceof Error ? err.message : "Unknown error";
    return { status: 409, body: { error: msg } };
  }
}