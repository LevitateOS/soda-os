// @vitest-environment jsdom
import { test, expect, vi } from "vite-plus/test";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { UpdatesPage } from "./UpdatesPage";
import type { Host, Release, NativeUpdates } from "../updates/types";

const reference = "ghcr.io/levitateos/soda-os@sha256:" + "a".repeat(64);
const release: Release = {
  version: "0.6.4",
  reference,
  revision: "b".repeat(40),
  architecture: "x86_64",
  notes_url: "https://github.com/LevitateOS/soda-os/releases/tag/v0.6.4",
};
function host(version = "0.6.3", pending = false): Host {
  const image = {
    version,
    imageDigest: reference.split("@")[1],
    architecture: "amd64",
    image: { image: reference, transport: "registry" },
  };
  return {
    apiVersion: "org.containers.bootc/v1",
    kind: "BootcHost",
    status: {
      booted: { image, downloadOnly: false, incompatible: false },
      staged: pending
        ? { image: { ...image, version: release.version }, downloadOnly: true, incompatible: false }
        : null,
      rollbackQueued: false,
      usrOverlay: null,
    },
  };
}
function setup(current = host()) {
  const native = {
    status: vi.fn<NativeUpdates["status"]>().mockResolvedValue(current),
    check: vi.fn<NativeUpdates["check"]>().mockResolvedValue(release),
    download: vi.fn<NativeUpdates["download"]>().mockResolvedValue(undefined),
    apply: vi.fn<NativeUpdates["apply"]>().mockResolvedValue(undefined),
  };
  render(<UpdatesPage native={native} />);
  return native;
}
async function ready() {
  await waitFor(() =>
    expect(
      (screen.getByRole("button", { name: "Check for updates" }) as HTMLButtonElement).disabled,
    ).toBe(false),
  );
}

test("check, verified download, then explicit restart confirmation", async () => {
  const native = setup();
  await ready();
  expect(native.check).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: "Check for updates" }));
  await screen.findByRole("button", { name: "Download update" });
  native.status.mockResolvedValue(host("0.6.3", true));
  fireEvent.click(screen.getByRole("button", { name: "Download update" }));
  await screen.findByText("Downloaded — not yet enabled for restart");
  expect(native.download).toHaveBeenCalledWith(release, expect.any(Function));
  expect(native.apply).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: "Apply and restart…" }));
  await screen.findByRole("dialog");
  expect(
    screen.getByText(/SSH sessions and running development workloads will be interrupted/),
  ).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: "Apply and restart" }));
  await waitFor(() =>
    expect(native.apply).toHaveBeenCalledWith(
      { version: release.version, reference },
      expect.any(Function),
    ),
  );
});

test("reload recovers the downloaded deployment without browser workflow state", async () => {
  setup(host("0.6.3", true));
  await screen.findByText("Downloaded — not yet enabled for restart");
  expect(screen.getByRole("button", { name: "Apply and restart…" })).toBeTruthy();
});

test("registry failure never displays up to date and clears a previous result", async () => {
  const native = setup();
  await ready();
  native.check.mockRejectedValue(new Error("registry unavailable"));
  fireEvent.click(screen.getByRole("button", { name: "Check for updates" }));
  await screen.findByText(/registry unavailable/);
  expect(screen.queryByText("Up to date.")).toBeNull();
  expect(screen.queryByRole("button", { name: "Download update" })).toBeNull();
});

test("no published release is informational, not up to date", async () => {
  const native = setup();
  await ready();
  native.check.mockRejectedValue(
    new Error("soda-updates: no published stable Soda release is available"),
  );
  fireEvent.click(screen.getByRole("button", { name: "Check for updates" }));
  await screen.findByText(/No published stable Soda release is available yet/);
  expect(screen.queryByText("Up to date.")).toBeNull();
  expect(screen.queryByRole("alert")).toBeNull();
});

test("a newer local version is not automatically downgraded", async () => {
  setup(host("0.7.0"));
  await ready();
  fireEvent.click(screen.getByRole("button", { name: "Check for updates" }));
  await screen.findByText(/No downgrade will be offered/);
  expect(screen.queryByRole("button", { name: "Download update" })).toBeNull();
});

test("an activation disconnect requires inspection rather than claiming success", async () => {
  const native = setup(host("0.6.3", true));
  await ready();
  native.apply.mockRejectedValue(new Error("connection closed"));
  fireEvent.click(screen.getByRole("button", { name: "Apply and restart…" }));
  fireEvent.click(await screen.findByRole("button", { name: "Apply and restart" }));
  await screen.findByText(/do not assume the update failed/);
  expect(screen.queryByText(/^Restart requested/)).toBeNull();
});

