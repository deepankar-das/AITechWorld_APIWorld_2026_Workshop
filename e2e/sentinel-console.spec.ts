/**
 * Author: Deepankar Das
 */

import { test, expect, type Page } from "@playwright/test";

async function loginAsSentinelOperator(page: Page) {
  await page.addInitScript(() => {
    window.sessionStorage.setItem("aa_sentinel_role", "operator");
    window.sessionStorage.setItem("aa_sentinel_token", "operator-token");
    window.sessionStorage.setItem("aa_sentinel_username", "developer-jane");
  });
}

test.describe("Sentinel Console", () => {
  test.beforeEach(async ({ page }) => {
    await loginAsSentinelOperator(page);
  });

  test("renders dashboard", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByRole("heading", { name: /Dashboard/i })).toBeVisible();
  });

  test("shows sentinel-only navigation", async ({ page }) => {
    await page.goto("/");
    const aside = page.locator("aside");
    await expect(aside.getByRole("link", { name: "Dashboard" })).toBeVisible();
    await expect(aside.getByRole("link", { name: "Sessions" })).toBeVisible();
    await expect(aside.getByRole("link", { name: "Search" })).toBeVisible();
    await expect(aside.getByRole("link", { name: "Export" })).toBeVisible();
    await expect(aside.getByRole("link", { name: "My Activity" })).toBeVisible();
    // Sentinel does NOT show admin-only pages
    await expect(aside.getByRole("link", { name: "Approvals" })).not.toBeVisible();
    await expect(aside.getByRole("link", { name: "Policies" })).not.toBeVisible();
    await expect(aside.getByRole("link", { name: "Analytics" })).not.toBeVisible();
  });

  test("sessions page renders", async ({ page }) => {
    await page.goto("/sessions");
    await expect(page.getByRole("heading", { name: /Sessions/i })).toBeVisible();
  });

  test("search page renders", async ({ page }) => {
    await page.goto("/search");
    await expect(page.getByRole("heading", { name: /Search/i })).toBeVisible();
  });

  test("export page renders", async ({ page }) => {
    await page.goto("/export");
    await expect(page.getByRole("heading", { name: /Export/i })).toBeVisible();
  });

  test("my activity renders", async ({ page }) => {
    await page.goto("/developer/me");
    await expect(page.getByText(/Compliance|Score|Activity/i).first()).toBeVisible();
  });

  test("approvals route is not part of sentinel app", async ({ page }) => {
    await page.goto("/approvals");
    await expect(page.getByText(/404|Not Found/i).first()).toBeVisible();
  });

  test("policies route is not part of sentinel app", async ({ page }) => {
    await page.goto("/policies");
    await expect(page.getByText(/404|Not Found/i).first()).toBeVisible();
  });
});
