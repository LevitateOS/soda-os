import { Card, CardBody, CardTitle } from "@patternfly/react-core";
import { CodeValue } from "../../atoms/CodeValue";
import type { Deployment } from "../../updates/types";

export function PendingDeploymentSection({ deployment }: { deployment: Deployment }) {
  return (
    <Card component="section" aria-label="Pending deployment">
      <CardTitle>
        {deployment.downloadOnly ? "Downloaded — finalization locked" : "Enabled for next restart"}
      </CardTitle>
      <CardBody>
        <p>Version: {deployment.image?.version || "Unknown version"}</p>
        <p>
          <CodeValue>{deployment.image?.image.image}</CodeValue>
        </p>
        <p>
          <CodeValue>{deployment.image?.imageDigest}</CodeValue>
        </p>
        <p>This is native pending state, not the image currently booted.</p>
      </CardBody>
    </Card>
  );
}
