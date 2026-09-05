import { test, expect, vi } from "vite-plus/test";
import type { HttpRequest, SpawnOptions } from "../cockpit/types";
import type { AuthenticationMessage } from "./types";
import { pendingProcess } from "../../tests/process";
import { nativeTailscale, cli } from "./native";

function fakeCockpit() {
  const pending: ReturnType<typeof pendingProcess>[] = [];
  const http = {
    request: vi.fn(async (_request: HttpRequest) => '{"ExitNodeAllowLANAccess":true}'),
    close: vi.fn(),
  };
  let output = "";
  const cockpit = {
    http: vi.fn((_path: string, _options: unknown) => http),
    spawn: vi.fn((args: string[], _options: SpawnOptions) => {
      const call = pendingProcess();
      if (args.includes("up")) pending.push(call);
      else call.resolve(output);
      return call.process;
    }),
  };
  return {
    cockpit,
    pending,
    http,
    set output(value: string) {
      output = value;
    },
  };
}
test("initial sign-in streams URL before completion and closes only the page process", async () => {
  const fake = fakeCockpit(),
    native = nativeTailscale(fake.cockpit),
    messages: AuthenticationMessage[] = [];
  let complete = false;
  const auth = native
    .signIn({ BackendState: "NeedsLogin" }, (message) => messages.push(message))
    .then(() => {
      complete = true;
    });
  fake.pending[0].emit('{\n "AuthURL": "https://login.tailscale.com/a/test"\n}\n');
  expect(messages[0].AuthURL).toBe("https://login.tailscale.com/a/test");
  expect(complete).toBe(false);
  fake.pending[0].emit('{"BackendState":"Running"}');
  fake.pending[0].resolve("");
  await auth;
  expect(fake.cockpit.spawn).toHaveBeenCalledWith([cli, "up", "--json"], {
    superuser: "require",
    err: "message",
  });
  const cancelled = native.signIn({}, () => {});
  native.close();
  await expect(cancelled).rejects.toMatchObject({ problem: "cancelled" });
  expect(fake.pending[1].process.close).toHaveBeenCalledWith("cancelled");
  expect(fake.http.close).toHaveBeenCalledWith("cancelled");
  expect(
    fake.cockpit.spawn.mock.calls.some(
      ([args]) => args.includes("logout") || args.includes("down"),
    ),
  ).toBe(false);
});
test("native preferences survive reconnect and reauthentication", async () => {
  const fake = fakeCockpit(),
    native = nativeTailscale(fake.cockpit);
  await native.signIn({ HaveNodeKey: true, BackendState: "Stopped" }, () => {});
  expect(fake.http.request).toHaveBeenCalledWith({
    method: "PATCH",
    path: "/localapi/v0/prefs",
    body: '{"WantRunning":true,"WantRunningSet":true}',
    headers: { "Content-Type": "application/json" },
  });
  await native.signIn({ HaveNodeKey: true, BackendState: "NeedsLogin" }, () => {});
  expect(fake.http.request.mock.calls[2][0]).toEqual({
    method: "POST",
    path: "/localapi/v0/login-interactive",
    body: "",
  });
  expect(fake.cockpit.spawn).not.toHaveBeenCalled();
});
test("read recovers native auth URL; mutations use only the requested settings", async () => {
  const fake = fakeCockpit(),
    native = nativeTailscale(fake.cockpit);
  fake.output = '{"BackendState":"NeedsLogin","AuthURL":"https://login.tailscale.com/a/pending"}';
  expect(await native.read()).toEqual({
    status: { BackendState: "NeedsLogin", AuthURL: "https://login.tailscale.com/a/pending" },
    prefs: { ExitNodeAllowLANAccess: true },
  });
  expect(fake.cockpit.http).toHaveBeenCalledWith("/var/run/tailscale/tailscaled.sock", {
    superuser: "require",
    headers: { Host: "local-tailscaled.sock" },
  });
  expect(fake.cockpit.spawn.mock.calls[0][1].superuser).toBeUndefined();
  await native.selectExitNode("100.64.0.2", true);
  await native.advertiseExitNode(true);
  await native.refreshForgejo();
  expect(fake.cockpit.spawn.mock.calls.slice(1).map(([args]) => args)).toEqual([
    [cli, "set", "--exit-node=100.64.0.2", "--exit-node-allow-lan-access=true"],
    [cli, "set", "--advertise-exit-node=true"],
    ["/usr/libexec/soda/forgejo-init", "refresh-tailnet"],
  ]);
});
test("daemon and privilege failures are propagated without another command", async () => {
  const fake = fakeCockpit();
  const call = pendingProcess();
  fake.cockpit.spawn.mockReturnValue(call.process);
  const operation = nativeTailscale(fake.cockpit).advertiseExitNode(true);
  call.reject({ problem: "access-denied" });
  await expect(operation).rejects.toMatchObject({ problem: "access-denied" });
  expect(fake.cockpit.spawn).toHaveBeenCalledTimes(1);
  const malformed = fakeCockpit();
  malformed.output = "{}";
  await expect(nativeTailscale(malformed.cockpit).read()).rejects.toThrow("connection state");
});
test("stream parse errors cancel their native process and preserve the diagnostic", async () => {
  const fake = fakeCockpit(),
    native = nativeTailscale(fake.cockpit);
  const operation = native.signIn({}, () => {});
  fake.pending[0].emit("not json");
  await expect(operation).rejects.toThrow("Invalid Tailscale authentication response.");
  expect(fake.pending[0].process.close).toHaveBeenCalledWith("cancelled");
});
