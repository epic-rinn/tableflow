import { expect, test, type Page } from "@playwright/test";

// ADM-001 / ADM-A1 / ADM-A4 / ACC-001-style fragment handling / ACC-004.
// Requires the disposable E2E database prepared by `make verify`
// (tooling/runtime/e2e-db.sh), which supplies E2E_MANAGER_TOKEN.
const managerToken = process.env.E2E_MANAGER_TOKEN ?? "";
const PASSWORD = "e2e password that is long enough";

test.describe.serial("staff access", () => {
  test.skip(!managerToken, "E2E_MANAGER_TOKEN not set; run `make verify`");

  let kitchenLink = "";

  async function activate(page: Page, link: string) {
    await page.goto(link);
    await expect(page.getByRole("heading", { name: "Activate your staff account" })).toBeVisible();
    // The token is removed from the visible URL and history immediately.
    expect(new URL(page.url()).hash).toBe("");
    await page.getByLabel("New password (at least 12 characters)").fill(PASSWORD);
    await page.getByLabel("Confirm password").fill(PASSWORD);
    await page.getByRole("button", { name: "Activate account" }).click();
    await expect(page.getByRole("main").getByRole("status")).toContainText("Your password is set");
  }

  async function signIn(page: Page, email: string) {
    await page.goto("/login");
    await page.getByLabel("Email").fill(email);
    await page.getByLabel("Password").fill(PASSWORD);
    await page.getByRole("button", { name: "Sign in" }).click();
    await expect(page.getByRole("heading", { level: 1 })).toContainText("Welcome");
  }

  test("manager activates, signs in, and invites kitchen staff", async ({ page, context }) => {
    await activate(page, `/activate#${managerToken}`);
    // Replaying the same link fails.
    await page.goto(`/activate#${managerToken}`);
    await page.getByLabel("New password (at least 12 characters)").fill(PASSWORD);
    await page.getByLabel("Confirm password").fill(PASSWORD);
    await page.getByRole("button", { name: "Activate account" }).click();
    await expect(page.getByRole("main").getByRole("alert")).toContainText("invalid or has expired");

    await signIn(page, "manager@e2e.test");
    const cookie = (await context.cookies()).find((c) => c.name === "__Host-tf_staff");
    expect(cookie).toMatchObject({ httpOnly: true, secure: true, sameSite: "Strict", path: "/" });

    const nav = page.getByRole("navigation", { name: "Workspaces" });
    await expect(nav.getByRole("link", { name: "Staff" })).toBeVisible();
    await expect(nav.getByRole("link", { name: "Kitchen" })).toHaveCount(0);

    await nav.getByRole("link", { name: "Staff" }).click();
    await page.getByLabel("Email").fill("kitchen@e2e.test");
    await page.getByLabel("Display name").fill("Kitchen Cook");
    await page.getByRole("group", { name: "Roles", exact: true }).getByLabel("Kitchen").check();
    await page.getByRole("button", { name: "Create invitation" }).click();
    kitchenLink = await page.getByLabel("Activation link").inputValue();
    expect(kitchenLink).toMatch(/\/activate#[A-Za-z0-9_-]{43}$/);
    await expect(page.getByRole("cell", { name: "kitchen@e2e.test" })).toBeVisible();
  });

  test("kitchen staff see only their workspace and are denied staff administration", async ({ browser }) => {
    const context = await browser.newContext();
    const page = await context.newPage();
    await activate(page, kitchenLink);
    await signIn(page, "kitchen@e2e.test");

    const nav = page.getByRole("navigation", { name: "Workspaces" });
    await expect(nav.getByRole("link", { name: "Kitchen" })).toBeVisible();
    await expect(nav.getByRole("link", { name: "Staff" })).toHaveCount(0);
    await expect(nav.getByRole("link", { name: "Cashier" })).toHaveCount(0);

    await page.goto("/staff");
    await expect(page.getByRole("heading", { name: "Not permitted" })).toBeVisible();
    await page.goto("/cashier");
    await expect(page.getByRole("heading", { name: "Not permitted" })).toBeVisible();

    // Go denies direct API calls made by the browser itself (ADM-A1).
    const direct = await page.evaluate(async () => {
      const me = await (await fetch("/api/v1/sessions/current")).json();
      const list = await fetch(`/api/v1/branches/${me.branch_id}/staff`);
      const invite = await fetch(`/api/v1/branches/${me.branch_id}/staff`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email: "x@e2e.test", display_name: "X", roles: ["manager"] }),
      });
      return { branch: me.branch_id as string, list: list.status, invite: invite.status };
    });
    expect(direct).toMatchObject({ list: 403, invite: 403 });

    // A cross-site request carrying the session cookie is rejected on Origin.
    // (Playwright's request client omits Secure cookies over http, so the
    // cookie is attached explicitly.)
    const session = (await context.cookies()).find((c) => c.name === "__Host-tf_staff");
    expect(session).toBeDefined();
    const forged = await page.request.post(`/api/v1/branches/${direct.branch}/staff`, {
      data: { email: "x@e2e.test", display_name: "X", roles: ["manager"] },
      headers: { Origin: "https://evil.example", Cookie: `__Host-tf_staff=${session!.value}` },
    });
    expect(forged.status()).toBe(403);
    expect((await forged.json()).error.code).toBe("ORIGIN_REJECTED");
    await context.close();
  });

  test("deactivation revokes an open session (ADM-A4) and sign-out works", async ({ page, browser }) => {
    const kitchenContext = await browser.newContext();
    const kitchen = await kitchenContext.newPage();
    await signIn(kitchen, "kitchen@e2e.test");

    await signIn(page, "manager@e2e.test");
    await page.goto("/staff");
    const row = page.getByRole("row").filter({ hasText: "kitchen@e2e.test" });
    await row.getByLabel("Reason to deactivate Kitchen Cook").fill("E2E test");
    await row.getByRole("button", { name: "Deactivate" }).click();
    await expect(row.getByRole("cell", { name: "disabled" })).toBeVisible();

    await kitchen.reload();
    await expect(kitchen).toHaveURL(/\/login$/);
    await kitchenContext.close();

    await page.getByRole("button", { name: "Sign out" }).click();
    await expect(page).toHaveURL(/\/login$/);
    expect(await page.evaluate(async () => (await fetch("/api/v1/sessions/current")).status)).toBe(401);
  });
});
