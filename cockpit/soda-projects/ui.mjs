export const formActions = Object.freeze([
  "add-existing",
  "create-forgejo",
  "edit",
  "setup",
  "remove",
  "delete-human",
  "add-person",
]);

const formActionSet = new Set(formActions);

export function payloadFor(action, data, reportInvalid) {
  if (!formActionSet.has(action)) {
    throw new TypeError(`unsupported form action: ${action}`);
  }
  if (action === "add-existing" || action === "edit") {
    return {
      id: data.get("id"),
      display_name: data.get("display_name"),
      canonical_url: data.get("canonical_url"),
    };
  }
  if (action === "create-forgejo") {
    return {
      id: data.get("id"),
      display_name: data.get("display_name"),
      password: data.get("password"),
    };
  }
  if (action === "setup") {
    return {
      id: data.get("id"),
      git_username: data.get("git_username"),
      git_password: data.get("git_password"),
    };
  }
  if (action === "add-person") {
    if (data.get("password_confirmation") !== data.get("password")) {
      reportInvalid("The password confirmation does not match.");
      return null;
    }
    return {
      username: data.get("username"),
      password: data.get("password"),
      authorized_key: data.get("authorized_key"),
    };
  }
  if (action === "remove") {
    const id = data.get("id");
    if (data.get("confirmation") !== id) {
      reportInvalid(`Type ${id} exactly to confirm project removal.`);
      return null;
    }
    return { id };
  }
  const username = data.get("username");
  if (data.get("confirmation") !== username) {
    reportInvalid("The confirmation username does not match.");
    return null;
  }
  return { username };
}

export function successMessage(action, payload, result) {
  if (action === "add-existing") {
    return `${result.project.display_name} was added to the catalog.`;
  }
  if (action === "create-forgejo") {
    return `${result.project.display_name} was created in Forgejo and added to the catalog.`;
  }
  if (action === "edit") {
    return `${result.project.display_name} was updated. Existing workspaces were not changed.`;
  }
  if (action === "setup") {
    return `Workspace ${result.workspace_username} is ready for ${payload.id}.`;
  }
  if (action === "remove") {
    return `${payload.id} and its local workspaces were removed. The canonical repository was not deleted.`;
  }
  if (action === "add-person") {
    return `${payload.username} was added as an ordinary Soda OS user with a private Forgejo login.`;
  }
  return `${payload.username} and their local Soda workspaces were removed. Forgejo was not changed.`;
}

export function clearSecrets(form) {
  for (const name of ["password", "password_confirmation", "git_password"]) {
    const input = form.elements.namedItem(name);
    if (input) {
      input.value = "";
    }
  }
}

export function clearPayloadSecrets(payload) {
  for (const name of ["password", "git_password"]) {
    if (Object.hasOwn(payload, name)) {
      payload[name] = "";
    }
  }
}

export function humanDeletionHidden(currentUser) {
  return currentUser.administrator !== true;
}

export function errorMessage(error) {
  if (typeof error?.message === "string" && error.message.trim() !== "") {
    return error.message;
  }
  return "The operation failed without a diagnostic message.";
}
