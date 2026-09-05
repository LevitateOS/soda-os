// @vitest-environment jsdom
import { afterEach, test, expect, vi } from "vite-plus/test";
import { act, render, screen, fireEvent, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ProjectsPage } from "./ProjectsPage";
import { createProjectsStore } from "../projects/store";
import { coordinator } from "../projects/native";
import { pendingProcess } from "../../tests/process";
import type {
  Invoke,
  ListResponse,
  Project,
  WorkspaceInspection,
  Requests,
  RemovalResponse,
} from "../projects/types";

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
const inspection: WorkspaceInspection = {
  username: "soda-w-abc",
  exists: true,
  checkout_ready: true,
  checkout_path: "/home/soda-w-abc/Projects/site",
  public_key: "ssh-ed25519 EXAMPLE-PUBLIC-KEY",
  primary_key_problem: "",
  workspace_key_problem: "",
  checkout_problem: "",
  git_key_problem: "",
};
const absent: WorkspaceInspection = {
  ...inspection,
  exists: false,
  checkout_ready: false,
  checkout_path: "",
  public_key: "",
};
function mockInvoke(data = catalog, workspace = inspection) {
  return vi.fn<Invoke>().mockImplementation((async (action, payload) => {
    if (action === "inspect") return { ok: true, workspace };
    if (action === "removal-inspect") {
      const request = payload as Requests["removal-inspect"];
      return {
        ok: true,
        preview: {
          ...request,
          revision: "a".repeat(64),
          catalog_present: action === "removal-inspect",
          accounts: [
            {
              username: inspection.username,
              uid: 2000,
              primary_username: "alice",
              project_id: "site",
              home: "/home/" + inspection.username,
            },
          ],
        },
      };
    }
    return data;
  }) as Invoke);
}
async function ready(invoke = mockInvoke()) {
  render(<ProjectsPage store={createProjectsStore(invoke as Invoke)} hostname="soda.lan" />);
  await screen.findByText("1 project available to alice.");
  return invoke;
}
async function open(name: string) {
  if (["Edit project", "Remove project", "Remove my workspace"].includes(name)) {
    fireEvent.click(screen.getByRole("button", { name: "Actions — Site" }));
    fireEvent.click(screen.getByRole("menuitem", { name }));
  } else if (name === "Remove person…") {
    fireEvent.click(screen.getByRole("button", { name: "People actions" }));
    fireEvent.click(screen.getByRole("menuitem", { name }));
  } else fireEvent.click(screen.getByRole("button", { name }));
  const dialog = within(await screen.findByRole("dialog"));
  if (["Remove project", "Remove my workspace"].includes(name))
    await waitFor(() => expect(dialog.getByLabelText(/Type site to confirm/)).toBeTruthy());
  return dialog;
}
afterEach(() => vi.unstubAllGlobals());
const removed: RemovalResponse = {
  ok: true,
  result: { removed: [inspection.username], uncertain: "", not_attempted: [], diagnostic: "" },
  catalog: "unchanged",
  problem: "",
};

test("catalog loading, failed read and recovery use the unprivileged native list", async () => {
  const call = pendingProcess();
  const spawn = vi.fn(() => call.process);
  render(<ProjectsPage store={createProjectsStore(coordinator({ spawn }))} />);
  expect(spawn).toHaveBeenCalledWith(["/usr/libexec/soda/soda-projects", "list"], {
    err: "message",
  });
  expect(call.process.input).toHaveBeenCalledWith("{}\n");
  expect((screen.getByRole("button", { name: "Refresh" }) as HTMLButtonElement).disabled).toBe(
    true,
  );
  call.resolve(JSON.stringify({ ...catalog, projects: [] }));
  await screen.findByRole("heading", { name: "No projects yet" });
  expect(screen.getAllByRole("button", { name: "Add repository" })).toHaveLength(1);
  const failed = pendingProcess();
  spawn.mockReturnValue(failed.process);
  fireEvent.click(screen.getByRole("button", { name: "Refresh" }));
  failed.reject(new Error("catalog unavailable"));
  await screen.findByText(/current catalog could not be refreshed.*catalog unavailable/);
  expect(screen.queryByRole("region", { name: "People management" })).toBeNull();
});

