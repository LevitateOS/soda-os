import {
  Alert,
  Button,
  ClipboardCopy,
  ExpandableSection,
  Flex,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  Spinner,
  Stack,
} from "@patternfly/react-core";
import { ExternalLink } from "../atoms/ExternalLink";
import { CodeValue } from "../atoms/CodeValue";
import type { Invoke, Project, WorkspaceInspection } from "../projects/types";
import { sshCommand, workspaceReady, workspaceCanSetup } from "../projects/ui";
import { useWorkspace } from "../projects/useWorkspace";

export function ProjectsWorkspaceDialog({
  project,
  invoke,
  hostname,
  startSetup,
  catalogReadError,
  onClose,
  onChanged,
  onInspected,
}: {
  project: Project;
  invoke: Invoke;
  hostname: string;
  startSetup: boolean;
  catalogReadError: string;
  onClose: () => void;
  onChanged: () => Promise<void>;
  onInspected: (id: string, inspection: WorkspaceInspection | null) => void;
}) {
  const { inspection, operation, readError, setupError, completed, refresh, setup } = useWorkspace(
    invoke,
    project.id,
    startSetup,
    onChanged,
    onInspected,
  );
  const busy = operation !== null;
  const ready = workspaceReady(inspection);
  const primaryKey = inspection && !inspection.exists && inspection.primary_key_problem;
  const canSetup = workspaceCanSetup(inspection) && !readError;
  const diagnostics = [
    setupError,
    readError,
    catalogReadError,
    inspection?.primary_key_problem,
    inspection?.workspace_key_problem,
    inspection?.checkout_problem,
    inspection?.git_key_problem,
  ]
    .filter(Boolean)
    .join("\n\n");
  function close() {
    if (!busy) onClose();
  }
  return (
    <Modal
      isOpen
      variant="medium"
      aria-labelledby="workspace-title"
      onClose={busy ? undefined : close}
      onEscapePress={close}
    >
      <ModalHeader
        title={ready ? `Connect to ${project.display_name}` : `Set up ${project.display_name}`}
        labelId="workspace-title"
      />
      {/* A stable target keeps keyboard focus inside the dialog as the working view changes. */}
      <ModalBody
        tabIndex={0}
        role="group"
        aria-label={operation === "setup" ? "Setting up workspace" : "Workspace setup"}
      >
        <Stack hasGutter>
          {busy ? (
            <Flex alignItems={{ default: "alignItemsCenter" }} role="status">
              <Spinner size="md" aria-hidden />
              {operation === "setup" ? "Setting up your workspace…" : "Checking workspace setup…"}
            </Flex>
          ) : !inspection ? (
            <Alert
              isInline
              variant="warning"
              title="Workspace status is not confirmed"
              role="alert"
            >
              {completed
                ? "Setup completed, but the current workspace could not be checked. Check setup before connecting."
                : "Check setup before retrying. Do not assume that a previous attempt left the workspace unchanged."}
            </Alert>
          ) : ready ? (
            <>
              <p>Open a terminal on your computer and run:</p>
              <ClipboardCopy
                isReadOnly
                textAriaLabel="SSH command"
                copyAriaLabel="Copy SSH command"
              >
                {sshCommand(inspection.username, hostname)}
              </ClipboardCopy>
              <p>Your repository is in:</p>
              <CodeValue>{inspection.checkout_path}</CodeValue>
              <ExpandableSection toggleText="Development tools">
                <p>
                  Use mise inside your workspace to configure tools. Sign in to Tea or GitHub CLI
                  separately there when you need them.
                </p>
              </ExpandableSection>
            </>
          ) : primaryKey ? (
            <>
              <Alert isInline variant="info" title="Add or check your SSH public key" role="status">
                In Accounts → your primary account → Authorized public SSH keys, add the public key
                from the computer you will connect from. Then check setup again.
              </Alert>
              <Button onClick={() => window.cockpit.jump("/users")}>Open Accounts</Button>
              <ExpandableSection toggleText="Which key do I need?">
                <p>
                  Use your personal public key, not a private key. This lets your computer connect
                  to the workspace. The workspace uses a separate key to access your Git host.
                </p>
              </ExpandableSection>
            </>
          ) : inspection.workspace_key_problem ? (
            <>
              <Alert
                isInline
                variant="warning"
                title="Your workspace’s SSH keys need attention"
                role="status"
              >
                An administrator needs to restore your public SSH keys in Accounts →{" "}
                {inspection.username} before setup can continue.
              </Alert>
              <Button onClick={() => window.cockpit.jump("/users")}>Open Accounts</Button>
            </>
          ) : inspection.checkout_problem ? (
            <Alert isInline variant="warning" title="Your checkout needs inspection" role="status">
              Its contents could not be verified. Ask an administrator to inspect{" "}
              {inspection.checkout_path} before retrying setup. Soda will not overwrite this
              checkout.
            </Alert>
          ) : !inspection.exists ? (
            setupError ? (
              <Alert isInline variant="danger" title="Setup did not finish" role="alert">
                No workspace account is currently present. Review the technical details, then try
                setup again.
              </Alert>
            ) : (
              <p>
                Set up your own workspace for this project. Your personal SSH public keys will be
                copied once; your workspace will use a separate key to access the repository.
              </p>
            )
          ) : (
            <>
              <Alert
                isInline
                variant="info"
                title="Your workspace exists; the repository is not downloaded"
                role="status"
              >
                {inspection.public_key
                  ? "If this workspace key is not registered with your Git host, add it before retrying setup."
                  : "Retry setup to prepare the workspace’s Git key and download the repository."}
              </Alert>
              {inspection.public_key && (
                <>
                  <ClipboardCopy
                    isReadOnly
                    textAriaLabel="Workspace public key"
                    copyAriaLabel="Copy workspace public key"
                  >
                    {inspection.public_key}
                  </ClipboardCopy>
                  {/^(git@github\.com:|ssh:\/\/git@github\.com(?::[0-9]+)?\/)/.test(
                    project.canonical_url,
                  ) ? (
                    <ExternalLink href="https://github.com/settings/keys">
                      Open GitHub key settings
                    </ExternalLink>
                  ) : (
                    <p>
                      On your Git host, open your account’s SSH key settings and add this public
                      key.
                    </p>
                  )}
                </>
              )}
              {setupError && (
                <p>
                  If access is already configured, check the repository address and connection
                  before retrying.
                </p>
              )}
            </>
          )}
          {!busy && catalogReadError && (
            <Alert
              isInline
              variant="warning"
              title="The project list could not be refreshed"
              role="alert"
            >
              Close this dialog and choose Refresh to check the project list again.
            </Alert>
          )}
          {!busy && diagnostics && (
            <ExpandableSection toggleText="Technical details">
              <pre className="soda-diagnostic">{diagnostics}</pre>
            </ExpandableSection>
          )}
        </Stack>
      </ModalBody>
      <ModalFooter>
        {canSetup && !busy && (
          <Button onClick={() => void setup()}>
            {inspection?.exists ? "Retry setup" : "Set up for me"}
          </Button>
        )}
        {!ready && !busy && (
          <Button variant="secondary" onClick={() => void refresh()}>
            Check setup
          </Button>
        )}
        <Button variant="link" isDisabled={busy} onClick={close}>
          Close
        </Button>
      </ModalFooter>
    </Modal>
  );
}
