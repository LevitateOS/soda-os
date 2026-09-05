import { test, expect } from "vite-plus/test";
import { chromium, type Page } from "playwright";
import { readFileSync, mkdirSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import { execFileSync } from "node:child_process";
import { packageInventory } from "../build/assets";

interface Target {
  url: string;
  username: string;
  passwordFile: string;
  architecture: "x86_64" | "aarch64";
  evidenceDirectory: string;
}
const targetFile = process.env.SODA_COCKPIT_TARGET_FILE;

async function openPackage(page: Page, name: string) {
  await page.getByRole("link", { name, exact: true }).click();
  const frame = page.frameLocator(`iframe[name$="/soda-${name.toLowerCase()}/index"]`);
  await frame.getByRole("heading", { name, exact: true }).waitFor();
  return frame;
}

// This suite uses installed packages and real Cockpit sessions, never a mock backend.
test.skipIf(!targetFile)(
  "installed Cockpit authenticates and loads all three independent packages",
  async () => {
    const target = JSON.parse(readFileSync(targetFile!, "utf8")) as Target;
    const root = resolve(import.meta.dirname, "..");
    const browser = await chromium.launch();
    const context = await browser.newContext({
      ignoreHTTPSErrors: true,
      viewport: { width: 1440, height: 1000 },
    });
    const page = await context.newPage();
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    const failedAssets: string[] = [];
    page.on("response", (response) => {
      if (response.url().includes("/soda-") && response.status() >= 400)
        failedAssets.push(`${response.status()} ${response.url()}`);
    });
    mkdirSync(target.evidenceDirectory, { recursive: true });
    try {
      await page.goto(target.url);
      await page.locator("#login-user-input").fill(target.username);
      await page
        .locator("#login-password-input")
        .fill(readFileSync(target.passwordFile, "utf8").trimEnd());
      await page.locator("#login-button").click();
      const evidence: Record<string, unknown> = {
        sourceRevision: execFileSync("git", ["rev-parse", "HEAD"], {
          cwd: root,
          encoding: "utf8",
        }).trim(),
        architecture: target.architecture,
        packages: {},
      };
      const records: Record<string, unknown> = {};
      for (const name of ["Projects", "Runners", "Tailscale"]) {
        const frame = await openPackage(page, name);
        if (name === "Projects") {
          await frame.getByText(/\d+ projects? available to /).waitFor();
        } else if (name === "Runners") {
          await frame.getByText(/\d+ local runners?; \d+ listening;/).waitFor();
        } else {
          await frame
            .getByText("Reading Tailscale state…", { exact: true })
            .waitFor({ state: "hidden" });
          expect(
            await frame.getByText("Tailscale state unavailable", { exact: true }).count(),
          ).toBe(0);
        }
        const installed = await frame.locator("#app").evaluate(async (_element, packageName) => {
          const architecture = await window.cockpit.spawn(["uname", "-m"], { err: "message" });
          const rpm = await window.cockpit.spawn(
            ["rpm", "-qf", `/usr/share/cockpit/${packageName}/index.html`],
            { err: "message" },
          );
          return { architecture: architecture.trim(), rpm: rpm.trim() };
        }, `soda-${name.toLowerCase()}`);
        expect(installed.architecture).toBe(target.architecture);
        const inventory = packageInventory(resolve(root, `dist/soda-${name.toLowerCase()}`));
        const hashes = await frame.locator("#app").evaluate(
          async (_element, files) => {
            const result = await window.cockpit.spawn(["sha256sum", ...files], { err: "message" });
            return result
              .trim()
              .split("\n")
              .map((line) => line.split(/\s+/)[0]);
          },
          Object.keys(inventory).map(
            (file) => `/usr/share/cockpit/soda-${name.toLowerCase()}/${file}`,
          ),
        );
        expect(hashes).toEqual(Object.values(inventory));
        for (const theme of ["light", "dark"]) {
          await page.evaluate((style) => localStorage.setItem("shell:style", style), theme);
          await frame.locator("html").evaluate(
            (html, dark) =>
              new Promise<void>((resolveTheme) => {
                const check = () => {
                  if (html.classList.contains("pf-v6-theme-dark") === dark) resolveTheme();
                  else requestAnimationFrame(check);
                };
                check();
              }),
            theme === "dark",
          );
          await page.screenshot({
            path: resolve(target.evidenceDirectory, `${name.toLowerCase()}-${theme}.png`),
          });
        }
        await page.setViewportSize({ width: 390, height: 844 });
        const overflow = await frame
          .locator("html")
          .evaluate((html) => html.scrollWidth > html.clientWidth + 1);
        expect(overflow).toBe(false);
        await page.screenshot({
          path: resolve(target.evidenceDirectory, `${name.toLowerCase()}-narrow.png`),
        });
        await page.setViewportSize({ width: 1440, height: 1000 });
        if (name === "Projects" || name === "Runners") {
          await frame.getByRole("button", { name: "Refresh", exact: true }).click();
          const label = name === "Projects" ? "Add repository" : "Create local runner";
          await frame.getByRole("button", { name: label, exact: true }).click();
          const modal = frame.getByRole("dialog");
          await modal.waitFor();
          await page.keyboard.press("Tab");
          expect(await modal.evaluate((element) => element.contains(document.activeElement))).toBe(
            true,
          );
          await page.keyboard.press("Escape");
          await modal.waitFor({ state: "hidden" });
        }
        records[name] = { ...installed, inventory };
      }
      await openPackage(page, "Projects");
      await openPackage(page, "Tailscale");
      expect(errors).toEqual([]);
      expect(failedAssets).toEqual([]);
      evidence.packages = records;
      writeFileSync(
        resolve(target.evidenceDirectory, "installed-cockpit.json"),
        JSON.stringify(evidence, null, 2) + "\n",
      );
    } finally {
      await context.close();
      await browser.close();
    }
  },
  180000,
);
