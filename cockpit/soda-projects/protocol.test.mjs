import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  actions,
  coordinatorCommand,
  coordinatorPath,
  decodeResponse,
  encodeRequest,
} from "./protocol.mjs";

test("manifest exposes exactly one stock Cockpit Projects page", async () => {
  const manifest = JSON.parse(await readFile(new URL("./manifest.json", import.meta.url), "utf8"));
  const page = await readFile(new URL("./index.html", import.meta.url), "utf8");
  assert.deepEqual(Object.keys(manifest.menu), ["index"]);
  assert.equal(manifest.menu.index.label, "Projects");
  assert.equal(manifest.menu.index.path, "index.html");
  assert.equal(manifest.bridges, undefined);
  assert.equal(manifest.dashboard, undefined);
  assert.equal(manifest.tools, undefined);
  assert.match(page, /src="\.\.\/base1\/cockpit\.js"/);
  assert.doesNotMatch(page, /htmx/i);
});

test("coordinator command contains only the executable and allow-listed action", () => {
  for (const action of actions) {
    assert.deepEqual(coordinatorCommand(action), [coordinatorPath, action]);
  }
  assert.throws(() => coordinatorCommand("shell"), /unsupported coordinator action/);
});

test("credentials are serialized only into stdin payload", () => {
  const payload = { id: "website" };
  assert.equal(
    encodeRequest("setup", payload),
    '{"id":"website"}\n',
  );
  assert.deepEqual(coordinatorCommand("setup"), [coordinatorPath, "setup"]);
});

test("request encoder accepts objects and rejects alternate wire shapes", () => {
  assert.equal(encodeRequest("list", {}), "{}\n");
  assert.throws(() => encodeRequest("list", []), /must be a JSON object/);
  assert.throws(() => encodeRequest("list", null), /must be a JSON object/);
});

test("list response requires native user and service context", () => {
  const response = {
    projects: [{
      id: "website",
      display_name: "Website",
      canonical_url: "git@example.test:team/website.git",
      workspace_username: "soda-w-0123456789abcdef01234567",
      workspace_ready: true,
    }],
    current_user: { username: "alice", administrator: true },
    forgejo_url: "https://soda.tail.example/forgejo",
    ssh_host: "soda.tail.example",
  };
  assert.deepEqual(decodeResponse("list", JSON.stringify(response)), response);
  assert.throws(
    () => decodeResponse("list", JSON.stringify({ ...response, projects: null })),
    /missing projects/,
  );
});

test("mutation responses contain the action-specific result", () => {
  const project = {
    id: "website",
    display_name: "Website",
    canonical_url: "git@example.test:team/website.git",
  };
  assert.deepEqual(
    decodeResponse("add-existing", JSON.stringify({ ok: true, project })),
    { ok: true, project },
  );
  assert.deepEqual(
    decodeResponse("setup", '{"ok":true,"workspace_username":"soda-w-0123456789abcdef01234567"}'),
    { ok: true, workspace_username: "soda-w-0123456789abcdef01234567" },
  );
  assert.deepEqual(decodeResponse("remove", '{"ok":true}'), { ok: true });
  assert.throws(() => decodeResponse("remove", '{"ok":false}'), /did not report success/);
});
