import { packageInventory } from "./build/assets.ts";
import { licenses } from "./build/licenses.ts";
import { defineConfig } from "vite-plus";
import { readFileSync, readdirSync } from "node:fs";
import { resolve } from "node:path";

const pages = ["projects", "runners", "tailscale"] as const;
export default defineConfig({
  base: "./",
  resolve: {
    alias: ["RedHatDisplay", "RedHatText", "RedHatMono"].flatMap((family) => {
      const dir = resolve(
        import.meta.dirname,
        "node_modules/@patternfly/patternfly/assets/fonts",
        family,
      );
      return readdirSync(dir).map((file) => ({
        find: `../../static/fonts/${file}`,
        replacement: resolve(dir, file),
      }));
    }),
  },
  builder: {
    async buildApp(builder) {
      for (const page of pages) await builder.build(builder.environments[page]);
    },
  },
  environments: Object.fromEntries(
    pages.map((page) => [
      page,
      {
        consumer: "client",
        build: {
          outDir: `dist/soda-${page}`,
          emptyOutDir: true,
          sourcemap: false,
          assetsInlineLimit: 0,
          rolldownOptions: { input: resolve(import.meta.dirname, `soda-${page}/index.html`) },
        },
      },
    ]),
  ),
  plugins: [
    {
      name: "soda-cockpit-package",
      enforce: "post",
      writeBundle() {
        packageInventory(resolve(import.meta.dirname, `dist/soda-${this.environment.name}`));
      },
      generateBundle: {
        order: "post",
        handler(_options, bundle) {
          const page = this.environment.name;
          if (!pages.includes(page as (typeof pages)[number])) return;
          const path = `soda-${page}/index.html`;
          const html = bundle[path];
          if (!html || html.type !== "asset") throw new Error(`Missing Cockpit entry: ${path}`);
          // Vite preserves the source HTML subdirectory. Installed packages have their own root.
          html.source = String(html.source).replaceAll("../assets/", "./assets/");
          this.emitFile({ type: "asset", fileName: "index.html", source: html.source });
          delete bundle[path];
          this.emitFile({
            type: "asset",
            fileName: "manifest.json",
            source: readFileSync(resolve(import.meta.dirname, `soda-${page}/manifest.json`)),
          });
          this.emitFile({
            type: "asset",
            fileName: "LICENSES.txt",
            source: licenses(
              Object.values(bundle).flatMap((item) =>
                item.type === "chunk" ? Object.keys(item.modules) : [],
              ),
              import.meta.dirname,
            ),
          });
        },
      },
    },
  ],
  lint: {
    options: { typeAware: true, typeCheck: true },
    ignorePatterns: ["vendor/**", "dist/**", "node_modules/**"],
  },
  fmt: { ignorePatterns: ["vendor/**", "dist/**", "node_modules/**", "soda-*/manifest.json"] },
  test: {
    include: ["src/**/*.test.{ts,tsx}", "tests/**/*.test.ts"],
    environment: "node",
    setupFiles: ["./tests/setup.ts"],
  },
});
