import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";

// UI-A1/UI-A2/UI-A4: axe scan of the guest screens at phone size, phone
// screenshots as review evidence, and no request to a third-party host.
const token = process.env.E2E_VISIT_TOKEN ?? "";
const branch = process.env.E2E_BRANCH_ID ?? "";
const SHOTS = "../../tmp/playwright/pwa-screens";

async function scan(page: Page, name: string) {
  const result = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"]).analyze();
  const blocking = result.violations.filter((v) => v.impact === "serious" || v.impact === "critical");
  const summary = blocking.map((v) => `${v.id} (${v.impact}): ${v.nodes.slice(0, 3).map((n) => `${n.target.join(" ")} «${n.html.slice(0, 80)}»`).join(" | ")}`);
  expect(summary, `axe violations on ${name}`).toEqual([]);
}

function recordHosts(page: Page) {
  const hosts = new Set<string>();
  page.on("request", (r) => hosts.add(new URL(r.url()).host));
  return hosts;
}

test.describe("guest screens at phone size", () => {
  test.skip(!token || !branch, "run through `make verify`");

  test("public screens", async ({ page, baseURL }) => {
    const hosts = recordHosts(page);
    for (const [path, name] of [["/", "home"], [`/join/${branch}`, "join"], ["/account/login", "account-login"], ["/account/signup", "account-signup"]]) {
      await page.goto(path);
      await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
      await page.waitForLoadState("networkidle");
      await scan(page, path);
      await page.screenshot({ path: `${SHOTS}/${name}.png`, fullPage: true });
    }
    expect([...hosts]).toEqual([new URL(baseURL!).host]);
  });

  test("dining screens", async ({ page, baseURL }) => {
    const hosts = recordHosts(page);
    await page.goto(`/t#${token}`);
    await expect(page.getByRole("heading", { name: "Table E2E-1" })).toBeVisible();
    await page.waitForLoadState("networkidle");
    await scan(page, "/t");
    await page.screenshot({ path: `${SHOTS}/dining.png`, fullPage: true });

    await page.getByRole("button", { name: "Add Iced Coffee" }).click();
    await page.getByRole("button", { name: /^View cart/ }).click();
    await expect(page.getByRole("region", { name: "Your cart (this phone)" })).toContainText("กาแฟเย็น");
    // Measure contrast after the sheet's slide/fade-in has finished.
    await page.getByRole("dialog").evaluate((el) => Promise.all(el.getAnimations({ subtree: true }).map((a) => a.finished)));
    await page.waitForFunction(() => document.getAnimations().every((a) => a.playState !== "running"));
    await scan(page, "/t cart sheet");
    await page.screenshot({ path: `${SHOTS}/cart-sheet.png` });
    // Leave the shared visit unchanged: empty this phone's cart again.
    await page.getByRole("button", { name: "Remove Iced Coffee" }).click();
    await page.keyboard.press("Escape");

    const bill = page.getByRole("region", { name: "Bill" });
    await bill.getByRole("button", { name: "View bill" }).click();
    await expect(bill).toContainText("Total");
    await scan(page, "/t bill");
    await bill.screenshot({ path: `${SHOTS}/bill.png` });
    expect([...hosts]).toEqual([new URL(baseURL!).host]);
  });
});
