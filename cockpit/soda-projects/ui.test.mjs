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
    ["git_username", "alice"],
    ["git_password", "one-use"],
  ]), assert.fail);
  assert.deepEqual(setup, { id: "site", git_username: "alice", git_password: "one-use" });
});

test("secret clearing covers form controls and request objects", () => {
  const controls = {
    password: { value: "forgejo-secret" },
    git_password: { value: "git-secret" },
  };
  clearSecrets({ elements: { namedItem: name => controls[name] ?? null } });
  assert.equal(controls.password.value, "");
  assert.equal(controls.git_password.value, "");

  const payload = { password: "forgejo-secret", git_password: "git-secret", id: "site" };
  clearPayloadSecrets(payload);
  assert.deepEqual(payload, { password: "", git_password: "", id: "site" });
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
  assert.deepEqual(messages, [
    "Type site exactly to confirm project removal.",
    "The confirmation username does not match.",
  ]);
});

test("human deletion presentation is wheel-status driven", () => {
  assert.equal(humanDeletionHidden({ administrator: true }), false);
  assert.equal(humanDeletionHidden({ administrator: false }), true);
  assert.equal(humanDeletionHidden({}), true);
});

test("native synchronous diagnostics and outcomes remain visible", () => {
  assert.equal(errorMessage(new Error("native Git authentication failed")), "native Git authentication failed");
  assert.equal(errorMessage({}), "The operation failed without a diagnostic message.");
  assert.equal(
    successMessage("remove", { id: "site" }, { ok: true }),
    "site and its local workspaces were removed. The canonical repository was not deleted.",
  );
});
