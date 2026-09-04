import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { actions, coordinatorCommand, coordinatorPath, decodeResponse, encodeRequest } from "./protocol.mjs";

test("manifest exposes one dedicated Cockpit Runners page", async () => {
  const manifest = JSON.parse(await readFile(new URL("./manifest.json", import.meta.url), "utf8"));
  const page = await readFile(new URL("./index.html", import.meta.url), "utf8");
  assert.deepEqual(Object.keys(manifest.menu), ["index"]);
  assert.equal(manifest.menu.index.label, "Runners");
  assert.equal(manifest.bridges, undefined);
  assert.match(page, /Runner jobs execute repository code/);
  assert.match(page, /Provider authority/);
  assert.match(page, /name="github_labels" required value="soda-local"/);
  assert.doesNotMatch(page, /project binding|workflow queue|retry queue/i);
});

test("coordinator command contains only its executable and allow-listed action", () => {
  for (const action of actions) assert.deepEqual(coordinatorCommand(action), [coordinatorPath, action]);
  assert.throws(() => coordinatorCommand("shell"), /unsupported runner action/);
});

test("registration input is serialized only into the stdin payload", () => {
  const payload = { id: "github-one", provider: "github", registration_url: "https://github.com/example/repo", registration_id: "", labels: "soda", registration_token: "provider-input" };
  assert.equal(encodeRequest("create", payload), `${JSON.stringify(payload)}\n`);
  assert.deepEqual(coordinatorCommand("create"), [coordinatorPath, "create"]);
});

test("list response requires exact count, listener, and one-slot capacity facts", () => {
  const response = {
    runners: [{ id: "one", provider: "github", registration_url: "https://github.com/example/repo", account: "soda-runner-one", architecture: "AArch64", version: "2.337.0", capacity: 1, service: { load: "loaded", active: "active", sub: "running", enabled: "enabled" } }],
    runner_count: 1, active_listeners: 1, total_capacity: 1,
  };
  assert.deepEqual(decodeResponse("list", JSON.stringify(response)), response);
  assert.throws(() => decodeResponse("list", JSON.stringify({ ...response, total_capacity: -1 })), /invalid total_capacity/);
  assert.throws(() => decodeResponse("list", JSON.stringify({ ...response, runner_count: 2 })), /inconsistent/);
});
