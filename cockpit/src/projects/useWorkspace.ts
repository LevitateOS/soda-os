import { useCallback, useEffect, useRef, useState } from "react";
import type { Invoke, WorkspaceInspection } from "./types";
import { errorMessage, workspaceReady, workspaceCanSetup } from "./ui";

export function useWorkspace(
  invoke: Invoke,
  id: string,
  startSetup: boolean,
  onChanged: () => Promise<void>,
  onInspected: (id: string, inspection: WorkspaceInspection | null) => void,
) {
  const [inspection, setInspection] = useState<WorkspaceInspection | null>(null);
  const [operation, setOperation] = useState<"inspect" | "setup" | null>("inspect");
  const [readError, setReadError] = useState("");
  const [setupError, setSetupError] = useState("");
  // Receipt for this dialog's last command, never persisted or used as readiness.
  const [completed, setCompleted] = useState(false);
  const active = useRef(true);
  const pending = useRef(false);
  const read = useCallback(async () => {
    if (!active.current) return null;
    setInspection(null);
    onInspected(id, null);
    try {
      const result = await invoke("inspect", { id });
      if (!active.current) return null;
      setInspection(result.workspace);
      setReadError("");
      onInspected(id, result.workspace);
      if (workspaceReady(result.workspace)) setSetupError("");
      return result.workspace;
    } catch (error) {
      if (active.current) setReadError(errorMessage(error));
      return null;
    }
  }, [invoke, id, onInspected]);
  const run = useCallback(
    async (setupRequested: boolean) => {
      if (pending.current || !active.current) return;
      pending.current = true;
      setOperation("inspect");
      try {
        const current = await read();
        if (!setupRequested || !workspaceCanSetup(current) || !active.current) return;
        setOperation("setup");
        setSetupError("");
        setCompleted(false);
        try {
          await invoke("setup", { id });
          if (active.current) setCompleted(true);
        } catch (error) {
          if (active.current) setSetupError(errorMessage(error));
        }
        if (active.current) {
          setOperation("inspect");
          await onChanged();
          await read();
        }
      } finally {
        pending.current = false;
        if (active.current) setOperation(null);
      }
    },
    [invoke, id, onChanged, read],
  );
  useEffect(() => {
    active.current = true;
    void run(startSetup);
    return () => {
      active.current = false;
    };
  }, [run, startSetup]);
  return {
    inspection,
    operation,
    readError,
    setupError,
    completed,
    refresh: () => run(false),
    setup: () => run(true),
  };
}