test("quiet list never equates account existence with readiness; inspection supplies connection details", async () => {
  const invoke = await ready();
  expect(screen.getByText("Setup not confirmed")).toBeTruthy();
  expect(screen.queryByText(project.canonical_url)).toBeNull();
  expect(screen.queryByText("ssh soda-w-abc@soda.lan")).toBeNull();
  expect(invoke).toHaveBeenCalledTimes(1);
  const dialog = await open("Review setup — Site");
  await waitFor(() => expect(dialog.getByLabelText("SSH command")).toBeTruthy());
  expect((dialog.getByLabelText("SSH command") as HTMLInputElement).value).toBe(
    "ssh soda-w-abc@soda.lan",
  );
  expect(dialog.getByText(inspection.checkout_path)).toBeTruthy();
  expect(invoke).toHaveBeenCalledWith("inspect", { id: "site" });
  expect(invoke).not.toHaveBeenCalledWith("setup", expect.anything());
  fireEvent.click(dialog.getAllByRole("button", { name: "Close" }).at(-1)!);
  expect(screen.getByText("Ready")).toBeTruthy();
  expect(screen.getByRole("button", { name: "Connection details — Site" })).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "Refresh" }));
  await screen.findByText("Setup not confirmed");
});

test("Accounts owns people management while Soda-aware deletion stays administrator-only", async () => {
  const jump = vi.fn();
  vi.stubGlobal("cockpit", { jump });
  await ready(
    mockInvoke({ ...catalog, current_user: { username: "alice", administrator: false } }),
  );
  fireEvent.click(screen.getByRole("button", { name: "Manage people in Accounts" }));
  expect(jump).toHaveBeenCalledWith("/users");
  expect(screen.queryByRole("button", { name: "People actions" })).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "Actions — Site" }));
  expect(screen.queryByRole("menuitem", { name: "Remove project" })).toBeNull();
  expect(screen.getByRole("menuitem", { name: "Remove my workspace" })).toBeTruthy();
});

test("edit keeps immutable fields readonly and preserves arbitrary metadata even while collapsed", async () => {
  const invoke = await ready();
  const dialog = await open("Edit project");
  expect((dialog.getByLabelText("Repository SSH address") as HTMLInputElement).readOnly).toBe(true);
  expect((dialog.getByLabelText("Project ID") as HTMLInputElement).readOnly).toBe(true);
  expect(dialog.getByText(/Replacing the address requires administrator removal/)).toBeTruthy();
  fireEvent.change(dialog.getByLabelText(/Project name/), { target: { value: "Renamed" } });
  fireEvent.click(dialog.getByRole("button", { name: "Additional metadata (optional)" }));
  fireEvent.change(dialog.getByLabelText("Metadata JSON"), {
    target: { value: '{"labels":["web"],"custom":{"enabled":true}}' },
  });
  fireEvent.click(dialog.getByRole("button", { name: "Additional metadata (optional)" }));
  invoke.mockResolvedValueOnce({ ok: true, project: { ...project, display_name: "Renamed" } });
  fireEvent.click(dialog.getByRole("button", { name: "Save changes" }));
  await screen.findByText("Renamed was updated. Existing workspaces were not changed.");
  expect(invoke).toHaveBeenCalledWith("edit", {
    id: "site",
    display_name: "Renamed",
    labels: ["web"],
    custom: { enabled: true },
  });
});

