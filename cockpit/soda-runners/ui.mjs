export function createPayload(data) {
  const provider = data.get("provider");
  return {
    id: data.get("id"),
    provider,
    registration_url: provider === "github" ? data.get("registration_url") : "",
    registration_id: provider === "forgejo" ? data.get("registration_id") : "",
    labels: data.get(provider === "forgejo" ? "forgejo_labels" : "github_labels"),
    registration_token: data.get("registration_token"),
  };
}

export function clearRegistrationSecret(form, payload) {
  const input = form.elements.namedItem("registration_token");
  if (input) {
    input.value = "";
  }
  if (payload && Object.hasOwn(payload, "registration_token")) {
    payload.registration_token = "";
  }
}

export function statusText(service) {
  if (service.active === "active" && service.sub === "running") {
    return "Listening";
  }
  if (service.active === "failed") {
    return "Failed";
  }
  if (service.active === "activating" || service.active === "deactivating") {
    return service.active === "activating" ? "Starting" : "Stopping";
  }
  return "Stopped";
}

export function statusClass(service) {
  if (service.active === "active" && service.sub === "running") {
    return "good";
  }
  return service.active === "failed" ? "bad" : "neutral";
}

export function providerName(provider) {
  return provider === "forgejo" ? "Forgejo" : "GitHub";
}

export function successMessage(action, id) {
  if (action === "create") return `${id} was registered and its local listener started.`;
  if (action === "remove") return `${id} and its local account and state were removed. Remove its offline record in the provider.`;
  return `${id} was ${action === "restart" ? "restarted" : `${action}ed`}.`;
}

export function errorMessage(error) {
  return typeof error?.message === "string" && error.message.trim() ? error.message : "The operation failed without a diagnostic message.";
}
