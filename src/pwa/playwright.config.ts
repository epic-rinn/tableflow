import { defineConfig, devices } from "@playwright/test";

const port = Number(process.env.SMOKE_PORT ?? 3000);
const baseURL = `http://127.0.0.1:${port}`;

// Browser smoke tests run against the production build. The Go API must be
// running at API_INTERNAL_URL (see docs/development/setup.md).
export default defineConfig({
  testDir: "./tests",
  forbidOnly: true,
  retries: 0,
  // Fail fast instead of hanging if the server or a browser stalls.
  globalTimeout: 180_000,
  timeout: 30_000,
  reporter: "list",
  use: { baseURL, trace: "retain-on-failure" },
  projects: [
    {
      name: "chromium",
      // PLAYWRIGHT_CHANNEL=chrome uses an installed Google Chrome when the
      // bundled Chromium cannot be downloaded.
      use: { ...devices["Desktop Chrome"], channel: process.env.PLAYWRIGHT_CHANNEL || undefined },
    },
  ],
  webServer: {
    command: `pnpm exec next start --hostname 127.0.0.1 --port ${port}`,
    url: baseURL,
    reuseExistingServer: false,
    timeout: 60_000,
    stdout: "pipe",
    stderr: "pipe",
    env: { API_INTERNAL_URL: process.env.API_INTERNAL_URL ?? "http://127.0.0.1:8080" },
  },
});
