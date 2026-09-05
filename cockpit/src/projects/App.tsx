import { CockpitPageTemplate } from "../templates/CockpitPageTemplate";
import { DiagnosticAlert } from "../molecules/DiagnosticAlert";
import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import {
  Alert,
  Button,
  EmptyState,
  EmptyStateBody,
  Form,
  FormGroup,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  PageSection,
  Spinner,
  TextArea,
  TextInput,
  Title,
  Toolbar,
  ToolbarContent,
  ToolbarItem,
} from "@patternfly/react-core";
import { Table, Thead, Tbody, Tr, Th, Td } from "@patternfly/react-table";
import type { FormAction, Invoke, ListResponse, Project } from "./types";
import {
  errorMessage,
  humanDeletionHidden,
  payloadFor,
  projectRemovalHidden,
  sshCommand,
  successMessage,
} from "./ui";

const dialogs = {
  "add-existing": [
    "Add an existing repository",
    "The repository URL is stored without credentials.",
    "Add repository",
  ],
  edit: [
    "Edit project",
    "The project ID and canonical Git URL remain unchanged. Display-name and metadata edits affect future setup only.",
    "Save changes",
  ],
  setup: [
    "Set up for me",
    "Creates your derived workspace and clones the repository through native SSH.",
    "Set up for me",
  ],
  "remove-workspace": [
    "Remove my workspace",
    "This permanently removes your workspace account, home, independent clone, dependencies, processes, project state, and uncommitted work. The shared project, other workspaces, and canonical repository are not deleted.",
    "Remove my workspace",
  ],
  remove: [
    "Remove project from Soda",
    "This permanently removes all local workspace accounts, homes, clones, dependencies, and uncommitted work for this project. The canonical repository is not deleted.",
    "Remove project",
  ],
  "delete-human": [
    "Remove person from Soda OS",
    "This permanently removes the person’s local Soda workspaces, then their primary Linux account. Their Forgejo account and repository data are unchanged. Delete a Forgejo account separately in Forgejo.",
    "Remove person",
  ],
} as const;
type Dialog = { action: FormAction; project?: Project };
type Notice = { message: string; kind: "danger" | "success" };

