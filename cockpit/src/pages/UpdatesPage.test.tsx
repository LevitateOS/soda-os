// @vitest-environment jsdom
import { test, expect, vi } from "vite-plus/test";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
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
