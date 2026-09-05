import { useState } from "react";
import {
  Alert,
  Button,
  ExpandableSection,
  Form,
  FormGroup,
  HelperText,
  HelperTextItem,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  Spinner,
  Stack,
  TextInput,
} from "@patternfly/react-core";
import { CodeValue } from "../atoms/CodeValue";
import type { Invoke, RemovalAction } from "../projects/types";
import { useRemoval } from "../projects/useRemoval";
import { removalAccountLabel, removalCatalogMessage } from "../projects/ui";

const labels = {
  "remove-workspace": "Remove my workspace",
  remove: "Remove project",
  "delete-human": "Remove person",
};

export function ProjectsRemovalDialog({
  action,
  initialTarget,
  viewer,
  invoke,
  catalogReadError,
  onChanged,
  onOutcome,
  onClose,
}: {
  action: RemovalAction;
  initialTarget: string;
  viewer: string;
  invoke: Invoke;
  catalogReadError: string;
  onChanged: () => Promise<void>;
  onOutcome: (message: string, success: boolean) => void;
  onClose: () => void;
}) {
  const [target, setTarget] = useState(initialTarget);
  const [confirmation, setConfirmation] = useState("");
  const [touched, setTouched] = useState(false);
  const [reviewing, setReviewing] = useState(false);
  const {
    preview,
    receipt,
    selectedAccounts,
    readError,
    unknown,
    operation,
    inspect,
    remove,
    invalidate,
  } = useRemoval(invoke, action, initialTarget, onChanged, onOutcome);
  const busy = operation !== null;
  const previousFailedAccount = selectedAccounts.find(
    (account) => account.username === receipt?.result.uncertain,
  );
  const failedIdentityChanged = Boolean(
    previousFailedAccount &&
    preview &&
    !preview.accounts.some(
      (account) =>
        account.username === previousFailedAccount.username &&
        account.uid === previousFailedAccount.uid &&
        account.home === previousFailedAccount.home,
    ),
  );
  const hasTargets = Boolean(
    preview && (preview.accounts.length || (action === "remove" && preview.catalog_present)),
  );
  const showSelection = reviewing || (!receipt && !unknown);
  const orphaned = Boolean(
    action === "remove" && preview && !preview.catalog_present && preview.accounts.length,
  );
  const canConfirm =
    preview && hasTargets && !failedIdentityChanged && !orphaned && !receipt?.ok && showSelection;
  const matches = confirmation === preview?.target;
  const diagnostics = [receipt?.result.diagnostic, unknown, readError, catalogReadError]
    .filter(Boolean)
    .join("\n\n");
  const people = [
    ...new Set(
      preview?.accounts
        .filter((account) => account.project_id)
        .map((account) => account.primary_username),
    ),
  ];
  function close() {
    if (!busy) onClose();
  }
  function check() {
    setReviewing(true);
    setConfirmation("");
    setTouched(false);
    void inspect(target);
  }
  return (
    <Modal
      isOpen
      variant="medium"
      aria-labelledby="removal-title"
      onClose={busy ? undefined : close}
      onEscapePress={close}
    >
      <ModalHeader
        title={
          receipt?.ok ? "Removal completed" : `${labels[action]}${target ? ` — ${target}` : ""}`
        }
        labelId="removal-title"
      />
      <ModalBody tabIndex={0} role="group" aria-label="Removal details">
        <Stack hasGutter>
          {busy ? (
            <p role="status">
              <Spinner size="md" aria-hidden />{" "}
              {operation === "inspect"
                ? "Checking affected accounts…"
                : `${labels[action]} in progress…`}
            </p>
          ) : (
            <>
              {action === "delete-human" && !preview && !receipt && !unknown && (
                <Form
                  id="inspect-person"
                  onSubmit={(event) => {
                    event.preventDefault();
                    check();
                  }}
                >
                  <FormGroup label="Primary username" fieldId="primary-username" isRequired>
                    <TextInput
                      id="primary-username"
                      value={target}
                      onChange={(_, value) => {
                        setTarget(value);
                        setConfirmation("");
                        invalidate();
                      }}
                      isRequired
                      pattern="[a-z][a-z0-9-]{0,23}"
                      maxLength={24}
                      autoComplete="off"
                    />
                  </FormGroup>
                  <Button variant="secondary" type="submit">
                    Check affected accounts
                  </Button>
                </Form>
              )}
              {unknown && (
                <Alert
                  isInline
                  variant="warning"
                  title="Removal outcome is not confirmed"
                  role="alert"
                >
                  Some local data may already have been permanently deleted. Check current state
                  before retrying; a disconnected request does not prove success or failure.
                </Alert>
              )}
              {receipt && (
                <Alert
                  isInline
                  variant={receipt.ok ? "success" : "warning"}
                  title={receipt.ok ? "Native removal completed" : "Removal is incomplete"}
                  role="alert"
                >
                  <p>
                    {receipt.result.removed.length} account
                    {receipt.result.removed.length === 1 ? "" : "s"} removed;{" "}
                    {receipt.result.not_attempted.length} not attempted.
                  </p>
                  {receipt.problem && <p>{receipt.problem}</p>}
                  {receipt.result.uncertain && (
                    <p>
                      Deletion of {removalAccountLabel(receipt.result.uncertain, selectedAccounts)}{" "}
                      did not finish. Its processes or local files may already have been changed or
                      deleted.
                    </p>
                  )}
                  {failedIdentityChanged && (
                    <p>
                      <strong>Inspect remaining local data before continuing</strong>. The failed
                      account is no longer listed with its earlier identity and home. That does not
                      prove all of its files were removed. An administrator needs to inspect its
                      previous home, shown in Account removal results.
                    </p>
                  )}
                  {action === "remove" && <p>{removalCatalogMessage(receipt.catalog)}</p>}
                  <p>Git-host accounts and repositories were not deleted.</p>
                </Alert>
              )}
              {receipt && (
                <ExpandableSection toggleText="Account removal results">
                  {receipt.result.uncertain && (
                    <p>
                      Unconfirmed account’s home:{" "}
                      <CodeValue>
                        {selectedAccounts.find(
                          (account) => account.username === receipt.result.uncertain,
                        )?.home ?? "not available"}
                      </CodeValue>
                    </p>
                  )}
                  <p>
                    Confirmed removed:{" "}
                    {receipt.result.removed
                      .map((name) => removalAccountLabel(name, selectedAccounts))
                      .join(", ") || "none"}
                    .
                  </p>
                  <p>
                    Not attempted:{" "}
                    {receipt.result.not_attempted
                      .map((name) => removalAccountLabel(name, selectedAccounts))
                      .join(", ") || "none"}
                    .
                  </p>
                </ExpandableSection>
              )}
              {readError && (
                <Alert
                  isInline
                  variant="warning"
                  title="Affected accounts could not be checked"
                  role="alert"
                >
                  Removal is disabled. Ask an administrator to inspect the account or local files,
                  then check again.
                </Alert>
              )}
              {preview && !receipt?.ok && showSelection && (
                <>
                  <p>
                    <strong>Current selection: {preview.target}</strong>
                  </p>
                  {action === "delete-human" && !receipt && !unknown && (
                    <Button
                      variant="link"
                      isInline
                      onClick={() => {
                        setConfirmation("");
                        invalidate();
                      }}
                    >
                      Change person
                    </Button>
                  )}
                  <p>
                    {people.length
                      ? `Local workspaces belonging to ${people.join(", ")}.`
                      : "No local workspace accounts are in the current selection."}{" "}
                    {action === "delete-human" && "The primary Linux account will be removed last."}
                  </p>
                  <ExpandableSection
                    toggleText={`Affected accounts and local folders (${preview.accounts.length})`}
                  >
                    <ul>
                      {preview.accounts.map((account) => (
                        <li key={account.username}>
                          <strong>
                            {account.primary_username}
                            {account.project_id ? ` / ${account.project_id}` : " (primary account)"}
                          </strong>
                          <br />
                          <CodeValue>{account.username}</CodeValue> (UID {account.uid})
                          <CodeValue>{account.home}</CodeValue>
                        </li>
                      ))}
                    </ul>
                  </ExpandableSection>
                  {orphaned && (
                    <Alert isInline variant="warning" title="The project entry is absent">
                      An administrator needs to inspect orphaned workspace accounts before cleanup.
                    </Alert>
                  )}
                  {!hasTargets && (
                    <p>
                      No accounts or project entry remain in this selection. This check does not
                      verify residual files from earlier deletion attempts.
                    </p>
                  )}
                </>
              )}
              {canConfirm && (
                <Form
                  id="confirm-removal"
                  onSubmit={(event) => {
                    event.preventDefault();
                    if (!matches) return;
                    setConfirmation("");
                    setReviewing(false);
                    void remove();
                  }}
                >
                  <p>
                    Permanently delete the selected local accounts and their homes, including
                    unpushed work. Running tasks will stop. Save, push, or copy needed work first.
                    There is no undo.
                  </p>
                  <p>
                    {action === "remove-workspace"
                      ? "Other people’s workspaces and the shared project stay. "
                      : action === "remove"
                        ? "The shared project entry will also be removed. "
                        : "Forgejo account deletion is separate, inside Forgejo. "}
                    The repository on the Git host stays.
                  </p>
                  {action === "delete-human" && target === viewer && (
                    <Alert
                      isInline
                      variant="warning"
                      title="You are removing your signed-in account"
                    >
                      This may end your Cockpit session. Another administrator will need to check
                      the result if the connection closes.
                    </Alert>
                  )}
                  <FormGroup
                    label={`Type ${preview.target} to confirm`}
                    fieldId="removal-confirmation"
                    isRequired
                  >
                    <TextInput
                      id="removal-confirmation"
                      value={confirmation}
                      onChange={(_, value) => setConfirmation(value)}
                      onBlur={() => setTouched(true)}
                      autoComplete="off"
                      aria-invalid={touched && !matches}
                      validated={touched && !matches ? "error" : "default"}
                      aria-describedby="removal-confirmation-help"
                    />
                    <HelperText>
                      <HelperTextItem
                        id="removal-confirmation-help"
                        variant={touched && !matches ? "error" : "default"}
                      >
                        Enter {preview.target} exactly to enable removal.
                      </HelperTextItem>
                    </HelperText>
                  </FormGroup>
                </Form>
              )}
              {catalogReadError && (
                <Alert
                  isInline
                  variant="warning"
                  title="The project list could not be refreshed"
                  role="alert"
                >
                  Close this dialog and choose Refresh to check the list again.
                </Alert>
              )}
              {(readError || failedIdentityChanged || receipt?.result.uncertain) && (
                <Button variant="secondary" onClick={() => window.cockpit.jump("/users")}>
                  Open Accounts
                </Button>
              )}
              {diagnostics && (
                <ExpandableSection toggleText="Technical details">
                  <pre className="soda-diagnostic">{diagnostics}</pre>
                </ExpandableSection>
              )}
            </>
          )}
        </Stack>
      </ModalBody>
      <ModalFooter>
        {canConfirm && !busy && (
          <Button variant="danger" type="submit" form="confirm-removal" isDisabled={!matches}>
            {labels[action]}
          </Button>
        )}
        {!busy && target && !receipt?.ok && (
          <Button variant="secondary" onClick={check}>
            {receipt && preview && !failedIdentityChanged && !reviewing
              ? "Review remaining removal"
              : "Check current state"}
          </Button>
        )}
        <Button variant="link" isDisabled={busy} onClick={close}>
          {receipt || unknown || readError ? "Close" : "Cancel"}
        </Button>
      </ModalFooter>
    </Modal>
  );
}
