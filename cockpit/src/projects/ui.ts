import type { FormAction, Requests, Responses, CurrentUser, WorkspaceInspection } from "./types";
type FormValues = { get(name: string): FormDataEntryValue | null | undefined };
export const formActions = Object.freeze([
  "add-existing",
  "edit",
  "remove-workspace",
  "remove",
  "delete-human",
]);

const formActionSet = new Set(formActions);

export function sshCommand(username: string, hostname: string) {
  const host = hostname.includes(":") && !hostname.startsWith("[") ? `[${hostname}]` : hostname;
  return `ssh ${username}@${host}`;
}

export function payloadFor<A extends FormAction>(
  action: A,
  data: FormValues,
  reportInvalid: (message: string) => void,
): Requests[A] | null;
export function payloadFor(
  action: FormAction,
  data: FormValues,
  reportInvalid: (message: string) => void,
): unknown {
  if (!formActionSet.has(action)) {
    throw new TypeError(`unsupported form action: ${action}`);
  }
  if (action === "add-existing" || action === "edit") {
    return catalogPayload(action, data, reportInvalid);
  }
  if (action === "remove" || action === "remove-workspace") {
    const id = data.get("id") as string | null;
    if ((data.get("confirmation") as string | null) !== id) {
      const target = action === "remove" ? "project" : "workspace";
      reportInvalid(`Type ${id} exactly to confirm ${target} removal.`);
      return null;
    }
    return { id };
  }
  const username = data.get("username") as string | null;
  if ((data.get("confirmation") as string | null) !== username) {
    reportInvalid("The confirmation username does not match.");
    return null;
  }
  return { username };
}

function catalogPayload(
  action: FormAction,
  data: FormValues,
  reportInvalid: (message: string) => void,
) {
  const text = String((data.get("additional_metadata") as string | null) ?? "").trim();
  let metadata: unknown = {};
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
  const payload: Record<string, unknown> = {
    ...metadata,
    id: data.get("id") as string | null,
    display_name: data.get("display_name") as string | null,
  };
  if (action === "add-existing") {
    payload.canonical_url = data.get("canonical_url") as string | null;
  }
  return payload;
}

export function successMessage(
  action: FormAction,
  payload: Requests[FormAction],
  result: Responses[FormAction],
) {
  if (action === "add-existing") {
    return `${(result as Responses["edit"]).project.display_name} was added to the catalog.`;
  }
  if (action === "edit") {
    return `${(result as Responses["edit"]).project.display_name} was updated. Existing workspaces were not changed.`;
  }
  if (action === "remove") {
    return `${(payload as { id: string }).id} and its local workspaces were removed. The canonical repository was not deleted.`;
  }
  if (action === "remove-workspace") {
    return `Your ${(payload as { id: string }).id} workspace was removed. The shared project and canonical repository were not deleted.`;
  }
  return `${(payload as { username: string }).username} and their local Soda workspaces were removed. Their Forgejo account was unchanged.`;
}

export function workspaceReady(workspace?: WorkspaceInspection | null) {
  return Boolean(workspace?.exists && workspace.checkout_ready && !workspace.workspace_key_problem);
}

export function workspaceCanSetup(workspace?: WorkspaceInspection | null) {
  return Boolean(
    workspace &&
    !workspaceReady(workspace) &&
    !workspace.workspace_key_problem &&
    !workspace.checkout_problem &&
    (workspace.exists || !workspace.primary_key_problem),
  );
}

export function humanDeletionHidden(currentUser: Partial<CurrentUser>) {
  return currentUser.administrator !== true;
}

export function projectRemovalHidden(currentUser: Partial<CurrentUser>) {
  return currentUser.administrator !== true;
}

export function errorMessage(error: unknown) {
  if (
    error !== null &&
    typeof error === "object" &&
    "message" in error &&
    typeof error.message === "string" &&
    error.message.trim() !== ""
  ) {
    return error.message;
  }
  return "The operation failed without a diagnostic message.";
}

export const dialogCopy = {
  "remove-workspace": [
    "Remove my workspace",
    "This permanently removes your workspace account, home, independent clone, dependencies, processes, project state, and uncommitted work. The shared project, other workspaces, and canonical repository are not deleted.",
    "Remove my workspace",
  ],
  remove: [
    "Remove project from Soda",
    "This permanently removes all local workspace accounts, homes, clones, dependencies, and uncommitted work for this project. The canonical repository is not deleted.",
    "Remove project",
  ],
  "delete-human": [
    "Remove person from Soda OS",
    "This permanently removes the person’s local Soda workspaces, then their primary Linux account. Their Forgejo account and repository data are unchanged. Delete a Forgejo account separately in Forgejo.",
    "Remove person",
  ],
} as const;
