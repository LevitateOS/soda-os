import { test, expect, vi } from "vite-plus/test";
import { nativeUpdates } from "./native";
import type { CockpitProcess, SpawnOptions } from "../cockpit/types";
import { hostFixture } from "../../tests/updates";

test("fixed native operations require administrative access with no browser selection arguments", async () => {
  const host = hostFixture();
  const spawn = vi.fn((_args: string[], _options: SpawnOptions) => {
    const result = Promise.resolve(JSON.stringify(host)) as unknown as CockpitProcess;
    result.stream = (callback) => {
      callback("native output");
      return result;
    };
    return result;
  });
  const native = nativeUpdates({ spawn });
  const progress = vi.fn();
  expect(await native.status()).toEqual(host);
  expect(await native.check()).toEqual(host);
  await native.update(progress);
  expect(spawn.mock.calls).toEqual([
    [["/usr/libexec/soda/soda-updates", "status"], { superuser: "require", err: "message" }],
    [["/usr/libexec/soda/soda-updates", "check"], { superuser: "require", err: "message" }],
    [["/usr/libexec/soda/soda-updates", "update"], { superuser: "require", err: "out" }],
  ]);
  expect(progress).toHaveBeenCalledWith("native output");
});

test("read transport failures and polluted JSON cannot become successful status", async () => {
  const spawn = vi
    .fn()
    .mockRejectedValueOnce(new Error("access denied"))
    .mockResolvedValueOnce("native progress\n{}");
  const native = nativeUpdates({ spawn });
  await expect(native.status()).rejects.toThrow("access denied");
  await expect(native.check()).rejects.toThrow();
});