export function Projects({
  invoke,
  hostname = window.location.hostname,
}: {
  invoke: Invoke;
  hostname?: string;
}) {
  const [data, setData] = useState<ListResponse | null>(null);
  const [busy, setBusy] = useState(true);
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState<Notice | null>(null);
  const [dialog, setDialog] = useState<Dialog | null>(null);
  const [formError, setFormError] = useState("");
  const pending = useRef(false);
  const active = useRef(true);
  const load = useCallback(async () => {
    setLoading(true);
    try {
      const result = await invoke("list", {});
      if (active.current) setData(result);
    } catch (error) {
      if (active.current) {
        setData(null);
        setNotice({ message: errorMessage(error), kind: "danger" });
      }
    } finally {
      if (active.current) setLoading(false);
    }
  }, [invoke]);
  const refresh = useCallback(async () => {
    if (pending.current) return;
    pending.current = true;
    setBusy(true);
    try {
      await load();
    } finally {
      pending.current = false;
      if (active.current) setBusy(false);
    }
  }, [load]);
  useEffect(() => {
    active.current = true;
    void refresh();
    return () => {
      active.current = false;
    };
  }, [refresh]);
  function open(action: FormAction, project?: Project) {
    if (pending.current) return;
    setFormError("");
    setDialog({ action, project });
  }
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!dialog || pending.current || !event.currentTarget.reportValidity()) return;
    const { action } = dialog;
    const payload = payloadFor(action, new FormData(event.currentTarget), (message) =>
      setNotice({ message, kind: "danger" }),
    );
    if (!payload) return;
    pending.current = true;
    setBusy(true);
    try {
      const result = await invoke(action, payload);
      if (!active.current) return;
      setDialog(null);
      const message = successMessage(action, payload, result);
      await load();
      if (active.current) setNotice({ message, kind: "success" });
    } catch (error) {
      const message = errorMessage(error);
      if (action === "setup") await load();
      if (active.current) {
        setFormError(message);
        setNotice({ message, kind: "danger" });
      }
    } finally {
      pending.current = false;
      if (active.current) setBusy(false);
    }
  }
  return (
    <CockpitPageTemplate
      title="Projects"
      busy={busy}
      description="Catalog repositories and create an isolated Linux workspace for each person."
      actions={
        <Toolbar>
          <ToolbarContent>
            <ToolbarItem>
              <Button variant="secondary" isDisabled={busy} onClick={() => void refresh()}>
                Refresh
              </Button>
            </ToolbarItem>
            <ToolbarItem>
              <Button isDisabled={busy} onClick={() => open("add-existing")}>
                Add repository
              </Button>
            </ToolbarItem>
          </ToolbarContent>
        </Toolbar>
      }
      feedback={notice && <DiagnosticAlert message={notice.message} variant={notice.kind} />}
    >
      <PageSection aria-labelledby="catalog-title">
        <Title headingLevel="h2" id="catalog-title">
          Project catalog
        </Title>
        <p>
          {loading
            ? "Loading projects…"
            : data
              ? `${data.projects.length} ${data.projects.length === 1 ? "project" : "projects"} available to ${data.current_user.username}.`
              : "The project catalog could not be loaded."}
        </p>
        {loading && <Spinner aria-label="Loading projects" size="lg" />}
        {data && data.projects.length === 0 && (
          <EmptyState titleText="No projects yet" headingLevel="h3">
            <EmptyStateBody>
              Create a repository in Forgejo or another Git host, then add its SSH clone URL here.
            </EmptyStateBody>
          </EmptyState>
        )}
        {!!data?.projects.length && (
          <Table aria-label="Project catalog">
            <Thead>
              <Tr>
                <Th>Project</Th>
                <Th>Canonical repository</Th>
                <Th>Your workspace</Th>
                <Th>Actions</Th>
              </Tr>
            </Thead>
            <Tbody>
              {data.projects.map((project) => (
                <Tr key={project.id}>
                  <Td dataLabel="Project">
                    <strong>{project.display_name}</strong>
                    <code className="soda-code">{project.id}</code>
                  </Td>
                  <Td dataLabel="Canonical repository">
                    <code className="soda-code">{project.canonical_url}</code>
                  </Td>
                  <Td dataLabel="Your workspace">
                    <span>
                      {project.workspace_exists
                        ? "Workspace account exists"
                        : "No workspace account"}
                    </span>
                    <code className="soda-code">
                      {sshCommand(project.workspace_username, hostname)}
                    </code>
                  </Td>
                  <Td dataLabel="Actions">
                    <div className="soda-actions">
                      <Button size="sm" isDisabled={busy} onClick={() => open("setup", project)}>
                        Set up for me
                      </Button>
                      {project.workspace_exists && (
                        <Button
                          size="sm"
                          variant="link"
                          isDanger
                          isDisabled={busy}
                          onClick={() => open("remove-workspace", project)}
                        >
                          Remove my workspace
                        </Button>
                      )}
                      <Button
                        size="sm"
                        variant="secondary"
                        isDisabled={busy}
                        onClick={() => open("edit", project)}
                      >
                        Edit
                      </Button>
                      {!projectRemovalHidden(data.current_user) && (
                        <Button
                          size="sm"
                          variant="link"
                          isDanger
                          isDisabled={busy}
                          onClick={() => open("remove", project)}
                        >
                          Remove project
                        </Button>
                      )}
                    </div>
                  </Td>
                </Tr>
              ))}
            </Tbody>
          </Table>
        )}
      </PageSection>
      {data && !humanDeletionHidden(data.current_user) && (
        <PageSection aria-labelledby="people-title">
          <Title headingLevel="h2" id="people-title">
            People
          </Title>
          <p>
            Stock Cockpit Accounts creates and lists primary Linux users and owns administrator
            status. Administrators can use this Soda-aware action to remove one: it deletes local
            Soda workspaces, then the primary Linux account. Forgejo account deletion remains
            separate inside Forgejo.
          </p>
          <Button variant="danger" isDisabled={busy} onClick={() => open("delete-human")}>
            Remove person…
          </Button>
        </PageSection>
      )}
      {dialog && (
        <ProjectDialog
          key={dialog.action + (dialog.project?.id ?? "")}
          dialog={dialog}
          busy={busy}
          error={formError}
          onClose={() => {
            if (!pending.current) setDialog(null);
          }}
          onSubmit={submit}
        />
      )}
    </CockpitPageTemplate>
  );
}

