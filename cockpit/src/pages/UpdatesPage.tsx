import { Button, PageSection, Stack, StackItem } from "@patternfly/react-core";
import { CockpitPageTemplate } from "../templates/CockpitPageTemplate";
import { UpdateFeedback } from "../molecules/updates/UpdateFeedback";
import { NativeOperationOutput } from "../molecules/updates/NativeOperationOutput";
import { InstalledImageSection } from "../organisms/updates/InstalledImageSection";
import { TrackedImageSection } from "../organisms/updates/TrackedImageSection";
import { PendingDeploymentSection } from "../organisms/updates/PendingDeploymentSection";
import { useEffect } from "react";
import { useStore } from "zustand";
import { operationLabels, type UpdatesStore } from "../updates/store";
import { updateDiagnostic } from "../updates/status";

export function UpdatesPage({ store }: { store: UpdatesStore }) {
  const state = useStore(store);
  useEffect(() => {
    const stop = store.getState().start();
    const focus = () => {
      void store.getState().refresh();
    };
    window.addEventListener("focus", focus);
    return () => {
      window.removeEventListener("focus", focus);
      stop();
    };
  }, [store]);
  const busy = Boolean(state.operation);
  const staged = state.host?.status.staged;
  const diagnostic = updateDiagnostic(state.host);
  return (
    <CockpitPageTemplate
      title="Soda Updates"
      description="Native OS image updates. You decide when to update and restart."
      busy={busy}
      actions={
        <Button variant="secondary" isDisabled={busy} onClick={() => void state.refresh()}>
          Refresh status
        </Button>
      }
      feedback={
        <UpdateFeedback
          operation={state.operation ? operationLabels[state.operation] : null}
          error={[...new Set([state.error, state.readError].filter(Boolean))].join("\n\n") || null}
          notice={state.notice}
          diagnostic={busy ? null : diagnostic}
        />
      }
    >
      <PageSection>
        <Stack hasGutter>
          <StackItem>
            <InstalledImageSection image={state.host?.status.booted.image} />
          </StackItem>
          <StackItem>
            <TrackedImageSection
              host={state.host}
              busy={busy}
              blocked={Boolean(diagnostic)}
              onCheck={() => void state.check()}
              onUpdate={() => void state.update()}
            />
          </StackItem>
          {staged && (
            <StackItem>
              <PendingDeploymentSection deployment={staged} />
            </StackItem>
          )}
          {state.progress && (
            <StackItem>
              <NativeOperationOutput output={state.progress} />
            </StackItem>
          )}
        </Stack>
      </PageSection>
    </CockpitPageTemplate>
  );
}
