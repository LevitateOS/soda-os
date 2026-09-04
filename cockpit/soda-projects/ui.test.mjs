import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  clearPayloadSecrets,
  clearSecrets,
  errorMessage,
  formActions,
  humanDeletionHidden,
  payloadFor,
  projectRemovalHidden,
  successMessage,
} from "./ui.mjs";

test("every destructive and mutating form is wired to one supported action", async () => {
  const html = await readFile(new URL("./index.html", import.meta.url), "utf8");
  const wired = [...html.matchAll(/data-action-form="([^"]+)"/g)].map(match => match[1]);
  assert.deepEqual(wired, formActions);
});

test("form payloads keep secrets only in the synchronous request object", () => {
  const forgejo = payloadFor("create-forgejo", new Map([
    ["id", "site"],
    ["display_name", "Site"],
    ["password", "one-use"],
  ]), assert.fail);
  assert.deepEqual(forgejo, { id: "site", display_name: "Site", password: "one-use" });

  const setup = payloadFor("setup", new Map([
    ["id", "site"],
	["forgejo_password", "one-use"],
	["workspace_tools", "node@22\n\npython@3.13"],
	["project_tools", "go@1.25"],
  ]), assert.fail);
	assert.deepEqual(setup, {
	  id: "site",
	  forgejo_password: "one-use",
	  workspace_tools: ["node@22", "python@3.13"],
	  project_tools: ["go@1.25"],
	});

	const tools = payloadFor("install-tools", new Map([
	  ["id", "site"],
	  ["scope", "project"],
	  ["tools", "ripgrep@latest"],
	]), assert.fail);
	assert.deepEqual(tools, { id: "site", scope: "project", tools: ["ripgrep@latest"] });

  const person = payloadFor("add-person", new Map([
    ["username", "bob"],
    ["password", "initial secret"],
    ["password_confirmation", "initial secret"],
    ["authorized_key", "ssh-ed25519 AAAA"],
  ]), assert.fail);
  assert.deepEqual(person, { username: "bob", password: "initial secret", authorized_key: "ssh-ed25519 AAAA" });
});

test("catalog forms accept arbitrary JSON metadata without a closed field list", () => {
  const messages = [];
  const payload = payloadFor("add-existing", new Map([
    ["id", "site"],
    ["display_name", "Site"],
    ["canonical_url", "git@git.example.test:team/site.git"],
    ["additional_metadata", '{"team":"web","labels":["public"],"workspace_username":"catalog-value"}'],
  ]), message => messages.push(message));
  assert.deepEqual(payload, {
    team: "web",
    labels: ["public"],
    workspace_username: "catalog-value",
    id: "site",
    display_name: "Site",
    canonical_url: "git@git.example.test:team/site.git",
  });
  assert.deepEqual(messages, []);

  assert.equal(payloadFor("edit", new Map([
    ["id", "site"],
    ["display_name", "Site"],
    ["canonical_url", "git@git.example.test:team/site.git"],
    ["additional_metadata", '{"id":"other"}'],
  ]), message => messages.push(message)), null);
  assert.deepEqual(messages, ["Additional metadata must not redefine id."]);
});

test("secret clearing covers form controls and request objects", () => {
  const controls = {
    password: { value: "forgejo-secret" },
    password_confirmation: { value: "forgejo-secret" },
	forgejo_password: { value: "forgejo-secret" },
  };
  clearSecrets({ elements: { namedItem: name => controls[name] ?? null } });
  assert.equal(controls.password.value, "");
  assert.equal(controls.password_confirmation.value, "");
	assert.equal(controls.forgejo_password.value, "");

	const payload = { password: "forgejo-secret", forgejo_password: "forgejo-secret", id: "site" };
  clearPayloadSecrets(payload);
	assert.deepEqual(payload, { password: "", forgejo_password: "", id: "site" });
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

test("workspace setup distinguishes bundled and external SSH-key ownership", async () => {
  const html = await readFile(new URL("./index.html", import.meta.url), "utf8");
  const dialog = html.match(/<dialog id="setup-project-dialog"[\s\S]*?<\/dialog>/)?.[0] ?? "";
  assert.match(dialog, /bundled Forgejo repository/);
  assert.match(dialog, /external host, that host owns access/);
  assert.match(dialog, /reports the public key for you to register there before retrying/);
  assert.match(dialog, /Required only for bundled Forgejo/);
});

test("add person requires matching password confirmation", () => {
  const messages = [];
  const payload = payloadFor("add-person", new Map([
    ["username", "bob"],
    ["password", "one"],
    ["password_confirmation", "two"],
    ["authorized_key", "ssh-ed25519 AAAA"],
  ]), message => messages.push(message));
  assert.equal(payload, null);
  assert.deepEqual(messages, ["The password confirmation does not match."]);
});

test("native synchronous diagnostics and outcomes remain visible", () => {
  assert.equal(errorMessage(new Error("native Git authentication failed")), "native Git authentication failed");
  assert.equal(errorMessage({}), "The operation failed without a diagnostic message.");
  assert.equal(
    successMessage("remove", { id: "site" }, { ok: true }),
    "site and its local workspaces were removed. The canonical repository was not deleted.",
  );
  assert.equal(
    successMessage("add-person", { username: "bob" }, { ok: true }),
    "bob was added with a matching Forgejo account and public SSH key.",
  );
  assert.equal(
    successMessage("delete-human", { username: "bob" }, { ok: true }),
    "bob, their local Soda workspaces, and their Forgejo account were removed.",
  );
});
