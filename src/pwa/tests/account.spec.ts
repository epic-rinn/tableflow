import { expect, test, type APIRequestContext } from "@playwright/test";

// ACC-002 member journey through the local Mailpit sink (make verify runs the
// API with SMTP to Mailpit). Emails are fetched from Mailpit's HTTP API.
const MAILPIT = process.env.MAILPIT_URL ?? "http://127.0.0.1:8025";
const enabled = !!process.env.E2E_VISIT_TOKEN; // set only by make verify
const PASSWORD = "member e2e password";

async function latestLink(request: APIRequestContext, to: string, path: string): Promise<string> {
  for (let i = 0; i < 50; i++) {
    const search = await request.get(`${MAILPIT}/api/v1/search?query=${encodeURIComponent(`to:"${to}"`)}`);
    const { messages } = (await search.json()) as { messages: { ID: string }[] };
    for (const m of messages ?? []) {
      const text = ((await (await request.get(`${MAILPIT}/api/v1/message/${m.ID}`)).json()) as { Text: string }).Text;
      const match = text.match(new RegExp(`(${path.replace(/\//g, "\\/")}#[A-Za-z0-9_-]{43})`));
      if (match) return match[1];
    }
    await new Promise((r) => setTimeout(r, 200));
  }
  throw new Error(`no ${path} email for ${to}`);
}

test.describe.serial("member account", () => {
  test.skip(!enabled, "run through `make verify` (needs Mailpit and the E2E API)");
  const email = `member-${Date.now()}@e2e.test`;

  test("sign up, confirm email, sign in and out", async ({ page, request }) => {
    await page.goto("/account/signup");
    await page.getByLabel("Email").fill(email);
    await page.getByLabel("Password (at least 12 characters)").fill(PASSWORD);
    await page.getByLabel("Language").selectOption("en");
    await page.getByRole("button", { name: "Create account" }).click();
    await expect(page.getByRole("heading", { name: "Check your email" })).toBeVisible();

    const verify = await latestLink(request, email, "/account/verify");
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

  test("password reset signs out other devices", async ({ page, browser, request }) => {
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

    await page.goto(await latestLink(request, email, "/account/reset/confirm"));
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
