import type { Action, Response, Runner } from "./types";
export const coordinatorPath = "/usr/libexec/soda/soda-runners";

export const actions = Object.freeze(["list", "create", "start", "stop", "restart", "remove"]);
const actionSet = new Set(actions);

export function coordinatorCommand(action: string) {
  assertAction(action);
  return [coordinatorPath, action];
}

export function encodeRequest(action: string, payload: unknown) {
  assertAction(action);
  if (payload === null || Array.isArray(payload) || typeof payload !== "object") {
    throw new TypeError("runner request must be a JSON object");
  }
  return `${JSON.stringify(payload)}\n`;
}

export function decodeResponse<A extends Action>(action: A, output: string): Response<A>;
export function decodeResponse(action: Action, output: string): unknown {
  assertAction(action);
  if (typeof output !== "string" || output.trim() === "") {
    throw new TypeError("runner coordinator returned an empty response");
  }
  const response: unknown = JSON.parse(output);
  assertObject(response, "runner coordinator response");
  if (action !== "list") {
    if (response.ok !== true) {
      throw new TypeError("runner mutation did not report success");
    }
    return response;
  }
  for (const field of ["runner_count", "active_listeners", "total_capacity"]) {
    if (!Number.isInteger(response[field]) || (response[field] as number) < 0) {
      throw new TypeError(`runner list has invalid ${field}`);
    }
  }
  if (!Array.isArray(response.runners) || response.runner_count !== response.runners.length) {
    throw new TypeError("runner list has inconsistent local capacity data");
  }
  response.runners.forEach(assertRunner);
  return response;
}

function assertRunner(runner: unknown): asserts runner is Runner {
  assertObject(runner, "runner");
  for (const field of [
    "id",
    "provider",
    "registration_url",
    "account",
    "architecture",
    "version",
  ]) {
    if (typeof runner[field] !== "string") {
      throw new TypeError(`runner is missing ${field}`);
    }
  }
  if (!Number.isInteger(runner.capacity) || runner.capacity !== 1) {
    throw new TypeError("runner does not report its one native slot");
  }
  assertObject(runner.service, "runner service");
  for (const field of ["load", "active", "sub", "enabled"]) {
    if (typeof runner.service[field] !== "string") {
      throw new TypeError(`runner service is missing ${field}`);
    }
  }
}

function assertAction(action: string) {
  if (!actionSet.has(action)) {
    throw new TypeError(`unsupported runner action: ${action}`);
  }
}

function assertObject(value: unknown, name: string): asserts value is Record<string, unknown> {
  if (value === null || Array.isArray(value) || typeof value !== "object") {
    throw new TypeError(`${name} must be a JSON object`);
  }
}
