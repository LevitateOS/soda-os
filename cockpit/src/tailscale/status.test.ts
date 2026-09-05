import type { Status } from "./types";
import { test } from "vite-plus/test";
import assert from "node:assert/strict";
import { connectionState, exitNodeChoices, exitNodeApproval, authenticationURL } from "./status";

test("native connection states remain distinct", () => {
  for (const [status, expected] of [
    [{ BackendState: "NeedsLogin" }, "Not signed in"],
    [{ BackendState: "NeedsLogin", HaveNodeKey: true }, "Authentication required"],
    [
      { BackendState: "NeedsLogin", AuthURL: "https://login.tailscale.com/a/test" },
      "Waiting for browser authentication",
    ],
    [{ BackendState: "NeedsMachineAuth" }, "Waiting for Tailnet administrator approval"],
    [{ BackendState: "Running" }, "Connected"],
    [{ BackendState: "Stopped" }, "Disconnected"],
    [{ BackendState: "Starting" }, "Connecting"],
    [{ BackendState: "Running", Self: { Expired: true } }, "Authentication expired"],
  ] as [Status, string][])
    assert.equal(connectionState(status), expected);
});

test("eligible exit nodes and advertisement approval use native facts", () => {
  const approved = { ID: "ok", DNSName: "approved.ts.net.", ExitNodeOption: true, Online: false };
  assert.deepEqual(
    exitNodeChoices({
      Peer: {
        a: approved,
        b: { ExitNodeOption: false },
        c: { ExitNodeOption: true, Expired: true },
      },
    }),
    [approved],
  );
  const prefs = { AdvertiseRoutes: ["0.0.0.0/0", "::/0"] };
  assert.equal(
    exitNodeApproval({ Self: { InNetworkMap: true } }, prefs),
    "Waiting for Tailnet administrator approval",
  );
  assert.equal(
    exitNodeApproval({ Self: { InNetworkMap: true, ExitNodeOption: true } }, prefs),
    "Approved by the Tailnet",
  );
  assert.equal(exitNodeApproval({}, prefs), "Approval status unavailable");
  assert.equal(exitNodeApproval({}, {}), "Not advertised");
});

test("authentication links never execute page script", () => {
  assert.equal(authenticationURL(""), null);
  assert.equal(
    authenticationURL("https://login.tailscale.com/a/test"),
    "https://login.tailscale.com/a/test",
  );
  assert.throws(() => authenticationURL("javascript:alert(1)"), /invalid/);
});
