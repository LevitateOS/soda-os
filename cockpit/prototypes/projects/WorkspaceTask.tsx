import {
  Alert,
  Button,
  ClipboardCopy,
  Content,
  ExpandableSection,
  Flex,
  Spinner,
  Stack,
  StackItem,
} from "@patternfly/react-core";
import { CodeValue } from "../../src/atoms/CodeValue";

export type WorkspaceState =
  | "absent"
  | "working"
  | "personal-key"
  | "git-key"
  | "unconfirmed"
  | "ready"
  | "unknown";

export const workspaceLabels: Record<WorkspaceState, string> = {
  absent: "Not set up",
  working: "Setting up…",
  "personal-key": "SSH key needed",
  "git-key": "Repository access needed",
  unconfirmed: "Setup not confirmed",
  ready: "Ready",
  unknown: "Outcome unknown",
};

export function WorkspaceTask({
  state,
  projectName,
  checking,
  onRetry,
  onDestination,
}: {
  state: WorkspaceState;
  projectName: string;
  checking: boolean;
  onRetry: () => void;
  onDestination: (destination: string) => void;
}) {
  if (state === "working") {
    return (
      <Flex alignItems={{ default: "alignItemsCenter" }} role="status" aria-live="polite">
        <Spinner size="md" aria-label={checking ? "Checking workspace" : "Setting up workspace"} />
        <span>
          {checking ? "Checking workspace setup" : "Setting up your workspace"} for {projectName}…
        </span>
      </Flex>
    );
  }
  if (state === "ready") {
    return (
      <Stack hasGutter>
        <p>Open a terminal on your computer and run:</p>
        <ClipboardCopy isReadOnly copyAriaLabel="Copy SSH command" textAriaLabel="SSH command">
          ssh soda-w-preview@soda.example
        </ClipboardCopy>
        <p>Your repository is in:</p>
        <CodeValue>~/Projects/website</CodeValue>
        <ExpandableSection toggleText="Development tools">
          <Content>
            <p>
              Use mise inside your workspace to configure tools. Sign in to Tea or GitHub CLI
              separately there when you need them.
            </p>
          </Content>
        </ExpandableSection>
      </Stack>
    );
  }
  if (state === "personal-key") {
    return (
      <Stack hasGutter>
        <Alert isInline variant="info" title="Add your SSH public key first">
          No workspace was created. In Accounts → your account → Authorized public SSH keys, add the
          public key from the computer you will connect from.
        </Alert>
        <Flex>
          <Button
            onClick={() =>
              onDestination("Cockpit Accounts → your account → Authorized public SSH keys")
            }
          >
            Open Accounts
          </Button>
          <Button variant="secondary" onClick={onRetry}>
            Try again
          </Button>
        </Flex>
        <ExpandableSection toggleText="Which key do I need?">
          <p>
            Use your personal public key, not a private key. It lets your computer connect to the
            workspace. The workspace will have a separate key for accessing GitHub.
          </p>
        </ExpandableSection>
      </Stack>
    );
  }
  if (state === "git-key") {
    return (
      <Stack hasGutter>
        <Alert isInline variant="info" title="Give your workspace access to GitHub">
          Your workspace was created, but the repository was not downloaded. Add this public key to
          your GitHub account, then retry setup.
        </Alert>
        <ClipboardCopy
          isReadOnly
          copyAriaLabel="Copy workspace public key"
          textAriaLabel="Example workspace public key"
        >
          ssh-ed25519 PREVIEW-ONLY-DO-NOT-REGISTER soda-workspace-example
        </ClipboardCopy>
        <Flex>
          <Button
            onClick={() => onDestination("GitHub → Settings → SSH and GPG keys → New SSH key")}
          >
            Open GitHub key settings
          </Button>
          <Button variant="secondary" onClick={onRetry}>
            Retry setup
          </Button>
        </Flex>
        <ExpandableSection toggleText="Technical details">
          <CodeValue>Example: Git returned Permission denied (publickey).</CodeValue>
        </ExpandableSection>
      </Stack>
    );
  }
  return (
    <Stack hasGutter>
      <Alert
        isInline
        variant={state === "unknown" ? "warning" : "info"}
        title={
          state === "unknown"
            ? "Check what completed before retrying"
            : "Check your workspace setup"
        }
      >
        {state === "unknown"
          ? "The connection was interrupted. Setup may have changed this workspace; its outcome is not yet known."
          : "Your workspace account exists, but we have not confirmed a complete repository download."}
      </Alert>
      <StackItem>
        <Button onClick={onRetry}>Check setup</Button>
      </StackItem>
      <ExpandableSection toggleText="Technical details">
        <CodeValue>
          {state === "unknown"
            ? "Example: connection closed before the setup response."
            : "Example: account exists; clone inspection has not completed."}
        </CodeValue>
      </ExpandableSection>
    </Stack>
  );
}