function ProjectDialog({
  dialog: { action, project },
  busy,
  error,
  onClose,
  onSubmit,
}: {
  dialog: Dialog;
  busy: boolean;
  error: string;
  onClose: () => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
}) {
  const [title, description, button] = dialogs[action];
  return (
    <Modal
      isOpen
      variant="medium"
      aria-labelledby="project-dialog-title"
      aria-describedby="project-dialog-description"
      onClose={busy ? undefined : onClose}
      onEscapePress={onClose}
    >
      <ModalHeader
        title={title}
        labelId="project-dialog-title"
        description={description}
        descriptorId="project-dialog-description"
      />
      <ModalBody>
        <Form id="project-action" onSubmit={onSubmit}>
          {action !== "add-existing" && action !== "delete-human" && (
            <input type="hidden" name="id" value={project?.id ?? ""} />
          )}
          {action === "add-existing" && (
            <FormGroup label="Project ID" fieldId="project-id" isRequired>
              <TextInput
                id="project-id"
                name="id"
                isRequired
                isDisabled={busy}
                pattern="[a-z][a-z0-9-]{0,23}"
                maxLength={24}
                autoComplete="off"
                aria-describedby="project-id-help"
              />
              <p id="project-id-help">A stable lowercase name used for workspace paths.</p>
            </FormGroup>
          )}
          {action === "edit" && (
            <p>
              Project ID: <code>{project?.id}</code>
            </p>
          )}
          {(action === "add-existing" || action === "edit") && (
            <>
              <FormGroup label="Display name" fieldId="display-name" isRequired>
                <TextInput
                  id="display-name"
                  name="display_name"
                  defaultValue={project?.display_name ?? ""}
                  isRequired
                  isDisabled={busy}
                  autoComplete="off"
                />
              </FormGroup>
              <FormGroup
                label="Canonical Git URL"
                fieldId="canonical-url"
                isRequired={action === "add-existing"}
              >
                <TextInput
                  id="canonical-url"
                  name="canonical_url"
                  defaultValue={project?.canonical_url ?? ""}
                  readOnlyVariant={action === "edit" ? "default" : undefined}
                  isRequired={action === "add-existing"}
                  isDisabled={busy}
                  inputMode="url"
                  autoComplete="off"
                  placeholder="git@example.test:team/project.git"
                  aria-describedby={action === "edit" ? "canonical-url-help" : undefined}
                />
                {action === "edit" && (
                  <p id="canonical-url-help">
                    To replace this URL, an administrator must remove the project and its local
                    workspaces, then add the project again with the replacement SSH URL. Removing
                    the project does not delete the canonical repository.
                  </p>
                )}
              </FormGroup>
              <FormGroup label="Additional metadata" fieldId="additional-metadata">
                <TextArea
                  id="additional-metadata"
                  name="additional_metadata"
                  rows={4}
                  defaultValue={project ? JSON.stringify(project.catalog_metadata, null, 2) : ""}
                  isDisabled={busy}
                  placeholder={'{"team":"web"}'}
                />
                <p>{action === "add-existing" ? "Optional JSON object" : "JSON object"}</p>
              </FormGroup>
            </>
          )}
          {action === "setup" && (
            <>
              <p>
                Project: <strong>{project?.display_name}</strong>
              </p>
              <p>
                Soda creates an SSH keypair only in this workspace and uses it for this clone. The
                authoritative Git host owns access. If clone authentication fails, Soda reports the
                public key for you to register with that host before retrying. Configure tools with{" "}
                <code>mise</code> and authenticate Tea and GitHub CLI directly inside the workspace.
              </p>
            </>
          )}
          {(action === "remove" || action === "remove-workspace") && (
            <FormGroup
              label={
                <>
                  Type <code>{project?.id}</code> to confirm
                </>
              }
              fieldId="project-confirmation"
              isRequired
            >
              <TextInput
                id="project-confirmation"
                name="confirmation"
                isRequired
                isDisabled={busy}
                autoComplete="off"
              />
            </FormGroup>
          )}
          {action === "delete-human" && (
            <>
              <FormGroup label="Primary username" fieldId="primary-username" isRequired>
                <TextInput
                  id="primary-username"
                  name="username"
                  isRequired
                  isDisabled={busy}
                  pattern="[a-z][a-z0-9-]{0,23}"
                  maxLength={24}
                  autoComplete="off"
                />
              </FormGroup>
              <FormGroup
                label="Re-enter the username to confirm"
                fieldId="human-confirmation"
                isRequired
              >
                <TextInput
                  id="human-confirmation"
                  name="confirmation"
                  isRequired
                  isDisabled={busy}
                  autoComplete="off"
                />
              </FormGroup>
            </>
          )}
          {error && (
            <Alert
              isInline
              variant="danger"
              title={error}
              role="alert"
              className="soda-diagnostic"
            />
          )}
        </Form>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" isDisabled={busy} onClick={onClose}>
          Cancel
        </Button>
        <Button
          variant={
            ["remove", "remove-workspace", "delete-human"].includes(action) ? "danger" : "primary"
          }
          type="submit"
          form="project-action"
          isDisabled={busy}
          isLoading={busy}
        >
          {button}
        </Button>
      </ModalFooter>
    </Modal>
  );
}
