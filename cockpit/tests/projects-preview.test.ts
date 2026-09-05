// @vitest-environment jsdom
import { createElement } from "react";
import { afterEach, expect, test, vi } from "vite-plus/test";
import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ProjectsPreview, type Scenario } from "../prototypes/projects/ProjectsPreview";

function show(initialScenario: Scenario = "absent") {
  render(createElement(ProjectsPreview, { initialScenario }));
}
function open(name: string | RegExp) {
  fireEvent.click(screen.getByRole("button", { name }));
  return within(screen.getByRole("dialog"));
}
afterEach(() => vi.useRealTimers());

test("ordinary view exposes setup without displaying connection instructions or deletion buttons", () => {
  show();
  expect(
    within(screen.getByRole("region", { name: "Project list" })).getByText("Not set up"),
  ).toBeTruthy();
  expect(screen.getByRole("button", { name: "Set up for me — Team website" })).toBeTruthy();
  expect(screen.queryByText(/ssh soda-w/)).toBeNull();
  expect(screen.queryByRole("button", { name: "Remove project" })).toBeNull();
  expect(screen.queryByText("Technical details")).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "Actions — Team website" }));
  expect(screen.getByRole("menuitem", { name: "Edit project" })).toBeTruthy();
  expect(screen.getByRole("menuitem", { name: "Remove project" })).toBeTruthy();
});

test("empty state has one addition action and no empty table", () => {
  show("empty");
  expect(screen.getAllByRole("button", { name: "Add repository" })).toHaveLength(1);
  expect(screen.queryByRole("grid")).toBeNull();
  expect(screen.getByRole("heading", { name: "No projects yet" })).toBeTruthy();
});

test("form errors stay inside the dialog and focus the field; arbitrary metadata is retained", async () => {
  show("empty");
  const dialog = open("Add repository");
  fireEvent.click(dialog.getByRole("button", { name: "Add repository" }));
  await waitFor(() => expect(document.activeElement).toBe(dialog.getByLabelText(/Project name/)));
  expect(
    dialog
      .getAllByRole("alert")
      .some((alert) => alert.textContent?.includes("Enter a project name.")),
  ).toBe(true);
  fireEvent.change(dialog.getByLabelText(/Project name/), { target: { value: "New site" } });
  fireEvent.change(dialog.getByLabelText("Project ID", { exact: false }), {
    target: { value: "site" },
  });
  fireEvent.change(dialog.getByLabelText(/Repository SSH address/), {
    target: { value: "git@example.test:team/site.git" },
  });
  fireEvent.click(dialog.getByRole("button", { name: "Additional metadata (optional)" }));
  fireEvent.change(dialog.getByLabelText("Metadata JSON"), { target: { value: "{bad}" } });
  fireEvent.click(dialog.getByRole("button", { name: "Add repository" }));
  await waitFor(() => expect(document.activeElement).toBe(dialog.getByLabelText("Metadata JSON")));
  expect(dialog.getByRole("alert").textContent).toContain("Use valid JSON");
  const metadata = '{"team":"web","custom":{"labels":["one"]}}';
  fireEvent.change(dialog.getByLabelText("Metadata JSON"), { target: { value: metadata } });
  fireEvent.click(dialog.getByRole("button", { name: "Add repository" }));
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(screen.getByText("New site")).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "Actions — New site" }));
  fireEvent.click(screen.getByRole("menuitem", { name: "Edit project" }));
  const edit = within(screen.getByRole("dialog"));
  expect((edit.getByLabelText(/Repository SSH address/) as HTMLInputElement).readOnly).toBe(true);
  fireEvent.click(edit.getByRole("button", { name: "Additional metadata (optional)" }));
  expect((edit.getByLabelText("Metadata JSON") as HTMLTextAreaElement).value).toBe(metadata);
});

test("cancel discards draft input and restores keyboard focus", async () => {
  show("empty");
  const user = userEvent.setup();
  const trigger = screen.getByRole("button", { name: "Add repository" });
  await user.click(trigger);
  await user.type(screen.getByRole("dialog").querySelector("input")!, "Draft");
  await user.keyboard("{Escape}");
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(document.activeElement).toBe(trigger);
  await user.click(trigger);
  expect((screen.getByLabelText(/Project name/) as HTMLInputElement).value).toBe("");
});

