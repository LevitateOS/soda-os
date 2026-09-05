import { ExternalLink } from "../../atoms/ExternalLink";
import { adminURL } from "../../tailscale/links";
import { advertisesExitNode, exitNodeApproval } from "../../tailscale/status";
import type { Snapshot } from "../../tailscale/types";
export function ExitNodeApprovalGuidance({
  snapshot,
  loading,
}: {
  snapshot: Snapshot | null;
  loading: boolean;
}) {
  const prefs = snapshot?.prefs,
    status = snapshot?.status;
  return (
    <>
      <p role="status">
        {snapshot
          ? exitNodeApproval(snapshot.status, snapshot.prefs)
          : loading
            ? "Not advertised"
            : "Approval status unavailable"}
      </p>
      {prefs &&
        advertisesExitNode(prefs) &&
        status?.Self?.InNetworkMap &&
        !status.Self.ExitNodeOption && (
          <p>
            A Tailnet administrator can approve this exit node in{" "}
            <ExternalLink href={adminURL}>Tailscale administration</ExternalLink>.
          </p>
        )}
    </>
  );
}
