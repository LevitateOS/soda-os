import { test, expect } from "vite-plus/test";
import { chromium } from "playwright";
import { mkdirSync } from "node:fs";
import { resolve } from "node:path";

// Installed native CSS references, not a simulated login or an installed app test.
const reference = process.env.SODA_THEME_REFERENCE;
const evidence = process.env.SODA_COCKPIT_BRANDING_EVIDENCE_DIRECTORY;
const root = resolve(import.meta.dirname, "../..");
const forgejo = resolve(root, "packaging/rpm/forgejo/sources/custom/public/assets/css");

test.skipIf(!reference)(
  "shared colors render through both native CSS adapters",
  async () => {
    const browser = await chromium.launch();
    if (evidence) mkdirSync(evidence, { recursive: true });
    try {
      for (const application of ["cockpit", "forgejo"]) {
        for (const mode of ["light", "dark", "auto"]) {
          const context = await browser.newContext({
            colorScheme: mode === "light" ? "dark" : "light",
            viewport: { width: 1100, height: 800 },
          });
          const page = await context.newPage();
          const failures: string[] = [];
          await page.route("**/*", async (route) => {
            const name = new URL(route.request().url()).pathname.slice(1);
            if (name === "palette.css")
              return route.fulfill({
                path: resolve(root, "assets/branding/theme/palette.css"),
                contentType: "text/css",
              });
            if (name === "theme.css")
              return route.fulfill({
                path: resolve(root, "cockpit/src/cockpit/theme.css"),
                contentType: "text/css",
              });
            if (name.startsWith("theme-soda-") || name === "soda-controls.css")
              return route.fulfill({ path: resolve(forgejo, name), contentType: "text/css" });
            if (
              [
                "cockpit.css",
                "forgejo.css",
                "theme-forgejo-light.css",
                "theme-forgejo-dark.css",
              ].includes(name)
            )
              return route.fulfill({ path: resolve(reference!, name), contentType: "text/css" });
            // Color proofs use system fallback fonts, not font/layout acceptance.
            if (/\.(woff2?|ttf)(\?|$)/.test(name)) return route.fulfill({ status: 204 });
            if (name === "")
              return route.fulfill({ contentType: "text/html", body: sheet(application, mode) });
            failures.push(name);
            return route.fulfill({ status: 404 });
          });
          await page.goto("http://soda-theme.test/");
          const themes = mode === "auto" ? ["light", "dark"] : [mode];
          for (const theme of themes) {
            if (mode === "auto") {
              await page.emulateMedia({ colorScheme: theme as "light" | "dark" });
              if (application === "cockpit") {
                // Cockpit owns this class in production; this fixture only exercises CSS.
                await page.evaluate(
                  (dark) => document.documentElement.classList.toggle("pf-v6-theme-dark", dark),
                  theme === "dark",
                );
              }
            }
            const dark = theme === "dark";
            expect(
              await page.locator("body").evaluate((el) => getComputedStyle(el).backgroundColor),
            ).toBe(dark ? "rgb(12, 16, 23)" : "rgb(255, 253, 248)");
            expect(await page.locator("#link").evaluate((el) => getComputedStyle(el).color)).toBe(
              dark ? "rgb(96, 165, 250)" : "rgb(21, 94, 239)",
            );
            expect(
              await page.locator("main").evaluate((el) => getComputedStyle(el).backgroundColor),
            ).toBe(dark ? "rgb(20, 26, 36)" : "rgb(255, 255, 255)");
            const primary = page.locator("#primary");
            await page.mouse.move(0, 0);
            await expect
              .poll(() => primary.evaluate((el) => getComputedStyle(el).backgroundColor))
              .toBe(dark ? "rgb(37, 99, 235)" : "rgb(21, 94, 239)");
            expect(await primary.evaluate((el) => getComputedStyle(el).color)).toBe(
              "rgb(255, 255, 255)",
            );
            await primary.hover();
            await expect
              .poll(() => primary.evaluate((el) => getComputedStyle(el).backgroundColor))
              .toBe(dark ? "rgb(29, 78, 216)" : "rgb(14, 86, 219)");
            await page.mouse.down();
            await expect
              .poll(() => primary.evaluate((el) => getComputedStyle(el).backgroundColor))
              .toBe(dark ? "rgb(30, 64, 175)" : "rgb(11, 70, 179)");
            await page.mouse.up();
            await page.locator("#field").focus();
            if (application === "forgejo")
              expect(
                await page.locator("#field").evaluate((el) => getComputedStyle(el).outlineStyle),
              ).toBe("solid");
            await page.mouse.move(0, 0);
            await expect
              .poll(() => primary.evaluate((el) => getComputedStyle(el).backgroundColor))
              .toBe(dark ? "rgb(37, 99, 235)" : "rgb(21, 94, 239)");
            for (const width of [1100, 390]) {
              await page.setViewportSize({ width, height: 800 });
              expect(
                await page.locator("html").evaluate((el) => el.scrollWidth > el.clientWidth + 1),
              ).toBe(false);
              if (evidence)
                await page.screenshot({
                  path: resolve(evidence, `${application}-${mode}-${theme}-${width}.png`),
                });
            }
            await page.setViewportSize({ width: 1100, height: 800 });
          }
          expect(failures).toEqual([]);
          await context.close();
        }
      }
    } finally {
      await browser.close();
    }
  },
  120000,
);

function sheet(application: string, mode: string) {
  const cockpit = application === "cockpit";
  const styles = cockpit
    ? '<link rel="stylesheet" href="cockpit.css"><link rel="stylesheet" href="palette.css"><link rel="stylesheet" href="theme.css">'
    : `<link rel="stylesheet" href="forgejo.css"><link rel="stylesheet" href="theme-soda-${mode}.css">`;
  return `<!doctype html><html class="${cockpit && mode === "dark" ? "pf-v6-theme-dark" : ""}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">${styles}
    <style>body { padding: 32px; } main { max-width: 650px; padding: 24px; } h1,p { margin-bottom: 24px; } .fields { margin-block:24px; } input { max-width:100%; } .actions { display:flex; flex-wrap:wrap; gap:12px; }</style>
    </head><body><main class="${cockpit ? "pf-v6-c-card" : "ui segment"}">
    <h1>Soda colors · ${application}</h1><p>Native controls, shared palette. This is a CSS reference proof, not a running application.</p>
    <p><a id="link" href="#">Repository and project link</a></p>
    <div class="fields ${cockpit ? "pf-v6-c-form-control" : "ui input"}"><input id="field" aria-label="Name" placeholder="Project name"></div>
    <div class="actions"><button id="primary" class="${cockpit ? "pf-v6-c-button pf-m-primary" : "ui primary button"}">Primary action</button>
    <button disabled class="${cockpit ? "pf-v6-c-button pf-m-primary" : "ui primary button"}">Disabled</button>
    <button class="${cockpit ? "pf-v6-c-button pf-m-danger" : "ui negative button"}">Danger</button></div>
    </main></body></html>`;
}
