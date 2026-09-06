import { test, expect, vi } from "vite-plus/test";
import { createUpdatesStore } from "./store";
import { hostFixture, nativeFixture, bootedDigest, nextDigest } from "../../tests/updates";

async function ready() {
  const native = nativeFixture();
  const store = createUpdatesStore(native);
  const stop = store.getState().start();
  await vi.waitFor(() => expect(store.getState().operation).toBeNull());
  return { native, store, stop };
}

test("construction is inert and each store has independent native observations", async () => {
  const native = nativeFixture();
  const first = createUpdatesStore(native),
    second = createUpdatesStore(native);
  await first.getState().update();
  await first.getState().check();
  expect(native.status).not.toHaveBeenCalled();
  expect(native.check).not.toHaveBeenCalled();
  expect(native.update).not.toHaveBeenCalled();
  const stop = first.getState().start();
  await vi.waitFor(() => expect(first.getState().host).not.toBeNull());
  expect(second.getState().host).toBeNull();
  stop();
});

test("update does not require Check, a higher version, or cached availability", async () => {
  const { native, store, stop } = await ready();
  await store.getState().update();
  expect(native.check).not.toHaveBeenCalled();
  expect(native.update).toHaveBeenCalledExactlyOnceWith(expect.any(Function));
  expect(native.status).toHaveBeenCalledTimes(2);
  expect(store.getState().host?.status.booted.image?.imageDigest).toBe(bootedDigest);
  expect(store.getState().notice).toMatch(/command completion alone is not boot proof/);
  stop();
});

test("Check is informational and Update reads the actual same-version booted image afterward", async () => {
  const { native, store, stop } = await ready();
  await store.getState().check();
  const bootedB = hostFixture();
  bootedB.status.booted.image!.imageDigest = nextDigest;
  native.status.mockResolvedValue(bootedB);
  await store.getState().update();
  expect(native.update).toHaveBeenCalledExactlyOnceWith(expect.any(Function));
  expect(store.getState().host).toBe(bootedB);
  stop();
});

test("staged and download-only native deployments do not add another activation ceremony", async () => {
  for (const downloadOnly of [true, false]) {
    const { native, store, stop } = await ready();
    const host = hostFixture();
    host.status.staged = { ...host.status.booted, downloadOnly };
    native.status.mockResolvedValue(host);
    await store.getState().refresh();
    await store.getState().update();
    expect(native.update).toHaveBeenCalledOnce();
    expect(store.getState().host?.status.staged).not.toBeNull();
    expect(store.getState().notice).not.toMatch(/successfully booted|Restart requested/);
    stop();
  }
});

test("direct duplicate calls and refresh cannot overlap an update; progress is bounded", async () => {
  const { native, store, stop } = await ready();
  let finish!: () => void;
  native.update.mockImplementationOnce((progress) => {
    progress("x".repeat(20000));
    progress(" native tail");
    return new Promise((resolve) => {
      finish = resolve;
    });
  });
  const command = store.getState().update();
  await store.getState().update();
  await store.getState().check();
  await store.getState().refresh();
  expect(native.update).toHaveBeenCalledOnce();
  expect(native.check).not.toHaveBeenCalled();
  expect(native.status).toHaveBeenCalledOnce();
  expect(store.getState().progress).toHaveLength(16384);
  expect(store.getState().progress).toMatch(/ native tail$/);
  finish();
  await command;
  stop();
});

test("disconnect and failed readback retain uncertainty until explicit reconnect refresh", async () => {
  const { native, store, stop } = await ready();
  native.update.mockRejectedValueOnce(new Error("connection closed"));
  native.status.mockRejectedValueOnce(new Error("host unavailable"));
  await store.getState().update();
  expect(store.getState()).toMatchObject({
    host: null,
    notice: null,
    readError: "Error: host unavailable",
  });
  expect(store.getState().error).toMatch(/proves neither success nor failure/);
  const recovered = hostFixture();
  recovered.status.booted.image!.imageDigest = nextDigest;
  native.status.mockResolvedValue(recovered);
  await store.getState().refresh();
  expect(store.getState().host).toBe(recovered);
  expect(store.getState().readError).toBeNull();
  expect(store.getState().error).toMatch(/connection closed/);
  expect(native.update).toHaveBeenCalledOnce();
  stop();
});

test("command completion and failed readback remain separate outcomes", async () => {
  const { native, store, stop } = await ready();
  native.status.mockRejectedValueOnce(new Error("readback failed"));
  await store.getState().update();
  expect(store.getState()).toMatchObject({
    host: null,
    error: null,
    readError: "Error: readback failed",
  });
  expect(store.getState().notice).toMatch(/not boot proof/);
  stop();
});

test("failed Check clears cached observations and refresh preserves its native error", async () => {
  const { native, store, stop } = await ready();
  native.check.mockRejectedValueOnce(new Error("registry failed"));
  await store.getState().check();
  expect(store.getState().host).toBeNull();
  await store.getState().refresh();
  expect(store.getState().host).not.toBeNull();
  expect(store.getState().error).toMatch(/registry failed/);
  stop();
});

test("leaving during update retires progress, failure, and automatic readback", async () => {
  const { native, store, stop } = await ready();
  let fail!: (error: Error) => void, output!: (chunk: string) => void;
  native.update.mockImplementationOnce((progress) => {
    output = progress;
    return new Promise((_resolve, reject) => {
      fail = reject;
    });
  });
  const command = store.getState().update();
  stop();
  const state = store.getState();
  output("late output");
  fail(new Error("connection closed"));
  await command;
  expect(store.getState()).toBe(state);
  expect(native.status).toHaveBeenCalledOnce();
});

test("old check completion and cleanup cannot overwrite a restarted lifecycle", async () => {
  const { native, store, stop } = await ready();
  let finish!: (host: ReturnType<typeof hostFixture>) => void;
  native.check.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
  );
  const check = store.getState().check();
  stop();
  const restart = store.getState().start();
  await vi.waitFor(() => expect(store.getState().operation).toBeNull());
  stop();
  const state = store.getState();
  const stale = hostFixture();
  stale.status.booted.image!.imageDigest = nextDigest;
  finish(stale);
  await check;
  expect(store.getState()).toBe(state);
  await store.getState().refresh();
  expect(native.status).toHaveBeenCalledTimes(3);
  restart();
});