test("metadata validation opens the field, focuses it, and stays out of background alerts", async () => {
  const invoke = await ready();
  const dialog = await open("Edit project");
  fireEvent.click(dialog.getByRole("button", { name: "Additional metadata (optional)" }));
  const metadata = dialog.getByLabelText("Metadata JSON");
  fireEvent.change(metadata, { target: { value: "{invalid}" } });
  fireEvent.click(dialog.getByRole("button", { name: "Additional metadata (optional)" }));
  fireEvent.click(dialog.getByRole("button", { name: "Save changes" }));
  await waitFor(() => expect(document.activeElement).toBe(metadata));
  expect(dialog.getByRole("alert").textContent).toContain(
    "Additional metadata must be a valid JSON object.",
  );
  expect(screen.getAllByText(/Additional metadata must be a valid JSON object/)).toHaveLength(1);
  fireEvent.click(dialog.getByRole("button", { name: "Additional metadata (optional)" }));
  fireEvent.click(dialog.getByRole("button", { name: "Save changes" }));
  await waitFor(() =>
    expect(
      dialog
        .getByRole("button", { name: "Additional metadata (optional)" })
        .getAttribute("aria-expanded"),
    ).toBe("true"),
  );
  await waitFor(() => expect(document.activeElement).toBe(metadata));
  expect(invoke).toHaveBeenCalledTimes(1);
});

test("a missing personal key is checked before mutation; Check setup never creates a workspace", async () => {
  const invoke = await ready(
    mockInvoke(
      { ...catalog, projects: [{ ...project, workspace_exists: false }] },
      { ...absent, primary_key_problem: "authorized_keys missing" },
    ),
  );
  const dialog = await open("Set up for me — Site");
  await waitFor(() => expect(dialog.getByText("Add or check your SSH public key")).toBeTruthy());
  expect(invoke).not.toHaveBeenCalledWith("setup", expect.anything());
  invoke.mockResolvedValueOnce({ ok: true, workspace: absent });
  fireEvent.click(dialog.getByRole("button", { name: "Check setup" }));
  await waitFor(() => expect(dialog.getByRole("button", { name: "Set up for me" })).toBeTruthy());
  expect(invoke).not.toHaveBeenCalledWith("setup", expect.anything());
});

test("explicit setup checks prerequisites, prevents duplicates, refreshes facts and reaches connection guidance", async () => {
  const invoke = await ready(
    mockInvoke({ ...catalog, projects: [{ ...project, workspace_exists: false }] }),
  );
  let finish!: (value: { ok: true; workspace_username: string }) => void;
  invoke.mockResolvedValueOnce({ ok: true, workspace: absent });
  invoke.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
  );
  invoke.mockResolvedValueOnce(catalog);
  invoke.mockResolvedValueOnce({ ok: true, workspace: inspection });
  const dialog = await open("Set up for me — Site");
  await waitFor(() => expect(invoke).toHaveBeenCalledWith("setup", { id: "site" }));
  expect(dialog.getByText("Setting up your workspace…")).toBeTruthy();
  expect((dialog.getByRole("button", { name: "Close" }) as HTMLButtonElement).disabled).toBe(true);
  finish({ ok: true, workspace_username: inspection.username });
  await waitFor(() => expect(dialog.getByLabelText("SSH command")).toBeTruthy());
  expect(invoke.mock.calls.filter(([action]) => action === "setup")).toHaveLength(1);
});

test("clone failure shows the retained native public key without diagnosing every failure as authorization", async () => {
  const invoke = await ready(
    mockInvoke({ ...catalog, projects: [{ ...project, workspace_exists: false }] }),
  );
  invoke.mockResolvedValueOnce({ ok: true, workspace: absent });
  invoke.mockRejectedValueOnce(new Error("repository host unavailable"));
  invoke.mockResolvedValueOnce(catalog);
  invoke.mockResolvedValueOnce({ ok: true, workspace: { ...inspection, checkout_ready: false } });
  const dialog = await open("Set up for me — Site");
  await waitFor(() =>
    expect(dialog.getByRole("button", { name: "Copy workspace public key" })).toBeTruthy(),
  );
  expect((dialog.getByLabelText("Workspace public key") as HTMLInputElement).value).toBe(
    inspection.public_key,
  );
  expect(dialog.getByText(/If this workspace key is not registered/)).toBeTruthy();
  expect(dialog.getByText(/check the repository address and connection/)).toBeTruthy();
  expect(dialog.getByRole("button", { name: "Retry setup" })).toBeTruthy();
  fireEvent.click(dialog.getByRole("button", { name: "Technical details" }));
  expect(dialog.getByText("repository host unavailable")).toBeTruthy();
});

