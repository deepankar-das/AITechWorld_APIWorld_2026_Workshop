/**
 * Author: Deepankar Das
 */

/**
 * Enforcer — Playwright E2E Configuration
 *
 * Tests the review console (Next.js) against a running daemon.
 * Daemon must be running on localhost:9100.
 * Console dev server started automatically by Playwright.
 */

import { defineConfig, devices } from "@playwright/test";

const CONSOLE_PORT = 6100;
const HUB_CONSOLE_PORT = 6101;
const DAEMON_PORT = 9100;

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: "html",
  timeout: 15000,

  use: {
    baseURL: `http://localhost:${CONSOLE_PORT}`,
    trace: "on-first-retry",
  },

  projects: [
    {
      name: "hub-chromium",
      testMatch: /hub-console\.spec\.ts/,
      use: {
        ...devices["Desktop Chrome"],
        baseURL: `http://localhost:${HUB_CONSOLE_PORT}`,
      },
    },
    {
      name: "sentinel-chromium",
      testMatch: /sentinel-console\.spec\.ts/,
      use: {
        ...devices["Desktop Chrome"],
        baseURL: `http://localhost:${CONSOLE_PORT}`,
      },
    },
  ],

  webServer: [
    {
      command: `cd console && npm run build:all && cd out-hub && python3 -m http.server ${HUB_CONSOLE_PORT}`,
      port: HUB_CONSOLE_PORT,
      reuseExistingServer: !process.env.CI,
      timeout: 120000,
      env: {
        NEXT_PUBLIC_HUB_API_URL: `http://127.0.0.1:${DAEMON_PORT}`,
        NEXT_PUBLIC_DAEMON_URL: `http://127.0.0.1:${DAEMON_PORT}`,
      },
    },
    {
      command: `bash -lc 'while [ ! -d console/out-sentinel ]; do sleep 1; done; cd console/out-sentinel && python3 -m http.server ${CONSOLE_PORT}'`,
      port: CONSOLE_PORT,
      reuseExistingServer: !process.env.CI,
      timeout: 120000,
    },
  ],
});
