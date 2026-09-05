import type { ReactNode } from "react";
import { Button, Toolbar, ToolbarContent, ToolbarItem } from "@patternfly/react-core";
import { CockpitPageTemplate } from "../templates/CockpitPageTemplate";
import { DiagnosticAlert } from "../molecules/DiagnosticAlert";
import { ProjectCatalog } from "../organisms/projects/ProjectCatalog";
import { PeopleSection } from "../organisms/projects/PeopleSection";
import { CatalogProjectDialog } from "../organisms/projects/CatalogProjectDialog";
import { WorkspaceSetupDialog } from "../organisms/projects/WorkspaceSetupDialog";
import { RemoveProjectDialog } from "../organisms/projects/RemoveProjectDialog";
import { RemoveHumanDialog } from "../organisms/projects/RemoveHumanDialog";
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
    busy,
    loading,
    notice,
    readError,
    dialog,
    formError,
    refresh,
    open,
    close,
    submit,
  } = useProjects(invoke);
  const refreshError = readError ? `The current catalog could not be refreshed. ${readError}` : "";
  const dialogProps = {
    busy,
    error: [formError, refreshError].filter(Boolean).join("\n\n"),
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
            action={dialog.action}
            project={dialog.project}
            {...dialogProps}
          />
        );
        break;
      case "setup":
        dialogView = <WorkspaceSetupDialog key={key} project={dialog.project} {...dialogProps} />;
        break;
      case "remove":
      case "remove-workspace":
        dialogView = (
          <RemoveProjectDialog
            key={key}
            action={dialog.action}
            project={dialog.project}
            {...dialogProps}
          />
        );
        break;
      case "delete-human":
        dialogView = <RemoveHumanDialog key={key} {...dialogProps} />;
        break;
    }
  }
  return (
    <CockpitPageTemplate
      title="Projects"
      description="Catalog repositories and create an isolated Linux workspace for each person."
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
        loading={loading}
        busy={busy}
        hostname={hostname}
        onAction={open}
      />
      {data && !humanDeletionHidden(data.current_user) && (
        <PeopleSection busy={busy} onRemove={() => open("delete-human")} />
      )}
    </CockpitPageTemplate>
  );
}
