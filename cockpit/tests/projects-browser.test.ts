import { test, expect } from "vite-plus/test";
import { chromium } from "playwright";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { resolve, extname } from "node:path";

const directory = process.env.SODA_PROJECTS_BROWSER_EVIDENCE_DIRECTORY;
const project = {
  id: "site",
  display_name: "Team website",
  canonical_url: "git@github.com:team/site.git",
  catalog_metadata: { team: "web" },
  workspace_username: "soda-w-alice-site",
  workspace_exists: true,
};
const accounts = ["alice", "bob", "carol"].map((name, index) => ({
  username: `soda-w-${name}-site`,
  uid: 2000 + index,
  primary_username: name,
  project_id: "site",
  home: `/home/soda-w-${name}-site`,
}));
const workspace = {
  username: project.workspace_username,
  exists: true,
  checkout_ready: true,
  checkout_path: `/home/${project.workspace_username}/Projects/site`,
  public_key: "ssh-ed25519 PUBLIC-EXAMPLE-KEY-FOR-BROWSER-EVIDENCE",
  primary_key_problem: "",
  workspace_key_problem: "",
  checkout_problem: "",
  git_key_problem: "",
};
const absent = {
  ...workspace,
  exists: false,
  checkout_ready: false,
  checkout_path: "",
  public_key: "",
};
const preview = {
  action: "remove",
  target: "site",
  revision: "a".repeat(64),
  accounts,
  catalog_present: true,
};
const partial = {
  ok: false,
  result: {
    removed: [accounts[0].username],
    uncertain: accounts[1].username,
    not_attempted: [accounts[2].username],
    diagnostic: "Native account deletion did not finish.",
  },
  catalog: "not_attempted",
  problem: "",
};
type Answer = { value?: unknown; error?: string; delay?: number; failedReceipt?: boolean };

