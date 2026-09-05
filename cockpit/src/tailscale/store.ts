import { createStore } from "zustand/vanilla";
import type { NativeTailscale, Snapshot, AuthenticationMessage } from "./types";
import { advertisesExitNode, authenticationURL, connectionState, exitSelection } from "./status";

interface State {
  snapshot: Snapshot | null;
  loading: boolean;
  notice: string;
  readError: string;
  forgejoError: string;
  forgejoIdentity: string;
  operation: "exit" | "advertise" | "signin" | "forgejo" | undefined;
  saved: "exit" | "advertise" | undefined;
  authURL: string | null;
  streamState: string | undefined;
  exitNode: string;
  allowLAN: boolean;
  advertise: boolean;
  dirty: { exit: boolean; advertise: boolean };
}
const initial: State = {
  snapshot: null,
  loading: true,
  notice: "",
  readError: "",
  forgejoError: "",
  forgejoIdentity: "",
  operation: undefined,
  saved: undefined,
  authURL: null,
  streamState: undefined,
  exitNode: "",
  allowLAN: false,
  advertise: false,
  dirty: { exit: false, advertise: false },
};
function diagnostic(error: unknown) {
  return error &&
    typeof error === "object" &&
    "message" in error &&
    typeof error.message === "string" &&
    error.message
    ? error.message
    : String(error);
}
function connected(snapshot: Snapshot | null) {
  return snapshot?.status.BackendState === "Running" && !snapshot.status.Self?.Expired;
}

