import { useCallback, useEffect, useRef, useState } from "react";
import type { Cockpit } from "../cockpit/types";
import type { AuthenticationMessage, Snapshot, NativeTailscale } from "./types";
import {
  advertisesExitNode,
  authenticationURL,
  connectionState,
  deviceName,
  exitNodeChoices,
  exitSelection,
} from "./status";
const reload = () => window.location.reload();
const diagnostic = (error: unknown) => {
  if (
    error &&
    typeof error === "object" &&
    "message" in error &&
    typeof error.message === "string" &&
    error.message
  )
    return error.message;
  return String(error);
};

export interface TailscaleOptions {
  native: NativeTailscale;
  cockpit?: Pick<Cockpit, "hidden" | "addEventListener" | "removeEventListener">;
  onReopen?: () => void;
}
export function useTailscale({
  native,
  cockpit = window.cockpit,
  onReopen = reload,
}: TailscaleOptions) {
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null);
  const [busy, setBusy] = useState(false),
    [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");
  const [authURL, setAuthURL] = useState<string | null>(null),
    [streamState, setStreamState] = useState<string | undefined>();
  const [exitNode, setExitNode] = useState(""),
    [allowLAN, setAllowLAN] = useState(false),
    [advertise, setAdvertise] = useState(false);
  const closed = useRef(false),
    reading = useRef(false),
    pending = useRef(false);
  const dirty = useRef(new Set<"exit" | "advertise">()),
    refreshedIdentity = useRef("");
  const timer = useRef<ReturnType<typeof setTimeout>>();
  const showAuthentication = useCallback((message: AuthenticationMessage) => {
    if (closed.current) return;
    setAuthURL(authenticationURL(message.AuthURL));
    if (message.BackendState) setStreamState(connectionState(message));
  }, []);
  const load = useCallback(async () => {
    if (closed.current || reading.current) return;
    reading.current = true;
    try {
      const next = await native.read();
      if (closed.current) return;
      setSnapshot(next);
      setStreamState(undefined);
      setAuthURL(
        authenticationURL(next.status.BackendState === "NeedsLogin" ? next.status.AuthURL : ""),
      );
      if (!dirty.current.has("exit")) {
        setExitNode(exitSelection(next));
        setAllowLAN(Boolean(next.prefs.ExitNodeAllowLANAccess));
      }
      if (!dirty.current.has("advertise")) setAdvertise(advertisesExitNode(next.prefs));
      const connected = next.status.BackendState === "Running" && !next.status.Self?.Expired;
      if (!connected) {
        refreshedIdentity.current = "";
        return;
      }
      const identity = `${next.status.Self?.DNSName || ""}/${(next.status.TailscaleIPs || []).join(",")}`;
      if (identity === refreshedIdentity.current) return;
      refreshedIdentity.current = identity;
      try {
        await native.refreshForgejo();
      } catch (error) {
        if (!closed.current)
          setNotice(
            `Tailscale connected, but Forgejo could not refresh its Tailnet address: ${diagnostic(error)}`,
          );
      }
    } catch (error) {
      if (!closed.current) {
        setSnapshot(null);
        setAuthURL(null);
        setStreamState(undefined);
        setNotice(diagnostic(error));
      }
    } finally {
      reading.current = false;
      if (!closed.current) setLoading(false);
    }
  }, [native]);
  useEffect(() => {
    closed.current = false;
    async function observe() {
      await load();
      if (!closed.current) timer.current = setTimeout(() => void observe(), 3000);
    }
    function close() {
      closed.current = true;
      clearTimeout(timer.current);
      native.close();
    }
    function visibility() {
      if (cockpit.hidden) close();
      else if (closed.current) onReopen();
    }
    window.addEventListener("pagehide", close);
    cockpit.addEventListener("visibilitychange", visibility);
    if (cockpit.hidden) close();
    else void observe();
    return () => {
      close();
      window.removeEventListener("pagehide", close);
      cockpit.removeEventListener("visibilitychange", visibility);
    };
  }, [cockpit, load, native, onReopen]);
  async function mutate(operation: () => Promise<void>, form?: "exit" | "advertise") {
    if (pending.current || closed.current) return;
    pending.current = true;
    setBusy(true);
    setNotice("");
    try {
      await operation();
      if (form) dirty.current.delete(form);
    } catch (error) {
      if (!closed.current) setNotice(diagnostic(error));
    } finally {
      pending.current = false;
      if (!closed.current) setBusy(false);
      await load();
    }
  }
  const status = snapshot?.status,
    prefs = snapshot?.prefs;
  const connected = status?.BackendState === "Running" && !status.Self?.Expired;
  const peers = Object.values(status?.Peer || {}).sort((a, b) =>
    deviceName(a).localeCompare(deviceName(b)),
  );
  const choices = status ? exitNodeChoices(status).filter((peer) => peer.TailscaleIPs?.[0]) : [];
  const selected = snapshot ? exitSelection(snapshot) : "";
  const missingSelection = Boolean(
    prefs?.ExitNodeID && selected && !choices.some((peer) => peer.TailscaleIPs?.[0] === selected),
  );

  function changeExitNode(value: string) {
    dirty.current.add("exit");
    setExitNode(value);
  }
  function changeAllowLAN(value: boolean) {
    dirty.current.add("exit");
    setAllowLAN(value);
  }
  function changeAdvertise(value: boolean) {
    dirty.current.add("advertise");
    setAdvertise(value);
  }
  function signIn() {
    if (status) void mutate(() => native.signIn(status, showAuthentication));
  }
  function applyExitNode() {
    void mutate(() => native.selectExitNode(exitNode, allowLAN), "exit");
  }
  function applyAdvertisement() {
    void mutate(() => native.advertiseExitNode(advertise), "advertise");
  }
  return {
    snapshot,
    busy,
    loading,
    notice,
    authURL,
    streamState,
    connected,
    peers,
    choices,
    selected,
    missingSelection,
    exitNode,
    allowLAN,
    advertise,
    changeExitNode,
    changeAllowLAN,
    changeAdvertise,
    signIn,
    applyExitNode,
    applyAdvertisement,
  };
}
