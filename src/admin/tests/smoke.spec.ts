import { expect, test } from "@playwright/test";

// AdminAndPwaBrowserSmoke (admin): the production build renders, sends baseline
// security headers, and routes same-origin /api/v1 to the Go API.

test("home page renders without console errors", async ({ page }) => {
  const errors: string[] = [];
  page.on("console", (m) => {
    if (m.type() === "error") errors.push(m.text());
  });
  page.on("pageerror", (e) => errors.push(e.message));

  const response = await page.goto("/");
  expect(response?.status()).toBe(200);
  const headers = response!.headers();
  expect(headers["x-content-type-options"]).toBe("nosniff");
  expect(headers["referrer-policy"]).toBe("no-referrer");
  expect(headers["x-powered-by"]).toBeUndefined();

  // Anonymous visitors are sent to sign-in.
  await expect(page).toHaveURL(/\/login$/);
  await expect(page.getByRole("heading", { level: 1, name: "Staff sign in", exact: true })).toBeVisible();
  expect(errors).toEqual([]);
});

test("same-origin /api/v1 reaches the Go API", async ({ request }) => {
  const live = await request.get("/api/v1/health/live");
  expect(live.status()).toBe(200);
  expect(await live.json()).toEqual({ status: "ok" });
  // Only the Go middleware sets a request ID; its presence proves the proxy hop.
  expect(live.headers()["x-request-id"]).toMatch(/^[a-f0-9]{32}$/);
  expect(live.headers()["cache-control"]).toBe("no-store");

  const missing = await request.get("/api/v1/not-a-route");
  expect(missing.status()).toBe(404);
  expect((await missing.json()).error.code).toBe("NOT_FOUND");
});

test("no service worker is registered", async ({ page }) => {
  await page.goto("/");
  const count = await page.evaluate(async () =>
    "serviceWorker" in navigator ? (await navigator.serviceWorker.getRegistrations()).length : 0,
  );
  expect(count).toBe(0);
});
