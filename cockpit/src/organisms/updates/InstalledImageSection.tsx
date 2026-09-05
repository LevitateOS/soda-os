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
            <p>Soda OS {image.version || "Unknown version"}</p>
            <details>
              <summary>Image details</summary>
              <p>Architecture: {image.architecture}</p>
              <p>
                <CodeValue>{image.image.image}</CodeValue>
              </p>
              <p>
                <CodeValue>{image.imageDigest}</CodeValue>
              </p>
            </details>
          </>
        ) : (
          <p>Enable Cockpit administrative access and refresh to read native bootc status.</p>
        )}
      </CardBody>
    </Card>
  );
}
