import { expect, test, type Page } from "@playwright/test";

// ADM-002 / ADM-006 host journey on the E2E database (run by `make verify`
// after staff.spec activated the manager).
const enabled = !!process.env.E2E_MANAGER_TOKEN;
const PASSWORD = "e2e password that is long enough";

async function signIn(page: Page) {
  await page.goto("/login");
  await page.getByLabel("Email").fill("manager@e2e.test");
  await page.getByLabel("Password").fill(PASSWORD);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toContainText("Welcome");
}

function countRequests(page: Page, pattern: RegExp) {
  const seen: number[] = [];
  page.on("request", (r) => {
    if (r.method() === "GET" && pattern.test(r.url())) seen.push(Date.now());
  });
  return seen;
}

async function setHidden(page: Page, hidden: boolean) {
  await page.evaluate((h) => {
    Object.defineProperty(document, "hidden", { value: h, configurable: true });
    Object.defineProperty(document, "visibilityState", { value: h ? "hidden" : "visible", configurable: true });
    document.dispatchEvent(new Event("visibilitychange"));
  }, hidden);
}

test.describe.serial("host workspace", () => {
  test.skip(!enabled, "run through `make verify`");

  test("configure tables, then queue → call → seat → move → close → ready", async ({ page }) => {
    await signIn(page);
    await page.goto("/configuration");
    for (const [label, seats] of [["H2", "2"], ["H4", "4"]]) {
      const form = page.locator("form").filter({ has: page.getByRole("button", { name: "Add table" }) });
      await form.getByLabel("Label").fill(label);
      await form.getByLabel("Seats").fill(seats);
      await form.getByRole("button", { name: "Add table" }).click();
      await expect(page.getByRole("main").getByRole("status")).toContainText(`Table ${label} added.`);
    }

    await page.goto("/host");
    await page.getByLabel("Party size").fill("2");
    await page.getByRole("button", { name: "Add to queue" }).click();
    const tracking = page.getByLabel(/tracking link/);
    await expect(tracking).toHaveValue(/\/q#[A-Za-z0-9_-]{43}$/);
    await page.getByRole("button", { name: "Done" }).click();

    const ticket = page.getByRole("article", { name: /^Ticket \d+$/ }).first();
    await ticket.getByRole("combobox").selectOption({ label: "H2 (2)" });
    await ticket.getByRole("button", { name: "Call" }).click();
    await expect(ticket).toContainText("called to H2");
    await expect(page.getByRole("article", { name: "Table H2" })).toContainText("held for");

    await ticket.getByRole("button", { name: "Seat" }).click();
    await expect(page.getByLabel(/Dining QR for table H2/)).toHaveValue(/\/t#[A-Za-z0-9_-]{43}$/);
    // MVP-18: the one-time link is also a scannable QR code with a print action.
    await expect(page.getByRole("img", { name: "Scannable QR code" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Print" })).toBeVisible();
    await page.getByRole("button", { name: "Done" }).click();

    const h2 = page.getByRole("article", { name: "Table H2" });
    await expect(h2).toContainText("occupied");
    await h2.getByRole("combobox").selectOption({ label: "H4 (4)" });
    await h2.getByRole("button", { name: "Move" }).click();
    await expect(page.getByRole("article", { name: "Table H2" })).toContainText("cleaning");
    const h4 = page.getByRole("article", { name: "Table H4" });
    await expect(h4).toContainText("occupied");

    await h4.getByRole("button", { name: "Depart (paid)" }).click();
    await expect(page.getByRole("main").getByRole("alert")).toContainText("can no longer do that");

    // UI-002: close-empty is confirmed in a dialog that asks for the reason.
    await h4.getByRole("button", { name: "Close empty" }).click();
    const dialog = page.getByRole("dialog", { name: "Close table H4 as empty" });
    await dialog.getByLabel("Reason for table H4").fill("left before ordering");
    await dialog.getByRole("button", { name: "Close visit" }).click();
    await expect(page.getByRole("article", { name: "Table H4" })).toContainText("cleaning");
    for (const label of ["H2", "H4"]) {
      await page.getByRole("article", { name: `Table ${label}` }).getByRole("button", { name: "Mark ready" }).click();
      await expect(page.getByRole("article", { name: `Table ${label}` })).toContainText("available");
    }
  });

  test("polling pauses in hidden tabs and backs off on errors (ADM-006)", async ({ page }) => {
    await signIn(page);
    const board = /\/api\/v1\/branches\/[^/]+\/queue-tickets/;
    await page.goto("/host");
    const seen = countRequests(page, board);
    // Two polls at 3 s ±20% jitter take at most 7.2 s.
    await page.waitForTimeout(8000);
    expect(seen.length).toBeGreaterThanOrEqual(2);

    await setHidden(page, true);
    const hiddenAt = seen.length;
    await page.waitForTimeout(7000);
    expect(seen.length - hiddenAt).toBe(0);

    await setHidden(page, false);
    await expect.poll(() => seen.length, { timeout: 1500 }).toBeGreaterThan(hiddenAt);

    await page.route(board, (route) => route.fulfill({ status: 503, contentType: "application/json",
      body: JSON.stringify({ error: { code: "DEPENDENCY_UNAVAILABLE", message: "Service temporarily unavailable", request_id: "", fields: {} } }) }));
    const failStart = seen.length;
    await page.waitForTimeout(12_000);
    // Without backoff ~4 polls would occur in 12 s; with 6/12 s backoff at most 3.
    expect(seen.length - failStart).toBeLessThanOrEqual(3);
    await expect(page.getByText("data may be out of date")).toBeVisible();
  });
});
