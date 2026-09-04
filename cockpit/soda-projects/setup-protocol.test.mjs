import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  clearSetupSecrets,
  decodeSetupResponse,
  encodeSetupRequest,
  setupCommand,
} from "./setup-protocol.mjs";

test("Soda Setup uses one fixed privileged executable", () => {
  assert.deepEqual(setupCommand(), ["/usr/libexec/soda/soda-setup", "status"]);
  for (const action of ["allow-local-network", "connect-tailscale"]) {
    assert.deepEqual(setupCommand(action), ["/usr/libexec/soda/soda-setup", action]);
  }
  assert.throws(() => setupCommand("shell"), /unsupported Soda Setup action/);
});

test("setup secrets are serialized only in stdin and then cleared", () => {
  const payload = { auth_key: "tskey-auth-one-use" };
  assert.equal(encodeSetupRequest("connect-tailscale", payload), '{"auth_key":"tskey-auth-one-use"}\n');
  assert.deepEqual(setupCommand("connect-tailscale"), ["/usr/libexec/soda/soda-setup", "connect-tailscale"]);
  const control = { value: "tskey-auth-one-use" };
  clearSetupSecrets({ elements: { namedItem: name => name === "auth_key" ? control : null } }, payload);
  assert.equal(control.value, "");
  assert.deepEqual(payload, { auth_key: "" });
});

test("native setup errors remain visible", () => {
  const status = { ready: false, administrators: [], connections: [] };
  assert.deepEqual(decodeSetupResponse("status", JSON.stringify(status)), status);
  assert.throws(
    () => decodeSetupResponse("connect-tailscale", JSON.stringify({ status, error: "required facts are incomplete" })),
    /required facts are incomplete/,
  );
});

test("Cockpit presents the approved setup and network contract", async () => {
  const html = await readFile(new URL("./index.html", import.meta.url), "utf8");
  assert.match(html, /<h2 id="soda-setup-title">Soda Setup<\/h2>/);
  assert.match(html, />Allow access from the local network\.<\/button>/);
  assert.match(html, /Tailscale auth key/);
  assert.doesNotMatch(html, /LAN detected|cloud detected|RFC1918|private address/i);
});

test("Setup has no account or key-entry action", async () => {
 for (const action of ["create-administrator", "dismiss"]) assert.throws(() => setupCommand(action));
 const html = await readFile(new URL("./index.html", import.meta.url), "utf8");
 assert.doesNotMatch(html, /setup-administrator|dismiss-setup/);
 assert.match(html, /Authorized public SSH keys/);
});
