import assert from "node:assert/strict";
import test from "node:test";

import { clearRegistrationSecret, createPayload, statusText, successMessage } from "./ui.mjs";

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

test("local status never claims provider availability or idle capacity", () => {
  assert.equal(statusText({ active: "active", sub: "running" }), "Listening");
  assert.equal(statusText({ active: "failed", sub: "failed" }), "Failed");
  assert.match(successMessage("remove", "one"), /provider/);
});
