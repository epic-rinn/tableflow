import { expect, test } from "@playwright/test";

// ACC-001 / ACC-004: QR fragment → POST exchange, history cleanup, HttpOnly
// guest cookie, multi-diner reuse. Requires E2E_VISIT_TOKEN from `make verify`.
const visitToken = process.env.E2E_VISIT_TOKEN ?? "";

test.describe("QR entry", () => {
  test.skip(!visitToken, "E2E_VISIT_TOKEN not set; run `make verify`");

  test("dining QR connects without exposing the token", async ({ page, context }) => {
    const urls: string[] = [];
    page.on("request", (r) => urls.push(r.url() + " " + (r.headers()["referer"] ?? "")));

    await page.goto(`/t#${visitToken}`);
    await expect(page.getByText("You are connected to your table.")).toBeVisible();
    await expect(page.getByRole("heading", { name: "Table E2E-1" })).toBeVisible();
    expect(new URL(page.url()).hash).toBe("");
    expect(await page.evaluate(() => window.history.length)).toBeLessThanOrEqual(2);
    for (const u of urls) expect(u).not.toContain(visitToken);

    const cookie = (await context.cookies()).find((c) => c.name === "__Host-tf_guest");
    expect(cookie).toMatchObject({ httpOnly: true, secure: true, sameSite: "Lax", path: "/" });
    expect(await page.evaluate(() => document.cookie)).not.toContain("tf_guest");
    const storage = await page.evaluate(() => JSON.stringify({ ...localStorage }) + JSON.stringify({ ...sessionStorage }));
    expect(storage).not.toContain(visitToken);

    const session = await page.evaluate(async () => {
      const r = await fetch("/api/v1/sessions/guest");
      return { status: r.status, body: await r.json() };
    });
    expect(session.status).toBe(200);
    expect(session.body.kind).toBe("visit");
  });

  test("a second diner can use the same QR code", async ({ browser }) => {
    const other = await browser.newContext();
    const page = await other.newPage();
    await page.goto(`/t#${visitToken}`);
    await expect(page.getByText("You are connected to your table.")).toBeVisible();
    await expect(page.getByRole("heading", { name: "Table E2E-1" })).toBeVisible();
    await other.close();
  });

  test("invalid, missing and wrong-kind codes are explained", async ({ page }) => {
    await page.goto("/t#not-a-real-token");
    await expect(page.getByRole("main").getByRole("alert")).toContainText("no longer valid");
    await page.goto("/q");
    await expect(page.getByRole("main").getByRole("alert")).toContainText("Scan the QR code again");
    await page.goto(`/q#${visitToken}`);
    await expect(page.getByRole("main").getByRole("alert")).toContainText("no longer valid");
  });
});