test.skipIf(!directory)(
  "Projects production journeys with simulated native responses",
  async () => {
    const output = resolve(directory!);
    mkdirSync(output, { recursive: true });
    const origin = "http://127.0.0.1:5175";
    const browser = await chromium.launch();
    const captures: unknown[] = [];
    const errors: string[] = [];
    const external: string[] = [];
    try {
      for (const width of [1440, 390])
        for (const theme of ["light", "dark"])
          for (const scenario of [
            "empty",
            "list",
            "personal-key",
            "git-key",
            "ready",
            "setup-unknown",
            "metadata",
            "validation",
            "removal-complete",
            "own-removal",
            "project-removal",
            "human-removal",
            "partial-removal",
            "removal-unknown",
            "changed-scope",
            "blocked-removal",
            "absent-after-failure",
          ]) {
            const catalog = {
              current_user: { username: "alice", administrator: true },
              projects:
                scenario === "empty"
                  ? []
                  : [
                      {
                        ...project,
                        workspace_exists: !["personal-key", "setup-unknown"].includes(scenario),
                      },
                    ],
            };
            const queues: Record<string, Answer[]> = {
              list: [{ value: catalog }],
              inspect: [{ value: { ok: true, workspace } }],
              "removal-inspect": [{ value: { ok: true, preview } }],
            };
            if (scenario === "personal-key")
              queues.inspect = [
                {
                  value: {
                    ok: true,
                    workspace: {
                      ...absent,
                      primary_key_problem: "Personal authorized_keys is missing.",
                    },
                  },
                },
              ];
            if (scenario === "git-key")
              queues.inspect = [
                { value: { ok: true, workspace: { ...workspace, checkout_ready: false } } },
              ];
            if (scenario === "setup-unknown") {
              queues.inspect = [
                { value: { ok: true, workspace: absent } },
                { error: "Inspection connection unavailable." },
              ];
              queues.setup = [{ error: "Setup connection closed." }];
            }
            if (scenario === "own-removal")
              queues["removal-inspect"] = [
                {
                  value: {
                    ok: true,
                    preview: {
                      ...preview,
                      action: "remove-workspace",
                      accounts: accounts.slice(0, 1),
                    },
                  },
                },
              ];
            if (scenario === "human-removal")
              queues["removal-inspect"] = [
                {
                  value: {
                    ok: true,
                    preview: {
                      ...preview,
                      action: "delete-human",
                      target: "alice",
                      catalog_present: false,
                      accounts: [
                        accounts[0],
                        {
                          username: "alice",
                          uid: 1000,
                          primary_username: "alice",
                          project_id: "",
                          home: "/home/alice",
                        },
                      ],
                    },
                  },
                },
              ];
            if (["partial-removal", "absent-after-failure", "changed-scope"].includes(scenario)) {
              const result =
                scenario === "changed-scope"
                  ? {
                      ...partial,
                      result: { removed: [], uncertain: "", not_attempted: [], diagnostic: "" },
                      problem:
                        "Removal scope changed; inspect again and confirm the new selection.",
                    }
                  : partial;
              queues.remove = [{ value: result, failedReceipt: true, delay: 350 }];
              queues["removal-inspect"].push({
                value: {
                  ok: true,
                  preview: {
                    ...preview,
                    revision: "b".repeat(64),
                    accounts: accounts.slice(scenario === "absent-after-failure" ? 2 : 1),
                  },
                },
              });
            }
            if (scenario === "removal-complete") {
              queues.remove = [
                {
                  value: {
                    ok: true,
                    result: {
                      removed: accounts.map((account) => account.username),
                      uncertain: "",
                      not_attempted: [],
                      diagnostic: "",
                    },
                    catalog: "removed",
                    problem: "",
                  },
                  delay: 350,
                },
              ];
              queues.list.push({ value: { ...catalog, projects: [] } });
            }
            if (scenario === "removal-unknown")
              queues.remove = [{ error: "Removal connection closed.", delay: 350 }];
            if (scenario === "blocked-removal")
              queues["removal-inspect"] = [
                { error: "Account preflight could not verify the home." },
              ];
            const context = await browser.newContext({
              viewport: { width, height: width === 390 ? 844 : 1000 },
              permissions: ["clipboard-read", "clipboard-write"],
            });
            await context.route("**/*", async (route) => {
              const url = new URL(route.request().url());
              if (url.origin !== origin) {
                external.push(url.href);
                await route.abort();
                return;
              }
              if (url.pathname === "/base1/cockpit.js") {
                await route.fulfill({ contentType: "text/javascript", body: "" });
                return;
              }
              expect(url.pathname).toMatch(/^\/soda-projects\//);
              await route.fulfill({
                contentType: (
                  {
                    ".html": "text/html",
                    ".js": "text/javascript",
                    ".css": "text/css",
                    ".woff2": "font/woff2",
                    ".svg": "image/svg+xml",
                  } as Record<string, string>
                )[extname(url.pathname)],
                body: readFileSync(resolve(import.meta.dirname, "../dist", "." + url.pathname)),
              });
            });
            await context.addInitScript(
              ({ queues, catalog }) => {
                const fixture = {
                  calls: [] as { action: string; payload?: unknown }[],
                  jumps: [] as string[],
                };
                Object.assign(window, { PROJECTS_BROWSER_FIXTURE: fixture });
                Object.assign(window, {
                  cockpit: {
                    jump: (path: string) => fixture.jumps.push(path),
                    spawn: (command: string[]) => {
                      if (command.length !== 2 || command[0] !== "/usr/libexec/soda/soda-projects")
                        throw new Error("Unexpected command");
                      const action = command[1];
                      const call: { action: string; payload?: unknown } = { action };
                      fixture.calls.push(call);
                      const answer = queues[action]?.shift() ?? { value: catalog };
                      let emit = (_chunk: string) => {};
                      const promise = new Promise<string>((yes, no) =>
                        setTimeout(() => {
                          if (answer.error) {
                            no(new Error(answer.error));
                            return;
                          }
                          const output = JSON.stringify(answer.value);
                          emit(output);
                          if (answer.failedReceipt) no(new Error("Removal exited nonzero"));
                          else yes(output);
                        }, answer.delay ?? 15),
                      );
                      return Object.assign(promise, {
                        input: (text: string) => {
                          call.payload = JSON.parse(text);
                          return promise;
                        },
                        stream: (callback: (chunk: string) => void) => {
                          emit = callback;
                          return promise;
                        },
                      });
                    },
                  },
                });
              },
              { queues, catalog },
            );
            const page = await context.newPage();
            page.setDefaultTimeout(8000);
            page.on("pageerror", (error) => errors.push(error.message));
            await page.goto(origin + "/soda-projects/index.html");
            await page.getByRole("button", { name: "Manage people in Accounts" }).waitFor();
            await page.evaluate(
              (style) =>
                window.dispatchEvent(new CustomEvent("cockpit-style", { detail: { style } })),
              theme,
            );
            expect(
              await page
                .locator("html")
                .evaluate((element) => element.classList.contains("pf-v6-theme-dark")),
            ).toBe(theme === "dark");
            const dialog = page.getByRole("dialog");
            if (["personal-key", "git-key", "ready", "setup-unknown"].includes(scenario)) {
              await page
                .getByRole("button", {
                  name: `${["personal-key", "setup-unknown"].includes(scenario) ? "Set up for me" : "Review setup"} — Team website`,
                })
                .click();
              const button = {
                "personal-key": "Open Accounts",
                "git-key": "Copy workspace public key",
                ready: "Copy SSH command",
                "setup-unknown": "Check setup",
              }[scenario]!;
              await dialog.getByRole("button", { name: button, exact: true }).waitFor();
              if (scenario === "ready") {
                await dialog.getByRole("button", { name: button, exact: true }).click();
                expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(
                  `ssh ${workspace.username}@127.0.0.1`,
                );
              }
            } else if (scenario === "validation") {
              const trigger = page.getByRole("button", { name: "Add repository", exact: true });
              await trigger.click();
              await dialog.getByRole("button", { name: "Add repository", exact: true }).click();
              await page.waitForFunction(() => document.activeElement?.id === "display-name");
              await dialog.getByLabel("Project name", { exact: false }).fill("Draft");
              await page.keyboard.press("Escape");
              await dialog.waitFor({ state: "hidden" });
              expect(await trigger.evaluate((element) => element === document.activeElement)).toBe(
                true,
              );
              await trigger.click();
              expect(await dialog.getByLabel("Project name", { exact: false }).inputValue()).toBe(
                "",
              );
            } else if (scenario === "metadata") {
              await page.getByRole("button", { name: "Actions — Team website" }).click();
              await page.getByRole("menuitem", { name: "Edit project", exact: true }).click();
              await dialog.getByRole("button", { name: "Additional metadata (optional)" }).click();
              await dialog.getByLabel("Metadata JSON").fill("{invalid}");
              for (let attempt = 0; attempt < 2; attempt++) {
                await dialog
                  .getByRole("button", { name: "Additional metadata (optional)" })
                  .click();
                await dialog.getByRole("button", { name: "Save changes" }).click();
                await page.waitForFunction(
                  () => document.activeElement?.id === "additional-metadata",
                );
              }
            } else if (!["empty", "list"].includes(scenario)) {
              if (scenario === "human-removal") {
                await page.getByRole("button", { name: "People actions" }).click();
                await page.getByRole("menuitem", { name: "Remove person…" }).click();
                await dialog.getByLabel("Primary username", { exact: false }).fill("alice");
                await dialog.getByRole("button", { name: "Check affected accounts" }).click();
                await dialog.getByLabel("Type alice to confirm", { exact: false }).waitFor();
              } else {
                await page.getByRole("button", { name: "Actions — Team website" }).click();
                await page
                  .getByRole("menuitem", {
                    name: scenario === "own-removal" ? "Remove my workspace" : "Remove project",
                    exact: true,
                  })
                  .click();
                if (scenario === "blocked-removal")
                  await dialog
                    .getByRole("alert")
                    .filter({ hasText: "Affected accounts could not be checked" })
                    .waitFor();
                else await dialog.getByLabel("Type site to confirm", { exact: false }).waitFor();
              }
              if (
                [
                  "partial-removal",
                  "removal-unknown",
                  "changed-scope",
                  "absent-after-failure",
                  "removal-complete",
                ].includes(scenario)
              ) {
                const remove = dialog.getByRole("button", { name: "Remove project", exact: true });
                expect(await remove.isDisabled()).toBe(true);
                await dialog.getByLabel("Type site to confirm", { exact: false }).fill("site");
                await remove.click();
                await dialog.getByText("Remove project in progress…", { exact: true }).waitFor();
                await dialog
                  .getByRole("alert")
                  .filter({
                    hasText:
                      scenario === "removal-unknown"
                        ? "Removal outcome is not confirmed"
                        : scenario === "removal-complete"
                          ? "Native removal completed"
                          : "Removal is incomplete",
                  })
                  .waitFor();
                expect(
                  await dialog.evaluate((element) => element.contains(document.activeElement)),
                ).toBe(true);
                if (
                  ["removal-unknown", "absent-after-failure", "removal-complete"].includes(scenario)
                )
                  expect(await remove.count()).toBe(0);
                else {
                  expect(await remove.count()).toBe(0);
                  await dialog.getByRole("button", { name: "Review remaining removal" }).waitFor();
                }
              }
            }
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
            expect(
              await page.evaluate(() => document.documentElement.scrollWidth > innerWidth),
            ).toBe(false);
            const file = `${scenario}-${width}-${theme}.png`;
            await page.screenshot({
              path: resolve(output, file),
              fullPage: true,
              animations: "disabled",
            });
            const calls = await page.evaluate(
              () =>
                (
                  window as unknown as {
                    PROJECTS_BROWSER_FIXTURE: { calls: { action: string; payload?: unknown }[] };
                  }
                ).PROJECTS_BROWSER_FIXTURE.calls,
            );
            const mutations = calls.filter((call) =>
              [
                "setup",
                "remove",
                "remove-workspace",
                "delete-human",
                "add-existing",
                "edit",
              ].includes(call.action),
            );
            expect(mutations).toHaveLength(
              [
                "setup-unknown",
                "partial-removal",
                "removal-unknown",
                "changed-scope",
                "absent-after-failure",
                "removal-complete",
              ].includes(scenario)
                ? 1
                : 0,
            );
            for (const call of mutations.filter((call) => call.action === "remove"))
              expect(call.payload).toEqual({ id: "site", expected: "a".repeat(64) });
            if (scenario === "empty") {
              expect(
                await page.getByRole("button", { name: "Add repository", exact: true }).count(),
              ).toBe(1);
              expect(await page.getByRole("table").count()).toBe(0);
              expect(await page.getByRole("grid").count()).toBe(0);
            }
            captures.push({ file, calls });
            await context.close();
          }
      expect(errors).toEqual([]);
      expect(external).toEqual([]);
    } finally {
      await browser.close();
      writeFileSync(
        resolve(output, "observations.json"),
        JSON.stringify(
          {
            evidence: "Production bundle; simulated native responses, not installed acceptance",
            architecture: process.arch,
            captures,
            errors,
            external,
          },
          null,
          2,
        ),
      );
    }
  },
  180000,
);
