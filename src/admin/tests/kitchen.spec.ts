import { expect, test, type Page } from "@playwright/test";

// ADM-002/003: assisted order → kitchen workflow → sold-out toggle → menu editor.
const enabled = !!process.env.E2E_MANAGER_TOKEN;

async function signIn(page: Page) {
  await page.goto("/login");
  await page.getByLabel("Email").fill("manager@e2e.test");
  await page.getByLabel("Password").fill("e2e password that is long enough");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toContainText("Welcome");
}

test.describe.serial("kitchen", () => {
  test.skip(!enabled, "run through `make verify`");

  test("assisted order flows through the kitchen", async ({ page }) => {
    await signIn(page);
    await page.goto("/host");
    await page.getByRole("link", { name: "Orders for E2E-1" }).click();
    await expect(page.getByRole("heading", { name: "Table E2E-1 orders" })).toBeVisible();
    await page.getByRole("button", { name: "Add Iced Coffee" }).click();
    await page.getByRole("button", { name: "Send order" }).click();
    await expect(page.getByRole("main").getByRole("status").filter({ hasText: "sent to the kitchen" })).toBeVisible();

    await page.goto("/kitchen");
    const card = page.getByRole("article", { name: "Iced Coffee for table E2E-1" });
    for (const step of ["Accept", "Start preparing", "Ready", "Served"]) {
      await card.getByRole("button", { name: step }).click();
      if (step !== "Served") await expect(card.getByRole("button", { name: { Accept: "Start preparing", "Start preparing": "Ready", Ready: "Served" }[step]! })).toBeVisible();
    }
    await expect(card).toHaveCount(0);
  });

  test("sold-out toggle and menu editor", async ({ page }) => {
    await signIn(page);
    await page.goto("/kitchen");
    const availability = page.getByRole("region", { name: "Availability" });
    await availability.getByRole("button", { name: "Mark Iced Coffee sold out" }).click();
    await expect(availability.getByRole("button", { name: "Mark Iced Coffee available" })).toBeVisible();
    await availability.getByRole("button", { name: "Mark Iced Coffee available" }).click();
    await expect(availability.getByRole("button", { name: "Mark Iced Coffee sold out" })).toBeVisible();

    await page.goto("/menu");
    await expect(page.getByText(/Revision \d+/)).toBeVisible();
    await page.getByRole("button", { name: "Save menu" }).click();
    await expect(page.getByRole("main").getByRole("status")).toContainText("Menu saved");
  });
});
