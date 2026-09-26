import { defineConfig, devices } from "@playwright/test";

const port = Number(process.env.SMOKE_PORT ?? 3000);
const baseURL = `http://127.0.0.1:${port}`;

// Browser smoke tests run against the production build. The Go API must be
// running at API_INTERNAL_URL (see docs/development/setup.md).
export default defineConfig({
  testDir: "./tests",
  // Failure artifacts (including Markdown error context) stay out of src/.
  outputDir: "../../tmp/playwright/pwa",
  forbidOnly: true,
  retries: 0,
  // Fail fast instead of hanging if the server or a browser stalls.
  globalTimeout: 300_000,
  timeout: 30_000,
  reporter: "list",
  use: { baseURL, trace: "retain-on-failure" },
  projects: [
    {
      name: "chromium",
      // PLAYWRIGHT_CHANNEL=chrome uses an installed Google Chrome when the
      // bundled Chromium cannot be downloaded.
      // UI-A2: guests use phones, so every journey runs at 390×844 with touch.
      use: {
        ...devices["Desktop Chrome"],
        viewport: { width: 390, height: 844 },
        isMobile: true,
        hasTouch: true,
        deviceScaleFactor: 2,
        channel: process.env.PLAYWRIGHT_CHANNEL || undefined,
      },
    },
  ],
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
