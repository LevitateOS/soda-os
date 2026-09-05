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
  setup: { id: string };
  "remove-workspace": { id: string };
  remove: { id: string };
  "delete-human": { username: string };
}
export interface Responses {
  list: ListResponse;
  "add-existing": { ok: true; project: CatalogEntry };
  edit: { ok: true; project: CatalogEntry };
  setup: { ok: true; workspace_username: string };
  "remove-workspace": { ok: true };
  remove: { ok: true };
  "delete-human": { ok: true };
}
export type Action = keyof Requests;
export type FormAction = Exclude<Action, "list">;
export type Invoke = <A extends Action>(action: A, payload: Requests[A]) => Promise<Responses[A]>;
