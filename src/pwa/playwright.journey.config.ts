import { defineConfig, devices } from "@playwright/test";

// Cross-app journey (MVP-19, ADM-A3): staff use the admin app while guests
// use phones on the PWA, against one Go API. Both production builds start
// here; the API and E2E database come from `make verify`.
const PWA = "http://127.0.0.1:3000";
const ADMIN = "http://127.0.0.1:3001";
const api = { API_INTERNAL_URL: process.env.API_INTERNAL_URL ?? "http://127.0.0.1:8080" };
const shutdown = { signal: "SIGTERM" as const, timeout: 5_000 };

export default defineConfig({
  testDir: "./journey",
  outputDir: "../../tmp/playwright/journey",
  forbidOnly: true,
  retries: 0,
  workers: 1,
  globalTimeout: 300_000,
  timeout: 120_000,
  reporter: "list",
  use: { trace: "retain-on-failure", channel: process.env.PLAYWRIGHT_CHANNEL || undefined },
  projects: [{ name: "journey", use: { ...devices["Desktop Chrome"], channel: process.env.PLAYWRIGHT_CHANNEL || undefined } }],
  webServer: [
    { command: "pnpm exec next start --hostname 127.0.0.1 --port 3000", url: PWA, reuseExistingServer: false, timeout: 60_000, env: api, gracefulShutdown: shutdown },
    { command: "pnpm exec next start --hostname 127.0.0.1 --port 3001", cwd: "../admin", url: ADMIN, reuseExistingServer: false, timeout: 60_000, env: api, gracefulShutdown: shutdown },
  ],
});