test("setup exposes only the current prerequisite and reaches connection guidance", () => {
  vi.useFakeTimers();
  show();
  const dialog = open("Set up for me — Team website");
  expect(dialog.getByRole("status").textContent).toContain("Setting up your workspace");
  expect((dialog.getByRole("button", { name: "Close" }) as HTMLButtonElement).disabled).toBe(true);
  act(() => {
    vi.advanceTimersByTime(1800);
  });
  expect(dialog.getByText("Add your SSH public key first")).toBeTruthy();
  expect(dialog.queryByRole("button", { name: "Copy workspace public key" })).toBeNull();
  fireEvent.click(dialog.getByRole("button", { name: "Open Accounts" }));
  expect(dialog.getByRole("status").textContent).toContain("Preview destination: Cockpit Accounts");
  fireEvent.click(dialog.getByRole("button", { name: "Try again" }));
  act(() => {
    vi.advanceTimersByTime(1800);
  });
  expect(dialog.getByRole("button", { name: "Copy workspace public key" })).toBeTruthy();
  expect(dialog.queryByText("Add your SSH public key first")).toBeNull();
  fireEvent.click(dialog.getByRole("button", { name: "Open GitHub key settings" }));
  expect(dialog.getByRole("status").textContent).toContain("GitHub → Settings");
  fireEvent.click(dialog.getByRole("button", { name: "Retry setup" }));
  act(() => {
    vi.advanceTimersByTime(1800);
  });
  expect(dialog.getByRole("button", { name: "Copy SSH command" })).toBeTruthy();
  expect(dialog.getByText("~/Projects/website")).toBeTruthy();
  expect(dialog.queryByText("Give your workspace access to GitHub")).toBeNull();
});

test.each(["unconfirmed", "unknown"] as const)(
  "%s requires verification, not a readiness claim",
  (scenario) => {
    vi.useFakeTimers();
    show(scenario);
    expect(screen.queryByText("Ready")).toBeNull();
    const dialog = open("Review setup — Team website");
    expect(dialog.queryByRole("button", { name: "Copy SSH command" })).toBeNull();
    expect(dialog.queryByRole("button", { name: "Retry setup" })).toBeNull();
    fireEvent.click(dialog.getByRole("button", { name: "Check setup" }));
    expect(dialog.getByRole("status").textContent).toContain("Checking workspace setup");
    act(() => {
      vi.advanceTimersByTime(1800);
    });
    expect(dialog.getByRole("button", { name: "Copy SSH command" })).toBeTruthy();
  },
);

test("destructive confirmation is exact; partial result never offers cancellation or hides lost data", () => {
  show("ready");
  fireEvent.click(screen.getByRole("button", { name: "Actions — Team website" }));
  fireEvent.click(screen.getByRole("menuitem", { name: "Remove project" }));
  const dialog = within(screen.getByRole("dialog"));
  const remove = dialog.getByRole("button", { name: "Remove project" }) as HTMLButtonElement;
  expect(remove.disabled).toBe(true);
  expect(dialog.getByText(/Alice’s and Bob’s local workspaces/)).toBeTruthy();
  expect(dialog.getByText(/There is no undo/)).toBeTruthy();
  fireEvent.change(dialog.getByRole("textbox"), { target: { value: "WEBSITE" } });
  fireEvent.blur(dialog.getByRole("textbox"));
  expect(dialog.getByRole("alert").textContent).toContain("Enter website exactly");
  expect(remove.disabled).toBe(true);
  fireEvent.change(dialog.getByRole("textbox"), { target: { value: "website" } });
  fireEvent.click(remove);
  expect(dialog.getByText("Some local data was permanently deleted")).toBeTruthy();
  expect(dialog.queryByRole("button", { name: "Cancel" })).toBeNull();
  expect(screen.getAllByText("Project removal is incomplete")).toHaveLength(1);
  fireEvent.click(dialog.getByRole("button", { name: "Inspect remaining workspace" }));
  expect(dialog.getByRole("status").textContent).toContain(
    "Cockpit Accounts → soda-w-bob (Bob’s workspace)",
  );
  fireEvent.click(dialog.getAllByRole("button", { name: "Close" }).at(-1)!);
  expect(screen.getByText("Your workspace removed")).toBeTruthy();
  expect(screen.getByText("Team website")).toBeTruthy();
});
