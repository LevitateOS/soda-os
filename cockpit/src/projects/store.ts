import { createStore } from "zustand/vanilla";
import type {
  Invoke,
  ListResponse,
  Project,
  ProjectAction,
  ProjectsTask,
  WorkspaceInspection,
} from "./types";
import {
  errorMessage,
  payloadFor,
  successMessage,
  workspaceReady,
  workspaceCanSetup,
  removalReview,
  removalAccountLabel,
  removalCatalogMessage,
} from "./ui";

interface State {
  data: ListResponse | null;
  inspections: Record<string, WorkspaceInspection>;
  loading: boolean;
  operation: "list" | "catalog" | "inspect" | "setup" | "remove" | null;
  task: ProjectsTask | null;
  notice: { message: string; kind: "success" | "danger" } | null;
  readError: string;
}
const initial: State = {
  data: null,
  inspections: {},
  loading: true,
  operation: null,
  task: null,
  notice: null,
  readError: "",
};
export function createProjectsStore(invoke: Invoke) {
  let active = false,
    generation = 0;
  const current = (version: number) => active && version === generation;
  const store = createStore(() => ({
    ...initial,
    start: () => {
      active = true;
      const version = ++generation;
      store.setState({ ...initial, inspections: {} });
      void store.getState().refresh();
      return () => {
        if (version === generation) {
          active = false;
          generation++;
        }
      };
    },
    refresh: async () => {
      if (!active || store.getState().operation) return;
      const version = generation;
      store.setState({ operation: "list" });
      await loadCatalog(version);
      if (current(version)) store.setState({ operation: null });
    },
    open: (action: ProjectAction, project?: Project) => {
      if (!active || store.getState().operation) return;
      if (action === "add-existing" || action === "edit") {
        store.setState({ task: { kind: "catalog", action, project, error: null } });
      } else if (action === "setup" || action === "inspect") {
        if (!project) return;
        store.setState({
          task: { kind: "workspace", project, readError: "", setupError: "", completed: false },
        });
        // Setup starts from this explicit user intent, never from mounting a view.
        void runWorkspace(action === "setup");
      } else {
        const target = project?.id ?? "";
        store.setState({
          task: {
            kind: "removal",
            action,
            target,
            confirmation: "",
            reviewing: false,
            preview: null,
            receipt: null,
            selectedAccounts: [],
            readError: "",
            unknown: "",
          },
        });
        if (target) void store.getState().checkRemoval();
      }
    },
    close: () => {
      if (!store.getState().operation) store.setState({ task: null });
    },
    submitCatalog: async (values: Record<string, string>) => {
      const task = store.getState().task;
      if (!active || store.getState().operation || task?.kind !== "catalog") return;
      const payload = payloadFor(task.action, { get: (name) => values[name] }, (message) => {
        store.setState({ task: { ...task, error: { message, field: "additional_metadata" } } });
      });
      if (!payload) return;
      const version = generation;
      store.setState({ operation: "catalog", notice: null, task: { ...task, error: null } });
      try {
        const result = await invoke(task.action, payload);
        if (!current(version)) return;
        store.setState({ task: null });
        await loadCatalog(version);
        if (current(version))
          store.setState({
            notice: { message: successMessage(task.action, result), kind: "success" },
          });
      } catch (error) {
        await loadCatalog(version);
        if (current(version))
          store.setState({
            task: { ...task, error: { message: errorMessage(error) } },
            notice: { message: errorMessage(error), kind: "danger" },
          });
      } finally {
        if (current(version)) store.setState({ operation: null });
      }
    },
    checkWorkspace: () => runWorkspace(false),
    setupWorkspace: () => runWorkspace(true),
    changeRemovalTarget: (target: string) => {
      const task = store.getState().task;
      if (
        !active ||
        store.getState().operation ||
        task?.kind !== "removal" ||
        task.receipt ||
        task.unknown
      )
        return;
      store.setState({ task: { ...task, target, preview: null, confirmation: "" } });
    },
    changeConfirmation: (confirmation: string) => {
      const task = store.getState().task;
      if (!active || store.getState().operation || task?.kind !== "removal") return;
      store.setState({ task: { ...task, confirmation } });
    },
    checkRemoval: async () => {
      const task = store.getState().task;
      if (
        !active ||
        store.getState().operation ||
        task?.kind !== "removal" ||
        !task.target ||
        task.receipt?.ok
      )
        return;
      const version = generation;
      store.setState({
        operation: "inspect",
        task: { ...task, reviewing: true, confirmation: "" },
      });
      await readRemoval(version);
      if (current(version)) store.setState({ operation: null });
    },
    remove: async () => {
      const task = store.getState().task;
      if (!active || store.getState().operation || task?.kind !== "removal" || !task.preview)
        return;
      const review = removalReview(task);
      if (!review.canConfirm || !review.matches) return;
      const version = generation;
      const preview = task.preview;
      store.setState({
        operation: "remove",
        task: { ...task, confirmation: "", reviewing: false },
      });
      try {
        const receipt =
          task.action === "delete-human"
            ? await invoke(task.action, { username: preview.target, expected: preview.revision })
            : await invoke(task.action, { id: preview.target, expected: preview.revision });
        if (!current(version)) return;
        const uncertain = receipt.result.uncertain
          ? `Deletion of ${removalAccountLabel(receipt.result.uncertain, preview.accounts)} is uncertain; some local data may already be deleted. `
          : "";
        const remaining = receipt.result.not_attempted
          .map((name) => removalAccountLabel(name, preview.accounts))
          .join(", ");
        const detail = `${receipt.result.removed.length} account removal(s) confirmed. ${uncertain}${remaining ? `Not attempted: ${remaining}. ` : ""}${task.action === "remove" ? removalCatalogMessage(receipt.catalog) + " " : ""}${receipt.problem ? receipt.problem + " " : ""}Git-host accounts and repositories were not deleted.`;
        store.setState({
          task: {
            ...task,
            receipt,
            selectedAccounts: preview.accounts,
            unknown: "",
            preview: null,
            confirmation: "",
            reviewing: false,
          },
          notice: {
            message: `${receipt.ok ? "Removal completed." : "Removal is incomplete."} ${detail}`,
            kind: receipt.ok ? "success" : "danger",
          },
        });
        await loadCatalog(version);
        if (!receipt.ok && current(version)) {
          store.setState({ operation: "inspect" });
          await readRemoval(version);
        }
      } catch (error) {
        if (!current(version)) return;
        const message = errorMessage(error);
        store.setState({
          task: { ...task, unknown: message, preview: null, confirmation: "", reviewing: false },
          notice: {
            message:
              "Removal outcome is not confirmed. Some local data may already have been permanently deleted. Check current state before retrying. " +
              message,
            kind: "danger",
          },
        });
        await loadCatalog(version);
      } finally {
        if (current(version)) store.setState({ operation: null });
      }
    },
  }));

  // Catalog reads invalidate only current observations, never command receipts.
  async function loadCatalog(version: number) {
    if (!current(version)) return;
    store.setState({ loading: true, inspections: {} });
    try {
      const data = await invoke("list", {});
      if (current(version)) store.setState({ data, readError: "" });
    } catch (error) {
      if (current(version)) store.setState({ data: null, readError: errorMessage(error) });
    } finally {
      if (current(version)) store.setState({ loading: false });
    }
  }

  // Workspace task: inspection is the only source of readiness for both views.
  async function readWorkspace(version: number) {
    const task = store.getState().task;
    if (!current(version) || task?.kind !== "workspace") return null;
    const inspections = { ...store.getState().inspections };
    delete inspections[task.project.id];
    store.setState({ inspections });
    try {
      const { workspace } = await invoke("inspect", { id: task.project.id });
      if (!current(version)) return null;
      store.setState((state) => ({
        inspections: { ...state.inspections, [task.project.id]: workspace },
        task: {
          ...task,
          readError: "",
          setupError: workspaceReady(workspace) ? "" : task.setupError,
        },
      }));
      return workspace;
    } catch (error) {
      if (current(version)) store.setState({ task: { ...task, readError: errorMessage(error) } });
      return null;
    }
  }
  async function runWorkspace(setup: boolean) {
    const task = store.getState().task;
    if (!active || store.getState().operation || task?.kind !== "workspace") return;
    const version = generation;
    store.setState({ operation: "inspect" });
    try {
      const inspection = await readWorkspace(version);
      if (!setup || !workspaceCanSetup(inspection) || !current(version)) return;
      const latest = store.getState().task;
      if (latest?.kind !== "workspace") return;
      store.setState({ operation: "setup", task: { ...latest, setupError: "", completed: false } });
      try {
        await invoke("setup", { id: task.project.id });
        if (current(version))
          store.setState({ task: { ...latest, setupError: "", completed: true } });
      } catch (error) {
        if (current(version))
          store.setState({
            task: { ...latest, setupError: errorMessage(error), completed: false },
          });
      }
      if (!current(version)) return;
      store.setState({ operation: "inspect" });
      await loadCatalog(version);
      await readWorkspace(version);
    } finally {
      if (current(version)) store.setState({ operation: null });
    }
  }

  // Removal reads never retry mutation or erase evidence from the last attempt.
  async function readRemoval(version: number) {
    const task = store.getState().task;
    if (!current(version) || task?.kind !== "removal") return;
    store.setState({ task: { ...task, preview: null } });
    try {
      const { preview } = await invoke("removal-inspect", {
        action: task.action,
        target: task.target,
      });
      if (preview.action !== task.action || preview.target !== task.target)
        throw new Error("Removal inspection does not match the requested target.");
      if (current(version)) store.setState({ task: { ...task, preview, readError: "" } });
    } catch (error) {
      if (current(version))
        store.setState({ task: { ...task, preview: null, readError: errorMessage(error) } });
    }
  }
  return store;
}
export type ProjectsStore = ReturnType<typeof createProjectsStore>;
