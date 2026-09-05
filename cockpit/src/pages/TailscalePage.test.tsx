// @vitest-environment jsdom
import { afterEach, test, expect, vi } from "vite-plus/test";
import { render, screen, fireEvent, act, within } from "@testing-library/react";
import { TailscalePage } from "./TailscalePage";
import type { Snapshot, AuthenticationMessage } from "../tailscale/types";
import type { NativeTailscale } from "../tailscale/types";
const connected: Snapshot = {
  status: {
    BackendState: "Running",
    Self: { DNSName: "soda.tail.test.", InNetworkMap: true },
    TailscaleIPs: ["100.64.0.1"],
    Peer: {
      exit: {
        ID: "exit",
        DNSName: "exit.tail.test.",
        TailscaleIPs: ["100.64.0.2"],
        ExitNodeOption: true,
        Online: true,
      },
    },
  },
  prefs: {},
};
function setup(snapshot = connected) {
  vi.useFakeTimers();
  let visibility = () => {};
  const cockpit = {
    hidden: false,
    addEventListener: vi.fn((_event: string, callback: () => void) => {
      visibility = callback;
    }),
    removeEventListener: vi.fn(),
  };
  const native = {
    read: vi.fn(async () => snapshot),
    readStatus: vi.fn(async () => snapshot.status),
    signIn: vi.fn<NativeTailscale["signIn"]>().mockResolvedValue(undefined),
    selectExitNode: vi.fn().mockResolvedValue(undefined),
    advertiseExitNode: vi.fn().mockResolvedValue(undefined),
    refreshForgejo: vi.fn().mockResolvedValue(undefined),
    close: vi.fn(),
  };
  const onReopen = vi.fn();
  const rendered = render(<TailscalePage native={native} cockpit={cockpit} onReopen={onReopen} />);
  return { native, cockpit, visibility: () => visibility(), onReopen, ...rendered };
}
const flush = async () => {
  await act(async () => {
    await Promise.resolve();
  });
};
afterEach(() => vi.useRealTimers());
test("opening connected page refreshes Forgejo once and polling preserves native-setting edits", async () => {
  const { native } = setup();
  await flush();
  expect(screen.getByText("Connected")).toBeTruthy();
  expect(native.refreshForgejo).toHaveBeenCalledTimes(1);
  fireEvent.change(screen.getByLabelText("Exit node"), { target: { value: "100.64.0.2" } });
  fireEvent.click(screen.getByLabelText("Advertise as an exit node"));
  await act(async () => {
    await vi.advanceTimersByTimeAsync(9000);
  });
  expect(native.refreshForgejo).toHaveBeenCalledTimes(1);
  expect((screen.getByLabelText("Exit node") as HTMLSelectElement).value).toBe("100.64.0.2");
  expect((screen.getByLabelText("Advertise as an exit node") as HTMLInputElement).checked).toBe(
    true,
  );
  fireEvent.click(
    within(screen.getByRole("region", { name: "Use an exit node" })).getByRole("button", {
      name: "Apply",
    }),
  );
  await flush();
  expect(native.selectExitNode).toHaveBeenCalledWith("100.64.0.2", false);
});
test("Cockpit navigation closes hidden-page work, reopens native state, and removes listeners", async () => {
  const app = setup();
  await flush();
  app.cockpit.hidden = true;
  act(app.visibility);
  const reads = app.native.read.mock.calls.length;
  await act(async () => {
    await vi.advanceTimersByTimeAsync(9000);
  });
  expect(app.native.read).toHaveBeenCalledTimes(reads);
  expect(app.native.close).toHaveBeenCalled();
  app.cockpit.hidden = false;
  act(app.visibility);
  expect(app.onReopen).toHaveBeenCalledOnce();
  app.unmount();
  expect(app.cockpit.removeEventListener).toHaveBeenCalledWith(
    "visibilitychange",
    expect.any(Function),
  );
});
test("leaving during streaming sign-in cancels page work without retaining authentication state", async () => {
  const app = setup({ status: { BackendState: "NeedsLogin" }, prefs: {} });
  await flush();
  let emit!: (message: AuthenticationMessage) => void;
  let finish!: () => void;
  app.native.signIn.mockImplementationOnce((_status, callback) => {
    emit = callback;
    return new Promise((resolve) => {
      finish = resolve;
    });
  });
  fireEvent.click(screen.getByRole("button", { name: "Sign in" }));
  act(() => emit({ AuthURL: "https://login.tailscale.com/a/test" }));
  expect(screen.getByRole("link", { name: "https://login.tailscale.com/a/test" })).toBeTruthy();
  act(() => {
    window.dispatchEvent(new Event("pagehide"));
  });
  expect(app.native.close).toHaveBeenCalled();
  await act(async () => {
    finish();
  });
  expect(app.native.read).toHaveBeenCalledTimes(1);
});
test("reopening pending authentication recovers native URL and later connection without completion state", async () => {
  const app = setup({
    status: {
      BackendState: "NeedsLogin",
      HaveNodeKey: true,
      AuthURL: "https://login.tailscale.com/a/pending",
    },
    prefs: {},
  });
  await flush();
  expect(screen.getByRole("link", { name: "https://login.tailscale.com/a/pending" })).toBeTruthy();
  app.native.read.mockResolvedValue(connected);
  await act(async () => {
    await vi.advanceTimersByTimeAsync(3000);
  });
  expect(screen.getByText("Connected")).toBeTruthy();
  expect(screen.queryByRole("link", { name: /login.tailscale.com\/a\// })).toBeNull();
});
test("Forgejo refresh failure keeps enrollment connected and is not retried by polling", async () => {
  const app = setup();
  app.native.refreshForgejo.mockRejectedValue({ message: "service failed" });
  await flush();
  expect(screen.getByText("Connected")).toBeTruthy();
  expect(
    screen.getByText(/Tailscale connected, but Forgejo could not refresh.*service failed/),
  ).toBeTruthy();
  await act(async () => {
    await vi.advanceTimersByTimeAsync(9000);
  });
  expect(app.native.refreshForgejo).toHaveBeenCalledOnce();
});
test("native failure clears stale device facts and disables controls; polling can recover", async () => {
  const app = setup();
  await flush();
  app.native.read.mockRejectedValueOnce({ message: "daemon unavailable" });
  await act(async () => {
    await vi.advanceTimersByTimeAsync(3000);
  });
  expect(screen.getByText("Tailscale state unavailable")).toBeTruthy();
  expect(screen.getByText("daemon unavailable")).toBeTruthy();
  expect(screen.getByText("Device list unavailable.")).toBeTruthy();
  expect((screen.getByLabelText("Exit node") as HTMLSelectElement).disabled).toBe(true);
  await act(async () => {
    await vi.advanceTimersByTimeAsync(3000);
  });
  expect(screen.getByText("Connected")).toBeTruthy();
});
test("an unavailable selected exit node remains visible and approval uses native state", async () => {
  setup({ ...connected, prefs: { ExitNodeID: "missing", AdvertiseRoutes: ["0.0.0.0/0", "::/0"] } });
  await flush();
  expect((screen.getByLabelText("Exit node") as HTMLSelectElement).value).toBe("unavailable");
  expect(
    screen.getByRole("option", { name: "Selected exit node unavailable" }).hasAttribute("disabled"),
  ).toBe(true);
  expect(screen.getByText("Waiting for Tailnet administrator approval")).toBeTruthy();
});
test("pending reads do not overlap and hidden initial pages perform no reads", async () => {
  const app = setup();
  await flush();
  let finish!: (value: Snapshot) => void;
  app.native.read.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
  );
  await act(async () => {
    await vi.advanceTimersByTimeAsync(15000);
  });
  expect(app.native.read).toHaveBeenCalledTimes(2);
  await act(async () => {
    finish(connected);
  });
  app.unmount();
  app.cockpit.hidden = true;
  render(<TailscalePage native={app.native} cockpit={app.cockpit} />);
  await flush();
  expect(app.native.read).toHaveBeenCalledTimes(2);
});
