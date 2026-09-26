import { expect, test, type Page } from "@playwright/test";

// MVP-11/12/13: charge policy → bill → begin → confirm cash → receipt →
// full refund → depart → clean (BIL-001..008, SEA-004).
const enabled = !!process.env.E2E_MANAGER_TOKEN;

async function signIn(page: Page) {
  await page.goto("/login");
  await page.getByLabel("Email").fill("manager@e2e.test");
  await page.getByLabel("Password").fill("e2e password that is long enough");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toContainText("Welcome");
}

const status = (page: Page, text: string | RegExp) => page.getByRole("main").getByRole("status").filter({ hasText: text });

test.describe.serial("cashier", () => {
  test.skip(!enabled, "run through `make verify`");

  test("settle, receipt, refund, depart and clean", async ({ page }) => {
    await signIn(page);

    // Operator-validated rates are entered by the manager (none by default).
    await page.goto("/charges");
    await expect(page.getByText("Not configured yet")).toBeVisible();
    await page.getByLabel("Service charge (%)").fill("10");
    await page.getByLabel("Tax (%)").fill("7");
    await page.getByRole("button", { name: "Save new version" }).click();
    await expect(status(page, "Saved as version 1")).toBeVisible();

    // Two Thai Teas: 120.00 + 10% = 132.00; 7% tax 9.24 → 141.24.
    await page.goto("/cashier");
    await page.getByRole("button", { name: "Table E2E-2" }).click();
    await expect(page.getByRole("heading", { name: "Table E2E-2 — open" })).toBeVisible();
    await expect(page.getByRole("main")).toContainText("141.24");
    await page.getByRole("button", { name: "Begin settlement" }).click();
    await expect(status(page, "Ordering is frozen")).toBeVisible();
    await expect(page.getByRole("heading", { name: /Table E2E-2 — settling/ })).toBeVisible();

    await page.getByLabel("Cash received (฿, optional)").fill("200");
    await expect(page.getByText("Change:")).toContainText("58.76");
    await page.getByLabel("Verification note").fill("counted at till");
    await page.getByRole("button", { name: /^Confirm .*141\.24 received$/ }).click();
    // UI-002: payment is confirmed once more in a dialog repeating the amount.
    const pay = page.getByRole("alertdialog", { name: /Record payment of .*141\.24/ });
    await pay.getByRole("button", { name: "Record payment" }).click();
    await expect(status(page, /Payment recorded\. Receipt R-[A-Z2-7]{10}/)).toBeVisible();
    await expect(page.getByRole("heading", { name: "Table E2E-2 — paid" })).toBeVisible();

    // Historical receipt and the single full refund (manager).
    await page.getByRole("link", { name: /^R-[A-Z2-7]{10}$/ }).click();
    await expect(page.getByRole("heading", { name: /^Receipt R-/ })).toBeVisible();
    await page.getByLabel("Reason").fill("guest complaint");
    await page.getByLabel("Refund reference").fill("BANK-123");
    await page.getByRole("button", { name: /Record full refund/ }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Record refund" }).click();
    await expect(status(page, "Full refund recorded")).toBeVisible();
    await expect(page.getByRole("main")).toContainText("Reason: guest complaint");
    await page.goto("/receipts");
    await expect(page.getByRole("row").filter({ hasText: "E2E-2" })).toContainText("Refunded");

    // Payment never releases the table: the host records departure, then cleaning.
    await page.goto("/host");
    const table = page.getByRole("article", { name: "Table E2E-2" });
    await expect(table).toContainText("occupied");
    await table.getByRole("button", { name: "Depart (paid)" }).click();
    await expect(table).toContainText("cleaning");
    await table.getByRole("button", { name: "Mark ready" }).click();
    await expect(table).toContainText("available");
  });
});
