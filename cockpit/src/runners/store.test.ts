import { test, expect, vi } from "vite-plus/test";
import { createRunnersStore } from "./store";
import type { Invoke, ListResponse, Registration } from "./types";

const empty: ListResponse = {
  runners: [],
  runner_count: 0,
  active_listeners: 0,
  total_capacity: 0,
};
async function ready() {
  const invoke = vi.fn<Invoke>().mockResolvedValue(empty);
  const store = createRunnersStore(invoke as Invoke);
  const stop = store.getState().start();
  await vi.waitFor(() => expect(store.getState().operation).toBeNull());
  return { store, invoke, stop };
}
test("instances own independent dialogs and provider choices; close/reopen resets the task", async () => {
  const a = await ready(),
    b = await ready();
  a.store.getState().openCreate();
  a.store.getState().changeProvider("github");
  expect(a.store.getState().dialog).toEqual({ kind: "create", provider: "github" });
  expect(b.store.getState().dialog).toBeNull();
  a.store.getState().close();
  a.store.getState().openCreate();
  expect(a.store.getState().dialog).toEqual({ kind: "create", provider: "forgejo" });
  a.stop();
  b.stop();
});
test("direct actions reject duplicate commands and preserve success plus failed readback", async () => {
  const { store, invoke, stop } = await ready();
  let finish!: () => void;
  invoke.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        finish = () => resolve({ ok: true });
      }),
  );
  invoke.mockRejectedValueOnce(new Error("readback unavailable"));
  const command = store.getState().changeListener("stop", "one");
  await store.getState().changeListener("stop", "one");
  expect(invoke).toHaveBeenCalledTimes(2);
  finish();
  await command;
  expect(store.getState()).toMatchObject({
    data: null,
    readError: "readback unavailable",
    notice: { kind: "success" },
    operation: null,
  });
  stop();
});
test("exact removal confirmation is enforced by the action without rendering", async () => {
  const { store, invoke, stop } = await ready();
  store.getState().openRemove("one");
  expect(store.getState().remove(" one")).toBe(false);
  expect(invoke).toHaveBeenCalledTimes(1);
  invoke.mockResolvedValueOnce({ ok: true });
  expect(store.getState().remove("one")).toBe(true);
  await vi.waitFor(() => expect(store.getState().operation).toBeNull());
  expect(invoke).toHaveBeenCalledWith("remove", { id: "one" });
  stop();
});
test("registration serializes before returning and never puts a token in observable state", async () => {
  const { store, invoke, stop } = await ready();
  const snapshots: string[] = [];
  const unsubscribe = store.subscribe((state) => snapshots.push(JSON.stringify(state)));
  store.getState().openCreate();
  let finish!: () => void;
  let serialized = "";
  invoke.mockImplementationOnce((_action, payload) => {
    serialized = JSON.stringify(payload);
    return new Promise((resolve) => {
      finish = () => resolve({ ok: true });
    });
  });
  const payload: Registration = {
    id: "one",
    provider: "forgejo",
    registration_url: "",
    registration_id: "id",
    labels: "host",
    registration_token: "synthetic-secret",
  };
  const command = store.getState().register(payload);
  expect(serialized).toContain("synthetic-secret");
  expect(payload.registration_token).toBe("");
  finish();
  await command;
  expect(snapshots.join("\n")).not.toContain("synthetic-secret");
  unsubscribe();
  stop();
});
test("retired continuations neither publish state nor start native readback, including after reactivation", async () => {
  const { store, invoke, stop } = await ready();
  let fail!: (reason: Error) => void;
  invoke.mockImplementationOnce(
    () =>
      new Promise((_resolve, reject) => {
        fail = reject;
      }),
  );
  const command = store.getState().changeListener("stop", "one");
  stop();
  const stopAgain = store.getState().start();
  await vi.waitFor(() => expect(store.getState().operation).toBeNull());
  const state = store.getState();
  fail(new Error("late failure"));
  await command;
  expect(store.getState()).toBe(state);
  expect(invoke).toHaveBeenCalledTimes(3);
  stopAgain();
});
