import { mkdirSync } from "node:fs";
import { resolve } from "node:path";
import { chromium } from "playwright";
import { expect, test } from "vite-plus/test";

// A native cockpit-ws --local-session endpoint, reachable only through the
// operator's loopback SSH relay. This suite never changes host configuration.
const url = process.env.SODA_COCKPIT_COLOR_URL;
const evidence = process.env.SODA_COCKPIT_BRANDING_EVIDENCE_DIRECTORY;

test.skipIf(!url)(
  "native shell and stock pages render the shared palette in both themes",
  async () => {
    const browser = await chromium.launch();
    if (evidence) mkdirSync(evidence, { recursive: true });
    try {
      for (const theme of ["light", "dark"] as const) {
        const expected =
          theme === "dark"
            ? {
                action: "rgb(37, 99, 235)",
                link: "rgb(96, 165, 250)",
                surface: "rgb(20, 26, 36)",
                canvas: "rgb(12, 16, 23)",
                text: "rgb(240, 244, 248)",
                onAction: "rgb(255, 255, 255)",
              }
            : {
                action: "rgb(21, 94, 239)",
                link: "rgb(21, 94, 239)",
                surface: "rgb(255, 255, 255)",
                canvas: "rgb(255, 253, 248)",
                text: "rgb(28, 25, 23)",
                onAction: "rgb(255, 255, 255)",
              };
        const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
        await context.addInitScript((style) => localStorage.setItem("shell:style", style), theme);
        const page = await context.newPage();
        await page.goto(url!);
        await page.locator("#host-apps").waitFor();
        expect(await page.locator("body").evaluate((el) => getComputedStyle(el).color)).toBe(
          expected.text,
        );
        expect(
          await page.locator("body").evaluate((el) => getComputedStyle(el).backgroundColor),
        ).toBe(expected.canvas);
        for (const path of [
          "system/index",
          "system/logs",
          "storage/index",
          "network/index",
          "users/index",
          "system/services",
          "system/terminal",
          "system/hwinfo",
          "network/firewall",
          "metrics/index",
        ]) {
          await page.goto(`${url!.replace(/\/$/, "")}/${path}`);
          const selector = `iframe.container-frame[src*="/${path}.html"][data-loaded]:visible`;
          await page.locator(selector).waitFor({ timeout: 60000 });
          const frame = page.frameLocator(selector);
          await frame
            .locator(":is(h1, h2, table, .pf-v6-c-card, .pf-v6-c-toolbar, .xterm-screen):visible")
            .first()
            .waitFor();
          if (path === "system/services") {
            await frame.locator("table").first().waitFor();
            const selected = frame.locator(".pf-v6-c-toggle-group__button.pf-m-selected");
            expect(await selected.evaluate((el) => getComputedStyle(el).color)).toBe(expected.text);
            expect(
              await selected.evaluate((el) => getComputedStyle(el, "::after").borderTopColor),
            ).toBe(theme === "dark" ? "rgb(147, 197, 253)" : "rgb(11, 70, 179)");
          }
          // Inspect rendered body text and real controls, not merely the presence
          // of a stylesheet or the names of unused custom properties.
          expect(await frame.locator("body").evaluate((el) => getComputedStyle(el).color)).toBe(
            expected.text,
          );
          const actual = await frame.locator("html").evaluate((el) => {
            const css = getComputedStyle(el);
            return [
              "--pf-t--global--background--color--primary--default",
              "--pf-t--global--background--color--secondary--default",
              "--pf-t--global--text--color--link--default",
            ].map((name) => css.getPropertyValue(name).trim());
          });
          expect(actual).toEqual(
            theme === "dark"
              ? ["#141a24", "#0c1017", "#60a5fa"]
              : ["#ffffff", "#fffdf8", "#155eef"],
          );
          const buttons = await frame
            .locator(
              ".pf-v6-c-button.pf-m-primary:not(:disabled):not(.pf-m-disabled):not([aria-disabled=true])",
            )
            .evaluateAll((els) =>
              els.map((el) => ({
                background: getComputedStyle(el).backgroundColor,
                color: getComputedStyle(el).color,
              })),
            );
          for (const button of buttons)
            expect(button).toEqual({ background: expected.action, color: expected.onAction });
          if (path === "system/index") {
            const card = frame.locator(".pf-v6-c-card").first();
            expect(await card.evaluate((el) => getComputedStyle(el).backgroundColor)).toBe(
              expected.surface,
            );
            const link = frame.getByText("View hardware details", { exact: true });
            expect(await link.evaluate((el) => getComputedStyle(el).color)).toBe(expected.link);
          }
          if (evidence)
            await page.screenshot({
              path: resolve(evidence, `${path.replaceAll("/", "-")}-${theme}.png`),
            });
        }
        await context.close();
      }
    } finally {
      await browser.close();
    }
  },
  300000,
);
