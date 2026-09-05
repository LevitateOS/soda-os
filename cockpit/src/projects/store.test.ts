import { test, expect, vi } from "vite-plus/test";
import { createProjectsStore } from "./store";
import type {
  Invoke,
  Project,
  WorkspaceInspection,
  RemovalPreview,
  RemovalResponse,
} from "./types";
const project: Project = {
  id: "site",
  display_name: "Site",
  canonical_url: "git@example.test:team/site.git",
  catalog_metadata: {},
  workspace_username: "soda-w-alice",
  workspace_exists: false,
};
const workspace: WorkspaceInspection = {
  username: project.workspace_username,
  exists: true,
  checkout_ready: true,
  checkout_path: "/home/soda-w-alice/Projects/site",
  public_key: "public",
  primary_key_problem: "",
  workspace_key_problem: "",
  checkout_problem: "",
  git_key_problem: "",
};
const preview: RemovalPreview = {
  action: "remove",
  target: "site",
  revision: "a".repeat(64),
  catalog_present: true,
  accounts: [
    {
      username: workspace.username,
      uid: 2000,
      home: "/home/soda-w-alice",
      primary_username: "alice",
      project_id: "site",
    },
  ],
};
const partial: RemovalResponse = {
  ok: false,
  catalog: "not_attempted",
  problem: "",
  result: {
    removed: [],
    uncertain: workspace.username,
    not_attempted: [],
    diagnostic: "partial deletion",
  },
};
async function ready() {
  const invoke = vi.fn<Invoke>((async (action) => {
    if (action === "list")
      return { projects: [project], current_user: { username: "alice", administrator: true } };
    if (action === "inspect") return { ok: true, workspace };
    return { ok: true, preview };
  }) as Invoke);
  const store = createProjectsStore(invoke as Invoke);
  const stop = store.getState().start();
  await idle(store);
  return { store, invoke, stop };
}
async function idle(store: ReturnType<typeof createProjectsStore>) {
  await vi.waitFor(() => expect(store.getState().operation).toBeNull());
}
test("one keyed observation supplies readiness; refresh invalidates it without inspecting other projects", async () => {
  const { store, invoke, stop } = await ready();
  store.getState().open("inspect", project);
  await idle(store);
  expect(store.getState().inspections).toEqual({ site: workspace });
  store.getState().close();
  await store.getState().refresh();
  expect(store.getState().inspections).toEqual({});
  expect(invoke.mock.calls.filter(([action]) => action === "inspect")).toHaveLength(1);
  stop();
});
test("direct setup actions honor native prerequisites and a check never sets up", async () => {
  const { store, invoke, stop } = await ready();
  invoke.mockResolvedValueOnce({
    ok: true,
    workspace: {
      ...workspace,
      exists: false,
      checkout_ready: false,
      primary_key_problem: "missing key",
    },
  });
  store.getState().open("setup", project);
  await idle(store);
  expect(invoke.mock.calls.filter(([action]) => action === "setup")).toHaveLength(0);
  await store.getState().checkWorkspace();
  expect(invoke.mock.calls.filter(([action]) => action === "setup")).toHaveLength(0);
  stop();
});
test("removal actions require exact confirmation and explicit renewed review after a partial receipt", async () => {
  const { store, invoke, stop } = await ready();
  store.getState().open("remove", project);
  await idle(store);
  store.getState().changeConfirmation(" site");
  await store.getState().remove();
  expect(invoke.mock.calls.filter(([action]) => action === "remove")).toHaveLength(0);
  invoke.mockResolvedValueOnce(partial);
  store.getState().changeConfirmation("site");
  await store.getState().remove();
  store.getState().changeConfirmation("site");
  await store.getState().remove();
  expect(invoke.mock.calls.filter(([action]) => action === "remove")).toHaveLength(1);
  await store.getState().checkRemoval();
  expect(store.getState().task).toMatchObject({
    confirmation: "",
    receipt: partial,
    selectedAccounts: preview.accounts,
  });
  stop();
});
test("missing or replaced failed identities block the action even after another review", async () => {
  const { store, invoke, stop } = await ready();
  store.getState().open("remove", project);
  await idle(store);
  invoke.mockResolvedValueOnce(partial);
  store.getState().changeConfirmation("site");
  await store.getState().remove();
  invoke.mockResolvedValueOnce({
    ok: true,
    preview: { ...preview, accounts: [{ ...preview.accounts[0], uid: 3000 }] },
  });
  await store.getState().checkRemoval();
  store.getState().changeConfirmation("site");
  await store.getState().remove();
  expect(invoke.mock.calls.filter(([action]) => action === "remove")).toHaveLength(1);
  stop();
});
test("an unknown response needs an explicit successful read before retry and sends only the newly reviewed revision", async () => {
  const { store, invoke, stop } = await ready();
  store.getState().open("remove", project);
  await idle(store);
  invoke.mockRejectedValueOnce(new Error("connection closed"));
  store.getState().changeConfirmation("site");
  await store.getState().remove();
  store.getState().changeConfirmation("site");
  await store.getState().remove();
  expect(invoke.mock.calls.filter(([action]) => action === "remove")).toHaveLength(1);
  invoke.mockResolvedValueOnce({ ok: true, preview: { ...preview, revision: "b".repeat(64) } });
  await store.getState().checkRemoval();
  invoke.mockResolvedValueOnce({
    ok: true,
    catalog: "removed",
    problem: "",
    result: { removed: [workspace.username], uncertain: "", not_attempted: [], diagnostic: "" },
  });
  store.getState().changeConfirmation("site");
  const command = store.getState().remove();
  await store.getState().remove();
  await command;
  expect(invoke).toHaveBeenCalledWith("remove", { id: "site", expected: "b".repeat(64) });
  expect(invoke.mock.calls.filter(([action]) => action === "remove")).toHaveLength(2);
  store.getState().close();
  store.getState().open("delete-human");
  expect(store.getState().task).toMatchObject({
    receipt: null,
    preview: null,
    confirmation: "",
    selectedAccounts: [],
    unknown: "",
  });
  stop();
});
test("retired inspections cannot overwrite the next activation's task", async () => {
  const { store, invoke, stop } = await ready();
  let finish!: (value: { ok: true; workspace: WorkspaceInspection }) => void;
  invoke.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
  );
  store.getState().open("inspect", project);
  stop();
  const stopAgain = store.getState().start();
  await idle(store);
  store.getState().open("delete-human");
  const state = store.getState();
  finish({ ok: true, workspace });
  await Promise.resolve();
  await Promise.resolve();
  expect(store.getState()).toBe(state);
  expect(store.getState().inspections).toEqual({});
  stopAgain();
});
