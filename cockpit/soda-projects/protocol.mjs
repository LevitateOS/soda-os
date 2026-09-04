export const coordinatorPath = "/usr/libexec/soda/soda-projects";

export const actions = Object.freeze([
  "list",
  "add-existing",
  "create-forgejo",
  "edit",
  "setup",
  "remove-workspace",
  "remove",
  "delete-human",
  "add-person",
]);

const actionSet = new Set(actions);

export function coordinatorCommand(action) {
  assertAction(action);
  return [coordinatorPath, action];
}

export function encodeRequest(action, payload) {
  assertAction(action);
  if (payload === null || Array.isArray(payload) || typeof payload !== "object") {
    throw new TypeError("coordinator request must be a JSON object");
  }
  return `${JSON.stringify(payload)}\n`;
}

export function decodeResponse(action, output) {
  assertAction(action);
  if (typeof output !== "string" || output.trim() === "") {
    throw new TypeError("coordinator returned an empty response");
  }

  const response = JSON.parse(output);
  assertObject(response, "coordinator response");

  if (action === "list") {
    assertListResponse(response);
    return response;
  }

  if (response.ok !== true) {
    throw new TypeError("coordinator mutation did not report success");
  }
  if (["add-existing", "create-forgejo", "edit"].includes(action)) {
    assertCatalogEntry(response.project);
  }
  if (action === "setup" && typeof response.workspace_username !== "string") {
    throw new TypeError("setup response is missing workspace_username");
  }
  return response;
}

function assertAction(action) {
  if (!actionSet.has(action)) {
    throw new TypeError(`unsupported coordinator action: ${action}`);
  }
}

function assertListResponse(response) {
  if (!Array.isArray(response.projects)) {
    throw new TypeError("list response is missing projects");
  }
  response.projects.forEach(assertProjectView);

  assertObject(response.current_user, "current_user");
  if (typeof response.current_user.username !== "string" ||
      typeof response.current_user.administrator !== "boolean") {
    throw new TypeError("list response has an invalid current_user");
  }
  if (typeof response.forgejo_url !== "string" || typeof response.ssh_host !== "string") {
    throw new TypeError("list response is missing native service locations");
  }
}

function assertProjectView(project) {
  assertCatalogEntry(project);
  if (typeof project.workspace_username !== "string") {
    throw new TypeError("project is missing workspace_username");
  }
  if (typeof project.workspace_ready !== "boolean") {
    throw new TypeError("project is missing workspace readiness");
  }
}

function assertCatalogEntry(project) {
  assertObject(project, "project");
  for (const field of ["id", "display_name", "canonical_url"]) {
    if (typeof project[field] !== "string") {
      throw new TypeError(`project is missing ${field}`);
    }
  }
}

function assertObject(value, name) {
  if (value === null || Array.isArray(value) || typeof value !== "object") {
    throw new TypeError(`${name} must be a JSON object`);
  }
}
