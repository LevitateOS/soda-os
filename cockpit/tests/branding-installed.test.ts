import { test, expect } from "vite-plus/test";
import { chromium } from "playwright";
import { createHash } from "node:crypto";
import { readFileSync, mkdirSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import { execFileSync } from "node:child_process";
import { brandingSources } from "./branding-assets";

// Opt-in real-session evidence on an operator-provisioned disposable native guest.
// No deliberately failed passwords, account edits, or update checks are performed.
const targetFile = process.env.SODA_COCKPIT_TARGET_FILE;
test.skipIf(!targetFile)(
  "installed stock Cockpit and all Soda pages use Soda branding",
  async () => {
    const target = JSON.parse(readFileSync(targetFile!, "utf8")) as {
      url: string;
      username: string;
      passwordFile: string;
      architecture: "aarch64" | "x86_64";
      evidenceDirectory: string;
    };
    const root = resolve(import.meta.dirname, "../..");
    const browser = await chromium.launch();
    const errors: string[] = [];
    const failures: string[] = [];
    mkdirSync(target.evidenceDirectory, { recursive: true });
    try {
      const context = await browser.newContext({
        ignoreHTTPSErrors: true,
        viewport: { width: 1440, height: 1000 },
      });
      const page = await context.newPage();
      page.on("pageerror", (error) => errors.push(error.message));
      page.on("response", (response) => {
        if (response.status() >= 400 && /\.(css|js|svg|png|ico|woff2)(\?|$)/.test(response.url()))
          failures.push(`${response.status()} ${response.url()}`);
      });
      await page.goto(target.url);
      for (const [name, source] of Object.entries(brandingSources)) {
        const url = new URL(`cockpit/static/${name}`, target.url.replace(/\/?$/, "/"));
        const response = await context.request.get(url.href);
        expect(response.ok()).toBe(true);
        expect(await response.body()).toEqual(readFileSync(resolve(root, source)));
      }
      for (const theme of ["light", "dark"]) {
        await page.evaluate((style) => localStorage.setItem("shell:style", style), theme);
        await page.reload();
        await page.locator("#login-user-input").waitFor();
        expect(await page.locator("#brand img:visible").count()).toBe(1);
        expect(await page.locator("#brand img:visible").getAttribute("alt")).toBe("Soda OS");
        expect(
          await page.locator("body").evaluate((el) => getComputedStyle(el).backgroundImage),
        ).toContain(`login-background-${theme}.svg`);
        await page.screenshot({
          path: resolve(target.evidenceDirectory, `stock-login-${theme}.png`),
        });
      }
      await page.locator("#login-user-input").fill(target.username);
      await page
        .locator("#login-password-input")
        .fill(readFileSync(target.passwordFile, "utf8").trimEnd());
      await page.locator("#login-button").click();
      await page.getByRole("link", { name: "Projects", exact: true }).waitFor();
      const observations: Record<string, unknown> = {};
      for (const [slug, label] of [
        ["projects", "Projects"],
        ["runners", "Runners"],
        ["tailscale", "Tailscale"],
        ["updates", "Soda Updates"],
      ]) {
        await page.getByRole("link", { name: label, exact: true }).click();
        const frame = page.frameLocator(`iframe[src*="/soda-${slug}/"]`);
        await frame.getByRole("heading", { name: label, exact: true }).waitFor();
        const native = await frame.locator("#app").evaluate(async () => ({
          architecture: (await window.cockpit.spawn(["uname", "-m"], { err: "message" })).trim(),
          owner: (
            await window.cockpit.spawn(
              ["rpm", "-qf", "/usr/share/cockpit/branding/sodaos/branding.css"],
              { err: "message" },
            )
          ).trim(),
        }));
        expect(native.architecture).toBe(target.architecture);
        expect(native.owner).toMatch(/^soda-projects-/);
        const symbol = frame.locator(".soda-eyebrow img");
        await expect
          .poll(() => symbol.evaluate((el) => (el as HTMLImageElement).naturalWidth > 0))
          .toBe(true);
        for (const theme of ["light", "dark"]) {
          await page.evaluate((style) => {
            localStorage.setItem("shell:style", style);
            window.dispatchEvent(new CustomEvent("cockpit-style", { detail: { style } }));
          }, theme);
          await expect
            .poll(() =>
              frame
                .locator("html")
                .evaluate((el) => getComputedStyle(el).getPropertyValue("--soda-brand").trim()),
            )
            .toBe(theme === "light" ? "#007885" : "#10d7e8");
          expect(
            await page
              .locator("html")
              .evaluate((el) =>
                getComputedStyle(el).getPropertyValue("--ct-color-host-accent").trim(),
              ),
          ).toBe(theme === "light" ? "#007885" : "#10d7e8");
          await page.setViewportSize({ width: 390, height: 844 });
          expect(
            await frame.locator("html").evaluate((el) => el.scrollWidth > el.clientWidth + 1),
          ).toBe(false);
          await page.screenshot({
            path: resolve(target.evidenceDirectory, `branded-${slug}-${theme}-narrow.png`),
          });
          await page.setViewportSize({ width: 1440, height: 1000 });
          await page.screenshot({
            path: resolve(target.evidenceDirectory, `branded-${slug}-${theme}.png`),
          });
        }
        observations[slug] = native;
      }
      // All shipped stock pages load the shared palette; these are read-only visits.
      for (const label of [
        "Overview",
        "Networking",
        "Storage",
        "Accounts",
        "Logs",
        "Services",
        "Terminal",
      ]) {
        await page.getByRole("link", { name: label, exact: true }).click();
        const selector = `iframe.container-frame[title="${label}"][data-loaded]:visible`;
        await page.locator(selector).waitFor();
        const frame = page.frameLocator(selector);
        expect(
          await frame
            .locator("html")
            .evaluate((el) => getComputedStyle(el).getPropertyValue("--soda-brand").trim()),
        ).toBe("#10d7e8");
        await page.screenshot({
          path: resolve(target.evidenceDirectory, `stock-${label.toLowerCase()}.png`),
        });
      }
      await page.getByRole("button", { name: "Session", exact: true }).click();
      await page.locator("#logout").click();
      await page.locator("#login-user-input").waitFor();
      expect(errors).toEqual([]);
      expect(failures).toEqual([]);
      writeFileSync(
        resolve(target.evidenceDirectory, "installed-branding.json"),
        JSON.stringify(
          {
            sourceRevision: execFileSync("git", ["rev-parse", "HEAD"], {
              cwd: root,
              encoding: "utf8",
            }).trim(),
            paletteSHA256: createHash("sha256")
              .update(readFileSync(resolve(root, "assets/branding/cockpit/palette.css")))
              .digest("hex"),
            architecture: target.architecture,
            observations,
          },
          null,
          2,
        ) + "\n",
      );
    } finally {
      await browser.close();
    }
  },
  180000,
);
