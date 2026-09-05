import { authenticationStream } from "./stream.mjs";

export const cli = "/usr/bin/tailscale";
export const adminURL = "https://login.tailscale.com/admin/machines";
export const cliURL = "https://tailscale.com/docs/reference/tailscale-cli";

export function nativeTailscale(cockpit) {
  const processes = new Set();
  const http = cockpit.http("/var/run/tailscale/tailscaled.sock", {
    superuser: "require", headers: { Host: "local-tailscaled.sock" },
  });
  function spawn(args, privileged = true) {
    const process = cockpit.spawn(args, { ...(privileged ? { superuser: "require" } : {}), err: "message" });
    processes.add(process);
    const remove = () => processes.delete(process);
    process.then(remove, remove);
    return process;
  }
  async function readStatus() {
    const status = JSON.parse(await spawn([cli, "status", "--json"], false));
    if (typeof status.BackendState !== "string") throw new Error("Tailscale did not report its connection state.");
    return status;
  }
  async function request(method, path, body) {
    return http.request({ method, path: `/localapi/v0/${path}`, body: body === undefined ? "" : JSON.stringify(body), ...(body === undefined ? {} : { headers: { "Content-Type": "application/json" } }) });
  }
  return {
    readStatus,
    async read() {
      const status = await readStatus();
      const prefs = JSON.parse(await request("GET", "prefs"));
      return { status, prefs };
    },
    async signIn(status, onMessage) {
      // Reauthentication preserves every native preference, including settings
      // managed outside this page. `up --json --reset` would erase them.
      if (status.HaveNodeKey) {
        await request("PATCH", "prefs", { WantRunning: true, WantRunningSet: true });
        if (status.BackendState === "NeedsLogin" || status.Self?.Expired) {
          await request("POST", "login-interactive");
        }
        return;
      }
      const process = spawn([cli, "up", "--json"]);
      let streamError;
      const stream = authenticationStream(message => {
        if (message.Error) throw new Error(message.Error);
        onMessage(message);
      });
      process.stream(chunk => {
        try { stream.push(chunk); }
        catch (error) { streamError = error; process.close("cancelled"); }
      });
      try { await process; }
      catch (error) { throw streamError || error; }
      stream.finish();
    },
    async selectExitNode(address, allowLAN) {
      await spawn([cli, "set", `--exit-node=${address}`, `--exit-node-allow-lan-access=${Boolean(address) && allowLAN}`]);
    },
    async advertiseExitNode(enabled) {
      await spawn([cli, "set", `--advertise-exit-node=${enabled}`]);
    },
    async refreshForgejo() {
      await spawn(["/usr/libexec/soda/forgejo-init", "refresh-tailnet"]);
    },
    close() {
      for (const process of processes) process.close("cancelled");
      http.close("cancelled");
    },
  };
}
