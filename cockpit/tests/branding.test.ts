import { test, expect } from "vite-plus/test";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { packageInventory } from "../build/assets";

test.each(["projects", "runners", "tailscale", "updates"])(
  "%s ships the canonical Soda symbol and shared theme tokens",
  (page) => {
    const root = resolve(import.meta.dirname, "../..");
    const directory = resolve(root, `cockpit/dist/soda-${page}`);
    const files = Object.keys(packageInventory(directory));
    const symbol = readFileSync(resolve(root, "assets/branding/source/soda-symbol.svg"), "utf8");
    expect(
      files
        .filter((file) => file.endsWith(".svg"))
        .some((file) => readFileSync(resolve(directory, file), "utf8") === symbol),
    ).toBe(true);
    const css = files
      .filter((file) => file.endsWith(".css"))
      .map((file) => readFileSync(resolve(directory, file), "utf8"))
      .join("\n");
    for (const token of [
      "--soda-surface",
      "--soda-text",
      "--soda-brand",
      "--soda-brand-hover",
      "--soda-on-brand",
    ])
      expect(css).toContain(token);
    expect(css).toContain(".soda-eyebrow");
    expect(css).not.toContain("soda-login-tagline");
    expect(css).not.toContain("login-background-dark.svg");
  },
);
