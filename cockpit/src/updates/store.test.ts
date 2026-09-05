import { test, expect, vi } from "vite-plus/test";
import { createUpdatesStore } from "./store";
import type { Host, Release, NativeUpdates } from "./types";
const reference = "ghcr.io/levitateos/soda-os@sha256:" + "a".repeat(64);
const release: Release = {
  version: "0.6.4",
  reference,
  revision: "b".repeat(40),
  architecture: "x86_64",
  notes_url: "https://example.test/release",
};
function host(): Host {
  const image = {
    version: "0.6.3",
    imageDigest: reference.split("@")[1],
    architecture: "amd64",
    image: { image: reference, transport: "registry" },
  };
  return {
    apiVersion: "org.containers.bootc/v1",
    kind: "BootcHost",
    status: {
      booted: { image, downloadOnly: false, incompatible: false },
      staged: { image: { ...image, version: "0.6.4" }, downloadOnly: true, incompatible: false },
      rollbackQueued: false,
      usrOverlay: null,
    },
  };
}
async function ready() {
  const native = {
    status: vi.fn<NativeUpdates["status"]>().mockResolvedValue(host()),
    check: vi.fn<NativeUpdates["check"]>().mockResolvedValue(release),
    download: vi.fn<NativeUpdates["download"]>().mockResolvedValue(undefined),
    apply: vi.fn<NativeUpdates["apply"]>().mockResolvedValue(undefined),
  };
  const store = createUpdatesStore(native);
  const stop = store.getState().start();
  await vi.waitFor(() => expect(store.getState().operation).toBeNull());
  return { native, store, stop };
}
test("status refresh retires only read errors, preserving failed verification and output", async () => {
  const { store, native, stop } = await ready();
  native.check.mockRejectedValueOnce(new Error("signature verification failed"));
  await store.getState().check();
  native.status.mockRejectedValueOnce(new Error("status unavailable"));
  await store.getState().refresh();
  expect(store.getState()).toMatchObject({
    host: null,
    error: "Error: signature verification failed",
    readError: "Error: status unavailable",
  });
  await store.getState().refresh();
  expect(store.getState()).toMatchObject({
    error: "Error: signature verification failed",
    readError: null,
  });
  expect(native.check).toHaveBeenCalledOnce();
  stop();
});
test("apply requires confirmation, keeps its exact selection, and rejects duplicate direct calls", async () => {
  const { store, native, stop } = await ready();
  await store.getState().apply();
  expect(native.apply).not.toHaveBeenCalled();
  store.getState().requestApply();
  const reviewed = store.getState().confirmation;
  const changed = host();
  changed.status.staged!.image!.image.image = reference.replace(/a{64}/, "c".repeat(64));
  native.status.mockResolvedValue(changed);
  await store.getState().refresh();
  expect(store.getState().confirmation).toBe(reviewed);
  let finish!: () => void;
  native.apply.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
  );
  const command = store.getState().apply();
  await store.getState().apply();
  expect(native.apply).toHaveBeenCalledOnce();
  expect(native.apply).toHaveBeenCalledWith(reviewed, expect.any(Function));
  finish();
  await command;
  expect(store.getState().host).toBeNull();
  expect(store.getState().notice).toMatch(/Reconnect and refresh/);
  stop();
});
test("leaving during apply ignores streamed output and failure without issuing a readback", async () => {
  const { store, native, stop } = await ready();
  let fail!: (error: Error) => void, output!: (chunk: string) => void;
  native.apply.mockImplementationOnce((_selection, progress) => {
    output = progress;
    return new Promise((_resolve, reject) => {
      fail = reject;
    });
  });
  store.getState().requestApply();
  const command = store.getState().apply();
  stop();
  const state = store.getState();
  output("late output");
  fail(new Error("connection closed"));
  await command;
  expect(store.getState()).toBe(state);
  expect(native.status).toHaveBeenCalledOnce();
});
test("constructing independent stores does not check, download, apply, or share confirmation", async () => {
  const { store, native, stop } = await ready();
  const other = createUpdatesStore(native);
  store.getState().requestApply();
  expect(other.getState().confirmation).toBeNull();
  expect(native.check).not.toHaveBeenCalled();
  expect(native.download).not.toHaveBeenCalled();
  expect(native.apply).not.toHaveBeenCalled();
  stop();
});
