import { test } from "node:test";
import assert from "node:assert/strict";

class Element {
  constructor() { this.children = []; this.listeners = {}; this.hidden = false; this.disabled = false; this.value = ""; }
  addEventListener(name, callback) { this.listeners[name] = callback; }
  replaceChildren(...children) { this.children = children; }
  append(child) { this.children.push(child); }
  removeAttribute(name) { delete this[name]; }
}

async function page(status, prefs = {}) {
  const elements = new Map();
  const get = id => { if (!elements.has(id)) elements.set(id, new Element()); return elements.get(id); };
  const calls = [], events = {}, timers = [], cockpitEvents = {}, authentications = [];
  let reloads = 0;
  let httpClosed = false;
  let refreshError;
  const globals = { window: globalThis.window, document: globalThis.document, Option: globalThis.Option, setTimeout: globalThis.setTimeout, clearTimeout: globalThis.clearTimeout };
  const current = { status, prefs };
  globalThis.document = {
    getElementById: get,
    querySelectorAll: selector => selector === "fieldset" ? [get("exit-fieldset"), get("advertise-fieldset")] : [get("admin-link")],
    createElement: () => new Element(),
  };
  globalThis.Option = class extends Element { constructor(label, value) { super(); this.textContent = label; this.value = value; } };
  globalThis.setTimeout = callback => { timers.push(callback); return timers.length; };
  globalThis.clearTimeout = () => {};
  globalThis.window = {
    location: { reload: () => { reloads++; } },
    addEventListener: (event, callback) => { events[event] = callback; },
    cockpit: {
      hidden: false,
      addEventListener: (event, callback) => { cockpitEvents[event] = callback; },
      http: () => ({ request: async () => JSON.stringify(current.prefs), close: () => { httpClosed = true; } }),
      spawn(args) {
        calls.push(args);
        if (args[1] === "up") {
          let reject;
          const process = new Promise((resolve, fail) => { reject = fail; });
          process.stream = callback => { process.emit = callback; };
          process.close = problem => { process.closed = problem; reject({ problem }); };
          authentications.push(process);
          return process;
        }
        const process = args[1] === "status" ? Promise.resolve(JSON.stringify(current.status)) : refreshError ? Promise.reject(refreshError) : Promise.resolve("");
        process.always = callback => { process.then(callback, callback); };
        process.close = () => {};
        return process;
      },
    },
  };
  await import(`./app.mjs?test=${Math.random()}`);
  const settle = () => new Promise(setImmediate);
  await settle();
  return {
    get, calls, current, authentications,
    get reloads() { return reloads; },
    get httpClosed() { return httpClosed; },
    async visibility(hidden) {
      window.cockpit.hidden = hidden;
      cockpitEvents.visibilitychange?.();
      await settle();
    },
    set refreshError(error) { refreshError = error; },
    async poll() { const callback = timers.shift(); assert.ok(callback); await callback(); await settle(); },
    async close() { events.pagehide(); await settle(); Object.assign(globalThis, globals); },
  };
}

const connected = { BackendState: "Running", Self: { DNSName: "atlas.ts.net.", InNetworkMap: true }, TailscaleIPs: ["100.64.0.1"] };

test("opening connected page refreshes Forgejo once and preserves in-progress native-setting edits", async () => {
  const app = await page(connected, { ExitNodeAllowLANAccess: true });
  try {
    assert.equal(app.get("connection").textContent, "Connected");
    assert.equal(app.calls.filter(args => args[1] === "refresh-tailnet").length, 1);
    app.get("allow-lan").checked = false;
    app.get("exit-form").listeners.input();
    await app.poll();
    assert.equal(app.get("allow-lan").checked, false);
    assert.equal(app.calls.filter(args => args[1] === "refresh-tailnet").length, 1);
  } finally { await app.close(); }
});

test("Cockpit navigation stops hidden-page work and reopening reloads native state", async () => {
  const app = await page({ BackendState: "NeedsLogin" });
  try {
    const before = app.calls.length;
    await app.visibility(true);
    assert.equal(app.httpClosed, true);
    app.current.status = connected;
    await app.poll();
    assert.equal(app.calls.length, before, "a cached hidden iframe must not poll or refresh Forgejo");
    await app.visibility(false);
    assert.equal(app.reloads, 1, "reopening must recover native state and page-open refresh");
  } finally { await app.close(); }
});

test("leaving Cockpit Tailscale cancels streaming sign-in without logging out", async () => {
  const app = await page({ BackendState: "NeedsLogin" });
  try {
    const signIn = app.get("sign-in").listeners.click();
    const process = app.authentications[0];
    process.emit('{"AuthURL":"https://login.tailscale.com/a/pending"}');
    assert.equal(app.get("authentication").hidden, false);
    await app.visibility(true);
    await signIn;
    assert.equal(process.closed, "cancelled");
    assert.equal(app.calls.some(args => ["logout", "down"].includes(args[1])), false);
  } finally { await app.close(); }
});

test("reopening pending authentication recovers URL and later connection without completion state", async () => {
  const app = await page({ BackendState: "NeedsLogin", AuthURL: "https://login.tailscale.com/a/pending" });
  try {
    assert.equal(app.get("auth-link").href, "https://login.tailscale.com/a/pending");
    assert.equal(app.get("authentication").hidden, false);
    assert.equal(app.calls.filter(args => args[1] === "up").length, 0);
    app.current.status = { BackendState: "NeedsMachineAuth" };
    await app.poll();
    assert.equal(app.get("machine-approval").hidden, false);
    app.current.status = connected;
    await app.poll();
    assert.equal(app.get("authentication").hidden, true);
    assert.equal(app.calls.filter(args => args[1] === "refresh-tailnet").length, 1);
  } finally { await app.close(); }
});

test("Forgejo refresh failure does not undo or relabel enrollment and is not retried by polling", async () => {
  const app = await page({ BackendState: "NeedsLogin" });
  try {
    app.refreshError = new Error("access-denied");
    app.current.status = connected;
    await app.poll();
    assert.equal(app.get("connection").textContent, "Connected");
    assert.match(app.get("notice").textContent, /Tailscale connected, but Forgejo/);
    await app.poll();
    assert.equal(app.calls.filter(args => args[1] === "refresh-tailnet").length, 1);
  } finally { await app.close(); }
});
