import type { Action, Responses, CatalogEntry, Project, ListResponse } from "./types";
export const coordinatorPath = "/usr/libexec/soda/soda-projects";

export const actions = Object.freeze([
  "list",
  "add-existing",
  "edit",
  "inspect",
  "removal-inspect",
  "setup",
  "remove-workspace",
  "remove",
  "delete-human",
]);

const actionSet = new Set(actions);

export function coordinatorCommand(action: string) {
  assertAction(action);
  return [coordinatorPath, action];
}

export function encodeRequest(action: string, payload: unknown) {
  assertAction(action);
  if (payload === null || Array.isArray(payload) || typeof payload !== "object") {
    throw new TypeError("coordinator request must be a JSON object");
  }
  return `${JSON.stringify(payload)}\n`;
}

export function decodeResponse<A extends Action>(action: A, output: string): Responses[A];
export function decodeResponse(action: Action, output: string): unknown {
  assertAction(action);
  if (typeof output !== "string" || output.trim() === "") {
    throw new TypeError("coordinator returned an empty response");
  }

  const response: unknown = JSON.parse(output);
  assertObject(response, "coordinator response");

  if (action === "list") {
    assertListResponse(response);
    return response;
  }

  if (["remove", "remove-workspace", "delete-human"].includes(action)) {
    assertRemovalResponse(response);
    return response;
  }
  if (action === "removal-inspect") assertRemovalPreview(response.preview);
  if (response.ok !== true) {
    throw new TypeError("coordinator mutation did not report success");
  }
  if (["add-existing", "edit"].includes(action)) {
    assertCatalogEntry(response.project);
  }
  if (action === "inspect") assertWorkspaceInspection(response.workspace);
  if (action === "setup" && typeof response.workspace_username !== "string") {
    throw new TypeError("setup response is missing workspace_username");
  }
  return response;
}

function assertRemovalPreview(value: unknown) {
  assertObject(value, "removal preview");
  if (
    !["remove", "remove-workspace", "delete-human"].includes(String(value.action)) ||
    typeof value.target !== "string" ||
    typeof value.revision !== "string" ||
    !/^[a-f0-9]{64}$/.test(value.revision) ||
    typeof value.catalog_present !== "boolean" ||
    !Array.isArray(value.accounts)
  )
    throw new TypeError("invalid removal preview");
  for (const account of value.accounts) {
    assertObject(account, "removal account");
    if (typeof account.uid !== "number" || !Number.isSafeInteger(account.uid) || account.uid < 0)
      throw new TypeError("invalid removal account UID");
    for (const field of ["username", "primary_username", "project_id", "home"])
      if (typeof account[field] !== "string") throw new TypeError("invalid removal account");
  }
}

function assertRemovalResponse(value: Record<string, unknown>) {
  if (
    typeof value.ok !== "boolean" ||
    typeof value.problem !== "string" ||
    !["unchanged", "not_attempted", "removed", "uncertain"].includes(String(value.catalog))
  )
    throw new TypeError("invalid removal receipt");
  assertObject(value.result, "removal result");
  const result = value.result;
  for (const field of ["removed", "not_attempted"])
    if (!Array.isArray(result[field]) || !result[field].every((item) => typeof item === "string"))
      throw new TypeError("incomplete removal receipt");
  if (typeof result.uncertain !== "string" || typeof result.diagnostic !== "string")
    throw new TypeError("incomplete removal receipt");
  const problems = value.problem || result.uncertain || result.diagnostic;
  if (
    value.ok &&
    (problems || (result.not_attempted as string[]).length || value.catalog === "uncertain")
  )
    throw new TypeError("removal success contains unresolved outcomes");
  if (!value.ok && !problems) throw new TypeError("failed removal has no diagnostic");
}

function assertWorkspaceInspection(value: unknown) {
  assertObject(value, "workspace inspection");
  for (const field of [
    "username",
    "checkout_path",
    "public_key",
    "primary_key_problem",
    "workspace_key_problem",
    "checkout_problem",
    "git_key_problem",
  ]) {
    if (typeof value[field] !== "string")
      throw new TypeError(`workspace inspection is missing ${field}`);
  }
  if (typeof value.exists !== "boolean" || typeof value.checkout_ready !== "boolean") {
    throw new TypeError("workspace inspection is missing account or checkout facts");
  }
  if (
    !value.username ||
    (value.checkout_ready && (!value.exists || !value.checkout_path || value.checkout_problem))
  ) {
    throw new TypeError("workspace inspection contains inconsistent checkout facts");
  }
}

function assertAction(action: string) {
  if (!actionSet.has(action)) {
    throw new TypeError(`unsupported coordinator action: ${action}`);
  }
}

function assertListResponse(
  response: Record<string, unknown>,
): asserts response is Record<string, unknown> & ListResponse {
  if (!Array.isArray(response.projects)) {
    throw new TypeError("list response is missing projects");
  }
  response.projects.forEach(assertProjectView);

  assertObject(response.current_user, "current_user");
  if (
    typeof response.current_user.username !== "string" ||
    typeof response.current_user.administrator !== "boolean"
  ) {
    throw new TypeError("list response has an invalid current_user");
  }
}

function assertProjectView(project: unknown): asserts project is Project {
  assertCatalogEntry(project);
  if (typeof project.workspace_username !== "string") {
    throw new TypeError("project is missing workspace_username");
  }
  if (typeof project.workspace_exists !== "boolean") {
    throw new TypeError("project is missing workspace existence");
  }
}

function assertCatalogEntry(
  project: unknown,
): asserts project is CatalogEntry & Record<string, unknown> {
  assertObject(project, "project");
  for (const field of ["id", "display_name", "canonical_url"]) {
    if (typeof project[field] !== "string") {
      throw new TypeError(`project is missing ${field}`);
    }
  }
  assertObject(project.catalog_metadata, "project catalog_metadata");
}

function assertObject(value: unknown, name: string): asserts value is Record<string, unknown> {
  if (value === null || Array.isArray(value) || typeof value !== "object") {
    throw new TypeError(`${name} must be a JSON object`);
  }
}
