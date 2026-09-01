/**
 * Author: Deepankar Das
 */

import { test, expect, type Page } from "@playwright/test";

async function loginAsHubAdmin(page: Page) {
  await page.addInitScript(() => {
    window.sessionStorage.setItem("aa_hub_role", "admin");
    window.sessionStorage.setItem("aa_hub_token", "admin-token");
    window.sessionStorage.setItem("aa_hub_username", "security-admin");
  });
}

test.describe("Hub Console", () => {
  test.beforeEach(async ({ page }) => {
    await loginAsHubAdmin(page);
  });

  test("renders dashboard", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByRole("heading", { name: /Dashboard/i })).toBeVisible();
  });

  test("shows hub-only navigation", async ({ page }) => {
    await page.goto("/");
    const aside = page.locator("aside");
    await expect(aside.getByRole("link", { name: "Dashboard" })).toBeVisible();
    await expect(aside.getByRole("link", { name: "Sessions" })).toBeVisible();
    await expect(aside.getByRole("link", { name: "Approvals" })).toBeVisible();
    await expect(aside.getByRole("link", { name: "Search" })).toBeVisible();
    await expect(aside.getByRole("link", { name: "Export" })).toBeVisible();
    await expect(aside.getByRole("link", { name: "Policies" })).toBeVisible();
    await expect(aside.getByRole("link", { name: "Analytics" })).toBeVisible();
    // Hub does NOT show developer-personal pages
    await expect(aside.getByRole("link", { name: "My Activity" })).not.toBeVisible();
  });

  test("shows pending approvals badge from pending_count", async ({ page }) => {
    await page.route("**/v1/approvals/metrics*", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          total_created: 5,
          total_approved: 1,
          total_denied: 1,
          total_expired: 0,
          pending_count: 3,
        }),
      });
    });

    await page.goto("/");
    const approvalsLink = page.locator("aside a", { hasText: "Approvals" });
    await expect(approvalsLink.locator("span", { hasText: "3" })).toBeVisible();
  });

  test("approvals page renders", async ({ page }) => {
    await page.goto("/approvals");
    await expect(page.getByRole("heading", { name: /Approvals/i })).toBeVisible();
  });

  test("policies page renders", async ({ page }) => {
    await page.goto("/policies");
    await expect(page.getByRole("heading", { name: /Policies/i })).toBeVisible();
  });

  test("analytics page renders", async ({ page }) => {
    await page.goto("/analytics");
    await expect(page.getByRole("heading", { name: /Analytics/i })).toBeVisible();
  });

  test("analytics group drill-down routes to supported destination", async ({ page }) => {
    await page.route("**/v1/analytics/groups*", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          groups: [{ id: "platform-team", name: "Platform Team", icon: "🛠️", member_count: 4, avg_block_rate: 12.5 }],
        }),
      });
    });

    await page.goto("/analytics");
    await page.getByText("Platform Team").click();
    await expect(page).toHaveURL(/\/search/);
  });
});
