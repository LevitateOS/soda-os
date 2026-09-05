import { useCallback, useEffect, useRef, useState } from "react";
import type {
  Invoke,
  RemovalAction,
  RemovalPreview,
  RemovalResponse,
  RemovalAccount,
} from "./types";
import { errorMessage, removalAccountLabel, removalCatalogMessage } from "./ui";

export function useRemoval(
  invoke: Invoke,
  action: RemovalAction,
  initialTarget: string,
  onChanged: () => Promise<void>,
  onOutcome: (message: string, success: boolean) => void,
) {
  const [preview, setPreview] = useState<RemovalPreview | null>(null);
  const [receipt, setReceipt] = useState<RemovalResponse | null>(null);
  const [selectedAccounts, setSelectedAccounts] = useState<RemovalAccount[]>([]);
  const [readError, setReadError] = useState("");
  const [unknown, setUnknown] = useState("");
  const [operation, setOperation] = useState<"inspect" | "remove" | null>(
    initialTarget ? "inspect" : null,
  );
  const active = useRef(true);
  const pending = useRef(false);
  const read = useCallback(
    async (target: string) => {
      if (!active.current) return;
      setPreview(null);
      try {
        const response = await invoke("removal-inspect", { action, target });
        if (response.preview.action !== action || response.preview.target !== target)
          throw new Error("Removal inspection does not match the requested target.");
        if (active.current) {
          setPreview(response.preview);
          setReadError("");
        }
      } catch (error) {
        if (active.current) setReadError(errorMessage(error));
      }
    },
    [invoke, action],
  );
  const inspect = useCallback(
    async (target: string) => {
      if (pending.current || !active.current || !target) return;
      pending.current = true;
      setOperation("inspect");
      try {
        await read(target);
      } finally {
        pending.current = false;
        if (active.current) setOperation(null);
      }
    },
    [read],
  );
  useEffect(() => {
    active.current = true;
    if (initialTarget) void inspect(initialTarget);
    return () => {
      active.current = false;
    };
  }, [inspect, initialTarget]);
  async function remove() {
    if (!preview || pending.current || !active.current) return;
    pending.current = true;
    setOperation("remove");
    try {
      const response =
        action === "delete-human"
          ? await invoke(action, { username: preview.target, expected: preview.revision })
          : await invoke(action, { id: preview.target, expected: preview.revision });
      if (!active.current) return;
      setReceipt(response);
      setSelectedAccounts(preview.accounts);
      setUnknown("");
      const uncertain = response.result.uncertain
        ? `Deletion of ${removalAccountLabel(response.result.uncertain, preview.accounts)} is uncertain; some local data may already be deleted. `
        : "";
      const remaining = response.result.not_attempted
        .map((name) => removalAccountLabel(name, preview.accounts))
        .join(", ");
      const detail = `${response.result.removed.length} account removal(s) confirmed. ${uncertain}${remaining ? `Not attempted: ${remaining}. ` : ""}${action === "remove" ? removalCatalogMessage(response.catalog) + " " : ""}${response.problem ? response.problem + " " : ""}Git-host accounts and repositories were not deleted.`;
      onOutcome(
        `${response.ok ? "Removal completed." : "Removal is incomplete."} ${detail}`,
        response.ok,
      );
      setPreview(null);
      await onChanged();
      if (!response.ok) {
        if (active.current) setOperation("inspect");
        await read(preview.target);
      }
    } catch (error) {
      if (active.current) {
        const message = errorMessage(error);
        setUnknown(message);
        setPreview(null);
        onOutcome(
          "Removal outcome is not confirmed. Some local data may already have been permanently deleted. Check current state before retrying. " +
            message,
          false,
        );
        await onChanged();
      }
    } finally {
      pending.current = false;
      if (active.current) setOperation(null);
    }
  }
  return {
    preview,
    receipt,
    selectedAccounts,
    readError,
    unknown,
    operation,
    inspect,
    remove,
    invalidate: () => setPreview(null),
  };
}
