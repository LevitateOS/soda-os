import type { Action, Responses, CatalogEntry, Project, ListResponse } from "./types";
export const coordinatorPath = "/usr/libexec/soda/soda-projects";

export const actions = Object.freeze([
  "list",
  "add-existing",
  "edit",
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

  if (response.ok !== true) {
    throw new TypeError("coordinator mutation did not report success");
  }
  if (["add-existing", "edit"].includes(action)) {
    assertCatalogEntry(response.project);
  }
  if (action === "setup" && typeof response.workspace_username !== "string") {
    throw new TypeError("setup response is missing workspace_username");
  }
  return response;
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
