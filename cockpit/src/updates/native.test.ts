import { test, expect, vi } from "vite-plus/test";
import { nativeUpdates } from "./native";
import type { CockpitProcess, SpawnOptions } from "../cockpit/types";

test("every operation requires Cockpit admin access and passes literal arguments", async () => {
  const stream = vi.fn();
  const spawn = vi.fn((_args: string[], _options: SpawnOptions) => {
    const result = Promise.resolve("{}") as unknown as CockpitProcess;
    result.stream = (callback) => {
      stream(callback);
      callback("native output");
      return result;
    };
    return result;
  });
  const native = nativeUpdates({ spawn });
  const progress = vi.fn();
  const selection = { version: "0.6.4", reference: "ghcr.io/levitateos/soda-os@sha256:literal" };
  await native.status();
  await native.check();
  await native.download(selection, progress);
  await native.apply(selection, progress);
  expect(spawn.mock.calls.map(([args]) => args)).toEqual([
    ["/usr/libexec/soda/soda-updates", "status"],
    ["/usr/libexec/soda/soda-updates", "check"],
    [
      "/usr/libexec/soda/soda-updates",
      "download",
      "--version",
      selection.version,
      "--reference",
      selection.reference,
    ],
    [
      "/usr/libexec/soda/soda-updates",
      "apply",
      "--version",
      selection.version,
      "--reference",
      selection.reference,
    ],
  ]);
  expect(spawn.mock.calls.every(([, options]) => options.superuser === "require")).toBe(true);
  expect(progress).toHaveBeenCalledWith("native output");
});
