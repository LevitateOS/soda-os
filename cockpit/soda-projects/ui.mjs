export const formActions = Object.freeze([
  "add-existing",
  "create-forgejo",
  "edit",
  "setup",
  "install-tools",
  "remove-workspace",
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
    return catalogPayload(data, reportInvalid);
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
	  forgejo_password: data.get("forgejo_password"),
	  workspace_tools: toolSelections(data.get("workspace_tools")),
	  project_tools: toolSelections(data.get("project_tools")),
	};
  }
  if (action === "install-tools") {
	return {
	  id: data.get("id"),
	  scope: data.get("scope"),
	  tools: toolSelections(data.get("tools")),
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
  if (action === "remove" || action === "remove-workspace") {
    const id = data.get("id");
    if (data.get("confirmation") !== id) {
      const target = action === "remove" ? "project" : "workspace";
      reportInvalid(`Type ${id} exactly to confirm ${target} removal.`);
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

function catalogPayload(data, reportInvalid) {
  const text = String(data.get("additional_metadata") ?? "").trim();
  let metadata = {};
  try {
    metadata = text === "" ? {} : JSON.parse(text);
  } catch {
    reportInvalid("Additional metadata must be a valid JSON object.");
    return null;
  }
  if (metadata === null || Array.isArray(metadata) || typeof metadata !== "object") {
    reportInvalid("Additional metadata must be a valid JSON object.");
    return null;
  }
  for (const field of ["id", "display_name", "canonical_url"]) {
    if (Object.hasOwn(metadata, field)) {
      reportInvalid(`Additional metadata must not redefine ${field}.`);
      return null;
    }
  }
  return {
    ...metadata,
    id: data.get("id"),
    display_name: data.get("display_name"),
    canonical_url: data.get("canonical_url"),
  };
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
  if (action === "install-tools") {
	return `mise installed and selected tools for ${payload.scope === "project" ? "the project" : "your workspace"}.`;
  }
  if (action === "remove") {
    return `${payload.id} and its local workspaces were removed. The canonical repository was not deleted.`;
  }
  if (action === "remove-workspace") {
    return `Your ${payload.id} workspace was removed. The shared project and canonical repository were not deleted.`;
  }
  if (action === "add-person") {
    return `${payload.username} was added with a matching Forgejo account and public SSH key.`;
  }
  return `${payload.username}, their local Soda workspaces, and their Forgejo account were removed.`;
}

export function clearSecrets(form) {
  for (const name of ["password", "password_confirmation", "forgejo_password"]) {
    const input = form.elements.namedItem(name);
    if (input) {
      input.value = "";
    }
  }
}

export function clearPayloadSecrets(payload) {
  for (const name of ["password", "forgejo_password"]) {
    if (Object.hasOwn(payload, name)) {
      payload[name] = "";
    }
  }
}

export function humanDeletionHidden(currentUser) {
  return currentUser.administrator !== true;
}

export function projectRemovalHidden(currentUser) {
  return currentUser.administrator !== true;
}

function toolSelections(value) {
  return String(value ?? "")
	.split(/\r?\n/)
	.map(tool => tool.trim())
	.filter(tool => tool !== "");
}

export function errorMessage(error) {
  if (typeof error?.message === "string" && error.message.trim() !== "") {
    return error.message;
  }
  return "The operation failed without a diagnostic message.";
}