export function createTailscaleStore(createNative: () => NativeTailscale) {
  let native: NativeTailscale | null = null;
  let generation = 0,
    preferencesRevision = 0;
  let reading: Promise<void> | null = null;
  let forgejo: Promise<void> | null = null;
  let timer: ReturnType<typeof setTimeout> | undefined;
  const current = (version: number) => native !== null && version === generation;
  const store = createStore(() => ({
    ...initial,
    start: () => {
      const previous = native;
      const version = ++generation;
      native = null;
      clearTimeout(timer);
      previous?.close();
      native = createNative();
      const client = native;
      preferencesRevision = 0;
      reading = null;
      forgejo = null;
      store.setState({ ...initial, dirty: { exit: false, advertise: false } });
      void observe(version);
      return () => {
        if (!current(version)) return;
        generation++;
        native = null;
        clearTimeout(timer);
        reading = null;
        forgejo = null;
        client.close();
        store.setState({ ...initial, loading: false, dirty: { exit: false, advertise: false } });
      };
    },
    changeExitNode: (exitNode: string) => {
      if (!native || store.getState().operation) return;
      store.setState((state) => ({
        exitNode,
        saved: undefined,
        dirty: { ...state.dirty, exit: true },
      }));
    },
    changeAllowLAN: (allowLAN: boolean) => {
      if (!native || store.getState().operation) return;
      store.setState((state) => ({
        allowLAN,
        saved: undefined,
        dirty: { ...state.dirty, exit: true },
      }));
    },
    changeAdvertise: (advertise: boolean) => {
      if (!native || store.getState().operation) return;
      store.setState((state) => ({
        advertise,
        saved: undefined,
        dirty: { ...state.dirty, advertise: true },
      }));
    },
    signIn: async () => {
      const status = store.getState().snapshot?.status;
      if (!status) return;
      await mutate("signin", (client, version) =>
        client.signIn(status, (message) => showAuthentication(version, message)),
      );
    },
    applyExitNode: async () => {
      const { snapshot, exitNode, allowLAN } = store.getState();
      if (!connected(snapshot)) return;
      await mutate("exit", (client) => client.selectExitNode(exitNode, allowLAN));
    },
    applyAdvertisement: async () => {
      const { snapshot, advertise } = store.getState();
      if (!connected(snapshot)) return;
      await mutate("advertise", (client) => client.advertiseExitNode(advertise));
    },
    retryForgejo: async () => {
      if (!native || store.getState().operation || !connected(store.getState().snapshot)) return;
      const version = generation;
      store.setState({ operation: "forgejo" });
      await refreshForgejo(version);
      if (current(version)) store.setState({ operation: undefined });
    },
  }));

  // One observer and one in-flight read per active page. Resources never enter state.
  async function observe(version: number) {
    await load(version);
    if (current(version)) timer = setTimeout(() => void observe(version), 3000);
  }
  function load(version: number): Promise<void> {
    if (!native || !current(version)) return Promise.resolve();
    if (reading) return reading;
    const request = readSnapshot(native, version, preferencesRevision);
    reading = request;
    void request.then(() => {
      if (reading === request) reading = null;
    });
    return request;
  }
  async function readSnapshot(client: NativeTailscale, version: number, revision: number) {
    try {
      const snapshot = await client.read();
      // A read begun before/during a write cannot stand in for its readback.
      if (!current(version) || revision !== preferencesRevision) return;
      const state = store.getState();
      store.setState({
        snapshot,
        readError: "",
        streamState: undefined,
        authURL: authenticationURL(
          snapshot.status.BackendState === "NeedsLogin" ? snapshot.status.AuthURL : "",
        ),
        ...(!state.dirty.exit
          ? {
              exitNode: exitSelection(snapshot),
              allowLAN: Boolean(snapshot.prefs.ExitNodeAllowLANAccess),
            }
          : {}),
        ...(!state.dirty.advertise ? { advertise: advertisesExitNode(snapshot.prefs) } : {}),
        ...(!connected(snapshot) ? { forgejoIdentity: "" } : {}),
      });
      if (!store.getState().operation) await syncForgejo(version);
    } catch (error) {
      if (current(version) && revision === preferencesRevision)
        store.setState({
          snapshot: null,
          authURL: null,
          streamState: undefined,
          readError: diagnostic(error),
        });
    } finally {
      if (current(version)) store.setState({ loading: false });
    }
  }
  async function syncForgejo(version: number) {
    if (!current(version)) return;
    const { snapshot, forgejoIdentity } = store.getState();
    if (!snapshot || !connected(snapshot)) {
      store.setState({ forgejoIdentity: "" });
      return;
    }
    const identity = `${snapshot.status.Self?.DNSName || ""}/${(snapshot.status.TailscaleIPs || []).join(",")}`;
    if (identity === forgejoIdentity) return;
    store.setState({ forgejoIdentity: identity });
    await refreshForgejo(version);
  }
  function refreshForgejo(version: number): Promise<void> {
    if (!native || !current(version)) return Promise.resolve();
    if (forgejo) return forgejo;
    const request = runForgejo(native, version);
    forgejo = request;
    void request.then(() => {
      if (forgejo === request) forgejo = null;
    });
    return request;
  }
  async function runForgejo(client: NativeTailscale, version: number) {
    try {
      await client.refreshForgejo();
      if (current(version)) store.setState({ forgejoError: "" });
    } catch (error) {
      if (current(version)) store.setState({ forgejoError: diagnostic(error) });
    }
  }
  function showAuthentication(version: number, message: AuthenticationMessage) {
    if (!current(version)) return;
    store.setState({
      authURL: authenticationURL(message.AuthURL),
      ...(message.BackendState ? { streamState: connectionState(message) } : {}),
    });
  }
  async function mutate(
    operation: "signin" | "exit" | "advertise",
    action: (client: NativeTailscale, version: number) => Promise<void>,
  ) {
    if (!native || store.getState().operation) return;
    const client = native,
      version = generation;
    const form = operation === "signin" ? undefined : operation;
    if (form) preferencesRevision++;
    store.setState((state) => ({
      operation,
      notice: "",
      saved: undefined,
      ...(form ? { dirty: { ...state.dirty, [form]: true } } : {}),
    }));
    let saved = false;
    try {
      await action(client, version);
      saved = true;
    } catch (error) {
      if (current(version)) store.setState({ notice: diagnostic(error) });
    }
    if (!current(version)) return;
    // Retire pre-write observations before clearing draft protection. This is a
    // single explicit post-command read, not a retry queue or saved workflow.
    if (form) preferencesRevision++;
    if (reading) await reading;
    if (!current(version)) return;
    if (saved && form)
      store.setState((state) => ({ saved: form, dirty: { ...state.dirty, [form]: false } }));
    await load(version);
    if (!current(version)) return;
    store.setState({ operation: undefined });
    await syncForgejo(version);
  }
  return store;
}
export type TailscaleStore = ReturnType<typeof createTailscaleStore>;
