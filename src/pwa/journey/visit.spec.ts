import { expect, test, type Browser, type Page } from "@playwright/test";
import { latestLink } from "../tests/support/mail";

// MVP-19 / ADM-A3 and the browser E2E in specs/quality/testing.md:
// join → call → seat → two phones order → kitchen serve → member claim →
// settle → points → depart → clean, across the admin and the PWA, all on
// one Go-managed visit and bill.
const PWA = "http://127.0.0.1:3000";
const ADMIN = "http://127.0.0.1:3001";
const branch = process.env.E2E_BRANCH_ID ?? "";
const enabled = !!branch && !!process.env.E2E_MAIL_DIR;
const PASSWORD = "journey member password";

async function phone(browser: Browser): Promise<Page> {
  const context = await browser.newContext({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true, locale: "en-US" });
  return context.newPage();
}

test.describe("complete visit across admin and PWA", () => {
  test.skip(!enabled, "run through `make verify`");

  test("queue to clean table with two phones, kitchen, member points and cashier", async ({ browser }) => {
    const label = `J${Date.now() % 100000}`;
    // Staff: manager (all roles' workspaces) on the admin origin.
    const staff = await (await browser.newContext({ viewport: { width: 1280, height: 800 } })).newPage();
    await staff.goto(`${ADMIN}/login`);
    await staff.getByLabel("Email").fill("manager@e2e.test");
    await staff.getByLabel("Password").fill("e2e password that is long enough");
    await staff.getByRole("button", { name: "Sign in" }).click();
    await expect(staff.getByRole("heading", { level: 1 })).toContainText("Welcome");
    await staff.goto(`${ADMIN}/configuration`);
    const add = staff.locator("form").filter({ has: staff.getByRole("button", { name: "Add table" }) });
    await add.getByLabel("Label").fill(label);
    await add.getByLabel("Seats").fill("4");
    await add.getByRole("button", { name: "Add table" }).click();
    await expect(staff.getByRole("main").getByRole("status")).toContainText(`Table ${label} added.`);

    // Earlier suites leave waiting parties; fairness (QUE-003) would require a
    // manager override to call a later party first, so the host clears them.
    await staff.goto(`${ADMIN}/host`);
    const tickets = staff.getByRole("article", { name: /^Ticket \d+$/ });
    while ((await tickets.count()) > 0) {
      const n = await tickets.count();
      await tickets.first().getByRole("button", { name: "Cancel ticket" }).click();
      await expect(tickets).toHaveCount(n - 1);
    }

    // Guest A joins the queue from the entrance QR.
    const a = await phone(browser);
    await a.goto(`${PWA}/join/${branch}`);
    await a.getByLabel("Number of people").fill("2");
    await a.getByRole("button", { name: "Join the queue" }).click();
    const ticketHeading = a.getByRole("heading", { name: /^Ticket \d+$/ });
    await expect(ticketHeading).toBeVisible();
    const number = (await ticketHeading.textContent())!.match(/\d+/)![0];

    // Host calls and seats the party at the new table.
    await staff.goto(`${ADMIN}/host`);
    const ticket = staff.getByRole("article", { name: `Ticket ${number}` });
    await ticket.getByRole("combobox").selectOption({ label: `${label} (4)` });
    await ticket.getByRole("button", { name: "Call" }).click();
    await expect(a.getByRole("main").getByRole("alert")).toContainText(`table ${label}`, { timeout: 20_000 }); // tracking poll
    await ticket.getByRole("button", { name: "Seat" }).click();
    const link = await staff.getByLabel(`Dining QR for table ${label}`).inputValue();
    const dining = link.slice(link.indexOf("#") + 1);
    await staff.getByRole("button", { name: "Done" }).click();

    // Two phones at the table order separately.
    const b = await phone(browser);
    for (const p of [a, b]) {
      await p.goto(`${PWA}/t#${dining}`);
      await expect(p.getByRole("heading", { name: `Table ${label}` })).toBeVisible();
    }
    for (const [p, item] of [[a, "Thai Tea"], [b, "Iced Coffee"]] as const) {
      await p.getByRole("button", { name: `Add ${item}` }).click();
      await p.getByRole("button", { name: /^View cart · 1 item/ }).click();
      await p.getByRole("dialog").getByRole("button", { name: /^Send order/ }).click();
      await expect(p.getByRole("main").getByRole("status").filter({ hasText: "sent to the kitchen" })).toBeVisible();
    }

    // Kitchen serves both lines.
    await staff.goto(`${ADMIN}/kitchen`);
    for (const item of ["Thai Tea", "Iced Coffee"]) {
      const card = staff.getByRole("article", { name: `${item} for table ${label}` });
      for (const step of ["Accept", "Start preparing", "Ready", "Served"]) {
        await card.getByRole("button", { name: step, exact: true }).click();
        if (step !== "Served") await expect(card.getByRole("button", { name: step, exact: true })).toHaveCount(0);
      }
      await expect(card).toHaveCount(0);
    }
    await expect(a.getByRole("region", { name: "Table orders" })).toContainText("Served", { timeout: 20_000 });

    // Phone B is a member and claims the visit.
    const email = `journey-${Date.now()}@e2e.test`;
    await b.goto(`${PWA}/account/signup`);
    await b.getByLabel("Email").fill(email);
    await b.getByLabel("Password (at least 12 characters)").fill(PASSWORD);
    await b.getByRole("button", { name: "Create account" }).click();
    await b.goto(PWA + (await latestLink(email, "/account/verify")));
    await expect(b.getByRole("main").getByRole("status")).toContainText("Your email is confirmed.");
    await b.goto(`${PWA}/account/login?next=/t`);
    await b.getByLabel("Email").fill(email);
    await b.getByLabel("Password").fill(PASSWORD);
    await b.getByRole("button", { name: "Sign in" }).click();
    await expect(b).toHaveURL(/\/t$/);
    const points = b.getByRole("region", { name: "Member points" });
    await points.getByRole("button", { name: "Add this visit to my points" }).click();
    await expect(points).toContainText("This visit earns points for you.");
    // Phone A learns nothing about the member.
    await a.reload();
    await expect(a.locator("body")).not.toContainText(email);

    // Cashier settles the shared bill.
    await staff.goto(`${ADMIN}/cashier`);
    await staff.getByRole("button", { name: `Table ${label}` }).click();
    await expect(staff.getByRole("heading", { name: `Table ${label} — open` })).toBeVisible();
    await expect(staff.getByRole("main")).toContainText("Member");
    await staff.getByRole("button", { name: "Begin settlement" }).click();
    await expect(staff.getByRole("heading", { name: /settling/ })).toBeVisible();
    await staff.getByLabel("Verification note").fill("counted at till");
    await staff.getByRole("button", { name: /^Confirm .* received$/ }).click();
    await staff.getByRole("alertdialog").getByRole("button", { name: "Record payment" }).click();
    await expect(staff.getByRole("main").getByRole("status").filter({ hasText: /Payment recorded\. Receipt R-.*Member earned 1 point/ })).toBeVisible();

    // Dining access ends with payment; the member sees the points.
    await a.reload();
    await expect(a.getByRole("main").getByRole("alert")).toBeVisible();
    await b.goto(`${PWA}/account`);
    const loyalty = b.getByRole("region", { name: "Points & tier" });
    await expect(loyalty).toContainText("1 points");
    await expect(loyalty).toContainText("Points earned");

    // Payment never frees the table: the host records departure, then cleaning.
    await staff.goto(`${ADMIN}/host`);
    const table = staff.getByRole("article", { name: `Table ${label}` });
    await expect(table).toContainText("occupied");
    await table.getByRole("button", { name: "Depart (paid)" }).click();
    await expect(table).toContainText("cleaning");
    await table.getByRole("button", { name: "Mark ready" }).click();
    await expect(table).toContainText("available");
  });
});
