import { PageSection } from "@patternfly/react-core";
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
      feedback={notice && <DiagnosticAlert message={notice} />}
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
