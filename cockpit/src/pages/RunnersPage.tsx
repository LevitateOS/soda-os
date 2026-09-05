import { Button, Toolbar, ToolbarContent, ToolbarItem } from "@patternfly/react-core";
import { CockpitPageTemplate } from "../templates/CockpitPageTemplate";
import { DiagnosticAlert } from "../molecules/DiagnosticAlert";
import { RunnerCapacity } from "../organisms/runners/RunnerCapacity";
import { RunnerExecutionNotice } from "../organisms/runners/RunnerExecutionNotice";
import { ProviderAuthoritySection } from "../organisms/runners/ProviderAuthoritySection";
import { RegisterRunnerDialog } from "../organisms/runners/RegisterRunnerDialog";
import { RemoveRunnerDialog } from "../organisms/runners/RemoveRunnerDialog";
import type { Invoke } from "../runners/types";
import { useRunners } from "../runners/useRunners";
export function RunnersPage({
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
    dialog,
    refresh,
    create,
    mutate,
    remove,
    close,
    openCreate,
    openRemove,
  } = useRunners(invoke);
  const error = notice?.kind === "danger" ? notice.message : "";
  return (
    <CockpitPageTemplate
      title="Runners"
      description="Create and manage generic local capacity for provider-owned CI workflows."
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
              <Button isDisabled={busy} onClick={openCreate}>
                Create local runner
              </Button>
            </ToolbarItem>
          </ToolbarContent>
        </Toolbar>
      }
      feedback={notice && <DiagnosticAlert message={notice.message} variant={notice.kind} />}
      dialogs={
        <>
          {dialog?.kind === "create" && (
            <RegisterRunnerDialog
              busy={busy}
              onClose={close}
              onSubmit={create}
              hostname={hostname}
              error={error}
            />
          )}
          {dialog?.kind === "remove" && (
            <RemoveRunnerDialog
              id={dialog.id}
              busy={busy}
              error={error}
              onClose={close}
              onSubmit={remove}
            />
          )}
        </>
      }
    >
      <RunnerExecutionNotice />
      <RunnerCapacity
        data={data}
        loading={loading}
        busy={busy}
        hostname={hostname}
        onAction={(action, id) => void mutate(action, id)}
        onRemove={openRemove}
      />
      <ProviderAuthoritySection />
    </CockpitPageTemplate>
  );
}
