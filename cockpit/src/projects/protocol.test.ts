import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { test } from "vite-plus/test";

import {
  actions,
  coordinatorCommand,
  coordinatorPath,
  decodeResponse,
  encodeRequest,
} from "./protocol";

test("manifest exposes exactly one stock Cockpit Projects page", async () => {
  const manifest = JSON.parse(
    await readFile(new URL("../../soda-projects/manifest.json", import.meta.url), "utf8"),
  );
  const page = await readFile(new URL("../../soda-projects/index.html", import.meta.url), "utf8");
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
  assert.deepEqual(actions, [
    "list",
    "add-existing",
    "edit",
    "inspect",
    "removal-inspect",
    "setup",
    "remove-workspace",
    "remove",
    "delete-human",
  ]);
  for (const action of actions) {
    assert.deepEqual(coordinatorCommand(action), [coordinatorPath, action]);
  }
  assert.throws(() => coordinatorCommand("shell"), /unsupported coordinator action/);
});

test("credentials are serialized only into stdin payload", () => {
  const payload = { id: "website" };
  assert.equal(encodeRequest("setup", payload), '{"id":"website"}\n');
  assert.deepEqual(coordinatorCommand("setup"), [coordinatorPath, "setup"]);
});

test("request encoder accepts objects and rejects alternate wire shapes", () => {
  assert.equal(encodeRequest("list", {}), "{}\n");
  assert.throws(() => encodeRequest("list", []), /must be a JSON object/);
  assert.throws(() => encodeRequest("list", null), /must be a JSON object/);
});

test("list response has only catalog, workspace-existence, and current-user fields", () => {
  const response = {
    projects: [
      {
        id: "website",
        display_name: "Website",
        canonical_url: "git@example.test:team/website.git",
        catalog_metadata: { team: "web" },
        workspace_username: "soda-w-0123456789abcdef01234567",
        workspace_exists: true,
      },
    ],
    current_user: { username: "alice", administrator: true },
  };
  const decoded = decodeResponse("list", JSON.stringify(response));
  assert.deepEqual(decoded, response);
  assert.deepEqual(Object.keys(decoded), ["projects", "current_user"]);
  assert.deepEqual(Object.keys(decoded.projects[0]), [
    "id",
    "display_name",
    "canonical_url",
    "catalog_metadata",
    "workspace_username",
    "workspace_exists",
  ]);
  assert.throws(
    () => decodeResponse("list", JSON.stringify({ ...response, projects: null })),
    /missing projects/,
  );
  assert.throws(
    () =>
      decodeResponse(
        "list",
        JSON.stringify({
          ...response,
          projects: [
            { ...response.projects[0], workspace_exists: undefined, workspace_ready: true },
          ],
        }),
      ),
    /missing workspace existence/,
  );
});

test("inspection requires native account and checkout facts, not a readiness guess", () => {
  const workspace = {
    username: "soda-w-example",
    exists: true,
    checkout_path: "/home/soda-w-example/Projects/site",
    checkout_ready: true,
    public_key: "ssh-ed25519 example",
    primary_key_problem: "",
    workspace_key_problem: "",
    checkout_problem: "",
    git_key_problem: "",
  };
  assert.deepEqual(
    decodeResponse("inspect", JSON.stringify({ ok: true, workspace })).workspace,
    workspace,
  );
  for (const changes of [
    { exists: false },
    { checkout_path: "" },
    { checkout_problem: "unreadable" },
    { checkout_ready: undefined },
    { public_key: undefined },
  ]) {
    assert.throws(() =>
      decodeResponse(
        "inspect",
        JSON.stringify({ ok: true, workspace: { ...workspace, ...changes } }),
      ),
    );
  }
  assert.deepEqual(coordinatorCommand("inspect"), [coordinatorPath, "inspect"]);
  assert.equal(encodeRequest("inspect", { id: "site" }), '{"id":"site"}\n');
});

test("mutation responses contain the action-specific result", () => {
  const project = {
    id: "website",
    display_name: "Website",
    canonical_url: "git@example.test:team/website.git",
    catalog_metadata: {},
  };
  assert.deepEqual(decodeResponse("add-existing", JSON.stringify({ ok: true, project })), {
    ok: true,
    project,
  });
  assert.deepEqual(
    decodeResponse("setup", '{"ok":true,"workspace_username":"soda-w-0123456789abcdef01234567"}'),
    { ok: true, workspace_username: "soda-w-0123456789abcdef01234567" },
  );
  const partial = {
    ok: false,
    result: {
      removed: ["alice-space"],
      uncertain: "bob-space",
      not_attempted: ["carol-space"],
      diagnostic: "native deletion failed",
    },
    catalog: "not_attempted",
    problem: "",
  };
  assert.deepEqual(decodeResponse("remove", JSON.stringify(partial)), partial);
  assert.throws(() => decodeResponse("remove", '{"ok":true}'), /invalid removal receipt/);
  assert.throws(
    () => decodeResponse("remove", JSON.stringify({ ...partial, ok: true })),
    /unresolved outcomes/,
  );
});
