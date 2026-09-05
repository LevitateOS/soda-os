import assert from "node:assert/strict";
import { test } from "vite-plus/test";

import { createPayload, forgejoBrowserURL, providerURL, statusText, successMessage } from "./ui";

function formData(values: Record<string, string>) {
  return { get: (name: string) => values[name] ?? "" };
}

test("provider forms retain only the provider-native registration fields", () => {
  const payload = createPayload(
    formData({
      id: "forgejo-one",
      provider: "forgejo",
      registration_url: "https://external.invalid",
      registration_id: "33834eef-e758-48c4-a676-1745426747aa",
      forgejo_labels: "soda:host",
      registration_token: "provider-input",
    }),
  );
  assert.deepEqual(payload, {
    id: "forgejo-one",
    provider: "forgejo",
    registration_url: "",
    registration_id: "33834eef-e758-48c4-a676-1745426747aa",
    labels: "soda:host",
    registration_token: "provider-input",
  });
  assert.equal(Object.hasOwn(payload, "project_id"), false);
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
  assert.equal(
    providerURL("forgejo", "http://127.0.0.1:30000", "soda.lan"),
    "http://soda.lan:30000",
  );
  assert.equal(
    providerURL("github", "https://github.com/example/repo", "soda.lan"),
    "https://github.com/example/repo",
  );
});
