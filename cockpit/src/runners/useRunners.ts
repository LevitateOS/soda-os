import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import type { Invoke, LifecycleAction, ListResponse } from "./types";
import { createPayload, errorMessage, successMessage } from "./ui";
type Dialog = { kind: "create" } | { kind: "remove"; id: string };
export function useRunners(invoke: Invoke) {
  const [data, setData] = useState<ListResponse | null>(null);
  const [busy, setBusy] = useState(true),
    [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState<{ message: string; kind: "danger" | "success" } | null>(
    null,
  );
  const [dialog, setDialog] = useState<Dialog | null>(null);
  const pending = useRef(false),
    active = useRef(true);
  const load = useCallback(async () => {
    setLoading(true);
    try {
      const result = await invoke("list", {});
      if (active.current) setData(result);
      return result;
    } catch (error) {
      if (active.current) {
        setData(null);
        setNotice({ message: errorMessage(error), kind: "danger" });
      }
      return null;
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
  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending.current || !event.currentTarget.reportValidity()) return;
    const form = event.currentTarget;
    const token = form.elements.namedItem("registration_token") as HTMLInputElement;
    const payload = createPayload(new FormData(form));
    const id = payload.id;
    pending.current = true;
    setBusy(true);
    try {
      let operation;
      try {
        operation = invoke("create", payload);
      } finally {
        token.value = "";
        payload.registration_token = "";
      }
      await operation;
      if (!active.current) return;
      setDialog(null);
      await load();
      if (active.current) setNotice({ message: successMessage("create", id), kind: "success" });
    } catch (error) {
      if (active.current) setNotice({ message: errorMessage(error), kind: "danger" });
    } finally {
      pending.current = false;
      if (active.current) setBusy(false);
    }
  }
  async function mutate(action: LifecycleAction, id: string) {
    if (pending.current) return;
    pending.current = true;
    setBusy(true);
    try {
      await invoke(action, { id });
      if (!active.current) return;
      const updated = await load();
      if (!active.current) return;
      if (action === "remove" && updated?.runners.every((runner) => runner.id !== id))
        setDialog(null);
      setNotice({ message: successMessage(action, id), kind: "success" });
    } catch (error) {
      if (active.current) setNotice({ message: errorMessage(error), kind: "danger" });
    } finally {
      pending.current = false;
      if (active.current) setBusy(false);
    }
  }
  function remove(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending.current || dialog?.kind !== "remove") return;
    if (new FormData(event.currentTarget).get("confirmation") !== dialog.id) {
      setNotice({
        message: `Type ${dialog.id} exactly to confirm local runner removal.`,
        kind: "danger",
      });
      return;
    }
    void mutate("remove", dialog.id);
  }
  const close = () => {
    if (!pending.current) setDialog(null);
  };
  function openCreate() {
    if (!pending.current) setDialog({ kind: "create" });
  }
  function openRemove(id: string) {
    if (!pending.current) setDialog({ kind: "remove", id });
  }
  return {
    data,
    busy,
    loading,
    notice,
    dialog,
    refresh,
    create,
    mutate,
    remove,
    close,
    openCreate,
    openRemove,
  };
}
