import type { Service, Registration, LifecycleAction } from "./types";
export function createPayload(data: {
  get(name: string): FormDataEntryValue | null | undefined;
}): Registration {
  const provider = String((data.get("provider") as string | null) ?? "");
  return {
    id: String((data.get("id") as string | null) ?? ""),
    provider,
    registration_url:
      provider === "github" ? String((data.get("registration_url") as string | null) ?? "") : "",
    registration_id:
      provider === "forgejo" ? String((data.get("registration_id") as string | null) ?? "") : "",
    labels: String(
      (data.get(provider === "forgejo" ? "forgejo_labels" : "github_labels") as string | null) ??
        "",
    ),
    registration_token: String((data.get("registration_token") as string | null) ?? ""),
  };
}

export function statusText(service: Pick<Service, "active" | "sub">) {
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

export function statusClass(service: Pick<Service, "active" | "sub">) {
  if (service.active === "active" && service.sub === "running") {
    return "good";
  }
  return service.active === "failed" ? "bad" : "neutral";
}

export function providerName(provider: string) {
  return provider === "forgejo" ? "Forgejo" : "GitHub";
}

export function forgejoBrowserURL(hostname: string) {
  const host = hostname.includes(":") && !hostname.startsWith("[") ? `[${hostname}]` : hostname;
  return `http://${host}:30000`;
}

export function providerURL(provider: string, registrationURL: string, hostname: string) {
  return provider === "forgejo" ? forgejoBrowserURL(hostname) : registrationURL;
}

export function successMessage(action: LifecycleAction | "create", id: string) {
  if (action === "create") return `${id} was registered and its local listener started.`;
  if (action === "remove")
    return `${id} and its local account and state were removed. Remove its offline record in the provider.`;
  const pastTense = { start: "started", stop: "stopped", restart: "restarted" };
  return `${id} was ${pastTense[action]}.`;
}

export function errorMessage(error: unknown) {
  return error !== null &&
    typeof error === "object" &&
    "message" in error &&
    typeof error.message === "string" &&
    error.message.trim()
    ? error.message
    : "The operation failed without a diagnostic message.";
}
