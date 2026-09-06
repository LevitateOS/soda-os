// @vitest-environment jsdom
import { test, expect } from "vite-plus/test";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { UpdatesPage } from "./UpdatesPage";
import { createUpdatesStore } from "../updates/store";
import { hostFixture, nativeFixture, source, bootedDigest, nextDigest } from "../../tests/updates";

function setup(host = hostFixture()) {
  const native = nativeFixture(host);
  render(<UpdatesPage store={createUpdatesStore(native)} />);
  return native;
}
async function ready() {
  await waitFor(() =>
    expect(
      (screen.getByRole("button", { name: "Refresh status" }) as HTMLButtonElement).disabled,
    ).toBe(false),
  );
}
function updateButton() {
  return screen.getByRole("button", {
    name: "Update and restart",
  }) as HTMLButtonElement;
}

test("one explicit action works without Check and shows tracked source and actual booted digest", async () => {
  const native = setup();
  await ready();
  expect(screen.getByRole("region", { name: "Tracked image source" }).textContent).toContain(
    source,
  );
  const installed = within(screen.getByRole("region", { name: "Installed image" }));
  expect(installed.getByText(bootedDigest).className).toContain("soda-code");
  expect(screen.getByText(/interrupting SSH sessions/)).toBeTruthy();
  expect(updateButton().disabled).toBe(false);
  fireEvent.click(updateButton());
  await ready();
  expect(native.update).toHaveBeenCalledExactlyOnceWith(expect.any(Function));
  expect(native.check).not.toHaveBeenCalled();
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(screen.queryByRole("button", { name: /Download|Apply/ })).toBeNull();
  expect(screen.getByText(/command completion alone is not boot proof/)).toBeTruthy();
});

test("same-version cached image is informational and stale Check does not select Update's target", async () => {
  const native = setup();
  await ready();
  const checked = hostFixture();
  checked.status.booted.cachedUpdate = { ...checked.status.booted.image!, imageDigest: nextDigest };
  native.check.mockResolvedValue(checked);
  fireEvent.click(screen.getByRole("button", { name: "Check for updates" }));
  await screen.findByText("Native cached update: 0.6.3");
  expect(updateButton().disabled).toBe(false);
  const booted = hostFixture();
  booted.status.booted.image!.imageDigest = "sha256:" + "c".repeat(64);
  native.status.mockResolvedValue(booted);
  fireEvent.click(updateButton());
  await ready();
  expect(native.update).toHaveBeenCalledExactlyOnceWith(expect.any(Function));
  expect(
    within(screen.getByRole("region", { name: "Installed image" })).getByText(
      booted.status.booted.image!.imageDigest,
    ),
  ).toBeTruthy();
});

test.each([true, false])(
  "native pending downloadOnly=%s is visible without blocking Update",
  async (downloadOnly) => {
    const host = hostFixture();
    host.status.staged = {
      ...host.status.booted,
      downloadOnly,
      image: { ...host.status.booted.image!, imageDigest: nextDigest },
    };
    setup(host);
    await ready();
    const pending = within(screen.getByRole("region", { name: "Pending deployment" }));
    expect(pending.getByText(nextDigest)).toBeTruthy();
    expect(pending.queryByRole("button")).toBeNull();
    expect(updateButton().disabled).toBe(false);
    expect(
      within(screen.getByRole("region", { name: "Installed image" })).getByText(bootedDigest),
    ).toBeTruthy();
  },
);

test.each([
  ["rollback", "A rollback is queued"],
  ["overlay", "A /usr overlay is active"],
  ["booted", "bootc cannot manage the current deployment"],
  ["staged", "bootc cannot manage the staged deployment"],
  ["read-only", "The bootc system is read-only"],
  ["source", "No native image source is configured"],
])("%s diagnostics block Update but not a native metadata check", async (kind, message) => {
  const host = hostFixture();
  switch (kind) {
    case "rollback":
      host.status.rollbackQueued = true;
      break;
    case "overlay":
      host.status.usrOverlay = { persistence: "transient", accessMode: "readWrite" };
      break;
    case "booted":
      host.status.booted.incompatible = true;
      break;
    case "staged":
      host.status.staged = { ...host.status.booted, incompatible: true };
      break;
    case "read-only":
      host.status.readOnly = true;
      break;
    case "source":
      host.spec.image = null;
      break;
  }
  const native = setup(host);
  await ready();
  expect(screen.getByText(new RegExp(message))).toBeTruthy();
  expect(updateButton().disabled).toBe(true);
  fireEvent.click(updateButton());
  expect(native.update).not.toHaveBeenCalled();
  expect(
    (screen.getByRole("button", { name: "Check for updates" }) as HTMLButtonElement).disabled,
  ).toBe(false);
});

test("administrator sources and unknown versions are not replaced with Soda policy", async () => {
  const host = hostFixture();
  host.spec.image = { image: "example.test/custom:branch", transport: "registry" };
  host.status.booted.image!.version = null;
  setup(host);
  await ready();
  expect(screen.getByText("example.test/custom:branch")).toBeTruthy();
  expect(screen.getByText("Version: Unknown version")).toBeTruthy();
  expect(updateButton().disabled).toBe(false);
});

test("digest-pinned source guidance does not silently switch sources", async () => {
  const host = hostFixture();
  host.spec.image = { image: "example.test/os@" + bootedDigest, transport: "registry" };
  const native = setup(host);
  await ready();
  expect(screen.getByText(/administrator explicitly selects that tag/)).toBeTruthy();
  expect(native.update).not.toHaveBeenCalled();
  expect(updateButton().disabled).toBe(false);
});

test("disconnect is not boot proof and focus rereads actual status without retrying", async () => {
  const native = setup();
  await ready();
  native.update.mockRejectedValueOnce(new Error("connection closed"));
  native.status.mockRejectedValueOnce(new Error("host unavailable"));
  fireEvent.click(updateButton());
  await screen.findByText(/proves neither success nor failure/);
  expect(updateButton().disabled).toBe(true);
  const booted = hostFixture();
  booted.status.booted.image!.imageDigest = nextDigest;
  native.status.mockResolvedValue(booted);
  fireEvent(window, new Event("focus"));
  await screen.findByText(nextDigest);
  expect(screen.getByText(/connection closed/)).toBeTruthy();
  expect(native.update).toHaveBeenCalledOnce();
});

test("native text is bounded output, not a source of success or eligibility", async () => {
  const native = setup();
  await ready();
  let finish!: () => void;
  native.update.mockImplementationOnce((progress) => {
    progress("x".repeat(20000) + " Update successful!");
    return new Promise((resolve) => {
      finish = resolve;
    });
  });
  fireEvent.click(updateButton());
  expect(updateButton().disabled).toBe(true);
  const output = screen.getByText(/Update successful!/);
  expect(output.textContent).toHaveLength(16384);
  expect(output.className).toBe("soda-diagnostic");
  expect(
    screen.getByText("Updating the native image and restarting when needed").getAttribute("role"),
  ).toBe("status");
  expect(screen.queryByText(/Native update command completed/)).toBeNull();
  finish();
  await ready();
});

test("a failed Check is never reported as up to date", async () => {
  const native = setup();
  await ready();
  native.check.mockRejectedValueOnce(new Error("registry unreachable"));
  fireEvent.click(screen.getByRole("button", { name: "Check for updates" }));
  await screen.findByText(/registry unreachable/);
  expect(screen.queryByText("Up to date.")).toBeNull();
  expect(updateButton().disabled).toBe(true);
  fireEvent(window, new Event("focus"));
  await ready();
  expect(screen.getByText(/registry unreachable/)).toBeTruthy();
});
