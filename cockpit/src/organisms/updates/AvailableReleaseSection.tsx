import { Button, Card, CardBody, CardTitle, Stack, StackItem } from "@patternfly/react-core";
import { CodeValue } from "../../atoms/CodeValue";
import { ExternalLink } from "../../atoms/ExternalLink";
import { availability } from "../../updates/status";
import type { Host, Release } from "../../updates/types";

export function AvailableReleaseSection({
  host,
  release,
  busy,
  blocked,
  onCheck,
  onDownload,
}: {
  host: Host | null;
  release: Release | null;
  busy: boolean;
  blocked: boolean;
  onCheck: () => void;
  onDownload: () => void;
}) {
  const available = availability(host, release);
  return (
    <Card component="section" aria-label="Available release">
      <CardTitle>Available release</CardTitle>
      <CardBody>
        <Stack hasGutter>
          <StackItem>
            <p>{available.message}</p>
          </StackItem>
          {release && (
            <StackItem>
              <p>Soda OS {release.version}</p>
              <ExternalLink href={release.notes_url}>Release notes</ExternalLink>
              <details>
                <summary>Verified release details</summary>
                <p>
                  Release-record signature, image signature, provenance, and OCI identity checked.
                </p>
                <p>
                  <CodeValue>{release.reference}</CodeValue>
                </p>
                <p>
                  Source commit: <CodeValue>{release.revision}</CodeValue>
                </p>
              </details>
            </StackItem>
          )}
          <StackItem>
            <Button isDisabled={busy || !host?.status.booted.image} onClick={onCheck}>
              Check for updates
            </Button>{" "}
            {available.newer && (
              <Button
                variant="secondary"
                isDisabled={busy || Boolean(host?.status.staged) || blocked}
                onClick={onDownload}
              >
                Download update
              </Button>
            )}
          </StackItem>
        </Stack>
      </CardBody>
    </Card>
  );
}
