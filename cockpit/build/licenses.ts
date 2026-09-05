import { existsSync, readFileSync, readdirSync } from "node:fs";
import { resolve } from "node:path";

export function licenses(moduleIDs: string[], root: string) {
  const packages = new Map<string, string>();
  for (const id of moduleIDs.filter((id) => id.includes("/node_modules/"))) {
    const directory = id.match(/^(.*\/node_modules\/(?:@[^/]+\/)?[^/]+)\//)?.[1];
    if (!directory || !existsSync(resolve(directory, "package.json"))) continue;
    const metadata = JSON.parse(readFileSync(resolve(directory, "package.json"), "utf8")) as {
      name: string;
      version: string;
      license?: string;
    };
    const name = `${metadata.name}@${metadata.version}`;
    if (packages.has(name)) continue;
    const files = readdirSync(directory).filter((file) =>
      /^(LICENSE|COPYING|NOTICE)(\.|$)/i.test(file),
    );
    const text = files.map((file) => readFileSync(resolve(directory, file), "utf8")).join("\n");
    if (!text && !metadata.name.startsWith("@patternfly/"))
      throw new Error(`No license text for bundled ${name}`);
    packages.set(name, `${name} (${metadata.license ?? "see license text"})\n${text}`);
  }
  const vendor = readdirSync(resolve(root, "vendor"))
    .filter((name) => name.endsWith(".txt"))
    .sort()
    .map((name) => `${name}\n${readFileSync(resolve(root, "vendor", name), "utf8")}`);
  return [
    ...vendor,
    ...[...packages].sort(([a], [b]) => a.localeCompare(b)).map(([, text]) => text),
  ].join("\n\n---\n\n");
}
