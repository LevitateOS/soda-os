import { Button, Card, CardBody, CardTitle } from "@patternfly/react-core";
import { CodeValue } from "../../atoms/CodeValue";
import { cachedUpdate } from "../../updates/status";
import type { Host } from "../../updates/types";

export function TrackedImageSection({
  host,
  busy,
  blocked,
  onCheck,
  onUpdate,
}: {
  host: Host | null;
  busy: boolean;
  blocked: boolean;
  onCheck: () => void;
  onUpdate: () => void;
}) {
  const source = host?.spec.image;
  const cached = cachedUpdate(host);
  return (
    <Card component="section" aria-label="Tracked image source">
      <CardTitle>Tracked image source</CardTitle>
      <CardBody>
        {source ? (
          <>
            <p>
              <CodeValue>{source.image}</CodeValue>
            </p>
            <p>Transport: {source.transport}</p>
          </>
        ) : (
          <p>Native image source unavailable.</p>
        )}
        {cached ? (
          <>
            <p>Native cached update: {cached.version || "Unknown version"}</p>
            <p>
              <CodeValue>{cached.imageDigest}</CodeValue>
            </p>
          </>
        ) : (
          <p>No cached update metadata is reported for this source.</p>
        )}
        <p>
          Check fetches metadata only. Update follows the native source when the operation starts,
          not a previously checked image. Native bootc owns pending deployments and activation.
        </p>
        {source?.image.includes("@") && (
          <p>
            This source is digest-pinned. Native upgrades cannot follow a moving tag until an
            administrator explicitly selects that tag with bootc switch.
          </p>
        )}
        <p>
          Updating may restart the server, interrupting SSH sessions and running development
          workloads.
        </p>
        <Button variant="secondary" isDisabled={busy} onClick={onCheck}>
          Check for updates
        </Button>{" "}
        <Button variant="warning" isDisabled={busy || blocked} onClick={onUpdate}>
          Update and restart
        </Button>
      </CardBody>
    </Card>
  );
}
