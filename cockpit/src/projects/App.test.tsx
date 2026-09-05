// @vitest-environment jsdom
import { test, expect, vi } from "vite-plus/test";
import { render, screen, fireEvent, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Projects } from "./App";
import { coordinator } from "./native";
import { pendingProcess } from "../../tests/process";
import type { Invoke, ListResponse, Project } from "./types";

const project: Project = {
  id: "site",
  display_name: "Site",
  canonical_url: "git@example.test:team/site.git",
  catalog_metadata: { team: "web" },
  workspace_username: "soda-w-abc",
  workspace_exists: true,
};
const catalog: ListResponse = {
  current_user: { username: "alice", administrator: true },
  projects: [project],
};
function mockInvoke(data = catalog) {
  const invoke = vi.fn<Invoke>();
  invoke.mockResolvedValue(data);
  return invoke;
}
async function ready(invoke = mockInvoke()) {
  render(<Projects invoke={invoke as Invoke} hostname="soda.lan" />);
  await screen.findByText("1 project available to alice.");
  return invoke;
}
async function open(name: string) {
  fireEvent.click(screen.getByRole("button", { name }));
  return within(await screen.findByRole("dialog"));
}

test("catalog loading, empty, error, refresh and native adapter input", async () => {
  const call = pendingProcess();
  const spawn = vi.fn(() => call.process);
  render(<Projects invoke={coordinator({ spawn })} />);
  expect(screen.getByRole("button", { name: "Refresh" }).hasAttribute("disabled")).toBe(true);
  expect(spawn).toHaveBeenCalledWith(["/usr/libexec/soda/soda-projects", "list"], {
    err: "message",
  });
  expect(call.process.input).toHaveBeenCalledWith("{}\n");
  call.resolve(JSON.stringify({ ...catalog, projects: [] }));
  await screen.findByRole("heading", { name: "No projects yet" });
  const failed = pendingProcess();
  spawn.mockReturnValue(failed.process);
  fireEvent.click(screen.getByRole("button", { name: "Refresh" }));
  failed.reject(new Error("catalog unavailable"));
  await screen.findByText("The project catalog could not be loaded.");
  expect(screen.queryByRole("heading", { name: "People" })).toBeNull();
  expect(screen.getByText("catalog unavailable")).toBeTruthy();
});
test("workspace wording states existence, uses browser hostname and retains setup/removal actions", async () => {
  await ready();
  expect(screen.getByText("Workspace account exists")).toBeTruthy();
  expect(screen.getByText("ssh soda-w-abc@soda.lan")).toBeTruthy();
  expect(screen.getByRole("button", { name: "Set up for me" })).toBeTruthy();
  expect(screen.getByRole("button", { name: "Remove my workspace" })).toBeTruthy();
});
test("People leaves account creation, listing and administrator promotion to Cockpit Accounts", async () => {
  await ready();
  expect(
    screen.getByText(
      /Stock Cockpit Accounts creates and lists primary Linux users and owns administrator status/,
    ),
  ).toBeTruthy();
});
test("ordinary humans cannot see administrator deletion actions", async () => {
  await ready(
    mockInvoke({ ...catalog, current_user: { username: "alice", administrator: false } }),
  );
  expect(screen.queryByRole("button", { name: "Remove project" })).toBeNull();
  expect(screen.queryByRole("button", { name: "Remove person…" })).toBeNull();
});
test("edit explains immutable replacement, exposes readonly URL, and preserves arbitrary metadata", async () => {
  const invoke = await ready();
  const dialog = await open("Edit");
  expect((dialog.getByLabelText("Canonical Git URL") as HTMLInputElement).readOnly).toBe(true);
  expect(
    dialog.getByText(
      /administrator must remove the project and its local workspaces, then add the project again/,
    ),
  ).toBeTruthy();
  fireEvent.change(dialog.getByLabelText("Display name", { exact: false }), {
    target: { value: "Renamed" },
  });
  fireEvent.change(dialog.getByLabelText("Additional metadata"), {
    target: { value: '{"labels":["web"]}' },
  });
  invoke.mockResolvedValueOnce({ ok: true, project: { ...project, display_name: "Renamed" } });
  fireEvent.click(dialog.getByRole("button", { name: "Save changes" }));
  await waitFor(() =>
    expect(invoke).toHaveBeenCalledWith("edit", {
      id: "site",
      display_name: "Renamed",
      labels: ["web"],
    }),
  );
  await screen.findByText("Renamed was updated. Existing workspaces were not changed.");
});
test("setup keeps manual Git key guidance and refreshes account existence after failure inside its dialog", async () => {
  const invoke = await ready(
    mockInvoke({ ...catalog, projects: [{ ...project, workspace_exists: false }] }),
  );
  const dialog = await open("Set up for me");
  expect(
    dialog.getByText(/reports the public key for you to register with that host before retrying/),
  ).toBeTruthy();
  invoke.mockRejectedValueOnce(new Error("Register ssh-ed25519 AAA-test before retrying"));
  invoke.mockResolvedValueOnce(catalog);
  fireEvent.click(dialog.getByRole("button", { name: "Set up for me" }));
  await waitFor(() => expect(invoke).toHaveBeenCalledWith("setup", { id: "site" }));
  await waitFor(() =>
    expect(within(screen.getByRole("dialog")).getByRole("alert").textContent).toContain(
      "Register ssh-ed25519",
    ),
  );
  expect(screen.getByText("Workspace account exists")).toBeTruthy();
});
test.each([
  ["Remove my workspace", "remove-workspace", "Remove my workspace"],
  ["Remove project", "remove", "Remove project"],
] as const)(
  "%s requires exact confirmation and sends only the project ID",
  async (label, action, submit) => {
    const invoke = await ready();
    const dialog = await open(label);
    fireEvent.change(dialog.getByRole("textbox"), { target: { value: "SITE" } });
    fireEvent.click(dialog.getByRole("button", { name: submit }));
    expect(invoke).toHaveBeenCalledTimes(1);
    fireEvent.change(dialog.getByRole("textbox"), { target: { value: "site" } });
    invoke.mockResolvedValueOnce({ ok: true });
    fireEvent.click(dialog.getByRole("button", { name: submit }));
    await waitFor(() => expect(invoke).toHaveBeenCalledWith(action, { id: "site" }));
  },
);
test("human deletion remains separate from Forgejo and submits only an exactly confirmed username", async () => {
  const invoke = await ready(),
    dialog = await open("Remove person…");
  expect(dialog.getByText(/Forgejo account and repository data are unchanged/)).toBeTruthy();
  expect(dialog.getByText(/Delete a Forgejo account separately in Forgejo/)).toBeTruthy();
  fireEvent.change(dialog.getByLabelText(/Primary username/), { target: { value: "bob" } });
  fireEvent.change(dialog.getByLabelText(/Re-enter the username/), { target: { value: "wrong" } });
  fireEvent.click(dialog.getByRole("button", { name: "Remove person" }));
  expect(invoke).toHaveBeenCalledTimes(1);
  fireEvent.change(dialog.getByLabelText(/Re-enter the username/), { target: { value: "bob" } });
  invoke.mockResolvedValueOnce({ ok: true });
  fireEvent.click(dialog.getByRole("button", { name: "Remove person" }));
  await waitFor(() => expect(invoke).toHaveBeenCalledWith("delete-human", { username: "bob" }));
});
test("add form sends catalog fields, blocks duplicate submissions and refreshes after success", async () => {
  const invoke = await ready(),
    dialog = await open("Add repository");
  fireEvent.change(dialog.getByLabelText(/Project ID/), { target: { value: "new" } });
  fireEvent.change(dialog.getByLabelText(/Display name/), { target: { value: "New" } });
  fireEvent.change(dialog.getByLabelText(/Canonical Git URL/), {
    target: { value: "git@example.test:team/new.git" },
  });
  let finish!: (value: { ok: true; project: Project }) => void;
  invoke.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
  );
  const submit = dialog.getByRole("button", { name: "Add repository" });
  fireEvent.click(submit);
  fireEvent.click(submit);
  expect(invoke).toHaveBeenCalledTimes(2);
  expect(submit.hasAttribute("disabled")).toBe(true);
  expect(invoke).toHaveBeenLastCalledWith("add-existing", {
    id: "new",
    display_name: "New",
    canonical_url: "git@example.test:team/new.git",
  });
  finish({ ok: true, project });
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
});
test("dialog cancellation resets fields and restores keyboard focus", async () => {
  await ready();
  const user = userEvent.setup();
  const button = screen.getByRole("button", { name: "Add repository" });
  await user.click(button);
  const dialog = within(screen.getByRole("dialog"));
  await user.type(dialog.getByLabelText(/Project ID/), "scratch");
  await user.keyboard("{Escape}");
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(document.activeElement).toBe(button);
  await user.click(button);
  expect(
    (within(screen.getByRole("dialog")).getByLabelText(/Project ID/) as HTMLInputElement).value,
  ).toBe("");
});
