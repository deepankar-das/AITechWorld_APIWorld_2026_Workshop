/**
 * Author: Deepankar Das
 */

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  getMetrics,
  getPendingApprovals,
  resolveApproval,
  togglePolicyRule,
  validateAdminToken,
} from "../../console/src/lib/api.js";

describe("RBAC security regression (console API client)", () => {
  const fetchMock = vi.fn();

  beforeEach(() => {
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    fetchMock.mockReset();
  });

  it("sends bearer and role-token headers for authenticated endpoints", async () => {
    fetchMock.mockResolvedValue({ ok: true, json: async () => ({ approvals: [], count: 0 }) });

    await getPendingApprovals("review-token");

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const args = fetchMock.mock.calls[0];
    const opts = args[1] as { headers?: Record<string, string> };

    expect(opts.headers?.Authorization).toBe("Bearer review-token");
    expect(opts.headers?.["X-AA-Token"]).toBe("review-token");
    expect(opts.headers?.["X-Admin-Token"]).toBe("review-token");
  });

  it("does not silently omit token on policy mutation calls", async () => {
    fetchMock.mockResolvedValue({ ok: true, json: async () => ({}) });

    await togglePolicyRule("org.block_non_project_writes", "admin-token");

    const opts = fetchMock.mock.calls[0]?.[1] as { headers?: Record<string, string> };
    expect(opts.headers?.Authorization).toBe("Bearer admin-token");
  });

  it("validateAdminToken checks authenticated /v1/auth/me", async () => {
    fetchMock.mockResolvedValue({ ok: true, json: async () => ({ role: "admin" }) });

    const ok = await validateAdminToken("admin-token");

    expect(ok).toBe(true);
    expect(fetchMock.mock.calls[0]?.[0]).toContain("/v1/auth/me");
  });

  it("resolveApproval sends reviewer token headers", async () => {
    fetchMock.mockResolvedValue({ ok: true, json: async () => ({ resolved: true }) });

    await resolveApproval("apr_1", {
      decision: "approve",
      approver_id: "reviewer-1",
      rationale: "approved",
    }, "review-token");

    const opts = fetchMock.mock.calls[0]?.[1] as { headers?: Record<string, string> };
    expect(opts.headers?.Authorization).toBe("Bearer review-token");
  });

  it("metrics endpoint remains role-token protected", async () => {
    fetchMock.mockResolvedValue({ ok: true, json: async () => ({}) });

    await getMetrics("operator-token");

    const opts = fetchMock.mock.calls[0]?.[1] as { headers?: Record<string, string> };
    expect(opts.headers?.Authorization).toBe("Bearer operator-token");
  });
});
