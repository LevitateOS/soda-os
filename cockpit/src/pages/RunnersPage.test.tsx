// @vitest-environment jsdom
import { test, expect, vi } from "vite-plus/test";
import { act, render, screen, within, fireEvent, waitFor } from "@testing-library/react";
import { RunnersPage } from "./RunnersPage";
import type { Invoke, ListResponse } from "../runners/types";
import { coordinator } from "../runners/native";
import { pendingProcess } from "../../tests/process";
const data: ListResponse = {
  runner_count: 1,
  active_listeners: 1,
  total_capacity: 1,
  runners: [
    {
      id: "one",
      provider: "github",
      registration_url: "https://github.com/example/repo",
      account: "soda-runner-one",
      architecture: "x86_64",
      version: "2.337.0",
      capacity: 1,
      service: { load: "loaded", active: "active", sub: "running", enabled: "enabled" },
    },
  ],
};
async function ready() {
  const invoke = vi.fn<Invoke>().mockResolvedValue(data);
  render(<RunnersPage invoke={invoke as Invoke} hostname="soda.lan" />);
  await screen.findByText("1 local runner; 1 listening; 1 configured slot.");
  return invoke;
}
function registration() {
  fireEvent.click(screen.getByRole("button", { name: "Create local runner" }));
  const dialog = within(screen.getByRole("dialog"));
  fireEvent.change(dialog.getByLabelText(/Runner ID/), { target: { value: "new" } });
  fireEvent.click(dialog.getByLabelText("GitHub"));
  fireEvent.change(dialog.getByLabelText(/GitHub registration URL/), {
    target: { value: "https://github.com/example/repo" },
  });
  fireEvent.change(dialog.getByLabelText(/Provider registration token/), {
    target: { value: "provider-secret" },
  });
  return dialog;
}
test("provider forms expose native requirements and clear token before an operation completes", async () => {
  const invoke = await ready(),
    dialog = registration();
  expect((dialog.getByLabelText(/Forgejo runner UUID/) as HTMLInputElement).required).toBe(false);
  expect((dialog.getByLabelText(/Custom GitHub labels/) as HTMLInputElement).required).toBe(true);
  const operation = pendingProcess();
  let payload: unknown;
  invoke.mockImplementationOnce((_action, request) => {
    payload = request;
    return Promise.resolve(operation.process).then(() => ({ ok: true as const }));
  });
  fireEvent.click(dialog.getByRole("button", { name: "Register and start" }));
  expect((dialog.getByLabelText(/Provider registration token/) as HTMLInputElement).value).toBe("");
  expect(payload).toMatchObject({ registration_token: "" });
  expect(dialog.getByRole("button", { name: /Register and start/ }).hasAttribute("disabled")).toBe(
    true,
  );
  operation.reject(new Error("registration failed"));
  await waitFor(() => expect(dialog.getByText("registration failed")).toBeTruthy());
  expect((dialog.getByLabelText(/Provider registration token/) as HTMLInputElement).value).toBe("");
});
test("real adapter writes token only to stdin before React clears form and payload", async () => {
  const initial = pendingProcess();
  initial.resolve(JSON.stringify(data));
  const registrationCall = pendingProcess();
  const spawn = vi
    .fn()
    .mockReturnValueOnce(initial.process)
    .mockReturnValueOnce(registrationCall.process)
    .mockReturnValueOnce(initial.process);
  render(<RunnersPage invoke={coordinator({ spawn })} />);
  await screen.findByText("1 local runner; 1 listening; 1 configured slot.");
  const dialog = registration();
  fireEvent.click(dialog.getByRole("button", { name: "Register and start" }));
  expect(spawn).toHaveBeenLastCalledWith(["/usr/libexec/soda/soda-runners", "create"], {
    err: "message",
  });
  expect(registrationCall.process.input).toHaveBeenCalledWith(
    JSON.stringify({
      id: "new",
      provider: "github",
      registration_url: "https://github.com/example/repo",
      registration_id: "",
      labels: "soda-local",
      registration_token: "provider-secret",
    }) + "\n",
  );
  expect((dialog.getByLabelText(/Provider registration token/) as HTMLInputElement).value).toBe("");
  registrationCall.reject(new Error("denied"));
  await waitFor(() => expect(dialog.getByText("denied")).toBeTruthy());
});
test("synchronous registration failure and dialog cancellation clear secrets", async () => {
  const invoke = await ready();
  let dialog = registration();
  invoke.mockImplementationOnce(() => {
    throw new Error("spawn failed");
  });
  fireEvent.click(dialog.getByRole("button", { name: "Register and start" }));
  expect((dialog.getByLabelText(/Provider registration token/) as HTMLInputElement).value).toBe("");
  await waitFor(() => expect(dialog.getByText("spawn failed")).toBeTruthy());
  fireEvent.change(dialog.getByLabelText(/Provider registration token/), {
    target: { value: "second-secret" },
  });
  const input = dialog.getByLabelText(/Provider registration token/) as HTMLInputElement;
  fireEvent.click(dialog.getAllByRole("button", { name: "Close" }).at(-1)!);
  expect(input.value).toBe("");
  dialog = registration();
  fireEvent.click(dialog.getByLabelText("Bundled Forgejo"));
  expect((dialog.getByLabelText(/Forgejo runner UUID/) as HTMLInputElement).required).toBe(true);
  expect((dialog.getByLabelText(/Custom GitHub labels/) as HTMLInputElement).required).toBe(false);
  expect(
    dialog.getByRole("link", { name: "Forgejo runner administration" }).getAttribute("href"),
  ).toBe("http://soda.lan:30000/admin/actions/runners");
});
test("local status and provider guidance remain honest; lifecycle actions refresh", async () => {
  const invoke = await ready();
  expect(screen.getByText("Listening")).toBeTruthy();
  expect(screen.getByText("Runner jobs execute repository code")).toBeTruthy();
  expect(screen.getByRole("heading", { name: "Provider authority" })).toBeTruthy();
  invoke.mockResolvedValueOnce({ ok: true });
  fireEvent.click(screen.getByRole("button", { name: "Stop" }));
  await screen.findByText("one was stopped.");
  expect(invoke).toHaveBeenCalledWith("stop", { id: "one" });
  invoke.mockResolvedValueOnce({ ok: true });
  fireEvent.click(screen.getByRole("button", { name: "Restart" }));
  await screen.findByText("one was restarted.");
  expect(invoke).toHaveBeenCalledWith("restart", { id: "one" });
});
test("runner removal requires exact confirmation and preserves provider history guidance", async () => {
  const invoke = await ready();
  fireEvent.click(screen.getByRole("button", { name: "Remove" }));
  const dialog = within(screen.getByRole("dialog"));
  expect(dialog.getByText(/provider record and CI history remain/)).toBeTruthy();
  fireEvent.change(dialog.getByRole("textbox"), { target: { value: "wrong" } });
  fireEvent.click(dialog.getByRole("button", { name: "Remove local runner" }));
  expect(invoke).toHaveBeenCalledTimes(1);
  fireEvent.change(dialog.getByRole("textbox"), { target: { value: "one" } });
  invoke.mockResolvedValueOnce({ ok: true });
  invoke.mockResolvedValueOnce({
    runner_count: 0,
    active_listeners: 0,
    total_capacity: 0,
    runners: [],
  });
  fireEvent.click(dialog.getByRole("button", { name: "Remove local runner" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(screen.getByRole("heading", { name: "No local runners" })).toBeTruthy();
});
test("load failure reports unavailable status without inferring permissions and recovers through refresh", async () => {
  const invoke = vi
    .fn<Invoke>()
    .mockRejectedValueOnce(new Error("access denied"))
    .mockResolvedValue(data);
  render(<RunnersPage invoke={invoke as Invoke} />);
  await screen.findByText("Local runner status is unavailable. Refresh to try again.");
  expect(screen.getByText(/access denied/)).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "Refresh" }));
  await screen.findByText("1 local runner; 1 listening; 1 configured slot.");
  expect(screen.queryByText(/access denied/)).toBeNull();
});

