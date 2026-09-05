import type { AuthenticationMessage } from "./types";
import { test } from "vite-plus/test";
import assert from "node:assert/strict";
import { authenticationStream } from "./stream";

test("auth response is delivered before the final Running object, across every chunk boundary", () => {
  const auth = {
    AuthURL: 'https://login.tailscale.com/a/test?value={a}\\"',
    BackendState: "NeedsLogin",
  };
  const first = JSON.stringify(auth, null, "\t");
  for (let split = 0; split <= first.length; split++) {
    const messages: AuthenticationMessage[] = [];
    const stream = authenticationStream((message) => messages.push(message));
    stream.push(first.slice(0, split));
    stream.push(first.slice(split));
    assert.deepEqual(messages, [auth]);
    stream.push('\n{\n "BackendState": "NeedsMachineAuth"\n}\n{ "BackendState": "Running" }\n');
    stream.finish();
    assert.equal(messages.length, 3);
    assert.equal(messages[2].BackendState, "Running");
  }
});

test("character-sized chunks, multiple objects and malformed termination", () => {
  const messages: AuthenticationMessage[] = [];
  const stream = authenticationStream((message) => messages.push(message));
  for (const character of ' {"AuthURL":"https://example.com"}\n {"BackendState":"Running"} ')
    stream.push(character);
  stream.finish();
  assert.equal(messages.length, 2);
  const truncated = authenticationStream(() => {});
  truncated.push('{"AuthURL":');
  assert.throws(() => truncated.finish(), /Incomplete/);
  assert.throws(() => authenticationStream(() => {}).push("not json"), /Invalid/);
});
