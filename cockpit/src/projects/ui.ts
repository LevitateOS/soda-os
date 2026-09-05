import type {
  FormAction,
  Requests,
  Responses,
  CurrentUser,
  WorkspaceInspection,
  RemovalAccount,
  RemovalResponse,
  RemovalTask,
} from "./types";
type FormValues = { get(name: string): FormDataEntryValue | null | undefined };
export const formActions = Object.freeze(["add-existing", "edit"]);

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
  return catalogPayload(action, data, reportInvalid);
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

export function successMessage(action: FormAction, result: Responses[FormAction]) {
  if (action === "add-existing") {
    return `${result.project.display_name} was added to the catalog.`;
  }
  return `${result.project.display_name} was updated. Existing workspaces were not changed.`;
}

export function removalCatalogMessage(state: RemovalResponse["catalog"]) {
  if (state === "removed") return "The shared project entry was removed.";
  if (state === "uncertain") return "The shared project entry’s removal is not confirmed.";
  return "The shared project entry was not removed by this operation.";
}

export function removalAccountLabel(username: string, accounts: RemovalAccount[]) {
  const account = accounts.find((item) => item.username === username);
  if (!account) return username;
  return account.project_id
    ? `${account.primary_username} / ${account.project_id}`
    : `${account.username} (primary account)`;
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

export function removalReview(task: RemovalTask) {
  const { preview, receipt, selectedAccounts, reviewing, unknown, confirmation, action } = task;
  const failed = selectedAccounts.find((account) => account.username === receipt?.result.uncertain);
  const failedIdentityChanged = Boolean(
    failed &&
    preview &&
    !preview.accounts.some(
      (account) =>
        account.username === failed.username &&
        account.uid === failed.uid &&
        account.home === failed.home,
    ),
  );
  const hasTargets = Boolean(
    preview && (preview.accounts.length || (action === "remove" && preview.catalog_present)),
  );
  const showSelection = reviewing || (!receipt && !unknown);
  const orphaned = Boolean(
    action === "remove" && preview && !preview.catalog_present && preview.accounts.length,
  );
  const canConfirm = Boolean(
    preview && hasTargets && !failedIdentityChanged && !orphaned && !receipt?.ok && showSelection,
  );
  return {
    failedIdentityChanged,
    hasTargets,
    showSelection,
    orphaned,
    canConfirm,
    matches: confirmation === preview?.target,
  };
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
