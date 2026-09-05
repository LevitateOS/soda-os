import { test, expect } from "vite-plus/test";
import { chromium } from "playwright";
import { readFileSync, mkdirSync } from "node:fs";
import { resolve } from "node:path";

// Read-only smoke test of the real installed page and native helper. The
// operator supplies an ephemeral VM account with passwordless sudo; this test
// never stages or reboots. The no-release result requires an empty release feed.
const targetFile = process.env.SODA_UPDATES_BROWSER_TARGET;
test.skipIf(!targetFile)(
  "installed Soda Updates reads native status and handles no published release",
  async () => {
    const target = JSON.parse(readFileSync(targetFile!, "utf8")) as {
      url: string;
      username: string;
      passwordFile: string;
      evidenceDirectory: string;
    };
    const browser = await chromium.launch();
    try {
      const context = await browser.newContext({ ignoreHTTPSErrors: true });
      const page = await context.newPage();
      const errors: string[] = [];
      page.on("pageerror", (error) => errors.push(error.message));
      await page.goto(`${target.url}/soda-updates`);
      await page.locator("#login-user-input").fill(target.username);
      await page
        .locator("#login-password-input")
        .fill(readFileSync(target.passwordFile, "utf8").trimEnd());
      await page.locator("#login-button").click();
      const frame = page.frameLocator('iframe[name$="/soda-updates"]');
      await frame.getByRole("heading", { name: "Soda Updates", exact: true }).waitFor();
      await page.getByRole("button", { name: "Limited access", exact: true }).click();
      await page.getByRole("button", { name: "Close", exact: true }).last().click();
      await frame.getByRole("button", { name: "Refresh status", exact: true }).click();
      const installed = frame.getByRole("region", { name: "Installed image" });
      await installed.getByText(/^Soda OS \d+\./).waitFor({ timeout: 30000 });
      await frame.getByRole("button", { name: "Check for updates", exact: true }).click();
      await frame
        .getByText(/No published stable Soda release is available yet/)
        .waitFor({ timeout: 60000 });
      await expect
        .poll(() => frame.getByRole("button", { name: "Refresh status", exact: true }).isEnabled())
        .toBe(true);
      expect(await frame.getByText("Up to date.", { exact: true }).count()).toBe(0);
      expect(
        await frame.getByRole("button", { name: "Download update", exact: true }).count(),
      ).toBe(0);
      expect(errors).toEqual([]);
      mkdirSync(target.evidenceDirectory, { recursive: true });
      await page.screenshot({
        path: resolve(target.evidenceDirectory, "updates-installed.png"),
        fullPage: true,
      });
    } finally {
      await browser.close();
    }
  },
  120000,
);
