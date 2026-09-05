import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import type { ProjectAction, Invoke, ListResponse, Project, WorkspaceInspection } from "./types";
import { errorMessage, payloadFor, successMessage } from "./ui";
type Dialog = { action: ProjectAction; project?: Project };
type Notice = { message: string; kind: "danger" | "success" };
export function useProjects(invoke: Invoke) {
  const [data, setData] = useState<ListResponse | null>(null);
  const [busy, setBusy] = useState(true);
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState<Notice | null>(null);
  const [dialog, setDialog] = useState<Dialog | null>(null);
  const [formError, setFormError] = useState<{ message: string; field?: string } | null>(null);
  const [readError, setReadError] = useState("");
  const [inspections, setInspections] = useState<Record<string, WorkspaceInspection>>({});
  // These are native read snapshots, discarded on refresh—not completion flags.
  const inspected = useCallback((id: string, inspection: WorkspaceInspection | null) => {
    if (!active.current) return;
    setInspections((previous) => {
      const next = { ...previous };
      if (inspection) next[id] = inspection;
      else delete next[id];
      return next;
    });
  }, []);
  const pending = useRef(false);
  const active = useRef(true);
  const load = useCallback(async () => {
    if (!active.current) return;
    setLoading(true);
    setInspections({});
    try {
      const result = await invoke("list", {});
      if (active.current) {
        setData(result);
        setReadError("");
      }
    } catch (error) {
      if (active.current) {
        setData(null);
        setReadError(errorMessage(error));
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
  function open(action: ProjectAction, project?: Project) {
    if (pending.current) return;
    setFormError(null);
    setDialog({ action, project });
  }
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (
      !dialog ||
      (dialog.action !== "add-existing" && dialog.action !== "edit") ||
      pending.current ||
      !event.currentTarget.reportValidity()
    )
      return;
    const { action } = dialog;
    const form = event.currentTarget;
    setFormError(null);
    const payload = payloadFor(action, new FormData(form), (message) => {
      setFormError({ message, field: "additional_metadata" });
    });
    if (!payload) return;
    pending.current = true;
    setBusy(true);
    setNotice(null);
    try {
      const result = await invoke(action, payload);
      if (!active.current) return;
      setDialog(null);
      const message = successMessage(action, result);
      await load();
      if (active.current) setNotice({ message, kind: "success" });
    } catch (error) {
      const message = errorMessage(error);
      await load();
      if (active.current) {
        setFormError({ message });
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
  const reportRemoval = useCallback((message: string, success: boolean) => {
    if (active.current) setNotice({ message, kind: success ? "success" : "danger" });
  }, []);
  return {
    data,
    inspections,
    inspected,
    busy,
    loading,
    notice,
    readError,
    dialog,
    formError,
    refresh,
    reportRemoval,
    open,
    close,
    submit,
  };
}
