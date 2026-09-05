import { test, expect } from "vite-plus/test";
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { mkdtempSync, readFileSync, readdirSync, rmSync, mkdirSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { resolve } from "node:path";
import { packageInventory } from "../build/assets";

const rpmDirectory = process.env.SODA_COCKPIT_RPM_DIRECTORY;
const builder = process.env.SODA_COCKPIT_RPM_BUILDER;

test.skipIf(!rpmDirectory)(
  "native RPM payloads exactly match all generated packages",
  () => {
    expect(
      builder,
      "Set SODA_COCKPIT_RPM_BUILDER to the matching native RPM builder image",
    ).toBeTruthy();
    const root = resolve(import.meta.dirname, "..");
    const architecture =
      process.arch === "arm64" ? "aarch64" : process.arch === "x64" ? "x86_64" : "unsupported";
    expect(architecture).not.toBe("unsupported");
    const extracted = mkdtempSync(resolve(tmpdir(), "soda-cockpit-rpm-"));
    const docker = (args: string[]) =>
      execFileSync(
        "docker",
        [
          "run",
          "--rm",
          "--network",
          "none",
          "--user",
          `${process.getuid!()}:${process.getgid!()}`,
          "--volume",
          `${resolve(rpmDirectory!)}:/rpms:ro`,
          "--volume",
          `${extracted}:/extracted`,
          "--workdir",
          "/extracted",
          builder!,
          ...args,
        ],
        { encoding: "utf8", timeout: 120000 },
      );
    const records: Record<string, unknown> = {};
    try {
      expect(docker(["uname", "-m"]).trim()).toBe(architecture);
      for (const [page, owner] of [
        ["projects", "soda-projects"],
        ["runners", "soda-runners"],
        ["tailscale", "soda-runtime"],
        ["updates", "soda-runtime"],
      ]) {
        const matches = readdirSync(rpmDirectory!).filter(
          (file) => file.startsWith(`${owner}-`) && file.endsWith(`.${architecture}.rpm`),
        );
        expect(matches).toHaveLength(1);
        const rpm = matches[0];
        expect(docker(["rpm", "-qp", "--qf", "%{NAME} %{ARCH}", `/rpms/${rpm}`])).toBe(
          `${owner} ${architecture}`,
        );
        docker([
          "bash",
          "-c",
          'set -euo pipefail; rpm2cpio "$1" | cpio -idm --quiet "./usr/share/cockpit/$2/*"',
          "extract-cockpit",
          `/rpms/${rpm}`,
          `soda-${page}`,
        ]);
        const inventory = packageInventory(resolve(root, `dist/soda-${page}`));
        expect(packageInventory(resolve(extracted, `usr/share/cockpit/soda-${page}`))).toEqual(
          inventory,
        );
        records[page] = {
          rpm,
          rpmSHA256: createHash("sha256")
            .update(readFileSync(resolve(rpmDirectory!, rpm)))
            .digest("hex"),
          inventory,
        };
      }
      const evidenceDirectory = process.env.SODA_COCKPIT_EVIDENCE_DIRECTORY;
      if (evidenceDirectory) {
        mkdirSync(evidenceDirectory, { recursive: true });
        writeFileSync(
          resolve(evidenceDirectory, `cockpit-rpm-${architecture}.json`),
          JSON.stringify(
            {
              sourceRevision: execFileSync("git", ["rev-parse", "HEAD"], {
                cwd: root,
                encoding: "utf8",
              }).trim(),
              architecture,
              builderImageID: execFileSync(
                "docker",
                ["image", "inspect", "--format", "{{.Id}}", builder!],
                { encoding: "utf8" },
              ).trim(),
              frontendLockSHA256: createHash("sha256")
                .update(readFileSync(resolve(root, "pnpm-lock.yaml")))
                .digest("hex"),
              packages: records,
            },
            null,
            2,
          ) + "\n",
        );
      }
    } finally {
      rmSync(extracted, { recursive: true, force: true });
    }
  },
  120000,
);
