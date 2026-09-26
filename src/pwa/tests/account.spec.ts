import { expect, test } from "@playwright/test";
import { latestLink } from "./support/mail";

// ACC-002 member journey. `make verify` runs the API with the test-only file
// outbox (MAIL_ADAPTER=file); emails are read from E2E_MAIL_DIR.
const MAIL_DIR = process.env.E2E_MAIL_DIR ?? "";
const enabled = !!process.env.E2E_VISIT_TOKEN && !!MAIL_DIR;
const PASSWORD = "member e2e password";

test.describe.serial("member account", () => {
  test.skip(!enabled, "run through `make verify` (needs the E2E API and mail outbox)");
  const email = `member-${Date.now()}@e2e.test`;

  test("sign up, confirm email, sign in and out", async ({ page }) => {
    await page.goto("/account/signup");
    await page.getByLabel("Email").fill(email);
    await page.getByLabel("Password (at least 12 characters)").fill(PASSWORD);
    await page.getByLabel("Language").selectOption("en");
    await page.getByRole("button", { name: "Create account" }).click();
    await expect(page.getByRole("heading", { name: "Check your email" })).toBeVisible();

    const verify = await latestLink(email, "/account/verify");
    await page.goto(verify);
    await expect(page.getByRole("main").getByRole("status")).toContainText("Your email is confirmed.");
    expect(new URL(page.url()).hash).toBe("");

    await page.goto("/account/login");
    await page.getByLabel("Email").fill(email);
    await page.getByLabel("Password").fill(PASSWORD);
    await page.getByRole("button", { name: "Sign in" }).click();
    await expect(page.getByText(`Signed in as ${email}`)).toBeVisible();
    await expect(page.getByText("Email confirmed.")).toBeVisible();

    const cookie = (await page.context().cookies()).find((c) => c.name === "__Host-tf_member");
    expect(cookie).toMatchObject({ httpOnly: true, secure: true, sameSite: "Lax" });
    const stored = await page.evaluate(() => JSON.stringify({ ...localStorage }) + JSON.stringify({ ...sessionStorage }) + document.cookie);
    expect(stored).not.toContain(PASSWORD);
    expect(stored).not.toContain("tf_member");

    await page.getByRole("button", { name: "Sign out" }).click();
    await expect(page.getByRole("link", { name: "Sign in" })).toBeVisible();
  });

  test("password reset signs out other devices", async ({ page, browser }) => {
    const other = await browser.newContext();
    const otherPage = await other.newPage();
    await otherPage.goto("/account/login");
    await otherPage.getByLabel("Email").fill(email);
    await otherPage.getByLabel("Password").fill(PASSWORD);
    await otherPage.getByRole("button", { name: "Sign in" }).click();
    await expect(otherPage.getByText(`Signed in as ${email}`)).toBeVisible();

    await page.goto("/account/reset");
    await page.getByLabel("Email").fill(email);
    await page.getByRole("button", { name: "Send reset link" }).click();
    await expect(page.getByRole("main").getByRole("status")).toContainText("reset link is on its way");

    await page.goto(await latestLink(email, "/account/reset/confirm"));
    await page.getByLabel("New password (at least 12 characters)").fill("a brand new member password");
    await page.getByLabel("Confirm new password").fill("a brand new member password");
    await page.getByRole("button", { name: "Change password" }).click();
    await expect(page.getByRole("heading", { name: "Password changed" })).toBeVisible();

    await otherPage.reload();
    await expect(otherPage.getByRole("link", { name: "Sign in" })).toBeVisible();
    await other.close();
  });

  test("unknown email gets the same reset acknowledgement", async ({ page }) => {
    await page.goto("/account/reset");
    await page.getByLabel("Email").fill(`nobody-${Date.now()}@e2e.test`);
    await page.getByRole("button", { name: "Send reset link" }).click();
    await expect(page.getByRole("main").getByRole("status")).toContainText("reset link is on its way");
  });
});
