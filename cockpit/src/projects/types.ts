export interface CatalogEntry {
  id: string;
  display_name: string;
  canonical_url: string;
  catalog_metadata: Record<string, unknown>;
}
export interface Project extends CatalogEntry {
  workspace_username: string;
  workspace_exists: boolean;
}
export interface WorkspaceInspection {
  username: string;
  exists: boolean;
  checkout_path: string;
  checkout_ready: boolean;
  public_key: string;
  primary_key_problem: string;
  workspace_key_problem: string;
  checkout_problem: string;
  git_key_problem: string;
}
export interface CurrentUser {
  username: string;
  administrator: boolean;
}
export interface ListResponse {
  projects: Project[];
  current_user: CurrentUser;
}
export type RemovalAction = "remove-workspace" | "remove" | "delete-human";
export interface RemovalAccount {
  uid: number;
  username: string;
  primary_username: string;
  project_id: string;
  home: string;
}
export interface RemovalPreview {
  action: RemovalAction;
  target: string;
  revision: string;
  accounts: RemovalAccount[];
  catalog_present: boolean;
}
export interface RemovalResponse {
  ok: boolean;
  result: { removed: string[]; uncertain: string; not_attempted: string[]; diagnostic: string };
  catalog: "unchanged" | "not_attempted" | "removed" | "uncertain";
  problem: string;
}
export interface Requests {
  list: Record<string, never>;
  "add-existing": {
    id: string;
    display_name: string;
    canonical_url: string;
    [key: string]: unknown;
  };
  edit: { id: string; display_name: string; [key: string]: unknown };
  inspect: { id: string };
  setup: { id: string };
  "removal-inspect": { action: RemovalAction; target: string };
  "remove-workspace": { id: string; expected: string };
  remove: { id: string; expected: string };
  "delete-human": { username: string; expected: string };
}
export interface Responses {
  list: ListResponse;
  "add-existing": { ok: true; project: CatalogEntry };
  edit: { ok: true; project: CatalogEntry };
  inspect: { ok: true; workspace: WorkspaceInspection };
  setup: { ok: true; workspace_username: string };
  "removal-inspect": { ok: true; preview: RemovalPreview };
  "remove-workspace": RemovalResponse;
  remove: RemovalResponse;
  "delete-human": RemovalResponse;
}
export type Action = keyof Requests;
export type ProjectAction = Exclude<Action, "list" | "removal-inspect">;
export type FormAction = "add-existing" | "edit";
export type Invoke = <A extends Action>(action: A, payload: Requests[A]) => Promise<Responses[A]>;

export interface CatalogTask {
  kind: "catalog";
  action: FormAction;
  project?: Project;
  error: { message: string; field?: string } | null;
}
export interface WorkspaceTask {
  kind: "workspace";
  project: Project;
  readError: string;
  setupError: string;
  completed: boolean;
}
export interface RemovalTask {
  kind: "removal";
  action: RemovalAction;
  target: string;
  confirmation: string;
  reviewing: boolean;
  preview: RemovalPreview | null;
  receipt: RemovalResponse | null;
  selectedAccounts: RemovalAccount[];
  readError: string;
  unknown: string;
}
export type ProjectsTask = CatalogTask | WorkspaceTask | RemovalTask;
