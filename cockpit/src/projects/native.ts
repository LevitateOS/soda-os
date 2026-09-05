import type { Cockpit } from "../cockpit/types";
import type { Invoke } from "./types";
import { coordinatorCommand, encodeRequest, decodeResponse } from "./protocol";

export function coordinator(cockpit: Pick<Cockpit, "spawn">): Invoke {
  return async (action, payload) => {
    const process = cockpit.spawn(coordinatorCommand(action), { err: "message" });
    process.input(encodeRequest(action, payload));
    if (!["remove", "remove-workspace", "delete-human"].includes(action))
      return decodeResponse(action, await process);
    // Failed removal exits nonzero but can deliver a complete receipt on stdout.
    // A lost/malformed response is unknown; never reconstruct it from stderr.
    let output = "";
    process.stream((chunk) => {
      output += chunk;
    });
    try {
      await process;
    } catch (error) {
      if (!output.trim()) throw error;
      const receipt = decodeResponse(action, output);
      if (!("ok" in receipt) || receipt.ok !== false) throw error;
      return receipt;
    }
    return decodeResponse(action, output);
  };
}
