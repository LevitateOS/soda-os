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

export function setProviderRequirements(forgejoInputs, githubInputs, provider) {
  for (const input of forgejoInputs) input.required = provider === "forgejo";
  for (const input of githubInputs) input.required = provider === "github";
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

export function forgejoBrowserURL(hostname) {
  const host = hostname.includes(":") && !hostname.startsWith("[") ? `[${hostname}]` : hostname;
  return `http://${host}:30000`;
}

export function providerURL(provider, registrationURL, hostname) {
  return provider === "forgejo" ? forgejoBrowserURL(hostname) : registrationURL;
}

export function successMessage(action, id) {
  if (action === "create") return `${id} was registered and its local listener started.`;
  if (action === "remove") return `${id} and its local account and state were removed. Remove its offline record in the provider.`;
  const pastTense = { start: "started", stop: "stopped", restart: "restarted" };
  return `${id} was ${pastTense[action]}.`;
}

export function errorMessage(error) {
  return typeof error?.message === "string" && error.message.trim() ? error.message : "The operation failed without a diagnostic message.";
}
