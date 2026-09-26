import { expect, test } from "@playwright/test";
import { latestLink } from "./support/mail";

// LOY-001/007 at the table: sign in from the dining page, claim the visit,
// other diners learn nothing about the member, the account shows points.
const token = process.env.E2E_VISIT_TOKEN ?? "";
const enabled = !!token && !!process.env.E2E_MAIL_DIR;
const PASSWORD = "loyalty e2e password";

test.describe.serial("member points at the table", () => {
  test.skip(!enabled, "run through `make verify`");
  const email = `points-${Date.now()}@e2e.test`;

  test("member claims the visit; other phones see no member data", async ({ page, browser }) => {
    await page.goto("/account/signup");
    await page.getByLabel("Email").fill(email);
    await page.getByLabel("Password (at least 12 characters)").fill(PASSWORD);
    await page.getByRole("button", { name: "Create account" }).click();
    await expect(page.getByRole("heading", { name: "Check your email" })).toBeVisible();
    await page.goto(await latestLink(email, "/account/verify"));
    await expect(page.getByRole("main").getByRole("status")).toContainText("Your email is confirmed.");

    await page.goto(`/t#${token}`);
    const points = page.getByRole("region", { name: "Member points" });
    await points.getByRole("link", { name: "Sign in to earn points" }).click();
    await page.getByLabel("Email").fill(email);
    await page.getByLabel("Password").fill(PASSWORD);
    await page.getByRole("button", { name: "Sign in" }).click();

    // Back on the table page (session resumed without the QR fragment).
    await expect(page).toHaveURL(/\/t$/);
    await points.getByRole("button", { name: "Add this visit to my points" }).click();
    await expect(points).toContainText("This visit earns points for you.");

    const other = await (await browser.newContext()).newPage();
    await other.goto(`/t#${token}`);
    const otherPoints = other.getByRole("region", { name: "Member points" });
    await expect(otherPoints.getByRole("link", { name: "Sign in to earn points" })).toBeVisible();
    await expect(other.getByRole("main")).not.toContainText(email);

    await page.goto("/account");
    const loyalty = page.getByRole("region", { name: "Points & tier" });
    await expect(loyalty).toContainText("0 points");
    await expect(loyalty).toContainText("more to Silver");
  });
});
