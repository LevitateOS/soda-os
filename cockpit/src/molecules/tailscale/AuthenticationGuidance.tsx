import { Alert } from "@patternfly/react-core";
import { ExternalLink } from "../../atoms/ExternalLink";
import type { Status } from "../../tailscale/types";
import { adminURL } from "../../tailscale/links";
export function AuthenticationGuidance({
  status,
  authURL,
}: {
  status?: Status;
  authURL: string | null;
}) {
  return (
    <>
      {authURL && (
        <Alert isInline variant="info" title="Continue in Tailscale">
          <ExternalLink href={authURL}>{authURL}</ExternalLink>
        </Alert>
      )}
      {status?.BackendState === "NeedsMachineAuth" && (
        <p>
          Ask a Tailnet administrator to approve this device in{" "}
          <ExternalLink href={adminURL}>Tailscale administration</ExternalLink>.
        </p>
      )}
    </>
  );
}
