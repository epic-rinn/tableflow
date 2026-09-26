import { expect, test, type Page } from "@playwright/test";

// QUE-001/002/004, PWA-005: join from the entrance link, track, cancel.
const branch = process.env.E2E_BRANCH_ID ?? "";

async function setHidden(page: Page, hidden: boolean) {
  await page.evaluate((h) => {
    Object.defineProperty(document, "hidden", { value: h, configurable: true });
    Object.defineProperty(document, "visibilityState", { value: h ? "hidden" : "visible", configurable: true });
    document.dispatchEvent(new Event("visibilitychange"));
  }, hidden);
}

test.describe("queue", () => {
  test.skip(!branch, "run through `make verify`");

  test("join, see group position, cancel", async ({ page }) => {
    await page.goto(`/join/${branch}`);
    await page.getByLabel("Number of people").fill("2");
    await page.getByLabel("High chair").check();
    await page.getByRole("button", { name: "Join the queue" }).click();
    await expect(page).toHaveURL(/\/q$/); // token removed from the address bar
    await expect(page.getByRole("heading", { name: /^Ticket \d+$/ })).toBeVisible();
    // Located by text within the ticket region, not by CSS class (UI-004).
    const ticket = page.getByRole("region", { name: /^Ticket \d+$/ });
    await expect(ticket.getByText(/in your group\.$/)).toBeVisible();
    await expect(ticket.getByText("not an exact order or wait time", { exact: false })).toBeVisible();

    await page.getByRole("button", { name: "Cancel ticket" }).click();
    await page.getByRole("button", { name: "Yes, cancel my ticket" }).click();
    await expect(page.getByText("This ticket was cancelled.")).toBeVisible();
  });

  test("tracking stops polling while hidden and refreshes on return", async ({ page }) => {
    const seen: number[] = [];
    page.on("request", (r) => {
      if (r.method() === "GET" && /\/api\/v1\/queue-tickets\//.test(r.url())) seen.push(Date.now());
    });
    await page.goto(`/join/${branch}`);
    await page.getByRole("button", { name: "Join the queue" }).click();
    await expect(page.getByRole("heading", { name: /^Ticket \d+$/ })).toBeVisible();
    await setHidden(page, true);
    const before = seen.length;
    await page.waitForTimeout(13_000); // longer than one 10 s ±20% interval
    expect(seen.length).toBe(before);
    await setHidden(page, false);
    await expect.poll(() => seen.length, { timeout: 1500 }).toBeGreaterThan(before);
  });

  test("oversized party is directed to staff", async ({ page }) => {
    await page.goto(`/join/${branch}`);
    await page.getByLabel("Number of people").fill("12");
    await page.getByRole("button", { name: "Join the queue" }).click();
    await expect(page.getByRole("main").getByRole("alert")).toContainText("ask a staff member");
  });
});
