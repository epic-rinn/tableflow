import { expect, test } from "@playwright/test";

// MVP-21: security, session and cache checks on the deployed HTTPS origins
// (ACC-004, ADM-006, PWA-001/002, security spec).
const ADMIN = process.env.STAGING_ADMIN ?? "";
const PWA = process.env.STAGING_PWA ?? "";
const TOKEN = process.env.STAGING_ACTIVATION_TOKEN ?? "";
const BRANCH = process.env.STAGING_BRANCH_ID ?? "";
const PASSWORD = "staging rehearsal password";

test.describe.serial("deployed origins", () => {
  test.skip(!ADMIN || !PWA || !BRANCH, "set STAGING_ADMIN, STAGING_PWA and STAGING_BRANCH_ID");

  test("HTTPS with HSTS and baseline headers on both origins", async ({ request }) => {
    for (const origin of [ADMIN, PWA]) {
      expect(origin).toMatch(/^https:\/\//);
      const r = await request.get(`${origin}/`);
      expect(r.status()).toBeLessThan(400);
      const h = r.headers();
      expect(h["strict-transport-security"]).toContain("max-age=");
      expect(h["x-content-type-options"]).toBe("nosniff");
      expect(h["referrer-policy"]).toBe("no-referrer");
      expect(h["x-powered-by"]).toBeUndefined();
      const ready = await request.get(`${origin}/api/v1/health/ready`);
      expect(ready.status()).toBe(200);
    }
  });

  test("staff session: host-only secure cookie, private responses, origin guard, no service worker", async ({ browser }) => {
    test.skip(!TOKEN, "set STAGING_ACTIVATION_TOKEN (one-time) to exercise staff sign-in");
    const context = await browser.newContext();
    const page = await context.newPage();
    await page.goto(`${ADMIN}/activate#${TOKEN}`);
    await page.getByLabel("New password (at least 12 characters)").fill(PASSWORD);
    await page.getByLabel("Confirm password").fill(PASSWORD);
    await page.getByRole("button", { name: "Activate account" }).click();
    await expect(page.getByRole("status")).toContainText("Your password is set");
    await page.goto(`${ADMIN}/login`);
    await page.getByLabel("Email").fill(process.env.STAGING_MANAGER_EMAIL ?? "manager@staging.test");
    await page.getByLabel("Password").fill(PASSWORD);
    await page.getByRole("button", { name: "Sign in" }).click();
    await expect(page.getByRole("heading", { level: 1 })).toContainText("Welcome");

    const cookie = (await context.cookies(ADMIN)).find((c) => c.name === "__Host-tf_staff");
    expect(cookie).toMatchObject({ secure: true, httpOnly: true, sameSite: "Strict", path: "/", domain: new URL(ADMIN).hostname });
    expect((await context.cookies(PWA)).some((c) => c.name.startsWith("__Host-tf_staff"))).toBe(false);

    const cache = await page.evaluate(async () => (await fetch("/api/v1/sessions/current")).headers.get("cache-control"));
    expect(cache).toBe("private, no-store");
    const forged = await page.request.post(`${ADMIN}/api/v1/branches/${BRANCH}/staff`, {
      data: { email: "x@staging.test", display_name: "X", roles: ["manager"] },
      headers: { Origin: "https://evil.example", Cookie: `__Host-tf_staff=${cookie!.value}` },
    });
    expect(forged.status()).toBe(403);
    expect(await page.evaluate(async () => (await navigator.serviceWorker.getRegistrations()).length)).toBe(0);
    await context.close();
  });

  test("guest PWA: service worker over HTTPS, public-only caches, guest cookies isolated", async ({ browser }) => {
    const context = await browser.newContext();
    const page = await context.newPage();
    await page.goto(`${PWA}/join/${BRANCH}`);
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await page.evaluate(() => navigator.serviceWorker.ready);
    await page.reload();
    await expect.poll(() => page.evaluate(() => !!navigator.serviceWorker.controller)).toBe(true);

    const anon = (await context.cookies(PWA)).find((c) => c.name === "__Host-tf_anon");
    expect(anon).toMatchObject({ secure: true, httpOnly: true, sameSite: "Lax", path: "/", domain: new URL(PWA).hostname });
    expect((await context.cookies(ADMIN)).length).toBe(0);

    const paths = await page.evaluate(async () => {
      const out: string[] = [];
      for (const name of await caches.keys()) for (const r of await (await caches.open(name)).keys()) out.push(new URL(r.url).pathname);
      return out;
    });
    for (const p of paths) expect(p).toMatch(/^\/(_next\/static\/|icons\/|offline\.html$|manifest\.webmanifest$)/);
    const manifest = await page.request.get(`${PWA}/manifest.webmanifest`);
    expect(manifest.status()).toBe(200);
    await context.close();
  });
});
