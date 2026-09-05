import { createStore } from "zustand/vanilla";
import type { Invoke, LifecycleAction, ListResponse, Registration } from "./types";
import { errorMessage, successMessage } from "./ui";

type Provider = "forgejo" | "github";
type Dialog = { kind: "create"; provider: Provider } | { kind: "remove"; id: string };
type Operation = { action: LifecycleAction | "create" | "list"; id: string };
interface State {
  data: ListResponse | null;
  loading: boolean;
  operation: Operation | null;
  notice: { message: string; kind: "danger" | "success" } | null;
  dialog: Dialog | null;
  readError: string;
  formError: string;
}
const initial: State = {
  data: null,
  loading: true,
  operation: null,
  notice: null,
  dialog: null,
  readError: "",
  formError: "",
};

export function createRunnersStore(invoke: Invoke) {
  let generation = 0;
  let active = false;
  const current = (version: number) => active && version === generation;
  const store = createStore(() => ({
    ...initial,
    start: () => {
      active = true;
      const version = ++generation;
      store.setState({ ...initial });
      void store.getState().refresh();
      return () => {
        if (version === generation) {
          active = false;
          generation++;
        }
      };
    },
    refresh: async () => {
      if (!active || store.getState().operation) return;
      const version = generation;
      store.setState({ operation: { action: "list", id: "" } });
      await load(version);
      if (current(version)) store.setState({ operation: null });
    },
    openCreate: () => {
      if (!active || store.getState().operation) return;
      store.setState({ formError: "", dialog: { kind: "create", provider: "forgejo" } });
    },
    changeProvider: (provider: Provider) => {
      if (!active || store.getState().operation || store.getState().dialog?.kind !== "create")
        return;
      store.setState({ dialog: { kind: "create", provider } });
    },
    openRemove: (id: string) => {
      if (!active || store.getState().operation) return;
      store.setState({ formError: "", dialog: { kind: "remove", id } });
    },
    close: () => {
      if (!store.getState().operation) store.setState({ dialog: null });
    },
    register: async (payload: Registration) => {
      if (!active || store.getState().operation || store.getState().dialog?.kind !== "create")
        return;
      const version = generation;
      const id = payload.id;
      store.setState({ operation: { action: "create", id }, notice: null, formError: "" });
      try {
        // The caller clears the input and payload as soon as this synchronous
        // adapter invocation returns. Never retain registration input in state.
        let registration;
        try {
          registration = invoke("create", payload);
        } finally {
          payload.registration_token = "";
        }
        await registration;
        if (!current(version)) return;
        store.setState({ dialog: null });
        await load(version);
        if (current(version))
          store.setState({ notice: { message: successMessage("create", id), kind: "success" } });
      } catch (error) {
        const updated = await load(version);
        if (!current(version)) return;
        store.setState({
          ...(updated?.runners.some((runner) => runner.id === id) ? { dialog: null } : {}),
          formError: errorMessage(error),
          notice: { message: errorMessage(error), kind: "danger" },
        });
      } finally {
        if (current(version)) store.setState({ operation: null });
      }
    },
    changeListener: (action: "start" | "stop" | "restart", id: string) => {
      return mutate(action, id);
    },
    remove: (confirmation: string) => {
      const { dialog, operation } = store.getState();
      if (!active || operation || dialog?.kind !== "remove") return false;
      if (confirmation !== dialog.id) {
        store.setState({ formError: `Type ${dialog.id} exactly to confirm local runner removal.` });
        return false;
      }
      void mutate("remove", dialog.id);
      return true;
    },
  }));
  async function load(version: number) {
    if (!current(version)) return null;
    store.setState({ loading: true });
    try {
      const data = await invoke("list", {});
      if (current(version)) store.setState({ data, readError: "" });
      return data;
    } catch (error) {
      if (current(version)) store.setState({ data: null, readError: errorMessage(error) });
      return null;
    } finally {
      if (current(version)) store.setState({ loading: false });
    }
  }
  async function mutate(action: LifecycleAction, id: string) {
    if (!active || store.getState().operation) return;
    const version = generation;
    store.setState({ operation: { action, id }, notice: null, formError: "" });
    try {
      await invoke(action, { id });
      if (!current(version)) return;
      if (action === "remove") store.setState({ dialog: null });
      await load(version);
      if (current(version))
        store.setState({ notice: { message: successMessage(action, id), kind: "success" } });
    } catch (error) {
      await load(version);
      if (current(version))
        store.setState({
          formError: errorMessage(error),
          notice: { message: errorMessage(error), kind: "danger" },
        });
    } finally {
      if (current(version)) store.setState({ operation: null });
    }
  }
  return store;
}
export type RunnersStore = ReturnType<typeof createRunnersStore>;
