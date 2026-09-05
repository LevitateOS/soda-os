import { useEffect, type FormEvent, type ReactNode } from "react";
import { useStore } from "zustand";
import { Button, Toolbar, ToolbarContent, ToolbarItem } from "@patternfly/react-core";
import { CockpitPageTemplate } from "../templates/CockpitPageTemplate";
import { DiagnosticAlert } from "../molecules/DiagnosticAlert";
import { ProjectCatalog } from "../organisms/projects/ProjectCatalog";
import { PeopleSection } from "../organisms/projects/PeopleSection";
import { CatalogProjectDialog } from "../organisms/projects/CatalogProjectDialog";
import { WorkspaceDialog } from "../organisms/projects/WorkspaceDialog";
import { RemovalDialog } from "../organisms/projects/RemovalDialog";
import { humanDeletionHidden } from "../projects/ui";
import type { ProjectsStore } from "../projects/store";

export function ProjectsPage({
  store,
  hostname = window.location.hostname,
}: {
  store: ProjectsStore;
  hostname?: string;
}) {
  const state = useStore(store);
  useEffect(() => store.getState().start(), [store]);
  const { data, inspections, loading, notice, readError, task, operation, refresh, open, close } =
    state;
  const busy = operation !== null;
  const refreshError = readError ? `The current catalog could not be refreshed. ${readError}` : "";
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!event.currentTarget.reportValidity()) return;
    const values: Record<string, string> = {};
    for (const [name, value] of new FormData(event.currentTarget))
      if (typeof value === "string") values[name] = value;
    void state.submitCatalog(values);
  }
  let dialogView: ReactNode;
  if (task?.kind === "catalog") {
    dialogView = (
      <CatalogProjectDialog
        key={task.action + (task.project?.id ?? "")}
        action={task.action}
        project={task.project}
        busy={busy}
        metadataError={task.error?.field === "additional_metadata" ? task.error : null}
        error={[
          task.error?.field === "additional_metadata" ? "" : task.error?.message,
          refreshError,
        ]
          .filter(Boolean)
          .join("\n\n")}
        onClose={close}
        onSubmit={submit}
      />
    );
  } else if (task?.kind === "workspace") {
    dialogView = (
      <WorkspaceDialog
        key={task.project.id}
        task={task}
        inspection={inspections[task.project.id] ?? null}
        operation={operation === null ? null : operation === "setup" ? "setup" : "inspect"}
        hostname={hostname}
        catalogReadError={readError}
        onClose={close}
        refresh={state.checkWorkspace}
        setup={state.setupWorkspace}
      />
    );
  } else if (task?.kind === "removal") {
    dialogView = (
      <RemovalDialog
        task={task}
        viewer={data?.current_user.username ?? ""}
        operation={operation === null ? null : operation === "remove" ? "remove" : "inspect"}
        catalogReadError={readError}
        onClose={close}
        changeTarget={state.changeRemovalTarget}
        changeConfirmation={state.changeConfirmation}
        inspect={state.checkRemoval}
        remove={state.remove}
      />
    );
  }
  return (
    <CockpitPageTemplate
      title="Projects"
      description="Choose a project and set up your own workspace."
      busy={busy}
      actions={
        <Toolbar>
          <ToolbarContent>
            <ToolbarItem>
              <Button variant="secondary" isDisabled={busy} onClick={() => void refresh()}>
                Refresh
              </Button>
            </ToolbarItem>
            <ToolbarItem>
              <Button isDisabled={busy} onClick={() => open("add-existing")}>
                Add repository
              </Button>
            </ToolbarItem>
          </ToolbarContent>
        </Toolbar>
      }
      feedback={
        !task && (
          <>
            {notice && <DiagnosticAlert message={notice.message} variant={notice.kind} />}
            {readError && <DiagnosticAlert message={refreshError} />}
          </>
        )
      }
      dialogs={dialogView}
    >
      <ProjectCatalog
        data={data}
        inspections={inspections}
        loading={loading}
        busy={busy}
        onAction={open}
      />
      {data && (
        <PeopleSection
          busy={busy}
          administrator={!humanDeletionHidden(data.current_user)}
          onRemove={() => open("delete-human")}
        />
      )}
    </CockpitPageTemplate>
  );
}
