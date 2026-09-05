import { Alert, Button, PageSection } from "@patternfly/react-core";
import { CockpitPageTemplate } from "../templates/CockpitPageTemplate";
import { DiagnosticAlert } from "../molecules/DiagnosticAlert";
import { ExternalLink } from "../atoms/ExternalLink";
import { TailscaleConnection } from "../organisms/tailscale/TailscaleConnection";
import { TailscaleDevices } from "../organisms/tailscale/TailscaleDevices";
import { ExitNodeForm } from "../organisms/tailscale/ExitNodeForm";
import { ExitNodeAdvertisement } from "../organisms/tailscale/ExitNodeAdvertisement";
import { cliURL } from "../tailscale/links";
import { useEffect } from "react";
import { useStore } from "zustand";
import type { Cockpit } from "../cockpit/types";
import type { TailscaleStore } from "../tailscale/store";
import { deviceName, exitNodeChoices, exitSelection } from "../tailscale/status";
const reload = () => window.location.reload();
export function TailscalePage({
  store,
  cockpit = window.cockpit,
  onReopen = reload,
}: {
  store: TailscaleStore;
  cockpit?: Pick<Cockpit, "hidden" | "addEventListener" | "removeEventListener">;
  onReopen?: () => void;
}) {
  const {
    snapshot,
    loading,
    notice,
    readError,
    forgejoError,
    operation,
    saved,
    retryForgejo,
    authURL,
    streamState,
    exitNode,
    allowLAN,
    advertise,
    changeExitNode,
    changeAllowLAN,
    changeAdvertise,
    signIn,
    applyExitNode,
    applyAdvertisement,
  } = useStore(store);
  useEffect(() => {
    let stop = cockpit.hidden ? undefined : store.getState().start();
    function close() {
      stop?.();
      stop = undefined;
    }
    function visibility() {
      if (cockpit.hidden) close();
      else if (!stop) onReopen();
    }
    window.addEventListener("pagehide", close);
    cockpit.addEventListener("visibilitychange", visibility);
    return () => {
      close();
      window.removeEventListener("pagehide", close);
      cockpit.removeEventListener("visibilitychange", visibility);
    };
  }, [store, cockpit, onReopen]);
  const busy = operation !== undefined;
  const status = snapshot?.status;
  const connected = status?.BackendState === "Running" && !status.Self?.Expired;
  const peers = Object.values(status?.Peer || {}).sort((a, b) =>
    deviceName(a).localeCompare(deviceName(b)),
  );
  const choices = status ? exitNodeChoices(status).filter((peer) => peer.TailscaleIPs?.[0]) : [];
  const selected = snapshot ? exitSelection(snapshot) : "";
  const missingSelection = Boolean(
    snapshot?.prefs.ExitNodeID &&
    selected &&
    !choices.some((peer) => peer.TailscaleIPs?.[0] === selected),
  );
  return (
    <CockpitPageTemplate
      title="Tailscale"
      busy={busy}
      feedback={
        <>
          {notice && <DiagnosticAlert message={notice} />}
          {readError && <DiagnosticAlert message={readError} />}
          {forgejoError && (
            <Alert
              isInline
              variant="warning"
              title={`${connected ? "Tailscale connected, but " : ""}Forgejo could not refresh its Tailnet address: ${forgejoError}`}
            >
              <Button
                variant="link"
                isInline
                isDisabled={busy || !connected}
                isLoading={operation === "forgejo"}
                onClick={() => void retryForgejo()}
              >
                Retry Forgejo address refresh
              </Button>
            </Alert>
          )}
        </>
      }
    >
      <TailscaleConnection
        status={snapshot?.status}
        streamState={streamState}
        loading={loading}
        busy={busy}
        connected={connected}
        authURL={authURL}
        onSignIn={() => void signIn()}
      />
      <TailscaleDevices peers={peers} available={Boolean(snapshot)} loading={loading} />
      <ExitNodeForm
        saving={operation === "exit"}
        saved={saved === "exit"}
        exitNode={exitNode}
        allowLAN={allowLAN}
        busy={busy}
        connected={connected}
        choices={choices}
        selected={selected}
        missingSelection={missingSelection}
        onExitNodeChange={changeExitNode}
        onAllowLANChange={changeAllowLAN}
        onApply={() => void applyExitNode()}
      />
      <ExitNodeAdvertisement
        saving={operation === "advertise"}
        saved={saved === "advertise"}
        snapshot={snapshot}
        loading={loading}
        busy={busy}
        connected={connected}
        advertise={advertise}
        onChange={changeAdvertise}
        onApply={() => void applyAdvertisement()}
      />
      <PageSection>
        <ExternalLink href={cliURL}>Tailscale CLI documentation</ExternalLink>
      </PageSection>
    </CockpitPageTemplate>
  );
}
