import { nativeTailscale, adminURL, cliURL } from "./native.mjs";
import { connectionState, deviceName, exitNodeChoices, advertisesExitNode, exitNodeApproval, authenticationURL } from "./status.mjs";

const native = nativeTailscale(window.cockpit);
const element = id => document.getElementById(id);
let snapshot;
let busy = false;
let closed = false;
let reading = false;
let timer;
let refreshedIdentity = "";
const dirty = new Set();

document.querySelectorAll(".admin-link").forEach(link => { link.href = adminURL; });
element("cli-docs").href = cliURL;
for (const id of ["exit-form", "advertise-form"]) {
  element(id).addEventListener("input", () => dirty.add(id));
}
element("sign-in").addEventListener("click", () => mutate(() => native.signIn(snapshot.status, showAuthentication)));
element("exit-form").addEventListener("submit", event => {
  event.preventDefault();
  mutate(() => native.selectExitNode(element("exit-node").value, element("allow-lan").checked), "exit-form");
});
element("advertise-form").addEventListener("submit", event => {
  event.preventDefault();
  mutate(() => native.advertiseExitNode(element("advertise").checked), "advertise-form");
});

function showNotice(error) {
  element("notice").textContent = error.message || String(error);
  element("notice").hidden = false;
}

function showAuthentication(message) {
  const url = authenticationURL(message.AuthURL);
  if (url) {
    element("auth-link").href = url;
    element("auth-link").textContent = url;
  } else {
    element("auth-link").removeAttribute("href");
  }
  element("authentication").hidden = !url;
  if (message.BackendState) element("connection").textContent = connectionState(message);
}

function updateControls() {
  const status = snapshot?.status;
  const connected = status?.BackendState === "Running" && !status.Self?.Expired;
  document.querySelectorAll("fieldset").forEach(fieldset => { fieldset.disabled = busy || !connected; });
  element("sign-in").hidden = connected || status?.BackendState === "NeedsMachineAuth";
  element("sign-in").disabled = busy || !status || ["Starting", "NoState"].includes(status.BackendState);
  element("sign-in").textContent = status?.BackendState === "Stopped" ? "Connect" : status?.HaveNodeKey ? "Sign in again" : "Sign in";
}

function render() {
  const { status, prefs } = snapshot;
  element("connection").textContent = connectionState(status);
  element("device-name").textContent = deviceName(status.Self);
  element("addresses").textContent = (status.TailscaleIPs || status.Self?.TailscaleIPs || []).join(", ") || "Unavailable";
  element("health").textContent = (status.Health || []).join("\n");
  element("health").hidden = !status.Health?.length;
  showAuthentication({ AuthURL: status.BackendState === "NeedsLogin" ? status.AuthURL : "" });
  element("machine-approval").hidden = status.BackendState !== "NeedsMachineAuth";
  renderPeers(status);
  if (!dirty.has("exit-form")) renderExitSelection(status, prefs);
  if (!dirty.has("advertise-form")) element("advertise").checked = advertisesExitNode(prefs);
  element("exit-approval").textContent = exitNodeApproval(status, prefs);
  element("exit-admin").hidden = !advertisesExitNode(prefs) || !status.Self?.InNetworkMap || status.Self.ExitNodeOption;
  updateControls();
}

function renderPeers(status) {
  const peers = Object.values(status.Peer || {}).sort((a, b) => deviceName(a).localeCompare(deviceName(b)));
  element("peers").replaceChildren(...peers.map(peer => {
    const row = document.createElement("tr");
    for (const value of [deviceName(peer), (peer.TailscaleIPs || []).join(", "), peer.Expired ? "Expired" : peer.Online ? "Online" : "Offline"]) {
      const cell = document.createElement("td");
      cell.textContent = value;
      row.append(cell);
    }
    return row;
  }));
  element("devices").hidden = peers.length === 0;
  element("no-peers").textContent = "No devices reported.";
  element("no-peers").hidden = peers.length !== 0;
}

function renderExitSelection(status, prefs) {
  const options = [new Option("None", "")];
  let selected = prefs.ExitNodeIP && prefs.ExitNodeIP !== "" ? prefs.ExitNodeIP : "";
  for (const peer of exitNodeChoices(status)) {
    const address = peer.TailscaleIPs?.[0];
    if (!address) continue;
    options.push(new Option(`${deviceName(peer)}${peer.Online ? "" : " (offline)"}`, address));
    if (peer.ID === prefs.ExitNodeID) selected = address;
  }
  // Keep an unavailable native selection visible instead of claiming None.
  if (prefs.ExitNodeID && !options.some(option => option.value === selected && selected)) {
    selected = status.ExitNodeStatus?.TailscaleIPs?.[0] || selected;
    const missing = new Option("Selected exit node unavailable", selected || "unavailable");
    missing.disabled = true;
    options.push(missing);
    selected = missing.value;
  }
  element("exit-node").replaceChildren(...options);
  element("exit-node").value = selected;
  element("allow-lan").checked = Boolean(prefs.ExitNodeAllowLANAccess);
}

async function load() {
  if (closed || reading) return;
  reading = true;
  try {
    snapshot = await native.read();
    if (closed) return;
    render();
    await refreshForgejo();
  } catch (error) {
    if (!closed) {
      snapshot = null;
      element("connection").textContent = "Tailscale state unavailable";
      element("device-name").textContent = "Unavailable";
      element("addresses").textContent = "Unavailable";
      element("devices").hidden = true;
      element("no-peers").hidden = false;
      element("no-peers").textContent = "Device list unavailable.";
      element("authentication").hidden = true;
      element("machine-approval").hidden = true;
      element("exit-admin").hidden = true;
      element("exit-approval").textContent = "Approval status unavailable";
      showNotice(error);
      updateControls();
    }
  } finally { reading = false; }
}

async function refreshForgejo() {
  const { status } = snapshot;
  const connected = status.BackendState === "Running" && !status.Self?.Expired;
  if (!connected) { refreshedIdentity = ""; return; }
  const identity = `${status.Self?.DNSName || ""}/${(status.TailscaleIPs || []).join(",")}`;
  if (identity === refreshedIdentity) return;
  refreshedIdentity = identity;
  try { await native.refreshForgejo(); }
  catch (error) {
    if (!closed) showNotice(new Error(`Tailscale connected, but Forgejo could not refresh its Tailnet address: ${error.message || error}`));
  }
}

async function mutate(operation, form) {
  if (busy || closed) return;
  busy = true;
  element("notice").hidden = true;
  updateControls();
  try {
    await operation();
    if (form) dirty.delete(form);
  } catch (error) { if (!closed) showNotice(error); }
  finally { busy = false; await load(); if (!closed) updateControls(); }
}

async function observe() {
  await load();
  if (!closed) timer = setTimeout(observe, 3000);
}
function close() {
  closed = true;
  clearTimeout(timer);
  native.close();
}
window.addEventListener("pagehide", close);
// Cockpit caches inactive pages instead of unloading their iframes.
window.cockpit.addEventListener("visibilitychange", () => {
  if (window.cockpit.hidden) close();
  else if (closed) window.location.reload();
});
if (window.cockpit.hidden) close();
else observe();
