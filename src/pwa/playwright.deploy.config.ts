import { defineConfig, devices } from "@playwright/test";

// Deployed-origin checks (MVP-21) against a running staging stack, e.g.
//   STAGING_ADMIN=https://admin.tableflow.localhost STAGING_PWA=https://pwa.tableflow.localhost \
//   STAGING_ACTIVATION_TOKEN=… STAGING_BRANCH_ID=… STAGING_INSECURE_TLS=1 pnpm exec playwright test -c playwright.deploy.config.ts
// STAGING_INSECURE_TLS=1 only for Caddy's local CA during a local rehearsal:
// Chrome refuses service-worker scripts on certificate errors even when the
// page ignores them, so the browser itself is started without verification.
const insecure = process.env.STAGING_INSECURE_TLS === "1";

export default defineConfig({
  testDir: "./deploy-checks",
  outputDir: "../../tmp/playwright/deploy",
  forbidOnly: true,
  retries: 0,
  workers: 1,
  timeout: 60_000,
  reporter: "list",
  use: {
    ignoreHTTPSErrors: insecure,
    launchOptions: insecure ? { args: ["--ignore-certificate-errors"] } : {},
    trace: "retain-on-failure",
  },
  projects: [{ name: "deploy", use: { ...devices["Desktop Chrome"], channel: process.env.PLAYWRIGHT_CHANNEL || undefined } }],
});
