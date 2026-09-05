import type { Cockpit } from "../cockpit/types";
import type { Host, NativeUpdates, Release, Selection } from "./types";

const cli = "/usr/libexec/soda/soda-updates";
export function nativeUpdates(cockpit: Pick<Cockpit, "spawn">): NativeUpdates {
  async function read<T>(operation: string): Promise<T> {
    return JSON.parse(
      await cockpit.spawn([cli, operation], { superuser: "require", err: "message" }),
    ) as T;
  }
  async function mutate(operation: string, selection: Selection, progress: (text: string) => void) {
    const process = cockpit.spawn(
      [cli, operation, "--version", selection.version, "--reference", selection.reference],
      { superuser: "require", err: "out" },
    );
    process.stream(progress);
    await process;
  }
  return {
    status: () => read<Host>("status"),
    check: () => read<Release>("check"),
    download: (selection, progress) => mutate("download", selection, progress),
    apply: (selection, progress) => mutate("apply", selection, progress),
  };
}
