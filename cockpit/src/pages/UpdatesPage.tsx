import {
  Alert,
  Button,
  Card,
  CardBody,
  CardTitle,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  ModalVariant,
  PageSection,
  Spinner,
  Stack,
  StackItem,
} from "@patternfly/react-core";
import { CockpitPageTemplate } from "../templates/CockpitPageTemplate";
import { ExternalLink } from "../atoms/ExternalLink";
import { useUpdates } from "../updates/useUpdates";
import { availability, stagedSelection } from "../updates/status";
import type { NativeUpdates } from "../updates/types";

export function UpdatesPage({ native }: { native: NativeUpdates }) {
  const state = useUpdates(native);
  const busy = Boolean(state.operation);
  const booted = state.host?.status.booted.image;
  const staged = state.host?.status.staged;
  const selected = stagedSelection(state.host);
  const available = availability(state.host, state.release);
  const blocked = Boolean(state.host?.status.rollbackQueued || state.host?.status.usrOverlay);
  return (
    <CockpitPageTemplate
      title="Soda Updates"
      description="Verified OS image updates. You decide when to download and restart."
      busy={busy}
      actions={
        <Button variant="secondary" isDisabled={busy} onClick={() => void state.refresh()}>
          Refresh status
        </Button>
      }
      feedback={
        <Stack hasGutter>
          {state.operation && (
            <StackItem>
              <div role="status">
                <Spinner size="sm" /> {state.operation}
              </div>
            </StackItem>
          )}
          {state.error && (
            <StackItem>
              <Alert
                isInline
                variant="danger"
                title="Operation could not be confirmed"
                role="alert"
              >
                <pre style={{ whiteSpace: "pre-wrap" }}>{state.error}</pre>
              </Alert>
            </StackItem>
          )}
          {state.notice && (
            <StackItem>
              <Alert isInline variant="info" title={state.notice} />
            </StackItem>
          )}
          {blocked && (
            <StackItem>
              <Alert
                isInline
                variant="warning"
                title="Resolve the queued rollback or transient /usr overlay with native tools before updating."
              />
            </StackItem>
          )}
        </Stack>
      }
      dialogs={
        <Modal
          variant={ModalVariant.small}
          isOpen={Boolean(state.confirmation)}
          onClose={() => state.setConfirmation(null)}
          aria-labelledby="updates-restart-title"
        >
          <ModalHeader title="Apply update and restart?" labelId="updates-restart-title" />
          <ModalBody>
            <p>
              Soda OS {state.confirmation?.version} will be enabled for the next boot and the server
              will restart. SSH sessions and running development workloads will be interrupted.
            </p>
            <p>
              Do not run other bootc or deployment commands while applying. The target is rechecked,
              but bootc does not provide an atomic expected-digest activation guard.
            </p>
            <p>
              <code>{state.confirmation?.reference}</code>
            </p>
          </ModalBody>
          <ModalFooter>
            <Button variant="danger" isDisabled={busy} onClick={() => void state.apply()}>
              Apply and restart
            </Button>
            <Button variant="link" onClick={() => state.setConfirmation(null)}>
              Keep working
            </Button>
          </ModalFooter>
        </Modal>
      }
    >
      <PageSection>
        <Stack hasGutter>
          <StackItem>
            <Card component="section" aria-label="Installed image">
              <CardTitle>Installed</CardTitle>
              <CardBody>
                {booted ? (
                  <>
                    <p>Soda OS {booted.version || "Unknown version"}</p>
                    <details>
                      <summary>Image details</summary>
                      <p>Architecture: {booted.architecture}</p>
                      <p>
                        <code>{booted.image.image}</code>
                      </p>
                      <p>
                        <code>{booted.imageDigest}</code>
                      </p>
                    </details>
                  </>
                ) : (
                  <p>
                    Enable Cockpit administrative access and refresh to read native bootc status.
                  </p>
                )}
              </CardBody>
            </Card>
          </StackItem>
          <StackItem>
            <Card component="section" aria-label="Available release">
              <CardTitle>Available release</CardTitle>
              <CardBody>
                <Stack hasGutter>
                  <StackItem>
                    <p>{available.message}</p>
                  </StackItem>
                  {state.release && (
                    <StackItem>
                      <p>Soda OS {state.release.version}</p>
                      <ExternalLink href={state.release.notes_url}>Release notes</ExternalLink>
                      <details>
                        <summary>Verified release details</summary>
                        <p>
                          Release-record signature, image signature, provenance, and OCI identity
                          checked.
                        </p>
                        <p>
                          <code>{state.release.reference}</code>
                        </p>
                        <p>
                          Source commit: <code>{state.release.revision}</code>
                        </p>
                      </details>
                    </StackItem>
                  )}
                  <StackItem>
                    <Button isDisabled={busy || !booted} onClick={() => void state.check()}>
                      Check for updates
                    </Button>{" "}
                    {available.newer && (
                      <Button
                        variant="secondary"
                        isDisabled={busy || Boolean(staged) || blocked}
                        onClick={() => void state.download()}
                      >
                        Download update
                      </Button>
                    )}
                  </StackItem>
                </Stack>
              </CardBody>
            </Card>
          </StackItem>
          {staged && (
            <StackItem>
              <Card component="section" aria-label="Pending deployment">
                <CardTitle>
                  {staged.downloadOnly
                    ? "Downloaded — not yet enabled for restart"
                    : "Enabled for next restart"}
                </CardTitle>
                <CardBody>
                  <p>Soda OS {staged.image?.version || "Unknown version"}</p>
                  <details>
                    <summary>Pending image details</summary>
                    <p>
                      <code>{staged.image?.image.image}</code>
                    </p>
                  </details>
                  <Button
                    variant="warning"
                    isDisabled={busy || !selected || blocked}
                    onClick={() => state.setConfirmation(selected)}
                  >
                    Apply and restart…
                  </Button>
                  <p>
                    The release is verified again before activation. Native CLI deployments that are
                    not approved Soda releases cannot be applied here.
                  </p>
                </CardBody>
              </Card>
            </StackItem>
          )}
          {state.progress && (
            <StackItem>
              <details open>
                <summary>Native operation output (most recent 16 KiB)</summary>
                <pre style={{ whiteSpace: "pre-wrap" }}>{state.progress}</pre>
              </details>
            </StackItem>
          )}
        </Stack>
      </PageSection>
    </CockpitPageTemplate>
  );
}
