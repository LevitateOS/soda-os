import { Button, PageSection, Spinner, Stack, StackItem, Title } from "@patternfly/react-core";
import { DiagnosticAlert } from "../../molecules/DiagnosticAlert";
import { DeviceIdentity } from "../../molecules/tailscale/DeviceIdentity";
import { AuthenticationGuidance } from "../../molecules/tailscale/AuthenticationGuidance";
import type { Status } from "../../tailscale/types";
import { connectionState } from "../../tailscale/status";
export function TailscaleConnection({
  status,
  streamState,
  loading,
  busy,
  connected,
  authURL,
  onSignIn,
}: {
  status?: Status;
  streamState?: string;
  loading: boolean;
  busy: boolean;
  connected: boolean;
  authURL: string | null;
  onSignIn: () => void;
}) {
  return (
    <PageSection aria-labelledby="connection-title">
      <Stack hasGutter>
        <Title headingLevel="h2" id="connection-title">
          Connection
        </Title>
        <p role="status" aria-live="polite">
          {streamState ||
            (status
              ? connectionState(status)
              : loading
                ? "Reading Tailscale state…"
                : "Tailscale state unavailable")}
        </p>
        {loading && <Spinner aria-label="Reading Tailscale state" size="lg" />}
        {!!status?.Health?.length && (
          <DiagnosticAlert variant="warning" message={status.Health.join("\n")} />
        )}
        {!connected && status?.BackendState !== "NeedsMachineAuth" && (
          <StackItem>
            <Button
              isDisabled={
                busy || !status || ["Starting", "NoState"].includes(status.BackendState ?? "")
              }
              onClick={onSignIn}
            >
              {status?.BackendState === "Stopped"
                ? "Connect"
                : status?.HaveNodeKey
                  ? "Sign in again"
                  : "Sign in"}
            </Button>
          </StackItem>
        )}
        <AuthenticationGuidance status={status} authURL={authURL} />
        <DeviceIdentity status={status} />
      </Stack>
    </PageSection>
  );
}
