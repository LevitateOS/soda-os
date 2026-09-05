// @vitest-environment jsdom
import { test, expect, vi } from "vite-plus/test";
import { render, screen, within, fireEvent, waitFor } from "@testing-library/react";
import { Runners } from "./App";
import type { Invoke, ListResponse } from "./types";
import { coordinator } from "./native";
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
  render(<Runners invoke={invoke as Invoke} hostname="soda.lan" />);
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
    .mockReturnValueOnce(registrationCall.process);
  render(<Runners invoke={coordinator({ spawn })} />);
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
  fireEvent.change(dialog.getByLabelText(/Provider registration token/), {
    target: { value: "second-secret" },
  });
  const input = dialog.getByLabelText(/Provider registration token/) as HTMLInputElement;
  fireEvent.click(dialog.getByRole("button", { name: "Cancel" }));
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
test("load failure shows administrator guidance and recovers through refresh", async () => {
  const invoke = vi
    .fn<Invoke>()
    .mockRejectedValueOnce(new Error("access denied"))
    .mockResolvedValue(data);
  render(<Runners invoke={invoke as Invoke} />);
  await screen.findByText("Local runner capacity is available only to Soda OS administrators.");
  fireEvent.click(screen.getByRole("button", { name: "Refresh" }));
  await screen.findByText("1 local runner; 1 listening; 1 configured slot.");
});