test("unknown setup outcomes require a successful inspection before retry; refresh can recover readiness", async () => {
  const invoke = await ready(
    mockInvoke({ ...catalog, projects: [{ ...project, workspace_exists: false }] }),
  );
  invoke.mockResolvedValueOnce({ ok: true, workspace: absent });
  invoke.mockRejectedValueOnce(new Error("connection closed"));
  invoke.mockResolvedValueOnce(catalog);
  invoke.mockRejectedValueOnce(new Error("inspection unavailable"));
  const dialog = await open("Set up for me — Site");
  await waitFor(() => expect(dialog.getByText("Workspace status is not confirmed")).toBeTruthy());
  expect(dialog.queryByRole("button", { name: "Retry setup" })).toBeNull();
  expect(dialog.queryByRole("button", { name: "Copy SSH command" })).toBeNull();
  invoke.mockResolvedValueOnce({ ok: true, workspace: inspection });
  fireEvent.click(dialog.getByRole("button", { name: "Check setup" }));
  await waitFor(() =>
    expect(dialog.getByRole("button", { name: "Copy SSH command" })).toBeTruthy(),
  );
  expect(dialog.queryByRole("button", { name: "Technical details" })).toBeNull();
  expect(invoke.mock.calls.filter(([action]) => action === "setup")).toHaveLength(1);
});

test.each([
  ["Remove my workspace", "remove-workspace"],
  ["Remove project", "remove"],
] as const)("%s still requires exact confirmation and sends only the ID", async (label, action) => {
  const invoke = await ready();
  const dialog = await open(label);
  fireEvent.change(dialog.getByRole("textbox"), { target: { value: "SITE" } });
  fireEvent.click(dialog.getByRole("button", { name: label }));
  expect(invoke).toHaveBeenCalledTimes(2);
  fireEvent.change(dialog.getByRole("textbox"), { target: { value: "site" } });
  invoke.mockResolvedValueOnce(removed);
  fireEvent.click(dialog.getByRole("button", { name: label }));
  await waitFor(() =>
    expect(invoke).toHaveBeenCalledWith(action, { id: "site", expected: "a".repeat(64) }),
  );
});

test("human deletion remains a separate Soda-aware action preserving Forgejo", async () => {
  const invoke = await ready();
  const dialog = await open("Remove person…");
  fireEvent.change(dialog.getByLabelText(/Primary username/), { target: { value: "bob" } });
  fireEvent.click(dialog.getByRole("button", { name: "Check affected accounts" }));
  await waitFor(() => expect(dialog.getByLabelText(/Type bob to confirm/)).toBeTruthy());
  expect(dialog.getByText(/Forgejo account deletion is separate/)).toBeTruthy();
  fireEvent.change(dialog.getByLabelText(/Type bob to confirm/), { target: { value: "wrong" } });
  fireEvent.click(dialog.getByRole("button", { name: "Remove person" }));
  expect(invoke).toHaveBeenCalledTimes(2);
  fireEvent.change(dialog.getByLabelText(/Type bob to confirm/), { target: { value: "bob" } });
  invoke.mockResolvedValueOnce(removed);
  fireEvent.click(dialog.getByRole("button", { name: "Remove person" }));
  await waitFor(() =>
    expect(invoke).toHaveBeenCalledWith("delete-human", {
      username: "bob",
      expected: "a".repeat(64),
    }),
  );
});

