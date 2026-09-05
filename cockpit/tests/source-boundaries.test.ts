import { test, expect } from "vite-plus/test";
import { existsSync } from "node:fs";
import { dirname, relative, resolve } from "node:path";
import { API } from "typescript/unstable/sync";
import { isStringLiteral } from "typescript/unstable/ast";

const root = resolve(import.meta.dirname, "..");
const sourceRoot = resolve(root, "src");
const features = ["projects", "runners", "tailscale"];
const layers = ["atoms", "molecules", "organisms", "templates", "pages"];
const pageNames = { projects: "ProjectsPage", runners: "RunnersPage", tailscale: "TailscalePage" };

function featureOf(path: string) {
  const parts = path.split("/");
  return features.find(
    (feature) =>
      parts.includes(feature) ||
      (parts[0] === "pages" && parts[1]?.toLowerCase().startsWith(feature)),
  );
}

function allowed(from: string, to: string) {
  const fromFeature = featureOf(from),
    toFeature = featureOf(to);
  if (toFeature && fromFeature !== toFeature) return false;
  const fromLayer = layers.indexOf(from.split("/")[0]);
  const toLayer = layers.indexOf(to.split("/")[0]);
  if (fromLayer >= 0) {
    if (toLayer >= 0) return toLayer <= fromLayer;
    if (!fromFeature || !to.startsWith(`${fromFeature}/`)) return false;
    const module = to.split("/")[1];
    return (
      ["types.ts", "ui.ts", "status.ts", "links.ts"].includes(module) ||
      (fromLayer === layers.indexOf("pages") && /^use\w+\.ts$/.test(module))
    );
  }
  if (toLayer >= 0) {
    return (
      from === `${fromFeature}/index.tsx` &&
      to === `pages/${pageNames[fromFeature as keyof typeof pageNames]}.tsx`
    );
  }
  return true;
}

test.each([
  ["molecules/projects/ProjectActions.tsx", "projects/types.ts", true],
  ["templates/CockpitPageTemplate.tsx", "molecules/PageHeading.tsx", true],
  ["pages/ProjectsPage.tsx", "projects/useProjects.ts", true],
  ["projects/index.tsx", "pages/ProjectsPage.tsx", true],
  ["atoms/CodeValue.tsx", "molecules/DiagnosticAlert.tsx", false],
  ["organisms/tailscale/TailscaleConnection.tsx", "tailscale/native.ts", false],
  ["pages/ProjectsPage.tsx", "projects/protocol.ts", false],
  ["molecules/DiagnosticAlert.tsx", "projects/types.ts", false],
  ["organisms/projects/ProjectCatalog.tsx", "organisms/runners/RunnerCapacity.tsx", false],
  ["projects/useProjects.ts", "runners/native.ts", false],
  ["projects/ui.ts", "pages/ProjectsPage.tsx", false],
  ["projects/index.tsx", "pages/RunnersPage.tsx", false],
])("source boundary %s → %s is %s", (from, to, expected) => {
  expect(allowed(from, to)).toBe(expected);
});

test("authored imports preserve atomic layers, native ownership, and independent entrypoints", () => {
  // The locked TypeScript 7 API parses TSX and includes type imports and re-exports.
  const api = new API({ cwd: root });
  try {
    const config = resolve(root, "tsconfig.json");
    const snapshot = api.updateSnapshot({ openProjects: [config] });
    try {
      const project = snapshot.getProject(config)!;
      const imports = new Map<string, string[]>();
      for (const path of project.rootFiles) {
        const from = relative(sourceRoot, path);
        if (from.startsWith("..") || /\.(test|d)\.tsx?$/.test(from)) continue;
        const file = project.program.getSourceFile(path)!;
        const targets: string[] = [];
        for (const node of file.imports) {
          expect(isStringLiteral(node), `Nonliteral import in ${from}`).toBe(true);
          if (!isStringLiteral(node) || !node.text.startsWith(".")) continue;
          const base = resolve(dirname(path), node.text);
          const target = [base, `${base}.ts`, `${base}.tsx`].find(existsSync);
          expect(target, `Unresolved import ${node.text} in ${from}`).toBeDefined();
          const to = relative(sourceRoot, target!);
          expect(allowed(from, to), `Disallowed source dependency: ${from} → ${to}`).toBe(true);
          targets.push(to);
        }
        imports.set(from, targets);
      }
      for (const [feature, page] of Object.entries(pageNames)) {
        expect(imports.get(`${feature}/index.tsx`)).toEqual(
          expect.arrayContaining([`pages/${page}.tsx`, `${feature}/native.ts`]),
        );
      }
    } finally {
      snapshot.dispose();
    }
  } finally {
    api.close();
  }
});
