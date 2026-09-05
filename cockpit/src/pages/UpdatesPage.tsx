import { Button, PageSection, Stack, StackItem } from "@patternfly/react-core";
import { CockpitPageTemplate } from "../templates/CockpitPageTemplate";
import { UpdateFeedback } from "../molecules/updates/UpdateFeedback";
import { NativeOperationOutput } from "../molecules/updates/NativeOperationOutput";
import { InstalledImageSection } from "../organisms/updates/InstalledImageSection";
import { AvailableReleaseSection } from "../organisms/updates/AvailableReleaseSection";
import { PendingDeploymentSection } from "../organisms/updates/PendingDeploymentSection";
import { ApplyUpdateDialog } from "../organisms/updates/ApplyUpdateDialog";
import { useEffect } from "react";
import { useStore } from "zustand";
import { operationLabels, type UpdatesStore } from "../updates/store";
import { stagedSelection } from "../updates/status";
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
  const selected = stagedSelection(state.host);
  const blocked = Boolean(state.host?.status.rollbackQueued || state.host?.status.usrOverlay);
  return (
    <CockpitPageTemplate
      title="Soda Updates"
      description="Verified OS image updates. You decide when to download and restart."
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
          blocked={blocked}
        />
      }
      dialogs={
        <ApplyUpdateDialog
          selection={state.confirmation}
          busy={busy}
          onClose={state.cancelApply}
          onApply={() => void state.apply()}
        />
      }
    >
      <PageSection>
        <Stack hasGutter>
          <StackItem>
            <InstalledImageSection image={state.host?.status.booted.image} />
          </StackItem>
          <StackItem>
            <AvailableReleaseSection
              host={state.host}
              release={state.release}
              busy={busy}
              blocked={blocked}
              onCheck={() => void state.check()}
              onDownload={() => void state.download()}
            />
          </StackItem>
          {staged && (
            <StackItem>
              <PendingDeploymentSection
                deployment={staged}
                selection={selected}
                busy={busy}
                blocked={blocked}
                onApply={state.requestApply}
              />
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
