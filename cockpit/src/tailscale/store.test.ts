import { afterEach, test, expect, vi } from "vite-plus/test";
import { createTailscaleStore } from "./store";
import type { NativeTailscale, Snapshot, AuthenticationMessage } from "./types";
const snapshot: Snapshot = {
  status: {
    BackendState: "Running",
    Self: { DNSName: "soda.tail.test." },
    TailscaleIPs: ["100.64.0.1"],
  },
  prefs: { ExitNodeIP: "100.64.0.2", ExitNodeAllowLANAccess: true },
};
function client() {
  return {
    read: vi.fn<NativeTailscale["read"]>().mockResolvedValue(snapshot),
    readStatus: vi.fn<NativeTailscale["readStatus"]>().mockResolvedValue(snapshot.status),
    signIn: vi.fn<NativeTailscale["signIn"]>().mockResolvedValue(undefined),
    selectExitNode: vi.fn<NativeTailscale["selectExitNode"]>().mockResolvedValue(undefined),
    advertiseExitNode: vi.fn<NativeTailscale["advertiseExitNode"]>().mockResolvedValue(undefined),
    refreshForgejo: vi.fn<NativeTailscale["refreshForgejo"]>().mockResolvedValue(undefined),
    close: vi.fn(),
  };
}
const stops: Array<() => void> = [];
afterEach(() => {
  for (const stop of stops.splice(0)) stop();
  vi.useRealTimers();
});
async function ready() {
  vi.useFakeTimers();
  const native = client();
  const factory = vi.fn(() => native);
  const store = createTailscaleStore(factory);
  const stop = store.getState().start();
  stops.push(stop);
  await vi.advanceTimersByTimeAsync(0);
  return { native, factory, store, stop };
}
test("instances own independent drafts, observers, and cleanup", async () => {
  const first = await ready();
  const second = await ready();
  first.store.getState().changeExitNode("100.64.0.9");
  expect(second.store.getState().exitNode).toBe("100.64.0.2");
  first.stop();
  await vi.advanceTimersByTimeAsync(3000);
  expect(first.native.read).toHaveBeenCalledOnce();
  expect(second.native.read).toHaveBeenCalledTimes(2);
  expect(second.native.close).not.toHaveBeenCalled();
});
test("a retired pending read cannot publish into a restarted store", async () => {
  const { native, factory, store, stop } = await ready();
  let finish!: (value: Snapshot) => void;
  native.read.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
  );
  await vi.advanceTimersByTimeAsync(3000);
  stop();
  const next = client();
  factory.mockReturnValueOnce(next);
  stops.push(store.getState().start());
  await vi.advanceTimersByTimeAsync(0);
  const state = store.getState();
  finish({
    status: { BackendState: "NeedsLogin", AuthURL: "https://login.tailscale.com/a/old" },
    prefs: {},
  });
  await vi.advanceTimersByTimeAsync(0);
  expect(store.getState()).toBe(state);
  await vi.advanceTimersByTimeAsync(3000);
  expect(native.read).toHaveBeenCalledTimes(2);
  expect(next.read).toHaveBeenCalledTimes(2);
});
test.each(["before", "during"] as const)(
  "a poll begun %s a save never substitutes for fresh readback or overwrites the draft",
  async (when) => {
    const { native, store } = await ready();
    let finishRead!: (value: Snapshot) => void, finishWrite!: () => void;
    native.read.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finishRead = resolve;
        }),
    );
    native.selectExitNode.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finishWrite = resolve;
        }),
    );
    store.getState().changeExitNode("100.64.0.9");
    if (when === "before") await vi.advanceTimersByTimeAsync(3000);
    const save = store.getState().applyExitNode();
    if (when === "during") await vi.advanceTimersByTimeAsync(3000);
    expect(native.read).toHaveBeenCalledTimes(2);
    finishWrite();
    await vi.advanceTimersByTimeAsync(0);
    expect(store.getState()).toMatchObject({
      exitNode: "100.64.0.9",
      operation: "exit",
      dirty: { exit: true },
    });
    native.read.mockResolvedValue({
      ...snapshot,
      prefs: { ExitNodeIP: "100.64.0.9", ExitNodeAllowLANAccess: true },
    });
    const observed: string[] = [];
    const unsubscribe = store.subscribe((state) => observed.push(state.exitNode));
    finishRead(snapshot);
    await save;
    expect(native.read).toHaveBeenCalledTimes(3);
    expect(observed).not.toContain("100.64.0.2");
    expect(store.getState()).toMatchObject({
      exitNode: "100.64.0.9",
      saved: "exit",
      operation: undefined,
      dirty: { exit: false },
    });
    unsubscribe();
  },
);
test("successful native save and unavailable readback remain separate facts; failure cannot become an empty device list", async () => {
  const { native, store } = await ready();
  store.getState().changeExitNode("100.64.0.9");
  native.read.mockRejectedValueOnce(new Error("readback unavailable"));
  await store.getState().applyExitNode();
  expect(store.getState()).toMatchObject({
    saved: "exit",
    snapshot: null,
    readError: "readback unavailable",
    exitNode: "100.64.0.9",
  });
  await store.getState().applyAdvertisement();
  expect(native.advertiseExitNode).not.toHaveBeenCalled();
});
test("direct duplicate commands are rejected and failed saves keep the draft and diagnostic through polling", async () => {
  const { native, store } = await ready();
  let fail!: (error: Error) => void;
  native.selectExitNode.mockImplementationOnce(
    () =>
      new Promise((_resolve, reject) => {
        fail = reject;
      }),
  );
  store.getState().changeExitNode("100.64.0.9");
  const save = store.getState().applyExitNode();
  await store.getState().applyExitNode();
  expect(native.selectExitNode).toHaveBeenCalledOnce();
  fail(new Error("save outcome unknown"));
  await save;
  await vi.advanceTimersByTimeAsync(9000);
  expect(store.getState()).toMatchObject({
    exitNode: "100.64.0.9",
    dirty: { exit: true },
    notice: "save outcome unknown",
    saved: undefined,
  });
});
test("each activation owns a fresh native adapter and retired authentication callbacks cannot update it", async () => {
  const { native, factory, store, stop } = await ready();
  let emit!: (message: AuthenticationMessage) => void, finish!: () => void;
  native.signIn.mockImplementationOnce((_status, callback) => {
    emit = callback;
    return new Promise((resolve) => {
      finish = resolve;
    });
  });
  const signIn = store.getState().signIn();
  emit({ AuthURL: "https://login.tailscale.com/a/old" });
  expect(store.getState().authURL).toContain("/old");
  stop();
  expect(native.close).toHaveBeenCalledOnce();
  expect(store.getState().authURL).toBeNull();
  const next = client();
  factory.mockReturnValueOnce(next);
  stops.push(store.getState().start());
  await vi.advanceTimersByTimeAsync(0);
  const state = store.getState();
  emit({ AuthURL: "https://login.tailscale.com/a/late" });
  finish();
  await signIn;
  expect(store.getState()).toBe(state);
  expect(factory).toHaveBeenCalledTimes(2);
  expect(native.read).toHaveBeenCalledOnce();
  expect(next.read).toHaveBeenCalledOnce();
});
test("Forgejo failure is not retried by polling but explicit retry recovers independently", async () => {
  const { store, native } = await ready();
  native.refreshForgejo.mockRejectedValueOnce(new Error("Forgejo unavailable"));
  await store.getState().retryForgejo();
  await vi.advanceTimersByTimeAsync(9000);
  expect(native.refreshForgejo).toHaveBeenCalledTimes(2);
  expect(store.getState()).toMatchObject({
    snapshot,
    forgejoError: "Forgejo unavailable",
    notice: "",
    readError: "",
  });
  await store.getState().retryForgejo();
  expect(store.getState().forgejoError).toBe("");
});
