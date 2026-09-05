import { Button, Card, CardBody, CardTitle } from "@patternfly/react-core";
import { CodeValue } from "../../atoms/CodeValue";
import type { Deployment, Selection } from "../../updates/types";

export function PendingDeploymentSection({
  deployment,
  selection,
  busy,
  blocked,
  onApply,
}: {
  deployment: Deployment;
  selection: Selection | null;
  busy: boolean;
  blocked: boolean;
  onApply: () => void;
}) {
  return (
    <Card component="section" aria-label="Pending deployment">
      <CardTitle>
        {deployment.downloadOnly
          ? "Downloaded — not yet enabled for restart"
          : "Enabled for next restart"}
      </CardTitle>
      <CardBody>
        <p>Soda OS {deployment.image?.version || "Unknown version"}</p>
        <details>
          <summary>Pending image details</summary>
          <p>
            <CodeValue>{deployment.image?.image.image}</CodeValue>
          </p>
        </details>
        <Button variant="warning" isDisabled={busy || !selection || blocked} onClick={onApply}>
          Apply and restart…
        </Button>
        <p>
          The release is verified again before activation. Native CLI deployments that are not
          approved Soda releases cannot be applied here.
        </p>
      </CardBody>
    </Card>
  );
}
