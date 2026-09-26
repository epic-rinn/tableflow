import { expect, test, type Page } from "@playwright/test";

// MVP-17: PWA-001 installable; PWA-A1 offline never restores private data and
// caches hold only public static assets; PWA-A3 updates keep the cart.
const token = process.env.E2E_VISIT_TOKEN ?? "";

async function controlled(page: Page) {
  await page.evaluate(() => navigator.serviceWorker.ready);
  if (!(await page.evaluate(() => !!navigator.serviceWorker.controller))) await page.reload();
  await expect.poll(() => page.evaluate(() => !!navigator.serviceWorker.controller)).toBe(true);
}

async function cachedPaths(page: Page) {
  return page.evaluate(async () => {
    const out: string[] = [];
    for (const name of await caches.keys()) {
      for (const req of await (await caches.open(name)).keys()) out.push(new URL(req.url).pathname + new URL(req.url).search);
    }
    return out;
  });
}

test.describe("installable PWA", () => {
  test.skip(!token, "run through `make verify`");

  test("manifest, icons and a controlling service worker", async ({ page, request }) => {
    await page.goto("/");
    const href = await page.locator('link[rel="manifest"]').getAttribute("href");
    expect(href).toBe("/manifest.webmanifest");
    const manifest = await (await request.get(href!)).json();
    expect(manifest).toMatchObject({ name: "TableFlow", display: "standalone", start_url: "/" });
    const sizes = (manifest.icons as { sizes: string; purpose: string; src: string }[]).map((i) => `${i.sizes}/${i.purpose}`);
    expect(sizes).toEqual(expect.arrayContaining(["192x192/any", "512x512/any", "512x512/maskable"]));
    for (const icon of manifest.icons) {
      const r = await request.get(icon.src);
      expect(r.status()).toBe(200);
      expect(r.headers()["content-type"]).toBe("image/png");
    }
    const sw = await request.get("/sw.js");
    expect(sw.headers()["cache-control"]).toBe("no-cache");
    await controlled(page);
  });

  test("caches hold only public static assets; offline shows the generic page, never a bill", async ({ page, context }) => {
    await page.goto(`/t#${token}`);
    await controlled(page);
    await expect(page.getByRole("heading", { name: "Table E2E-1" })).toBeVisible();
    const bill = page.getByRole("region", { name: "Bill" });
    await bill.getByRole("button", { name: "View bill" }).click();
    await expect(bill).toContainText("Total");
    await page.goto("/account");
    await page.goto("/t");
    await expect(page.getByRole("heading", { name: "Table E2E-1" })).toBeVisible();

    const paths = await cachedPaths(page);
    expect(paths.length).toBeGreaterThan(0);
    for (const p of paths) expect(p).toMatch(/^\/(_next\/static\/|icons\/|offline\.html$|manifest\.webmanifest$)/);

    await context.setOffline(true);
    for (const path of ["/t", "/account", "/q"]) {
      await page.goto(path);
      await expect(page.getByRole("heading", { level: 1 })).toContainText("You are offline");
      await expect(page.locator("body")).not.toContainText("ชาไทย");
      await expect(page.locator("body")).not.toContainText("Total");
    }
    await context.setOffline(false);
  });

  test("an update waits for the guest and keeps the cart", async ({ page }) => {
    await page.goto(`/t#${token}`);
    await controlled(page);
    await page.getByRole("button", { name: "Add Thai Tea" }).click();
    const cartBar = page.getByRole("button", { name: /^View cart · 1 item/ });
    await expect(cartBar).toBeVisible();

    // Simulate a new deployment: a new worker version for the same scope.
    await page.evaluate(() => navigator.serviceWorker.register("/sw.js?v=update-test", { scope: "/" }));
    const banner = page.getByRole("status").filter({ hasText: "An update is available" });
    await expect(banner).toBeVisible();
    await expect(cartBar).toBeVisible(); // nothing applied silently
    await banner.getByRole("button", { name: "Reload to update" }).click();
    await page.waitForEvent("load");
    await expect(page.getByRole("heading", { name: "Table E2E-1" })).toBeVisible();
    await expect(page.getByRole("button", { name: /^View cart · 1 item/ })).toBeVisible();
    expect(await page.evaluate(() => navigator.serviceWorker.controller?.scriptURL)).toContain("v=update-test");
  });
});