test("failed removal refreshes facts and retains its partial outcome after closing", async () => {
  const invoke = await ready();
  const dialog = await open("Remove project");
  fireEvent.change(dialog.getByRole("textbox"), { target: { value: "site" } });
  invoke.mockRejectedValueOnce(
    new Error("Alice’s workspace was deleted; Bob’s workspace remains."),
  );
  invoke.mockResolvedValueOnce({ ...catalog, projects: [{ ...project, workspace_exists: false }] });
  fireEvent.click(dialog.getByRole("button", { name: "Remove project" }));
  await waitFor(() => expect(dialog.getByText("Removal outcome is not confirmed")).toBeTruthy());
  fireEvent.click(dialog.getByRole("button", { name: "Technical details" }));
  expect(screen.getByText("Not set up")).toBeTruthy();
  expect(screen.getAllByText(/Bob’s workspace remains/)).toHaveLength(1);
  fireEvent.click(dialog.getAllByRole("button", { name: "Close" }).at(-1)!);
  expect(screen.getByText(/Bob’s workspace remains/)).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "Refresh" }));
  await screen.findByText("1 project available to alice.");
  expect(screen.getByText(/Bob’s workspace remains/)).toBeTruthy();
});

test("successful catalog mutation does not hide failed refresh, and recovered reads retire only the read error", async () => {
  const invoke = await ready();
  const dialog = await open("Edit project");
  invoke.mockResolvedValueOnce({ ok: true, project });
  invoke.mockRejectedValueOnce(new Error("catalog unavailable"));
  fireEvent.click(dialog.getByRole("button", { name: "Save changes" }));
  await screen.findByText("Site was updated. Existing workspaces were not changed.");
  expect(
    screen.getByText(/current catalog could not be refreshed.*catalog unavailable/),
  ).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "Refresh" }));
  await screen.findByText("1 project available to alice.");
  expect(screen.queryByText(/catalog unavailable/)).toBeNull();
  expect(screen.getByText("Site was updated. Existing workspaces were not changed.")).toBeTruthy();
});

test("inspecting one project never marks a sibling project ready", async () => {
  const second = { ...project, id: "api", display_name: "API", workspace_username: "soda-w-api" };
  const invoke = mockInvoke({ ...catalog, projects: [project, second] });
  render(<ProjectsPage store={createProjectsStore(invoke as Invoke)} hostname="soda.lan" />);
  await screen.findByText("2 projects available to alice.");
  const dialog = await open("Review setup — Site");
  await waitFor(() => expect(dialog.getByLabelText("SSH command")).toBeTruthy());
  fireEvent.click(dialog.getAllByRole("button", { name: "Close" }).at(-1)!);
  expect(screen.getAllByText("Ready")).toHaveLength(1);
  expect(screen.getByRole("button", { name: "Review setup — API" })).toBeTruthy();
  expect(invoke).not.toHaveBeenCalledWith("inspect", { id: "api" });
});

test("a checkout inspection problem blocks setup instead of overwriting work", async () => {
  const invoke = await ready(
    mockInvoke(catalog, {
      ...inspection,
      checkout_ready: false,
      checkout_problem: "Git metadata unreadable",
    }),
  );
  const dialog = await open("Review setup — Site");
  await waitFor(() => expect(dialog.getByText("Your checkout needs inspection")).toBeTruthy());
  expect(dialog.queryByRole("button", { name: "Retry setup" })).toBeNull();
  expect(invoke).not.toHaveBeenCalledWith("setup", expect.anything());
});