test("invalid inactive provider fields neither block registration nor enter FormData", async () => {
  const invoke = await ready();
  const dialog = registration();
  fireEvent.change(dialog.getByLabelText(/GitHub registration URL/), {
    target: { value: "not a URL" },
  });
  fireEvent.click(dialog.getByLabelText("Bundled Forgejo"));
  fireEvent.change(dialog.getByLabelText(/Forgejo runner UUID/), {
    target: { value: "12345678-1234-1234-1234-123456789abc" },
  });
  const url = dialog.getByLabelText(/GitHub registration URL/) as HTMLInputElement;
  expect(url.disabled).toBe(true);
  expect(new FormData(url.form!).has("registration_url")).toBe(false);
  expect(url.form!.checkValidity()).toBe(true);
  invoke.mockResolvedValueOnce({ ok: true });
  fireEvent.click(dialog.getByRole("button", { name: "Register and start" }));
  await waitFor(() =>
    expect(invoke).toHaveBeenCalledWith(
      "create",
      expect.objectContaining({
        provider: "forgejo",
        registration_url: "",
        labels: "soda-linux:host",
      }),
    ),
  );
});

test("registration with a retained runner reconciles the list, closes creation, and keeps the failure", async () => {
  const invoke = await ready();
  const dialog = registration();
  const token = dialog.getByLabelText(/Provider registration token/) as HTMLInputElement;
  invoke.mockRejectedValueOnce(
    new Error("Runner registered and retained, but listener did not start."),
  );
  invoke.mockResolvedValueOnce({
    ...data,
    runners: [
      {
        ...data.runners[0],
        id: "new",
        service: { load: "loaded", active: "inactive", sub: "dead", enabled: "disabled" },
      },
    ],
  });
  fireEvent.click(dialog.getByRole("button", { name: "Register and start" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(token.value).toBe("");
  expect(screen.getByText("new")).toBeTruthy();
  expect(screen.getByRole("button", { name: "Start" })).toBeTruthy();
  expect(screen.getAllByText(/registered and retained/)).toHaveLength(1);
  fireEvent.click(screen.getByRole("button", { name: "Create local runner" }));
  const next = within(screen.getByRole("dialog"));
  expect(next.queryByText(/registered and retained/)).toBeNull();
  fireEvent.click(next.getByRole("button", { name: "Cancel" }));
  expect(screen.getByText(/registered and retained/)).toBeTruthy();
});

test("Stop names the in-flight operation and preserves successful mutation plus failed refresh", async () => {
  const invoke = await ready();
  let finish!: () => void;
  invoke.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        finish = () => resolve({ ok: true });
      }),
  );
  invoke.mockRejectedValueOnce(new Error("listener status unavailable"));
  fireEvent.click(screen.getByRole("button", { name: "Stop" }));
  expect(screen.getByText("Stopping one…").getAttribute("role")).toBe("status");
  expect((screen.getByRole("button", { name: "Stop" }) as HTMLButtonElement).disabled).toBe(true);
  finish();
  await screen.findByText("one was stopped.");
  expect(
    screen.getByText(/current runner list could not be refreshed.*listener status unavailable/),
  ).toBeTruthy();
  expect(screen.queryByText("Stopping one…")).toBeNull();
});

