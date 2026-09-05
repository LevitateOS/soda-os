import { test, expect } from "vite-plus/test";
import { execFileSync } from "node:child_process";
import { cpSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
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

test.each([
  ["sibling package import", "assets/invalid.js", 'import "../../soda-runners/assets/index.js";'],
  ["unresolved package import", "assets/invalid.js", 'import "react";'],
  ["missing font", "assets/invalid.css", 'body { src: url("./missing.woff2"); }'],
  ["development client", "assets/invalid.js", 'import "http://localhost:5173/@vite/client";'],
  ["authored source", "source.ts", "export const value = 1;"],
])("the production asset check rejects %s", (_name, file, contents) => {
  const directory = mkdtempSync(resolve(tmpdir(), "soda-cockpit-assets-"));
  try {
    cpSync(resolve(import.meta.dirname, "../dist/soda-projects"), directory, { recursive: true });
    writeFileSync(resolve(directory, file), contents);
    expect(() => packageInventory(directory)).toThrow();
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});
