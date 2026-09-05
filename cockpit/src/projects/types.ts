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
  "remove-workspace": { id: string };
  remove: { id: string };
  "delete-human": { username: string };
}
export interface Responses {
  list: ListResponse;
  "add-existing": { ok: true; project: CatalogEntry };
  edit: { ok: true; project: CatalogEntry };
  inspect: { ok: true; workspace: WorkspaceInspection };
  setup: { ok: true; workspace_username: string };
  "remove-workspace": { ok: true };
  remove: { ok: true };
  "delete-human": { ok: true };
}
export type Action = keyof Requests;
export type FormAction = Exclude<Action, "list" | "inspect">;
export type Invoke = <A extends Action>(action: A, payload: Requests[A]) => Promise<Responses[A]>;
