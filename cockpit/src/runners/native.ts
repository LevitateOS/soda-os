import type { Cockpit } from "../cockpit/types";
import type { Invoke } from "./types";
import { coordinatorCommand, encodeRequest, decodeResponse } from "./protocol";

export function coordinator(cockpit: Pick<Cockpit, "spawn">): Invoke {
  return async (action, payload) => {
    const process = cockpit.spawn(coordinatorCommand(action), { err: "message" });
    process.input(encodeRequest(action, payload));
    return decodeResponse(action, await process);
  };
}
