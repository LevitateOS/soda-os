import { test, expect } from "vite-plus/test";
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { packageInventory } from "../build/assets";

test("all three installed asset graphs are self-contained and clean rebuilds are identical", () => {
  const root = resolve(import.meta.dirname, "..");
  const pages = ["projects", "runners", "tailscale"];
  const before = pages.map((page) => packageInventory(resolve(root, `dist/soda-${page}`)));
  execFileSync("vp", ["build"], {
    cwd: root,
    stdio: "pipe",
    timeout: 120000,
    env: { ...process.env, NODE_ENV: "production" },
  });
  pages.forEach((page, index) => {
    const directory = resolve(root, `dist/soda-${page}`);
    expect(packageInventory(directory)).toEqual(before[index]);
    expect(readFileSync(resolve(directory, "manifest.json"), "utf8")).toBe(
      readFileSync(resolve(root, `soda-${page}/manifest.json`), "utf8"),
    );
    const html = readFileSync(resolve(directory, "index.html"), "utf8");
    expect(html).toContain('src="../base1/cockpit.js"');
    expect(html).toContain('id="app"');
    expect(html).not.toMatch(/src\/|\.tsx|app\.mjs|htmx/);
  });
}, 120000);
