export function connectionState(status) {
  if (status.Self?.Expired) return "Authentication expired";
  switch (status.BackendState) {
    case "NeedsLogin": return status.AuthURL ? "Waiting for browser authentication" : status.HaveNodeKey ? "Authentication required" : "Not signed in";
    case "NeedsMachineAuth": return "Waiting for Tailnet administrator approval";
    case "Running": return "Connected";
    case "Stopped": return "Disconnected";
    case "Starting": return "Connecting";
    case "NoState": return "Tailscale is starting";
    default: return status.BackendState;
  }
}

export function deviceName(device) {
  return device?.DNSName?.replace(/\.$/, "") || device?.HostName || "Unavailable";
}

export function exitNodeChoices(status) {
  return Object.values(status.Peer || {})
    .filter(peer => peer.ExitNodeOption && !peer.Expired)
    .sort((a, b) => deviceName(a).localeCompare(deviceName(b)));
}

export function advertisesExitNode(prefs) {
  return ["0.0.0.0/0", "::/0"].every(route => prefs.AdvertiseRoutes?.includes(route));
}

export function exitNodeApproval(status, prefs) {
  if (!advertisesExitNode(prefs)) return "Not advertised";
  if (!status.Self?.InNetworkMap) return "Approval status unavailable";
  return status.Self.ExitNodeOption ? "Approved by the Tailnet" : "Waiting for Tailnet administrator approval";
}

export function authenticationURL(value) {
  if (!value) return null;
  const url = new URL(value);
  if (url.protocol !== "https:") throw new Error("Tailscale returned an invalid authentication URL.");
  return url.href;
}
