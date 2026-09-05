import type { ReactNode } from "react";
import { Button, Toolbar, ToolbarContent, ToolbarItem } from "@patternfly/react-core";
import { CockpitPageTemplate } from "../templates/CockpitPageTemplate";
import { DiagnosticAlert } from "../molecules/DiagnosticAlert";
import { ProjectCatalog } from "../organisms/projects/ProjectCatalog";
import { PeopleSection } from "../organisms/projects/PeopleSection";
import { CatalogProjectDialog } from "../organisms/projects/CatalogProjectDialog";
import { ProjectsWorkspaceDialog } from "./ProjectsWorkspaceDialog";
import { ProjectsRemovalDialog } from "./ProjectsRemovalDialog";
import type { Invoke } from "../projects/types";
import { humanDeletionHidden } from "../projects/ui";
import { useProjects } from "../projects/useProjects";
export function ProjectsPage({
  invoke,
  hostname = window.location.hostname,
}: {
  invoke: Invoke;
  hostname?: string;
}) {
  const {
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
  } = useProjects(invoke);
  const refreshError = readError ? `The current catalog could not be refreshed. ${readError}` : "";
  const dialogProps = {
    busy,
    error: [formError?.field === "additional_metadata" ? "" : formError?.message, refreshError]
      .filter(Boolean)
      .join("\n\n"),
    onClose: close,
    onSubmit: submit,
  };
  let dialogView: ReactNode;
  if (dialog) {
    const key = dialog.action + (dialog.project?.id ?? "");
    switch (dialog.action) {
      case "add-existing":
      case "edit":
        dialogView = (
          <CatalogProjectDialog
            key={key}
            metadataError={formError?.field === "additional_metadata" ? formError : null}
            action={dialog.action}
            project={dialog.project}
            {...dialogProps}
          />
        );
        break;
      case "setup":
      case "inspect":
        if (dialog.project)
          dialogView = (
            <ProjectsWorkspaceDialog
              key={key}
              project={dialog.project}
              invoke={invoke}
              hostname={hostname}
              startSetup={dialog.action === "setup"}
              catalogReadError={readError}
              onClose={close}
              onChanged={refresh}
              onInspected={inspected}
            />
          );
        break;
      case "remove":
      case "remove-workspace":
      case "delete-human":
        dialogView = (
          <ProjectsRemovalDialog
            key={key}
            action={dialog.action}
            initialTarget={dialog.project?.id ?? ""}
            viewer={data?.current_user.username ?? ""}
            invoke={invoke}
            catalogReadError={readError}
            onChanged={refresh}
            onOutcome={reportRemoval}
            onClose={close}
          />
        );
        break;
    }
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
        !dialog && (
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
