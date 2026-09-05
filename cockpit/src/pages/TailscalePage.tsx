import { Alert, Button, PageSection } from "@patternfly/react-core";
import { CockpitPageTemplate } from "../templates/CockpitPageTemplate";
import { DiagnosticAlert } from "../molecules/DiagnosticAlert";
import { ExternalLink } from "../atoms/ExternalLink";
import { TailscaleConnection } from "../organisms/tailscale/TailscaleConnection";
import { TailscaleDevices } from "../organisms/tailscale/TailscaleDevices";
import { ExitNodeForm } from "../organisms/tailscale/ExitNodeForm";
import { ExitNodeAdvertisement } from "../organisms/tailscale/ExitNodeAdvertisement";
import { cliURL } from "../tailscale/links";
import { useTailscale, type TailscaleOptions } from "../tailscale/useTailscale";
export function TailscalePage(props: TailscaleOptions) {
  const {
    snapshot,
    busy,
    loading,
    notice,
    readError,
    forgejoError,
    operation,
    saved,
    retryForgejo,
    authURL,
    streamState,
    connected,
    peers,
    choices,
    selected,
    missingSelection,
    exitNode,
    allowLAN,
    advertise,
    changeExitNode,
    changeAllowLAN,
    changeAdvertise,
    signIn,
    applyExitNode,
    applyAdvertisement,
  } = useTailscale(props);
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
        onSignIn={signIn}
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
        onApply={applyExitNode}
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
        onApply={applyAdvertisement}
      />
      <PageSection>
        <ExternalLink href={cliURL}>Tailscale CLI documentation</ExternalLink>
      </PageSection>
    </CockpitPageTemplate>
  );
}
