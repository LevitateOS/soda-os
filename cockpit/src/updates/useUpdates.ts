import { useCallback, useEffect, useRef, useState } from "react";
import type { Host, NativeUpdates, Release, Selection } from "./types";

export function useUpdates(native: NativeUpdates) {
  const [host, setHost] = useState<Host | null>(null);
  const [release, setRelease] = useState<Release | null>(null);
  const [operation, setOperation] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [progress, setProgress] = useState("");
  const [confirmation, setConfirmation] = useState<Selection | null>(null);
  const active = useRef(false);
  const mounted = useRef(true);
  const readHost = useCallback(async () => {
    const current = await native.status();
    if (mounted.current) setHost(current);
  }, [native]);

  const run = useCallback(async (name: string, action: () => Promise<void>) => {
    if (active.current) return;
    active.current = true;
    setOperation(name);
    setError(null);
    setNotice(null);
    setProgress("");
    try {
      await action();
    } catch (failure) {
      if (mounted.current) setError(String(failure));
    } finally {
      active.current = false;
      if (mounted.current) setOperation(null);
    }
  }, []);
  const refresh = useCallback(
    () =>
      run("Reading deployment status", async () => {
        // Never retain stale deployment data when reconnect/status fails.
        setHost(null);
        await readHost();
      }),
    [run, readHost],
  );

  useEffect(() => {
    mounted.current = true;
    void refresh();
    const onFocus = () => {
      if (!active.current) void refresh();
    };
    window.addEventListener("focus", onFocus);
    return () => {
      mounted.current = false;
      window.removeEventListener("focus", onFocus);
    };
  }, [refresh]);

  const check = () =>
    run("Checking and verifying the latest release", async () => {
      setRelease(null);
      await readHost();
      try {
        const selected = await native.check();
        if (mounted.current) setRelease(selected);
      } catch (failure) {
        if (!String(failure).includes("no published stable Soda release is available"))
          throw failure;
        if (mounted.current)
          setNotice(
            "No published stable Soda release is available yet. Development candidates are not offered as updates.",
          );
      }
    });
  const onProgress = (chunk: string) => {
    if (mounted.current) setProgress((previous) => (previous + chunk).slice(-16384));
  };
  const download = () =>
    run("Verifying and downloading the selected image", async () => {
      if (!release) return;
      setHost(null);
      try {
        await native.download(release, onProgress);
      } catch (failure) {
        try {
          await readHost();
        } catch {
          /* Preserve the original download error. */
        }
        throw failure;
      }
      await readHost();
    });
  const apply = () =>
    run("Verifying, enabling the update, and requesting restart", async () => {
      if (!confirmation) return;
      const selected = confirmation;
      setConfirmation(null);
      setHost(null);
      try {
        await native.apply(selected, onProgress);
        if (mounted.current)
          setNotice("Restart requested. Reconnect and refresh to confirm the booted version.");
      } catch (failure) {
        try {
          await readHost();
        } catch {
          /* Connection loss is not proof of activation or failure. */
        }
        throw new Error(
          `${String(failure)} Reconnect and refresh native deployment status before retrying; do not assume the update failed.`,
        );
      }
    });
  return {
    host,
    release,
    operation,
    error,
    notice,
    progress,
    confirmation,
    setConfirmation,
    refresh,
    check,
    download,
    apply,
  };
}
