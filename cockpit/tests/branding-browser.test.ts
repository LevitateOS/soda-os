import { test, expect } from "vite-plus/test";
import { chromium } from "playwright";
import { createHash } from "node:crypto";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { resolve, extname } from "node:path";
import { brandingSources as sources } from "./branding-assets";

// Opt-in upstream DOM evidence, NOT installed authentication acceptance. The
// unmodified Cockpit 366 installed login files are operator-fetched; only HTTP auth is simulated.
const reference = process.env.SODA_COCKPIT_LOGIN_REFERENCE;
const evidence = process.env.SODA_COCKPIT_BRANDING_EVIDENCE_DIRECTORY;
const root = resolve(import.meta.dirname, "../..");
const mime: Record<string, string> = {
  ".html": "text/html",
  ".css": "text/css",
  ".js": "text/javascript",
  ".svg": "image/svg+xml",
  ".ico": "image/x-icon",
  ".png": "image/png",
  ".woff2": "font/woff2",
};

test.skipIf(!reference || !evidence)(
  "Soda branding on upstream Cockpit login DOM",
  async () => {
    mkdirSync(evidence!, { recursive: true });
    const browser = await chromium.launch();
    const failures: string[] = [];
    const hashes = Object.fromEntries(
      ["login.html", "login.css", "login.js"].map((name) => [
        name,
        createHash("sha256")
          .update(readFileSync(resolve(reference!, name)))
          .digest("hex"),
      ]),
    );
    try {
      for (const prefix of ["", "/console"])
        for (const theme of ["light", "dark", "auto-light", "auto-dark"])
          for (const width of [1440, 390]) {
            const context = await browser.newContext({
              viewport: { width, height: 1000 },
              colorScheme: theme === "auto-light" ? "light" : "dark",
            });
            const page = await context.newPage();
            page.on("pageerror", (error) => failures.push(error.message));
            await context.addInitScript(
              (style) => localStorage.setItem("shell:style", style),
              theme.startsWith("auto") ? "auto" : theme,
            );
            await page.route("**/*", async (route) => {
              const url = new URL(route.request().url());
              const path = url.pathname.slice(prefix.length);
              if (url.origin !== "http://localhost:5176") {
                failures.push(url.href);
                await route.abort();
                return;
              }
              if (path === "/cockpit/login") {
                const authorization = route.request().headers().authorization;
                const challenge =
                  authorization ===
                  "Basic " + Buffer.from("challenge-user:example\0").toString("base64");
                await route.fulfill({
                  status: 401,
                  contentType: "application/json",
                  body: "{}",
                  headers: challenge
                    ? {
                        "WWW-Authenticate":
                          "X-Conversation test-token " +
                          Buffer.from("Verification code:").toString("base64"),
                      }
                    : {},
                });
                return;
              }
              if (path === "/system") {
                const environment = {
                  hostname: "soda-test",
                  page: { connect: true, allow_multihost: false },
                  logged_into: [],
                };
                const html = readFileSync(resolve(reference!, "login.html"), "utf8").replace(
                  '<meta insert="dynamic_content_here" />',
                  `<base href="${prefix}/"><script>window.environment=${JSON.stringify(environment)};</script>`,
                );
                await route.fulfill({
                  contentType: "text/html",
                  body: html,
                  headers: { "Content-Security-Policy": "default-src 'self' 'unsafe-inline'" },
                });
                return;
              }
              const name = path.replace(/^\/cockpit\/static\//, "");
              let file = sources[name] ? resolve(root, sources[name]) : undefined;
              if (["login.js", "login.css"].includes(name)) file = resolve(reference!, name);
              if (name.startsWith("fonts/"))
                file = resolve(root, "cockpit/node_modules/@patternfly/patternfly/assets", name);
              if (path === "/favicon.ico") file = resolve(root, sources["favicon.ico"]);
              if (!file) {
                failures.push(url.href);
                await route.fulfill({ status: 404 });
                return;
              }
              await route.fulfill({ contentType: mime[extname(file)], body: readFileSync(file) });
            });
            await page.goto(`http://localhost:5176${prefix}/system`);
            await page.locator("#login-user-input").waitFor();
            const dark = theme === "dark" || theme === "auto-dark";
            expect(
              await page
                .locator("html")
                .evaluate((el) => el.classList.contains("pf-v6-theme-dark")),
            ).toBe(dark);
            expect(await page.locator("#brand img:visible").count()).toBe(1);
            expect(await page.locator("#brand img:visible").getAttribute("alt")).toBe("Soda OS");
            expect(
              await page
                .locator("#brand img:visible")
                .evaluate((el) => (el as HTMLImageElement).naturalWidth > 0),
            ).toBe(true);
            expect(
              await page.locator("body").evaluate((el) => getComputedStyle(el).backgroundImage),
            ).toContain(`login-background-${dark ? "dark" : "light"}.svg`);
            const colors = await page.locator("#login-button").evaluate((el) => {
              const css = getComputedStyle(el);
              return [css.backgroundColor, css.color];
            });
            expect(colors).toEqual(
              dark
                ? ["rgb(37, 99, 235)", "rgb(255, 255, 255)"]
                : ["rgb(21, 94, 239)", "rgb(255, 255, 255)"],
            );
            expect(await page.locator("#server-name").textContent()).toBe("soda-test");
            expect(
              await page.locator("html").evaluate((el) => el.scrollWidth > el.clientWidth + 1),
            ).toBe(false);
            await page.locator("#login-password-input").focus();
            expect(
              await page
                .locator("#login-password-input")
                .evaluate((el) => getComputedStyle(el).outlineStyle),
            ).toBe("solid");
            await page.locator("#login-password-toggle").click();
            expect(await page.locator("#login-password-input").getAttribute("type")).toBe("text");
            await page.locator("#login-password-toggle").click();
            const name = `${prefix ? "prefix" : "root"}-${theme}-${width}`;
            await page.screenshot({
              path: resolve(evidence!, `login-${name}.png`),
              fullPage: true,
            });
            if (theme.startsWith("auto")) {
              await page.emulateMedia({ colorScheme: dark ? "light" : "dark" });
              await expect
                .poll(() => page.locator("#brand .soda-logo-dark").isVisible())
                .toBe(!dark);
              await page.emulateMedia({ colorScheme: dark ? "dark" : "light" });
              await expect
                .poll(() => page.locator("#brand .soda-logo-dark").isVisible())
                .toBe(dark);
            }
            await page.locator("#login-user-input").fill("example");
            await page.locator("#login-password-input").fill("example");
            await page.locator("#login-button").click();
            await page.getByText("Wrong user name or password", { exact: true }).waitFor();
            await page.screenshot({
              path: resolve(evidence!, `error-${name}.png`),
              fullPage: true,
            });
            await page.locator("#login-user-input").fill("challenge-user");
            await page.locator("#login-password-input").fill("example");
            await page.locator("#login-button").click();
            await page.getByLabel("Verification code:", { exact: true }).waitFor();
            expect(await page.locator("#conversation-input").getAttribute("type")).toBe("password");
            await page.screenshot({
              path: resolve(evidence!, `conversation-${name}.png`),
              fullPage: true,
            });
            await context.close();
          }
      expect(failures).toEqual([]);
      writeFileSync(
        resolve(evidence!, "login-reference.json"),
        JSON.stringify(
          {
            kind: "upstream-login-dom-with-simulated-auth",
            upstream: "Cockpit 366",
            hashes,
            widths: [1440, 390],
            themes: ["light", "dark", "auto-light", "auto-dark"],
            urlRoots: ["/", "/console/"],
          },
          null,
          2,
        ) + "\n",
      );
    } finally {
      await browser.close();
    }
  },
  120000,
);
