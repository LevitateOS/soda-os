import { Button, Spinner, Toolbar, ToolbarContent, ToolbarItem } from "@patternfly/react-core";
import { CockpitPageTemplate } from "../templates/CockpitPageTemplate";
import { DiagnosticAlert } from "../molecules/DiagnosticAlert";
import { RunnerCapacity } from "../organisms/runners/RunnerCapacity";
import { RunnerExecutionNotice } from "../organisms/runners/RunnerExecutionNotice";
import { ProviderAuthoritySection } from "../organisms/runners/ProviderAuthoritySection";
import { RegisterRunnerDialog } from "../organisms/runners/RegisterRunnerDialog";
import { RemoveRunnerDialog } from "../organisms/runners/RemoveRunnerDialog";
import { useEffect, type FormEvent } from "react";
import { useStore } from "zustand";
import type { RunnersStore } from "../runners/store";
import { createPayload } from "../runners/ui";
export function RunnersPage({
  store,
  hostname = window.location.hostname,
}: {
  store: RunnersStore;
  hostname?: string;
}) {
  const {
    data,
    loading,
    notice,
    readError,
    formError,
    operation: pending,
    dialog,
    refresh,
    register,
    changeProvider,
    changeListener,
    remove,
    close,
    openCreate,
    openRemove,
  } = useStore(store);
  useEffect(() => store.getState().start(), [store]);
  const busy = pending !== null;
  const operation =
    pending && pending.action !== "list" && pending.action !== "create"
      ? `${{ start: "Starting", stop: "Stopping", restart: "Restarting", remove: "Removing" }[pending.action]} ${pending.id}…`
      : "";
  function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    if (!form.reportValidity()) return;
    const token = form.elements.namedItem("registration_token") as HTMLInputElement;
    const payload = createPayload(new FormData(form));
    try {
      void register(payload);
    } finally {
      token.value = "";
      payload.registration_token = "";
    }
  }
  function confirmRemoval(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const confirmation = new FormData(event.currentTarget).get("confirmation");
    if (!remove(typeof confirmation === "string" ? confirmation : ""))
      (event.currentTarget.elements.namedItem("confirmation") as HTMLElement | null)?.focus();
  }
  const refreshError = readError
    ? `The current runner list could not be refreshed. ${readError}`
    : "";
  const error = [formError, refreshError].filter(Boolean).join("\n\n");
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
      feedback={
        !dialog && (
          <>
            {operation && (
              <div role="status">
                <Spinner size="sm" aria-hidden /> {operation}
              </div>
            )}
            {notice && <DiagnosticAlert message={notice.message} variant={notice.kind} />}
            {readError && <DiagnosticAlert message={refreshError} />}
          </>
        )
      }
      dialogs={
        <>
          {dialog?.kind === "create" && (
            <RegisterRunnerDialog
              provider={dialog.provider}
              onProviderChange={changeProvider}
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
              onSubmit={confirmRemoval}
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
        onAction={(action, id) => void changeListener(action, id)}
        onRemove={openRemove}
      />
      <ProviderAuthoritySection />
    </CockpitPageTemplate>
  );
}
