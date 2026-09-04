import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  errorMessage,
  formActions,
  humanDeletionHidden,
  payloadFor,
  projectRemovalHidden,
  sshCommand,
  successMessage,
} from "./ui.mjs";

test("every destructive and mutating form is wired to one supported action", async () => {
  assert.deepEqual(formActions, [
    "add-existing",
    "edit",
    "setup",
    "remove-workspace",
    "remove",
    "delete-human",
  ]);
  const html = await readFile(new URL("./index.html", import.meta.url), "utf8");
  const wired = [...html.matchAll(/data-action-form="([^"]+)"/g)].map(match => match[1]);
  assert.deepEqual(wired, formActions);
});

test("setup sends only the selected project id", () => {
  const setup = payloadFor("setup", new Map([
    ["id", "site"],
  ]), assert.fail);
  assert.deepEqual(setup, { id: "site" });
});

test("catalog forms preserve arbitrary metadata while edit omits the immutable URL", () => {
  const messages = [];
  const add = payloadFor("add-existing", new Map([
    ["id", "site"],
    ["display_name", "Site"],
    ["canonical_url", "git@git.example.test:team/site.git"],
    ["additional_metadata", '{"team":"web","labels":["public"],"workspace_username":"catalog-value"}'],
  ]), message => messages.push(message));
  assert.deepEqual(add, {
    team: "web",
    labels: ["public"],
    workspace_username: "catalog-value",
    id: "site",
    display_name: "Site",
    canonical_url: "git@git.example.test:team/site.git",
  });
  const edit = payloadFor("edit", new Map([
    ["id", "site"],
    ["display_name", "Renamed site"],
    ["canonical_url", "git@replacement.example.test:team/site.git"],
    ["additional_metadata", '{"team":"platform"}'],
  ]), message => messages.push(message));
  assert.deepEqual(edit, {
    team: "platform",
    id: "site",
    display_name: "Renamed site",
  });
  assert.equal(Object.hasOwn(edit, "canonical_url"), false);
  assert.deepEqual(messages, []);

  assert.equal(payloadFor("edit", new Map([
    ["id", "site"],
    ["display_name", "Site"],
    ["canonical_url", "git@git.example.test:team/site.git"],
    ["additional_metadata", '{"id":"other"}'],
  ]), message => messages.push(message)), null);
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
  const messages = [];
  assert.equal(payloadFor("remove", new Map([
    ["id", "site"],
    ["confirmation", "SITE"],
  ]), message => messages.push(message)), null);
  assert.equal(payloadFor("delete-human", new Map([
    ["username", "alice"],
    ["confirmation", "bob"],
  ]), message => messages.push(message)), null);
	assert.equal(payloadFor("remove-workspace", new Map([
	  ["id", "site"],
	  ["confirmation", "wrong"],
	]), message => messages.push(message)), null);
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

test("People leaves creation, listing, and administrator promotion to stock Cockpit Accounts", async () => {
  const html = await readFile(new URL("./index.html", import.meta.url), "utf8");
  const panel = html.match(/<section class="panel danger-panel" id="human-deletion-panel"[\s\S]*?<\/section>/)?.[0] ?? "";
  assert.match(panel, /Stock Cockpit Accounts creates and lists primary Linux users and owns administrator status/);
});

test("whole-project removal presentation is wheel-status driven", () => {
  assert.equal(projectRemovalHidden({ administrator: true }), false);
  assert.equal(projectRemovalHidden({ administrator: false }), true);
});

test("human deletion confirmation states Forgejo's native consequences", async () => {
  const html = await readFile(new URL("./index.html", import.meta.url), "utf8");
  const dialog = html.match(/<dialog id="delete-human-dialog"[\s\S]*?<\/dialog>/)?.[0] ?? "";
  assert.match(dialog, /without <code>--purge<\/code>/);
  assert.match(dialog, /SSH and GPG keys, access tokens, email addresses, settings, and user data/);
  assert.match(dialog, /issues, pull requests, and comments as deleted-user history/);
  assert.match(dialog, /owns a repository or package, belongs to an organization, or is its last administrator/);
  assert.match(dialog, /Linux account remains so an administrator can retry/);
});

test("workspace setup leaves SSH-key registration to every authoritative host", async () => {
  const html = await readFile(new URL("./index.html", import.meta.url), "utf8");
  const dialog = html.match(/<dialog id="setup-project-dialog"[\s\S]*?<\/dialog>/)?.[0] ?? "";
  assert.match(dialog, /authoritative Git host owns access/);
  assert.match(dialog, /reports the public key for you to register with that host before retrying/);
  assert.doesNotMatch(dialog, /Forgejo password/);
  assert.doesNotMatch(dialog, /bundled.*register/i);
  assert.match(dialog, /data-form-notice role="alert" hidden/);
});

test("edit explains immutable repository replacement and exposes a read-only URL", async () => {
  const html = await readFile(new URL("./index.html", import.meta.url), "utf8");
  const dialog = html.match(/<dialog id="edit-project-dialog"[\s\S]*?<\/dialog>/)?.[0] ?? "";
  assert.match(dialog, /name="canonical_url" readonly/);
  assert.doesNotMatch(dialog, /name="canonical_url"[^>]*disabled/);
  assert.match(dialog, /administrator must remove the project and its local workspaces, then add the project again/);
  assert.match(dialog, /does not delete the canonical repository/);
});

test("workspace wording states existence without claiming clone readiness", async () => {
  const app = await readFile(new URL("./app.mjs", import.meta.url), "utf8");
  assert.match(app, /project\.workspace_exists \? "Workspace account exists" : "No workspace account"/);
  assert.doesNotMatch(app, /workspace_ready/);
  assert.match(app, /actionsCell\.append\(projectButton\("Set up for me"/);
  assert.match(app, /if \(project\.workspace_exists\)[\s\S]*?projectButton\("Remove my workspace"/);
  assert.match(app, /window\.location\.hostname/);
});

test("native synchronous diagnostics and outcomes remain visible", () => {
  assert.equal(errorMessage(new Error("native Git authentication failed")), "native Git authentication failed");
  assert.equal(errorMessage({}), "The operation failed without a diagnostic message.");
  assert.equal(
    successMessage("remove", { id: "site" }, { ok: true }),
    "site and its local workspaces were removed. The canonical repository was not deleted.",
  );
  assert.equal(
    successMessage("delete-human", { username: "bob" }, { ok: true }),
    "bob, their local Soda workspaces, and their Forgejo account were removed.",
  );
});

test("failed setup refreshes workspace existence and keeps its diagnostic in the open dialog", async () => {
  const app = await readFile(new URL("./app.mjs", import.meta.url), "utf8");
  assert.match(app, /if \(action === "setup"\) \{\s*await loadProjects\(\);\s*\}/);
  assert.match(app, /showFormNotice\(form, message\)/);
});
