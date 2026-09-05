// @vitest-environment jsdom
import { test, expect, vi } from "vite-plus/test";
import { act, render, screen, fireEvent, waitFor } from "@testing-library/react";
import { ProjectsPage } from "./ProjectsPage";
import { createProjectsStore } from "../projects/store";
import type { Invoke, RemovalPreview, RemovalResponse, Project } from "../projects/types";

const accounts = ["alice", "bob", "carol"].map((name, index) => ({
  username: `soda-w-${name}`,
  uid: 2000 + index,
  primary_username: name,
  project_id: "site",
  home: `/home/soda-w-${name}`,
}));
const project: Project = {
  id: "site",
  display_name: "Site",
  canonical_url: "git@example.test:team/site.git",
  catalog_metadata: {},
  workspace_username: "soda-w-admin",
  workspace_exists: false,
};
const preview: RemovalPreview = {
  action: "remove",
  target: "site",
  revision: "a".repeat(64),
  accounts,
  catalog_present: true,
};
const partial: RemovalResponse = {
  ok: false,
  result: {
    removed: [accounts[0].username],
    uncertain: accounts[1].username,
    not_attempted: [accounts[2].username],
    diagnostic: "userdel failed",
  },
  catalog: "not_attempted",
  problem: "",
};
async function mount(invoke = vi.fn<Invoke>().mockResolvedValue({ ok: true, preview })) {
  const list = vi.fn(async () => ({
    projects: [project],
    current_user: { username: "admin", administrator: true },
  }));
  const adapter = ((action, payload) =>
    action === "list" ? list() : invoke(action, payload)) as Invoke;
  const store = createProjectsStore(adapter);
  const view = render(<ProjectsPage store={store} />);
  await screen.findByRole("button", { name: "Actions — Site" });
  fireEvent.click(screen.getByRole("button", { name: "Actions — Site" }));
  fireEvent.click(screen.getByRole("menuitem", { name: "Remove project" }));
  return { invoke, list, store, ...view };
}
async function confirm() {
  await screen.findByLabelText(/Type site to confirm/);
  fireEvent.change(screen.getByLabelText(/Type site to confirm/), { target: { value: "site" } });
  fireEvent.click(screen.getByRole("button", { name: "Remove project" }));
}
test("partial receipts identify people, preserve uncertainty and require renewed confirmation", async () => {
  const invoke = vi.fn<Invoke>().mockResolvedValue({
    ok: true,
    preview: { ...preview, revision: "b".repeat(64), accounts: accounts.slice(1) },
  });
  invoke.mockResolvedValueOnce({ ok: true, preview });
  invoke.mockResolvedValueOnce(partial);
  const page = await mount(invoke);
  await confirm();
  await screen.findByText(/Deletion of bob \/ site did not finish/);
  expect(screen.getByText(/may already have been changed or deleted/)).toBeTruthy();
  expect(screen.queryByRole("button", { name: "Remove project" })).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "Review remaining removal" }));
  await screen.findByLabelText(/Type site to confirm/);
  expect(screen.getByText(/Deletion of bob \/ site did not finish/)).toBeTruthy();
  expect(invoke.mock.calls.filter(([action]) => action === "remove")).toHaveLength(1);
  expect(page.store.getState().notice).toMatchObject({
    message: expect.stringContaining("bob / site"),
    kind: "danger",
  });
});
test.each([
  { remaining: accounts.slice(2) },
  { remaining: [{ ...accounts[1], uid: 3000 }, accounts[2]] },
  { remaining: [{ ...accounts[1], home: "/home/replacement" }, accounts[2]] },
])(
  "an absent or replaced failed account does not prove its old home was removed",
  async ({ remaining }) => {
    const invoke = vi
      .fn<Invoke>()
      .mockResolvedValue({ ok: true, preview: { ...preview, accounts: remaining } });
    invoke.mockResolvedValueOnce({ ok: true, preview });
    invoke.mockResolvedValueOnce(partial);
    await mount(invoke);
    await confirm();
    await screen.findByText("Inspect remaining local data before continuing");
    expect(screen.queryByRole("button", { name: "Remove project" })).toBeNull();
    expect(screen.getByText(/does not prove all of its files were removed/)).toBeTruthy();
  },
);
test("lost responses block retries until an explicit read succeeds", async () => {
  const invoke = vi.fn<Invoke>().mockResolvedValue({ ok: true, preview });
  invoke.mockResolvedValueOnce({ ok: true, preview });
  invoke.mockRejectedValueOnce(new Error("connection closed"));
  await mount(invoke);
  await confirm();
  await screen.findByText("Removal outcome is not confirmed");
  expect(screen.queryByRole("button", { name: "Remove project" })).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "Check current state" }));
  await screen.findByLabelText(/Type site to confirm/);
  expect(screen.getByText("Removal outcome is not confirmed")).toBeTruthy();
  expect(
    (screen.getByRole("button", { name: "Remove project" }) as HTMLButtonElement).disabled,
  ).toBe(true);
});
test("inspection failure cannot be bypassed by typing confirmation", async () => {
  await mount(vi.fn<Invoke>().mockRejectedValue(new Error("account evidence unreadable")));
  await screen.findByText("Affected accounts could not be checked");
  expect(screen.queryByLabelText(/Type site to confirm/)).toBeNull();
  expect(screen.queryByRole("button", { name: "Remove project" })).toBeNull();
});
test("leaving during removal starts no subsequent refresh or native inspection", async () => {
  let finish!: (response: RemovalResponse) => void;
  const invoke = vi.fn<Invoke>().mockResolvedValueOnce({ ok: true, preview });
  invoke.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
  );
  const page = await mount(invoke);
  await confirm();
  expect((screen.getByRole("button", { name: "Cancel" }) as HTMLButtonElement).disabled).toBe(true);
  page.unmount();
  await act(async () => {
    finish(partial);
  });
  expect(page.list).toHaveBeenCalledOnce();
  expect(invoke).toHaveBeenCalledTimes(2);
});
test("a changed scope is visible and refreshed without automatically resubmitting", async () => {
  const invoke = vi
    .fn<Invoke>()
    .mockResolvedValue({ ok: true, preview: { ...preview, revision: "b".repeat(64) } });
  invoke.mockResolvedValueOnce({ ok: true, preview });
  invoke.mockResolvedValueOnce({
    ...partial,
    result: { removed: [], uncertain: "", not_attempted: [], diagnostic: "" },
    problem: "Removal scope changed; inspect again and confirm the new selection.",
  });
  await mount(invoke);
  await confirm();
  await screen.findByText(/Removal scope changed/);
  fireEvent.click(screen.getByRole("button", { name: "Review remaining removal" }));
  await waitFor(() =>
    expect((screen.getByLabelText(/Type site to confirm/) as HTMLInputElement).value).toBe(""),
  );
  expect(invoke.mock.calls.filter(([action]) => action === "remove")).toHaveLength(1);
});
