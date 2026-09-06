import type { Cockpit } from "../cockpit/types";
import type { Host, NativeUpdates } from "./types";

const cli = "/usr/libexec/soda/soda-updates";
export function nativeUpdates(cockpit: Pick<Cockpit, "spawn">): NativeUpdates {
  async function read(operation: string): Promise<Host> {
    // Check progress is stderr; successful stdout contains exactly one JSON value.
    return JSON.parse(
      await cockpit.spawn([cli, operation], { superuser: "require", err: "message" }),
    ) as Host;
  }
  return {
    status: () => read("status"),
    check: () => read("check"),
    update: async (progress) => {
      const process = cockpit.spawn([cli, "update"], { superuser: "require", err: "out" });
      process.stream(progress);
      await process;
    },
  };
}
