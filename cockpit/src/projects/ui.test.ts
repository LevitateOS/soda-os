import assert from "node:assert/strict";
import { test } from "vite-plus/test";

import {
  errorMessage,
  humanDeletionHidden,
  payloadFor,
  projectRemovalHidden,
  sshCommand,
  successMessage,
} from "./ui";

test("setup sends only the selected project id", () => {
  const setup = payloadFor("setup", new Map([["id", "site"]]), assert.fail);
  assert.deepEqual(setup, { id: "site" });
});

test("catalog forms preserve arbitrary metadata while edit omits the immutable URL", () => {
  const messages: string[] = [];
  const add = payloadFor(
    "add-existing",
    new Map([
      ["id", "site"],
      ["display_name", "Site"],
      ["canonical_url", "git@git.example.test:team/site.git"],
      [
        "additional_metadata",
        '{"team":"web","labels":["public"],"workspace_username":"catalog-value"}',
      ],
    ]),
    (message) => messages.push(message),
  );
  assert.deepEqual(add, {
    team: "web",
    labels: ["public"],
    workspace_username: "catalog-value",
    id: "site",
    display_name: "Site",
    canonical_url: "git@git.example.test:team/site.git",
  });
  const edit = payloadFor(
    "edit",
    new Map([
      ["id", "site"],
      ["display_name", "Renamed site"],
      ["canonical_url", "git@replacement.example.test:team/site.git"],
      ["additional_metadata", '{"team":"platform"}'],
    ]),
    (message) => messages.push(message),
  );
  assert.deepEqual(edit, {
    team: "platform",
    id: "site",
    display_name: "Renamed site",
  });
  assert.equal(Object.hasOwn(edit!, "canonical_url"), false);
  assert.equal(messages.length, 0);

  assert.equal(
    payloadFor(
      "edit",
      new Map([
        ["id", "site"],
        ["display_name", "Site"],
        ["canonical_url", "git@git.example.test:team/site.git"],
        ["additional_metadata", '{"id":"other"}'],
      ]),
      (message) => messages.push(message),
    ),
    null,
  );
  assert.deepEqual(messages, ["Additional metadata must not redefine id."]);
});

test("SSH guidance follows the browser hostname and brackets IPv6", () => {
  const username = "soda-w-0123456789abcdef01234567";
  assert.equal(sshCommand(username, "192.0.2.10"), `ssh ${username}@192.0.2.10`);
  assert.equal(sshCommand(username, "soda.example.test"), `ssh ${username}@soda.example.test`);
  assert.equal(sshCommand(username, "2001:db8::10"), `ssh ${username}@[2001:db8::10]`);
  assert.equal(sshCommand(username, "[2001:db8::10]"), `ssh ${username}@[2001:db8::10]`);
});

test("destructive actions require exact confirmation", () => {
  const messages: string[] = [];
  assert.equal(
    payloadFor(
      "remove",
      new Map([
        ["id", "site"],
        ["confirmation", "SITE"],
      ]),
      (message) => messages.push(message),
    ),
    null,
  );
  assert.equal(
    payloadFor(
      "delete-human",
      new Map([
        ["username", "alice"],
        ["confirmation", "bob"],
      ]),
      (message) => messages.push(message),
    ),
    null,
  );
  assert.equal(
    payloadFor(
      "remove-workspace",
      new Map([
        ["id", "site"],
        ["confirmation", "wrong"],
      ]),
      (message) => messages.push(message),
    ),
    null,
  );
  assert.deepEqual(messages, [
    "Type site exactly to confirm project removal.",
    "The confirmation username does not match.",
    "Type site exactly to confirm workspace removal.",
  ]);
});

test("human deletion presentation is wheel-status driven", () => {
  assert.equal(humanDeletionHidden({ administrator: true }), false);
  assert.equal(humanDeletionHidden({ administrator: false }), true);
  assert.equal(humanDeletionHidden({}), true);
});

test("whole-project removal presentation is wheel-status driven", () => {
  assert.equal(projectRemovalHidden({ administrator: true }), false);
  assert.equal(projectRemovalHidden({ administrator: false }), true);
});

test("native synchronous diagnostics and outcomes remain visible", () => {
  assert.equal(
    errorMessage(new Error("native Git authentication failed")),
    "native Git authentication failed",
  );
  assert.equal(errorMessage({}), "The operation failed without a diagnostic message.");
  assert.equal(
    successMessage("remove", { id: "site" }, { ok: true }),
    "site and its local workspaces were removed. The canonical repository was not deleted.",
  );
  assert.equal(
    successMessage("delete-human", { username: "bob" }, { ok: true }),
    "bob and their local Soda workspaces were removed. Their Forgejo account was unchanged.",
  );
});
