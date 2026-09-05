import { createStore } from "zustand/vanilla";
import type { Host, NativeUpdates, Release, Selection } from "./types";
import { availability, stagedSelection } from "./status";

interface State {
  host: Host | null;
  release: Release | null;
  operation: "read" | "check" | "download" | "apply" | null;
  error: string | null;
  readError: string | null;
  notice: string | null;
  progress: string;
  confirmation: Selection | null;
}
const initial: State = {
  host: null,
  release: null,
  operation: null,
  error: null,
  readError: null,
  notice: null,
  progress: "",
  confirmation: null,
};
export const operationLabels = {
  read: "Reading deployment status",
  check: "Checking and verifying the latest release",
  download: "Verifying and downloading the selected image",
  apply: "Verifying, enabling the update, and requesting restart",
};
export function createUpdatesStore(native: NativeUpdates) {
  let active = false,
    generation = 0;
  const current = (version: number) => active && generation === version;
  const store = createStore(() => ({
    ...initial,
    start: () => {
      active = true;
      const version = ++generation;
      store.setState({ ...initial });
      void store.getState().refresh();
      return () => {
        if (generation === version) {
          active = false;
          generation++;
        }
      };
    },
    refresh: async () => {
      if (!active || store.getState().operation) return;
      const version = generation;
      store.setState({ operation: "read" });
      try {
        await readHost(version);
      } catch {
        /* readHost owns the read error; command outcomes survive refresh. */
      } finally {
        if (current(version)) store.setState({ operation: null });
      }
    },
    check: () =>
      run("check", async (version) => {
        store.setState({ release: null });
        await readHost(version);
        if (!current(version)) return;
        try {
          const release = await native.check();
          if (current(version)) store.setState({ release });
        } catch (error) {
          if (!String(error).includes("no published stable Soda release is available")) throw error;
          if (current(version))
            store.setState({
              notice:
                "No published stable Soda release is available yet. Development candidates are not offered as updates.",
            });
        }
      }),
    download: async () => {
      const { host, release } = store.getState();
      if (
        !host ||
        !release ||
        host.status.staged ||
        host.status.rollbackQueued ||
        host.status.usrOverlay ||
        !availability(host, release).newer
      )
        return;
      await run("download", async (version) => {
        try {
          await native.download(release, (chunk) => progress(version, chunk));
        } catch (error) {
          if (!current(version)) return;
          try {
            await readHost(version);
          } catch {
            /* Preserve the command error independently. */
          }
          throw error;
        }
        if (!current(version)) return;
        try {
          await readHost(version);
        } catch (error) {
          if (current(version))
            store.setState({
              readError: `The image download completed, but current deployment status could not be read. Refresh status before applying the update. ${String(error)}`,
            });
        }
      });
    },
    requestApply: () => {
      const { host, operation } = store.getState();
      if (!active || operation || !host || host.status.rollbackQueued || host.status.usrOverlay)
        return;
      store.setState({ confirmation: stagedSelection(host) });
    },
    cancelApply: () => {
      if (!store.getState().operation) store.setState({ confirmation: null });
    },
    apply: async () => {
      const { confirmation, host } = store.getState();
      if (!confirmation || !host || host.status.rollbackQueued || host.status.usrOverlay) return;
      await run("apply", async (version) => {
        store.setState({ confirmation: null, host: null });
        try {
          await native.apply(confirmation, (chunk) => progress(version, chunk));
          if (current(version))
            store.setState({
              notice: "Restart requested. Reconnect and refresh to confirm the booted version.",
            });
        } catch (error) {
          if (!current(version)) return;
          try {
            await readHost(version);
          } catch {
            /* A disconnect proves neither activation nor failure. */
          }
          throw new Error(
            `${String(error)} Reconnect and refresh native deployment status before retrying; do not assume the update failed.`,
          );
        }
      });
    },
  }));
  async function readHost(version: number) {
    if (!current(version)) return;
    try {
      const host = await native.status();
      if (current(version)) store.setState({ host, readError: null });
    } catch (error) {
      if (current(version)) store.setState({ host: null, readError: String(error) });
      throw error;
    }
  }
  function progress(version: number, chunk: string) {
    if (current(version))
      store.setState((state) => ({ progress: (state.progress + chunk).slice(-16384) }));
  }
  async function run(
    operation: "check" | "download" | "apply",
    action: (version: number) => Promise<void>,
  ) {
    if (!active || store.getState().operation) return;
    const version = generation;
    store.setState({ operation, error: null, notice: null, progress: "" });
    try {
      await action(version);
    } catch (error) {
      if (current(version)) store.setState({ error: String(error) });
    } finally {
      if (current(version)) store.setState({ operation: null });
    }
  }
  return store;
}
export type UpdatesStore = ReturnType<typeof createUpdatesStore>;
