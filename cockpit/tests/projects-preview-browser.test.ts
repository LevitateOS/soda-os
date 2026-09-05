import { test, expect } from "vite-plus/test";
import { createServer } from "vite-plus";
import { chromium } from "playwright";
import { mkdirSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import type { AddressInfo } from "node:net";

const evidenceDirectory = process.env.SODA_PROJECTS_PREVIEW_EVIDENCE_DIRECTORY;

test.skipIf(!evidenceDirectory)(
  "Projects design preview: real browser, simulated operations only",
  async () => {
    const server = await createServer({
      root: resolve(import.meta.dirname, ".."),
      server: { host: "127.0.0.1", port: 0 },
      logLevel: "error",
    });
    try {
      await server.listen();
      const origin = `http://127.0.0.1:${(server.httpServer!.address() as AddressInfo).port}`;
      const browser = await chromium.launch();
      try {
        const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
        await context.grantPermissions(["clipboard-read", "clipboard-write"]);
        const page = await context.newPage();
        const errors: string[] = [];
        const external: string[] = [];
        page.on("pageerror", (error) => errors.push(error.message));
        await context.route("**/*", async (route) => {
          if (new URL(route.request().url()).origin !== origin) {
            external.push(route.request().url());
            await route.abort();
          } else await route.continue();
        });
        const directory = resolve(evidenceDirectory!);
        mkdirSync(directory, { recursive: true });
        await page.goto(`${origin}/prototypes/projects/`);
        await page.getByRole("heading", { name: "Projects", exact: true }).waitFor();
        expect(await page.evaluate(() => typeof window.cockpit)).toBe("undefined");
        const captures: string[] = [];
        for (const width of [1440, 390]) {
          await page.setViewportSize({ width, height: width === 390 ? 844 : 1000 });
          for (const theme of ["light", "dark"]) {
            await page.evaluate(
              (style) =>
                window.dispatchEvent(new CustomEvent("cockpit-style", { detail: { style } })),
              theme,
            );
            expect(
              await page
                .locator("html")
                .evaluate((html) => html.classList.contains("pf-v6-theme-dark")),
            ).toBe(theme === "dark");
            for (const scenario of [
              "empty",
              "absent",
              "personal-key",
              "git-key",
              "unconfirmed",
              "ready",
              "unknown",
              "partial-removal",
            ]) {
              await page.getByLabel("Preview scenario").selectOption(scenario);
              if (scenario === "ready")
                await page
                  .getByRole("button", { name: "Connection details — Team website" })
                  .click();
              else if (["personal-key", "git-key", "unconfirmed", "unknown"].includes(scenario)) {
                await page.getByRole("button", { name: "Review setup — Team website" }).click();
              } else if (scenario === "partial-removal")
                await page.getByRole("button", { name: "Review remaining work" }).click();
              expect(
                await page
                  .locator("html")
                  .evaluate((html) => html.scrollWidth > html.clientWidth + 1),
              ).toBe(false);
              const dialog = page.getByRole("dialog");
              if (await dialog.count()) {
                expect(
                  await dialog.evaluate((element) => element.scrollWidth > element.clientWidth + 1),
                ).toBe(false);
                for (let index = 0; index < 4; index++) {
                  await page.keyboard.press("Tab");
                  expect(
                    await dialog.evaluate((element) => element.contains(document.activeElement)),
                  ).toBe(true);
                }
              }
              const file = `${scenario}-${width}-${theme}.png`;
              await page.screenshot({
                path: resolve(directory, file),
                fullPage: true,
                animations: "disabled",
              });
              captures.push(file);
              if (await dialog.count()) {
                await page.keyboard.press("Escape");
                await dialog.waitFor({ state: "hidden" });
              }
            }
          }
        }
        // The complete simulated setup journey, including its busy focus target.
        await page.getByLabel("Preview scenario").selectOption("absent");
        await page.getByRole("button", { name: "Set up for me — Team website" }).click();
        const dialog = page.getByRole("dialog");
        await dialog.getByText(/Setting up your workspace for/).waitFor();
        await page.keyboard.press("Tab");
        expect(await dialog.evaluate((element) => element.contains(document.activeElement))).toBe(
          true,
        );
        await dialog.getByRole("button", { name: "Open Accounts", exact: true }).waitFor();
        expect(await dialog.evaluate((element) => element.contains(document.activeElement))).toBe(
          true,
        );
        await dialog.getByRole("button", { name: "Open Accounts", exact: true }).click();
        await dialog
          .getByRole("status")
          .filter({ hasText: "Preview destination: Cockpit Accounts" })
          .waitFor();
        await dialog.getByRole("button", { name: "Try again", exact: true }).click();
        await dialog.getByRole("button", { name: "Open GitHub key settings", exact: true }).click();
        await dialog
          .getByRole("status")
          .filter({ hasText: "Preview destination: GitHub" })
          .waitFor();
        await dialog.getByRole("button", { name: "Retry setup", exact: true }).click();
        await dialog.getByRole("button", { name: "Copy SSH command", exact: true }).waitFor();
        expect(await dialog.getByLabel("SSH command", { exact: true }).inputValue()).toBe(
          "ssh soda-w-preview@soda.example",
        );
        await dialog.getByRole("button", { name: "Copy SSH command", exact: true }).click();
        expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(
          "ssh soda-w-preview@soda.example",
        );
        await page.keyboard.press("Escape");
        await dialog.waitFor({ state: "hidden" });
        // Real browser constraint validation must not conceal our field guidance.
        await page.getByRole("button", { name: "Add repository", exact: true }).click();
        await dialog.getByRole("button", { name: "Add repository", exact: true }).click();
        await dialog.getByRole("alert").filter({ hasText: "Enter a project name." }).waitFor();
        expect(
          await dialog
            .getByLabel("Project name", { exact: false })
            .evaluate((input) => input === document.activeElement),
        ).toBe(true);
        await page.screenshot({
          path: resolve(directory, "validation-390-dark.png"),
          fullPage: true,
          // Capture the completed PatternFly error fade-in, not its transparent first frame.
          animations: "disabled",
        });
        captures.push("validation-390-dark.png");
        expect(errors).toEqual([]);
        expect(external).toEqual([]);
        writeFileSync(
          resolve(directory, "preview.json"),
          JSON.stringify(
            {
              evidence:
                "Local Vite preview with simulated data; not installed Cockpit or native acceptance",
              architecture: process.arch,
              captures,
              pageErrors: errors,
              externalRequests: external,
            },
            null,
            2,
          ) + "\n",
        );
        await context.close();
      } finally {
        await browser.close();
      }
    } finally {
      await server.close();
    }
  },
  120000,
);
