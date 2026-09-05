import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import type { FormAction, Invoke, ListResponse, Project } from "./types";
import { errorMessage, payloadFor, successMessage } from "./ui";
type Dialog = { action: FormAction; project?: Project };
type Notice = { message: string; kind: "danger" | "success" };
export function useProjects(invoke: Invoke) {
  const [data, setData] = useState<ListResponse | null>(null);
  const [busy, setBusy] = useState(true);
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState<Notice | null>(null);
  const [dialog, setDialog] = useState<Dialog | null>(null);
  const [formError, setFormError] = useState("");
  const pending = useRef(false);
  const active = useRef(true);
  const load = useCallback(async () => {
    setLoading(true);
    try {
      const result = await invoke("list", {});
      if (active.current) setData(result);
    } catch (error) {
      if (active.current) {
        setData(null);
        setNotice({ message: errorMessage(error), kind: "danger" });
      }
    } finally {
      if (active.current) setLoading(false);
    }
  }, [invoke]);
  const refresh = useCallback(async () => {
    if (pending.current) return;
    pending.current = true;
    setBusy(true);
    try {
      await load();
    } finally {
      pending.current = false;
      if (active.current) setBusy(false);
    }
  }, [load]);
  useEffect(() => {
    active.current = true;
    void refresh();
    return () => {
      active.current = false;
    };
  }, [refresh]);
  function open(action: FormAction, project?: Project) {
    if (pending.current) return;
    setFormError("");
    setDialog({ action, project });
  }
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!dialog || pending.current || !event.currentTarget.reportValidity()) return;
    const { action } = dialog;
    const payload = payloadFor(action, new FormData(event.currentTarget), (message) =>
      setNotice({ message, kind: "danger" }),
    );
    if (!payload) return;
    pending.current = true;
    setBusy(true);
    try {
      const result = await invoke(action, payload);
      if (!active.current) return;
      setDialog(null);
      const message = successMessage(action, payload, result);
      await load();
      if (active.current) setNotice({ message, kind: "success" });
    } catch (error) {
      const message = errorMessage(error);
      if (action === "setup") await load();
      if (active.current) {
        setFormError(message);
        setNotice({ message, kind: "danger" });
      }
    } finally {
      pending.current = false;
      if (active.current) setBusy(false);
    }
  }
  function close() {
    if (!pending.current) setDialog(null);
  }
  return { data, busy, loading, notice, dialog, formError, refresh, open, close, submit };
}
