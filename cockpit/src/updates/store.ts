import { createStore } from "zustand/vanilla";
import type { Host, NativeUpdates } from "./types";
import { updateDiagnostic } from "./status";

interface State {
  host: Host | null;
  operation: "read" | "check" | "update" | null;
  error: string | null;
  readError: string | null;
  notice: string | null;
  progress: string;
}
const initial: State = {
  host: null,
  operation: null,
  error: null,
  readError: null,
  notice: null,
  progress: "",
};
export const operationLabels = {
  read: "Reading deployment status",
  check: "Checking the native image source",
  update: "Updating the native image and restarting when needed",
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
        store.setState({ host: null, readError: null });
        const host = await native.check();
        if (current(version)) store.setState({ host });
      }),
    update: async () => {
      if (updateDiagnostic(store.getState().host)) return;
      await run("update", async (version) => {
        store.setState({ host: null });
        try {
          await native.update((chunk) => progress(version, chunk));
          if (current(version))
            store.setState({
              notice:
                "Native update command completed. Reconnect and refresh to verify the actual booted digest; command completion alone is not boot proof.",
            });
        } catch (error) {
          throw new Error(
            `${String(error)} Reconnect and refresh native status before retrying; a disconnect proves neither success nor failure.`,
          );
        } finally {
          if (current(version)) {
            try {
              await readHost(version);
            } catch {
              /* Readback and command outcomes are independent, including disconnects. */
            }
          }
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
  async function run(operation: "check" | "update", action: (version: number) => Promise<void>) {
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
