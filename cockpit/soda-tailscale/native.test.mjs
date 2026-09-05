import { test } from "node:test";
import assert from "node:assert/strict";
import { nativeTailscale, cli } from "./native.mjs";

function fakeCockpit() {
  const calls = [], requests = [], pending = [];
  let nextOutput = "";
  const cockpit = {
    calls, requests, pending,
    set output(value) { nextOutput = value; },
    http(path, options) {
      calls.push({ path, options });
      return { request: async value => { assert.ok(Object.hasOwn(value, "body"), "Cockpit requests must complete their input"); requests.push(value); return '{"ExitNodeAllowLANAccess":true}'; }, close: reason => calls.push({ httpClosed: reason }) };
    },
    spawn(args, options) {
      calls.push({ args, options });
      let resolve, reject;
      const process = new Promise((yes, no) => { resolve = yes; reject = no; });
      process.always = callback => { process.then(callback, callback); };
      process.stream = callback => { process.emit = callback; };
      process.close = reason => { process.closed = reason; reject({ problem: reason }); };
      process.resolve = resolve;
      process.reject = reject;
      if (args.includes("up") && args.includes("--json")) pending.push(process);
      else resolve(nextOutput);
      return process;
    },
  };
  return cockpit;
}

test("initial sign-in streams URL before completion and closes only the page process", async () => {
  const cockpit = fakeCockpit(), native = nativeTailscale(cockpit), messages = [];
  let complete = false;
  const auth = native.signIn({ BackendState: "NeedsLogin" }, message => messages.push(message)).then(() => { complete = true; });
  const process = cockpit.pending[0];
  process.emit('{\n "AuthURL": "https://login.tailscale.com/a/test"\n}\n');
  assert.equal(messages[0].AuthURL, "https://login.tailscale.com/a/test");
  assert.equal(complete, false);
  process.emit('{"BackendState":"Running"}');
  process.resolve("");
  await auth;
  assert.deepEqual(cockpit.calls[1], { args: [cli, "up", "--json"], options: { superuser: "require", err: "message" } });
  const cancelled = native.signIn({}, () => {});
  native.close();
  await assert.rejects(cancelled, error => error.problem === "cancelled");
  assert.equal(cockpit.pending[1].closed, "cancelled");
  assert.equal(cockpit.calls.some(call => call.args?.includes("logout") || call.args?.includes("down")), false);
});

test("native preferences survive reconnect and reauthentication", async () => {
  const cockpit = fakeCockpit(), native = nativeTailscale(cockpit);
  await native.signIn({ HaveNodeKey: true, BackendState: "Stopped" }, () => {});
  assert.deepEqual(cockpit.requests.map(request => [request.method, request.path, request.body]), [
    ["PATCH", "/localapi/v0/prefs", '{"WantRunning":true,"WantRunningSet":true}'],
  ]);
  await native.signIn({ HaveNodeKey: true, BackendState: "NeedsLogin" }, () => {});
  assert.equal(cockpit.requests[2].path, "/localapi/v0/login-interactive");
  assert.equal(cockpit.calls.filter(call => call.args).length, 0);
});

test("read recovers native auth URL; mutations use only the requested settings", async () => {
  const cockpit = fakeCockpit(), native = nativeTailscale(cockpit);
  cockpit.output = '{"BackendState":"NeedsLogin","AuthURL":"https://login.tailscale.com/a/pending"}';
  const data = await native.read();
  assert.equal(data.status.AuthURL, "https://login.tailscale.com/a/pending");
  assert.equal(data.prefs.ExitNodeAllowLANAccess, true);
  assert.equal(cockpit.calls[1].options.superuser, undefined);
  await native.selectExitNode("100.64.0.2", true);
  await native.advertiseExitNode(true);
  await native.refreshForgejo();
  assert.deepEqual(cockpit.calls.slice(2).map(call => call.args), [
    [cli, "set", "--exit-node=100.64.0.2", "--exit-node-allow-lan-access=true"],
    [cli, "set", "--advertise-exit-node=true"],
    ["/usr/libexec/soda/forgejo-init", "refresh-tailnet"],
  ]);
});

test("daemon and privilege failures are propagated without another command", async () => {
  const cockpit = fakeCockpit();
  cockpit.spawn = () => {
    const process = Promise.reject({ problem: "access-denied" });
    process.always = callback => process.catch(callback);
    return process;
  };
  await assert.rejects(nativeTailscale(cockpit).advertiseExitNode(true), error => error.problem === "access-denied");
  const malformed = fakeCockpit();
  malformed.output = "{}";
  await assert.rejects(nativeTailscale(malformed).read(), /connection state/);
});