test("Apply dialog identifies the exact image and cancels without a native mutation", async () => {
  const native = setup(host("0.6.3", true));
  await ready();
  fireEvent.click(screen.getByRole("button", { name: "Apply and restart…" }));
  const dialog = await screen.findByRole("dialog", { name: "Apply update and restart?" });
  expect(within(dialog).getByText(reference).className).toBe("soda-code");
  expect(within(dialog).getByText(/atomic expected-digest activation guard/)).toBeTruthy();
  await waitFor(() => expect(dialog.contains(document.activeElement)).toBe(true));
  fireEvent.click(within(dialog).getByRole("button", { name: "Keep working" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(native.apply).not.toHaveBeenCalled();
  expect(native.download).not.toHaveBeenCalled();
});

test.each(["rollbackQueued", "usrOverlay"] as const)(
  "%s blocks download and Apply, not release checks",
  async (blocker) => {
    const current = host("0.6.3", true);
    if (blocker === "rollbackQueued") current.status.rollbackQueued = true;
    else current.status.usrOverlay = { persistence: "transient" };
    const native = setup(current);
    await ready();
    expect(screen.getByText(/Resolve the queued rollback or transient/)).toBeTruthy();
    const apply = screen.getByRole("button", { name: "Apply and restart…" }) as HTMLButtonElement;
    expect(apply.disabled).toBe(true);
    fireEvent.click(apply);
    expect(screen.queryByRole("dialog")).toBeNull();
    current.status.staged = null;
    fireEvent.click(screen.getByRole("button", { name: "Check for updates" }));
    const download = (await screen.findByRole("button", {
      name: "Download update",
    })) as HTMLButtonElement;
    await ready();
    expect(download.disabled).toBe(true);
    fireEvent.click(download);
    expect(native.check).toHaveBeenCalledOnce();
    expect(native.download).not.toHaveBeenCalled();
    expect(native.apply).not.toHaveBeenCalled();
  },
);

test("enabled deployments recover and incompatible pending images cannot be applied", async () => {
  const current = host("0.6.3", true);
  current.status.staged!.downloadOnly = false;
  current.status.staged!.incompatible = true;
  setup(current);
  await ready();
  expect(screen.getByText("Enabled for next restart")).toBeTruthy();
  expect(
    (screen.getByRole("button", { name: "Apply and restart…" }) as HTMLButtonElement).disabled,
  ).toBe(true);
});

test("native output stays bounded and wraps while a download is busy", async () => {
  const native = setup();
  await ready();
  fireEvent.click(screen.getByRole("button", { name: "Check for updates" }));
  await screen.findByRole("button", { name: "Download update" });
  let complete!: () => void;
  const output = "x".repeat(20000) + "\nlast native line";
  native.download.mockImplementation((_selection, progress) => {
    progress(output);
    return new Promise<void>((resolve) => {
      complete = resolve;
    });
  });
  fireEvent.click(screen.getByRole("button", { name: "Download update" }));
  const log = await screen.findByText(/last native line/);
  expect(log.textContent).toBe(output.slice(-16384));
  expect(log.className).toBe("soda-diagnostic");
  expect(screen.getByText("Soda OS 0.6.3")).toBeTruthy();
  expect(screen.queryByText(/Enable Cockpit administrative access/)).toBeNull();
  expect(
    screen
      .getByText("Native operation output (most recent 16 KiB)")
      .parentElement?.hasAttribute("open"),
  ).toBe(true);
  expect(
    screen.getByText("Verifying and downloading the selected image").getAttribute("role"),
  ).toBe("status");
  expect(
    (screen.getByRole("button", { name: "Refresh status" }) as HTMLButtonElement).disabled,
  ).toBe(true);
  complete();
  await ready();
  expect(native.apply).not.toHaveBeenCalled();
});

test("image identities and detailed failures use shared wrapping styles", async () => {
  const native = setup();
  await ready();
  const installed = within(screen.getByRole("region", { name: "Installed image" }));
  expect(installed.getByText(reference).className).toBe("soda-code");
  fireEvent.click(screen.getByRole("button", { name: "Check for updates" }));
  await screen.findByRole("button", { name: "Download update" });
  const available = within(screen.getByRole("region", { name: "Available release" }));
  expect(available.getByText(reference).className).toBe("soda-code");
  expect(available.getByRole("link", { name: "Release notes" }).getAttribute("href")).toBe(
    release.notes_url,
  );
  const diagnostic = `verification failed\n${reference}`;
  native.check.mockRejectedValue(new Error(diagnostic));
  fireEvent.click(screen.getByRole("button", { name: "Check for updates" }));
  const alert = await screen.findByRole("alert");
  expect(within(alert).getByText("Operation could not be confirmed")).toBeTruthy();
  const detail = within(alert).getByText(/verification failed/);
  expect(detail.textContent).toBe(`Error: ${diagnostic}`);
  expect(detail.className).toBe("soda-diagnostic");
});

test("a failed status read clears previous deployment facts without diagnosing permissions", async () => {
  const native = setup(host("0.6.3", true));
  await ready();
  native.status.mockRejectedValueOnce(new Error("bootc status unavailable"));
  fireEvent.click(screen.getByRole("button", { name: "Refresh status" }));
  await screen.findByText(/bootc status unavailable/);
  expect(screen.queryByText("Soda OS 0.6.3")).toBeNull();
  expect(screen.queryByRole("button", { name: "Apply and restart…" })).toBeNull();
  expect(
    screen.getByText("Installed image unavailable. Refresh status to try again."),
  ).toBeTruthy();
  expect(screen.queryByText(/Enable Cockpit administrative access/)).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "Refresh status" }));
  await screen.findByText("Soda OS 0.6.3");
  expect(screen.queryByText(/bootc status unavailable/)).toBeNull();
});

test("completed download is distinguished from failed deployment readback", async () => {
  const native = setup();
  await ready();
  fireEvent.click(screen.getByRole("button", { name: "Check for updates" }));
  await screen.findByRole("button", { name: "Download update" });
  native.status.mockRejectedValueOnce(new Error("readback unavailable"));
  fireEvent.click(screen.getByRole("button", { name: "Download update" }));
  await screen.findByText(
    /image download completed, but current deployment status could not be read/,
  );
  expect(screen.queryByRole("button", { name: "Apply and restart…" })).toBeNull();
  expect(native.apply).not.toHaveBeenCalled();
});
