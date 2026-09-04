import assert from "node:assert/strict";
import test from "node:test";

import { clearRegistrationSecret, createPayload, forgejoBrowserURL, providerURL, setProviderRequirements, statusText, successMessage } from "./ui.mjs";

function formData(values) { return { get: name => values[name] ?? "" }; }

test("provider forms retain only the provider-native registration fields", () => {
  const payload = createPayload(formData({ id: "forgejo-one", provider: "forgejo", registration_url: "https://external.invalid", registration_id: "33834eef-e758-48c4-a676-1745426747aa", forgejo_labels: "soda:host", registration_token: "provider-input" }));
  assert.deepEqual(payload, { id: "forgejo-one", provider: "forgejo", registration_url: "", registration_id: "33834eef-e758-48c4-a676-1745426747aa", labels: "soda:host", registration_token: "provider-input" });
  assert.equal(Object.hasOwn(payload, "project_id"), false);
});

test("registration input is cleared from form and payload", () => {
  const token = { value: "provider-input" };
  const payload = { registration_token: "provider-input" };
  clearRegistrationSecret({ elements: { namedItem: name => name === "registration_token" ? token : null } }, payload);
  assert.equal(token.value, "");
  assert.equal(payload.registration_token, "");
});

test("only the selected provider fields participate in validation", () => {
  const forgejoInputs = [{ required: false }, { required: false }];
  const githubInputs = [{ required: false }, { required: false }];

  setProviderRequirements(forgejoInputs, githubInputs, "github");
  assert.deepEqual(forgejoInputs.map(input => input.required), [false, false]);
  assert.deepEqual(githubInputs.map(input => input.required), [true, true]);

  setProviderRequirements(forgejoInputs, githubInputs, "forgejo");
  assert.deepEqual(forgejoInputs.map(input => input.required), [true, true]);
  assert.deepEqual(githubInputs.map(input => input.required), [false, false]);
});

test("local status never claims provider availability or idle capacity", () => {
  assert.equal(statusText({ active: "active", sub: "running" }), "Listening");
  assert.equal(statusText({ active: "failed", sub: "failed" }), "Failed");
  assert.match(successMessage("remove", "one"), /provider/);
  assert.equal(successMessage("stop", "one"), "one was stopped.");
});

test("bundled Forgejo links follow the Cockpit LAN or Tailnet host", () => {
  assert.equal(forgejoBrowserURL("soda.lan"), "http://soda.lan:30000");
  assert.equal(forgejoBrowserURL("fd7a:115c:a1e0::1"), "http://[fd7a:115c:a1e0::1]:30000");
  assert.equal(providerURL("forgejo", "http://127.0.0.1:30000", "soda.lan"), "http://soda.lan:30000");
  assert.equal(providerURL("github", "https://github.com/example/repo", "soda.lan"), "https://github.com/example/repo");
});
