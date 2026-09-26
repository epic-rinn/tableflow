import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";

// UI-A1/UI-A3: axe scan of every workspace (serious/critical fail the gate)
// plus desktop/tablet screenshots kept as review evidence (not pixel diffs).
const enabled = !!process.env.E2E_MANAGER_TOKEN;
const SHOTS = "../../tmp/playwright/admin-screens";
const PAGES = ["/", "/host", "/kitchen", "/cashier", "/receipts", "/menu", "/configuration", "/staff", "/charges", "/loyalty"];

async function scan(page: Page, name: string) {
  const result = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"]).analyze();
  const blocking = result.violations.filter((v) => v.impact === "serious" || v.impact === "critical");
  const summary = blocking.map((v) => `${v.id} (${v.impact}): ${v.nodes.slice(0, 3).map((n) => `${n.target.join(" ")} «${n.html.slice(0, 80)}»`).join(" | ")}`);
  expect(summary, `axe violations on ${name}`).toEqual([]);
}

async function signIn(page: Page) {
  await page.goto("/login");
  await page.getByLabel("Email").fill("manager@e2e.test");
  await page.getByLabel("Password").fill("e2e password that is long enough");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toContainText("Welcome");
}

test.describe("accessibility and layout", () => {
  test.skip(!enabled, "run through `make verify`");

  test("login is accessible", async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto("/login");
    await scan(page, "/login");
    await page.screenshot({ path: `${SHOTS}/login-1280.png`, fullPage: true });
  });

  for (const viewport of [{ width: 1280, height: 800 }, { width: 1024, height: 768 }]) {
    test(`workspaces at ${viewport.width}px`, async ({ page }) => {
      test.setTimeout(60_000);
      await page.setViewportSize(viewport);
      await signIn(page);
      for (const path of PAGES) {
        await page.goto(path);
        await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
        await page.waitForLoadState("networkidle");
        if (viewport.width === 1280) await scan(page, path);
        const name = path === "/" ? "home" : path.slice(1);
        await page.screenshot({ path: `${SHOTS}/${name}-${viewport.width}.png`, fullPage: true });
      }
    });
  }

  test("dark theme and keyboard dialog", async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await signIn(page);
    await page.goto("/host");
    await page.getByRole("button", { name: "Use dark theme" }).click();
    await expect(page.locator("html")).toHaveClass(/dark/);
    await page.waitForLoadState("networkidle");
    await scan(page, "/host (dark)");
    await page.screenshot({ path: `${SHOTS}/host-dark-1280.png`, fullPage: true });
    await page.getByRole("button", { name: "Use light theme" }).click();

    // Dialogs are keyboard operable: Enter opens, focus moves inside, Escape
    // closes and returns focus to the trigger.
    const trigger = page.getByRole("article", { name: "Table E2E-1" }).getByRole("button", { name: "New QR" });
    await trigger.focus();
    await page.keyboard.press("Enter");
    const dialog = page.getByRole("dialog", { name: "New dining QR for table E2E-1" });
    await expect(dialog).toBeVisible();
    await expect(dialog.getByLabel("Reason for new QR at E2E-1")).toBeFocused();
    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
    await expect(trigger).toBeFocused();
  });
});
