const actions = new Set([
  "allow-local-network",
  "connect-tailscale",
  "dismiss",
]);

const inputActions = new Set(["allow-local-network", "connect-tailscale"]);

export function setupCommand(action = "status") {
  if (action !== "status" && !actions.has(action)) {
    throw new TypeError(`unsupported Soda Setup action: ${action}`);
  }
  return ["/usr/libexec/soda/soda-setup", action];
}

export function encodeSetupRequest(action, payload) {
  if (!inputActions.has(action)) {
    throw new TypeError(`unsupported Soda Setup mutation: ${action}`);
  }
  return `${JSON.stringify(payload)}\n`;
}

export function decodeSetupResponse(action, output) {
  const value = JSON.parse(output);
  if (action === "status") {
    return value;
  }
  if (typeof value?.error === "string" && value.error !== "") {
    throw new Error(value.error);
  }
  return value.status;
}

export function clearSetupSecrets(form, payload = {}) {
  for (const name of ["auth_key"]) {
    const field = form?.elements?.namedItem(name);
    if (field) {
      field.value = "";
    }
    if (Object.hasOwn(payload, name)) {
      payload[name] = "";
    }
  }
}
