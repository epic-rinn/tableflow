import { expect, test } from "@playwright/test";

// ORD-001/003/005/006 from two phones at the seeded E2E-1 visit.
const token = process.env.E2E_VISIT_TOKEN ?? "";

test.describe.serial("dining", () => {
  test.skip(!token, "run through `make verify`");

  test("two phones keep separate carts and share confirmed orders", async ({ browser }) => {
    const a = await (await browser.newContext()).newPage();
    const b = await (await browser.newContext()).newPage();
    for (const p of [a, b]) {
      await p.goto(`/t#${token}`);
      await expect(p.getByRole("heading", { name: "Table E2E-1" })).toBeVisible();
    }
    await a.getByRole("button", { name: "Add Thai Tea" }).click();
    // UI-003: the cart lives behind a sticky "View cart" bar and opens as a sheet.
    const cartBar = (p: typeof a) => p.getByRole("button", { name: /^View cart · \d+ items?/ });
    await expect(cartBar(a)).toContainText("1 item");
    await expect(cartBar(b)).toHaveCount(0); // the other phone's cart stays empty
    await cartBar(a).click();
    await expect(a.getByRole("region", { name: "Your cart (this phone)" })).toContainText("ชาไทย");
    await a.keyboard.press("Escape");

    // The cart survives a reload of the same phone.
    await a.reload();
    await expect(cartBar(a)).toContainText("1 item");
    await cartBar(a).click();
    await expect(a.getByRole("region", { name: "Your cart (this phone)" })).toContainText("ชาไทย");

    await a.getByRole("button", { name: /^Send order/ }).click();
    await expect(a.getByRole("main").getByRole("status").filter({ hasText: "sent to the kitchen" })).toBeVisible();
    await expect(cartBar(a)).toHaveCount(0);

    await b.reload();
    const shared = b.getByRole("region", { name: "Table orders" });
    await expect(shared).toContainText("ชาไทย");
    await expect(shared).toContainText("Sent");
  });

  test("allergy question shows as waiting for staff without a safety claim", async ({ page }) => {
    await page.goto(`/t#${token}`);
    await page.getByLabel("Allergy details (optional)").fill("peanuts");
    await page.getByRole("button", { name: "Allergy question" }).click();
    const help = page.getByRole("region", { name: "Need something?" });
    await expect(help).toContainText("Allergy question: waiting for staff");
    await expect(help).toContainText("cannot confirm that a dish is safe");
  });

  test("guests can read the itemised bill but never settle it", async ({ page }) => {
    await page.goto(`/t#${token}`);
    const bill = page.getByRole("region", { name: "Bill" });
    await bill.getByRole("button", { name: "View bill" }).click();
    await expect(bill).toContainText("ชาไทย");
    await expect(bill).toContainText("Total");
    await expect(bill).toContainText("Please pay a staff member at the counter");
    await expect(bill.getByRole("button", { name: /pay|confirm/i })).toHaveCount(0);
  });
});