test("add still sends native catalog fields and prevents duplicate submissions", async () => {
  const invoke = await ready();
  const dialog = await open("Add repository");
  fireEvent.change(dialog.getByLabelText(/Project name/), { target: { value: "New" } });
  fireEvent.change(dialog.getByLabelText(/Project ID/), { target: { value: "new" } });
  fireEvent.change(dialog.getByLabelText(/Repository SSH address/), {
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
  expect(submit.hasAttribute("disabled")).toBe(true);
  expect(invoke).toHaveBeenCalledTimes(2);
  expect(invoke).toHaveBeenLastCalledWith("add-existing", {
    id: "new",
    display_name: "New",
    canonical_url: "git@example.test:team/new.git",
  });
  finish({ ok: true, project: { ...project, display_name: "New" } });
  await screen.findByText("New was added to the catalog.");
});

test("failed setup with no account has visible failure feedback, not just hidden diagnostics", async () => {
  const invoke = await ready(
    mockInvoke({ ...catalog, projects: [{ ...project, workspace_exists: false }] }),
  );
  invoke.mockResolvedValueOnce({ ok: true, workspace: absent });
  invoke.mockRejectedValueOnce(new Error("account creation failed"));
  invoke.mockResolvedValueOnce(catalog);
  invoke.mockResolvedValueOnce({ ok: true, workspace: absent });
  const dialog = await open("Set up for me — Site");
  await waitFor(() =>
    expect(dialog.getByRole("alert").textContent).toContain("Setup did not finish"),
  );
  expect(dialog.getByRole("button", { name: "Set up for me" })).toBeTruthy();
});

test("a completed setup command is distinguished from failed verification", async () => {
  const invoke = await ready(
    mockInvoke({ ...catalog, projects: [{ ...project, workspace_exists: false }] }),
  );
  invoke.mockResolvedValueOnce({ ok: true, workspace: absent });
  invoke.mockResolvedValueOnce({ ok: true, workspace_username: inspection.username });
  invoke.mockResolvedValueOnce(catalog);
  invoke.mockRejectedValueOnce(new Error("inspection unavailable"));
  const dialog = await open("Set up for me — Site");
  await waitFor(() =>
    expect(
      dialog.getByText(/Setup completed, but the current workspace could not be checked/),
    ).toBeTruthy(),
  );
  expect(dialog.queryByRole("button", { name: "Retry setup" })).toBeNull();
});

test("verified workspace readiness does not hide a failed catalog refresh", async () => {
  const invoke = await ready(
    mockInvoke({ ...catalog, projects: [{ ...project, workspace_exists: false }] }),
  );
  invoke.mockResolvedValueOnce({ ok: true, workspace: absent });
  invoke.mockResolvedValueOnce({ ok: true, workspace_username: inspection.username });
  invoke.mockRejectedValueOnce(new Error("catalog unavailable"));
  invoke.mockResolvedValueOnce({ ok: true, workspace: inspection });
  const dialog = await open("Set up for me — Site");
  await waitFor(() => expect(dialog.getByLabelText("SSH command")).toBeTruthy());
  expect(dialog.getByRole("alert").textContent).toContain(
    "The project list could not be refreshed",
  );
  fireEvent.click(dialog.getAllByRole("button", { name: "Close" }).at(-1)!);
  expect(
    screen.getByText(/current catalog could not be refreshed.*catalog unavailable/),
  ).toBeTruthy();
});

test("leaving Projects during setup does not start later native reads", async () => {
  const invoke = mockInvoke({ ...catalog, projects: [{ ...project, workspace_exists: false }] });
  const page = render(<ProjectsPage store={createProjectsStore(invoke as Invoke)} />);
  await screen.findByText("1 project available to alice.");
  let finish!: (value: { ok: true; workspace_username: string }) => void;
  invoke.mockResolvedValueOnce({ ok: true, workspace: absent });
  invoke.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
  );
  await open("Set up for me — Site");
  await waitFor(() => expect(invoke).toHaveBeenCalledWith("setup", { id: "site" }));
  page.unmount();
  await act(async () => {
    finish({ ok: true, workspace_username: inspection.username });
  });
  expect(invoke.mock.calls.map(([action]) => action)).toEqual(["list", "inspect", "setup"]);
});

test("cancel discards draft input and restores keyboard focus", async () => {
  await ready();
  const user = userEvent.setup();
  const trigger = screen.getByRole("button", { name: "Add repository" });
  await user.click(trigger);
  await user.type(within(screen.getByRole("dialog")).getByLabelText(/Project name/), "Draft");
  await user.keyboard("{Escape}");
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(document.activeElement).toBe(trigger);
  await user.click(trigger);
  expect(
    (within(screen.getByRole("dialog")).getByLabelText(/Project name/) as HTMLInputElement).value,
  ).toBe("");
});
