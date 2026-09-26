import { expect, test, type Page } from "@playwright/test";
import { latestLink } from "./support/mail";

// MVP-18: PWA-004 Thai/English and keyboard use, PWA-A2 polling, PWA-003
// recovery without duplicates, and member/anonymous switching privacy.
const token = process.env.E2E_VISIT_TOKEN ?? "";
const enabled = !!token && !!process.env.E2E_MAIL_DIR;

async function setHidden(page: Page, hidden: boolean) {
  await page.evaluate((h) => {
    Object.defineProperty(document, "hidden", { value: h, configurable: true });
    Object.defineProperty(document, "visibilityState", { value: h ? "hidden" : "visible", configurable: true });
    document.dispatchEvent(new Event("visibilitychange"));
  }, hidden);
}

test.describe("guest journey hardening", () => {
  test.skip(!enabled, "run through `make verify`");

  test("Thai by default for Thai phones, switchable to English, THB formatting", async ({ browser }) => {
    const context = await browser.newContext({ locale: "th-TH" });
    const page = await context.newPage();
    await page.goto(`/t#${token}`);
    await expect(page.getByRole("heading", { name: "โต๊ะ E2E-1" })).toBeVisible();
    await expect(page.locator("html")).toHaveAttribute("lang", "th");
    await expect(page.getByRole("button", { name: "เพิ่ม ชาไทย" })).toBeVisible();
    await expect(page.getByRole("article", { name: "Thai Tea" })).toContainText("฿60.00");
    await expect(page.getByRole("region", { name: "ต้องการอะไรเพิ่มไหม?" })).toContainText("แอปไม่สามารถยืนยันได้ว่าอาหารปลอดภัย");

    await page.getByRole("button", { name: "Switch to English" }).click();
    await expect(page.getByRole("heading", { name: "Table E2E-1" })).toBeVisible();
    await expect(page.locator("html")).toHaveAttribute("lang", "en");
    await page.reload(); // the choice is remembered on this phone
    await expect(page.getByRole("heading", { name: "Table E2E-1" })).toBeVisible();
    await context.close();
  });

  test("an order can be placed with the keyboard alone", async ({ page }) => {
    await page.goto(`/t#${token}`);
    await expect(page.getByRole("heading", { name: "Table E2E-1" })).toBeVisible();
    await page.getByRole("button", { name: "Add Thai Tea" }).focus();
    await page.keyboard.press("Enter");
    const cartBar = page.getByRole("button", { name: /^View cart · 1 item/ });
    await cartBar.focus();
    await page.keyboard.press("Enter");
    const cart = page.getByRole("dialog");
    await expect(cart).toBeVisible();
    await expect(cart.locator(":focus")).toHaveCount(1); // focus moved into the sheet
    await cart.getByRole("button", { name: /^Send order/ }).focus();
    await page.keyboard.press("Enter");
    await expect(page.getByRole("main").getByRole("status").filter({ hasText: "sent to the kitchen" })).toBeVisible();
  });

  test("polling pauses in hidden tabs and resumes on focus and reconnect (PWA-A2)", async ({ page, context }) => {
    const seen: number[] = [];
    page.on("request", (r) => {
      if (r.method() === "GET" && /\/api\/v1\/visits\/[^/]+\/orders/.test(r.url())) seen.push(Date.now());
    });
    await page.goto(`/t#${token}`);
    await expect(page.getByRole("heading", { name: "Table E2E-1" })).toBeVisible();
    await setHidden(page, true);
    await page.waitForTimeout(1000);
    const hiddenAt = seen.length;
    await page.waitForTimeout(13_000); // longer than the 10 s ±20% interval
    expect(seen.length - hiddenAt).toBe(0);
    await setHidden(page, false);
    await expect.poll(() => seen.length, { timeout: 2000 }).toBeGreaterThan(hiddenAt);

    const beforeOffline = seen.length;
    await context.setOffline(true);
    await page.waitForTimeout(500);
    await context.setOffline(false); // the "online" event fetches at once
    await expect.poll(() => seen.length, { timeout: 3000 }).toBeGreaterThan(beforeOffline);
  });

  test("a lost response is retried without ordering twice (PWA-003)", async ({ page }) => {
    await page.goto(`/t#${token}`);
    await expect(page.getByRole("heading", { name: "Table E2E-1" })).toBeVisible();
    // Count confirmed Iced Coffee lines from the API (the source of truth).
    const coffees = () =>
      page.evaluate(async () => {
        const session = await (await fetch("/api/v1/sessions/guest")).json();
        const page = await (await fetch(`/api/v1/visits/${session.resource_id}/orders?limit=100`)).json();
        return (page.items as { lines: { name_en: string }[] }[]).flatMap((o) => o.lines).filter((l) => l.name_en === "Iced Coffee").length;
      });
    const before = await coffees();

    await page.getByRole("button", { name: "Add Iced Coffee" }).click();
    await page.getByRole("button", { name: /^View cart · 1 item/ }).click();
    // The server commits the order, but the phone never sees the response.
    let dropped = false;
    await page.route(/\/api\/v1\/visits\/[^/]+\/orders$/, async (route) => {
      if (route.request().method() === "POST" && !dropped) {
        dropped = true;
        await route.fetch();
        await route.abort("connectionreset");
        return;
      }
      await route.continue();
    });
    await page.getByRole("dialog").getByRole("button", { name: /^Send order/ }).click();
    await expect(page.getByRole("main").getByRole("alert")).toContainText("it will not be ordered twice");
    // The cart is kept; the guest retries with the same idempotency key.
    await page.getByRole("button", { name: /^View cart · 1 item/ }).click();
    await page.getByRole("dialog").getByRole("button", { name: /^Send order/ }).click();
    await expect(page.getByRole("main").getByRole("status").filter({ hasText: "sent to the kitchen" })).toBeVisible();
    await expect.poll(coffees).toBe(before + 1);
  });

  test("signing out removes member data from this phone", async ({ page }) => {
    const email = `switch-${Date.now()}@e2e.test`;
    await page.goto("/account/signup");
    await page.getByLabel("Email").fill(email);
    await page.getByLabel("Password (at least 12 characters)").fill("switching e2e password");
    await page.getByRole("button", { name: "Create account" }).click();
    await page.goto(await latestLink(email, "/account/verify"));
    await expect(page.getByRole("main").getByRole("status")).toContainText("Your email is confirmed.");
    await page.goto("/account/login");
    await page.getByLabel("Email").fill(email);
    await page.getByLabel("Password").fill("switching e2e password");
    await page.getByRole("button", { name: "Sign in" }).click();
    await expect(page.getByText(`Signed in as ${email}`)).toBeVisible();

    await page.getByRole("button", { name: "Sign out" }).click();
    await expect(page.getByRole("link", { name: "Sign in" })).toBeVisible();
    expect(await page.evaluate(async () => (await fetch("/api/v1/members/me")).status)).toBe(401);
    await page.goto(`/t#${token}`);
    await expect(page.getByRole("region", { name: "Member points" }).getByRole("link", { name: "Sign in to earn points" })).toBeVisible();
    await expect(page.locator("body")).not.toContainText(email);
    await page.goto("/account");
    await expect(page.locator("body")).not.toContainText(email);
  });
});