test("a failure arriving after leaving the page does not start another native read", async () => {
  const invoke = vi.fn<Invoke>().mockResolvedValue(data);
  const { unmount } = render(<RunnersPage invoke={invoke as Invoke} />);
  await screen.findByText("1 local runner; 1 listening; 1 configured slot.");
  let fail!: (error: Error) => void;
  invoke.mockImplementationOnce(
    () =>
      new Promise((_, reject) => {
        fail = reject;
      }),
  );
  fireEvent.click(screen.getByRole("button", { name: "Stop" }));
  unmount();
  await act(async () => {
    fail(new Error("connection closed"));
  });
  expect(invoke).toHaveBeenCalledTimes(2);
});

test("failed lifecycle changes still re-read native listener status", async () => {
  const invoke = await ready();
  invoke.mockRejectedValueOnce(new Error("Stop outcome unknown"));
  invoke.mockResolvedValueOnce({
    ...data,
    active_listeners: 0,
    runners: [
      {
        ...data.runners[0],
        service: { load: "loaded", active: "inactive", sub: "dead", enabled: "disabled" },
      },
    ],
  });
  fireEvent.click(screen.getByRole("button", { name: "Stop" }));
  await screen.findByText("Stop outcome unknown");
  expect(invoke).toHaveBeenLastCalledWith("list", {});
  expect(screen.getByRole("button", { name: "Start" })).toBeTruthy();
});
