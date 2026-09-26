import { defineConfig, devices } from "@playwright/test";

const port = Number(process.env.SMOKE_PORT ?? 3001);
const baseURL = `http://127.0.0.1:${port}`;

// Browser smoke tests run against the production build. The Go API must be
// running at API_INTERNAL_URL (see docs/development/setup.md).
export default defineConfig({
  testDir: "./tests",
  // Failure artifacts (including Markdown error context) stay out of src/.
  outputDir: "../../tmp/playwright/admin",
  forbidOnly: true,
  retries: 0,
  // Fail fast instead of hanging if the server or a browser stalls.
  globalTimeout: 300_000,
  timeout: 30_000,
  reporter: "list",
  use: { baseURL, trace: "retain-on-failure" },
  projects: [
    // PLAYWRIGHT_CHANNEL=chrome uses an installed Google Chrome when the
    // bundled Chromium cannot be downloaded.
    // The staff spec activates the E2E manager that later specs sign in as.
    {
      name: "staff",
      testMatch: /staff\.spec\.ts/,
      use: { ...devices["Desktop Chrome"], channel: process.env.PLAYWRIGHT_CHANNEL || undefined },
    },
    {
      name: "chromium",
      testIgnore: /staff\.spec\.ts/,
      dependencies: ["staff"],
      use: { ...devices["Desktop Chrome"], channel: process.env.PLAYWRIGHT_CHANNEL || undefined },
    },
  ],
  workers: 1,
  webServer: {
    command: `pnpm exec next start --hostname 127.0.0.1 --port ${port}`,
    url: baseURL,
    reuseExistingServer: false,
    // Bounded teardown: SIGTERM, then a hard kill after 5 s, so a stuck
    // next-server can never outlive the run and block the next one.
    gracefulShutdown: { signal: "SIGTERM", timeout: 5_000 },
    timeout: 60_000,
    stdout: "pipe",
    stderr: "pipe",
    env: { API_INTERNAL_URL: process.env.API_INTERNAL_URL ?? "http://127.0.0.1:8080" },
  },
});
