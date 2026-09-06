import { Card, CardBody, CardTitle } from "@patternfly/react-core";
import { CodeValue } from "../../atoms/CodeValue";
import type { Image } from "../../updates/types";

export function InstalledImageSection({ image }: { image: Image | null | undefined }) {
  return (
    <Card component="section" aria-label="Installed image">
      <CardTitle>Installed</CardTitle>
      <CardBody>
        {image ? (
          <>
            <p>Version: {image.version || "Unknown version"}</p>
            <p>
              Actual booted digest: <CodeValue>{image.imageDigest}</CodeValue>
            </p>
            <details>
              <summary>Image details</summary>
              <p>Architecture: {image.architecture}</p>
              <p>
                <CodeValue>{image.image.image}</CodeValue>
              </p>
            </details>
          </>
        ) : (
          <p>Installed image unavailable. Refresh status to try again.</p>
        )}
      </CardBody>
    </Card>
  );
}
