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
  Label,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  PageSection,
  Radio,
  Spinner,
  TextInput,
  Title,
  Toolbar,
  ToolbarContent,
  ToolbarItem,
} from "@patternfly/react-core";
import { Table, Thead, Tbody, Tr, Th, Td } from "@patternfly/react-table";
import type { Invoke, LifecycleAction, ListResponse } from "./types";
import {
  createPayload,
  errorMessage,
  forgejoBrowserURL,
  providerName,
  providerURL,
  statusText,
  successMessage,
} from "./ui";

type Dialog = { kind: "create" } | { kind: "remove"; id: string };
export function Runners({
  invoke,
  hostname = window.location.hostname,
}: {
  invoke: Invoke;
  hostname?: string;
}) {
  const [data, setData] = useState<ListResponse | null>(null);
  const [busy, setBusy] = useState(true),
    [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState<{ message: string; kind: "danger" | "success" } | null>(
    null,
  );
  const [dialog, setDialog] = useState<Dialog | null>(null);
  const pending = useRef(false),
    active = useRef(true);
  const load = useCallback(async () => {
    setLoading(true);
    try {
      const result = await invoke("list", {});
      if (active.current) setData(result);
      return result;
    } catch (error) {
      if (active.current) {
        setData(null);
        setNotice({ message: errorMessage(error), kind: "danger" });
      }
      return null;
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
  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending.current || !event.currentTarget.reportValidity()) return;
    const form = event.currentTarget;
    const token = form.elements.namedItem("registration_token") as HTMLInputElement;
    const payload = createPayload(new FormData(form));
    const id = payload.id;
    pending.current = true;
    setBusy(true);
    try {
      let operation;
      try {
        operation = invoke("create", payload);
      } finally {
        token.value = "";
        payload.registration_token = "";
      }
      await operation;
      if (!active.current) return;
      setDialog(null);
      await load();
      if (active.current) setNotice({ message: successMessage("create", id), kind: "success" });
    } catch (error) {
      if (active.current) setNotice({ message: errorMessage(error), kind: "danger" });
    } finally {
      pending.current = false;
      if (active.current) setBusy(false);
    }
  }
  async function mutate(action: LifecycleAction, id: string) {
    if (pending.current) return;
    pending.current = true;
    setBusy(true);
    try {
      await invoke(action, { id });
      if (!active.current) return;
      const updated = await load();
      if (!active.current) return;
      if (action === "remove" && updated?.runners.every((runner) => runner.id !== id))
        setDialog(null);
      setNotice({ message: successMessage(action, id), kind: "success" });
    } catch (error) {
      if (active.current) setNotice({ message: errorMessage(error), kind: "danger" });
    } finally {
      pending.current = false;
      if (active.current) setBusy(false);
    }
  }
  function remove(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending.current || dialog?.kind !== "remove") return;
    if (new FormData(event.currentTarget).get("confirmation") !== dialog.id) {
      setNotice({
        message: `Type ${dialog.id} exactly to confirm local runner removal.`,
        kind: "danger",
      });
      return;
    }
    void mutate("remove", dialog.id);
  }
  const close = () => {
    if (!pending.current) setDialog(null);
  };
  return (
    <CockpitPageTemplate
      title="Runners"
      busy={busy}
      description="Create and manage generic local capacity for provider-owned CI workflows."
      actions={
        <Toolbar>
          <ToolbarContent>
            <ToolbarItem>
              <Button variant="secondary" isDisabled={busy} onClick={() => void refresh()}>
                Refresh
              </Button>
            </ToolbarItem>
            <ToolbarItem>
              <Button isDisabled={busy} onClick={() => setDialog({ kind: "create" })}>
                Create local runner
              </Button>
            </ToolbarItem>
          </ToolbarContent>
        </Toolbar>
      }
      feedback={notice && <DiagnosticAlert message={notice.message} variant={notice.kind} />}
    >
      <PageSection>
        <Alert isInline variant="warning" title="Runner jobs execute repository code">
          Each local runner has its own unprivileged Linux account, one job slot, no sudo access,
          and no access to human home directories. Its files persist between jobs and it can use the
          network. Run jobs only for repositories and contributors you trust.
        </Alert>
      </PageSection>
      <PageSection aria-labelledby="capacity-title">
        <Title headingLevel="h2" id="capacity-title">
          Local capacity
        </Title>
        <p>
          {loading
            ? "Loading local runner capacity…"
            : data
              ? `${data.runner_count} local ${data.runner_count === 1 ? "runner" : "runners"}; ${data.active_listeners} listening; ${data.total_capacity} configured ${data.total_capacity === 1 ? "slot" : "slots"}.`
              : "Local runner capacity is available only to Soda OS administrators."}
        </p>
        {loading && <Spinner aria-label="Loading local runner capacity" size="lg" />}
        {data && data.runners.length === 0 && (
          <EmptyState titleText="No local runners" headingLevel="h3">
            <EmptyStateBody>
              Create generic Forgejo or GitHub capacity. Provider-hosted runners require no Soda
              configuration.
            </EmptyStateBody>
          </EmptyState>
        )}
        {!!data?.runners.length && (
          <Table aria-label="Local capacity">
            <Thead>
              <Tr>
                <Th>Runner</Th>
                <Th>Provider</Th>
                <Th>Local listener</Th>
                <Th>Capacity</Th>
                <Th>Actions</Th>
              </Tr>
            </Thead>
            <Tbody>
              {data.runners.map((runner) => (
                <Tr key={runner.id}>
                  <Td dataLabel="Runner">
                    <strong>{runner.id}</strong>
                    <code className="soda-code">{runner.account}</code>
                  </Td>
                  <Td dataLabel="Provider">
                    <a
                      href={providerURL(runner.provider, runner.registration_url, hostname)}
                      target="_blank"
                      rel="noreferrer"
                    >
                      {providerName(runner.provider)}
                    </a>
                    <code className="soda-code">{runner.version}</code>
                  </Td>
                  <Td dataLabel="Local listener">
                    <Label
                      color={
                        runner.service.active === "failed"
                          ? "red"
                          : runner.service.active === "active" && runner.service.sub === "running"
                            ? "green"
                            : "grey"
                      }
                    >
                      {statusText(runner.service)}
                    </Label>
                    <code className="soda-code">
                      {runner.service.active}/{runner.service.sub}; {runner.service.enabled}
                    </code>
                  </Td>
                  <Td dataLabel="Capacity">
                    {runner.capacity} slot · {runner.architecture}
                  </Td>
                  <Td dataLabel="Actions">
                    <div className="soda-actions">
                      <Button
                        size="sm"
                        variant={runner.service.active === "active" ? "secondary" : "primary"}
                        isDisabled={busy}
                        onClick={() =>
                          void mutate(
                            runner.service.active === "active" ? "stop" : "start",
                            runner.id,
                          )
                        }
                      >
                        {runner.service.active === "active" ? "Stop" : "Start"}
                      </Button>
                      <Button
                        size="sm"
                        variant="secondary"
                        isDisabled={busy}
                        onClick={() => void mutate("restart", runner.id)}
                      >
                        Restart
                      </Button>
                      <Button
                        size="sm"
                        variant="link"
                        isDanger
                        isDisabled={busy}
                        onClick={() => setDialog({ kind: "remove", id: runner.id })}
                      >
                        Remove
                      </Button>
                    </div>
                  </Td>
                </Tr>
              ))}
            </Tbody>
          </Table>
        )}
      </PageSection>
      <PageSection aria-labelledby="provider-title">
        <Title headingLevel="h2" id="provider-title">
          Provider authority
        </Title>
        <p>
          Forgejo and GitHub own runner registration, tokens, labels, workflows, scheduling,
          results, and history. Soda owns only this machine's account, service, status, capacity,
          and local lifecycle.
        </p>
      </PageSection>
      {dialog?.kind === "create" && (
        <RegistrationDialog
          busy={busy}
          onClose={close}
          onSubmit={create}
          hostname={hostname}
          error={notice?.kind === "danger" ? notice.message : ""}
        />
      )}
      {dialog?.kind === "remove" && (
        <Modal
          isOpen
          variant="medium"
          aria-labelledby="remove-title"
          onClose={busy ? undefined : close}
          onEscapePress={close}
        >
          <ModalHeader
            title="Remove local runner"
            labelId="remove-title"
            description="This permanently stops the listener and deletes its Linux account, provider client state, working files, dependencies, and uncommitted job changes. Its provider record and CI history remain with Forgejo or GitHub; remove the offline record there."
          />
          <ModalBody>
            <Form id="remove-runner" onSubmit={remove}>
              <FormGroup
                label={
                  <>
                    Type <code>{dialog.id}</code> to confirm
                  </>
                }
                fieldId="runner-confirmation"
                isRequired
              >
                <TextInput
                  id="runner-confirmation"
                  name="confirmation"
                  isRequired
                  isDisabled={busy}
                  autoComplete="off"
                />
              </FormGroup>
              {notice?.kind === "danger" && (
                <Alert isInline variant="danger" title={notice.message} />
              )}
            </Form>
          </ModalBody>
          <ModalFooter>
            <Button variant="secondary" isDisabled={busy} onClick={close}>
              Cancel
            </Button>
            <Button variant="danger" type="submit" form="remove-runner" isDisabled={busy}>
              Remove local runner
            </Button>
          </ModalFooter>
        </Modal>
      )}
    </CockpitPageTemplate>
  );
}

function RegistrationDialog({
  busy,
  onClose,
  onSubmit,
  hostname,
  error,
}: {
  busy: boolean;
  onClose: () => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  hostname: string;
  error: string;
}) {
  const [provider, setProvider] = useState("forgejo");
  const token = useRef<HTMLInputElement | null>(null);
  const tokenRef = useCallback((node: HTMLInputElement | null) => {
    if (!node && token.current) token.current.value = "";
    token.current = node;
  }, []);
  // Keep the token solely in the native input; clear even if Cockpit tears down the page.

  return (
    <Modal
      isOpen
      variant="medium"
      aria-labelledby="registration-title"
      onClose={busy ? undefined : onClose}
      onEscapePress={onClose}
    >
      <ModalHeader
        title="Create local runner"
        labelId="registration-title"
        description="The provider creates the registration input. Soda passes it only to that provider's runner."
      />
      <ModalBody>
        <Form id="register-runner" onSubmit={onSubmit}>
          <FormGroup label="Runner ID" fieldId="runner-id" isRequired>
            <TextInput
              id="runner-id"
              name="id"
              isRequired
              isDisabled={busy}
              pattern="[a-z][a-z0-9-]{0,15}"
              maxLength={16}
              autoComplete="off"
              aria-describedby="runner-id-help"
            />
            <p id="runner-id-help">
              A stable lowercase local name. It is also the provider runner name for GitHub.
            </p>
          </FormGroup>
          <FormGroup label="Provider" role="group" fieldId="provider">
            <Radio
              id="forgejo"
              name="provider"
              value="forgejo"
              label="Bundled Forgejo"
              isChecked={provider === "forgejo"}
              isDisabled={busy}
              onChange={() => setProvider("forgejo")}
            />
            <Radio
              id="github"
              name="provider"
              value="github"
              label="GitHub"
              isChecked={provider === "github"}
              isDisabled={busy}
              onChange={() => setProvider("github")}
            />
          </FormGroup>
          <div hidden={provider !== "forgejo"}>
            <p>
              Create a system runner in{" "}
              <a
                href={`${forgejoBrowserURL(hostname)}/admin/actions/runners`}
                target="_blank"
                rel="noreferrer"
              >
                Forgejo runner administration
              </a>
              , then copy its UUID and confidential token here.
            </p>
            <FormGroup
              label="Forgejo runner UUID"
              fieldId="registration-id"
              isRequired={provider === "forgejo"}
            >
              <TextInput
                id="registration-id"
                name="registration_id"
                isRequired={provider === "forgejo"}
                isDisabled={busy}
                pattern="[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"
                autoComplete="off"
              />
            </FormGroup>
            <FormGroup
              label="Native Forgejo host labels"
              fieldId="forgejo-labels"
              isRequired={provider === "forgejo"}
            >
              <TextInput
                id="forgejo-labels"
                name="forgejo_labels"
                defaultValue="soda-linux:host"
                isRequired={provider === "forgejo"}
                isDisabled={busy}
                pattern="[A-Za-z0-9][A-Za-z0-9._-]{0,63}:host(,[A-Za-z0-9][A-Za-z0-9._-]{0,63}:host)*"
                autoComplete="off"
              />
              <p>
                Use comma-separated <code>name:host</code> labels. Host jobs run directly as this
                runner's isolated Linux account.
              </p>
            </FormGroup>
          </div>
          <div hidden={provider !== "github"}>
            <p>
              In the repository, organization, or enterprise Actions settings, choose{" "}
              <strong>New self-hosted runner</strong> and copy the short-lived registration token.
            </p>
            <FormGroup
              label="GitHub registration URL"
              fieldId="registration-url"
              isRequired={provider === "github"}
            >
              <TextInput
                id="registration-url"
                name="registration_url"
                type="url"
                isRequired={provider === "github"}
                isDisabled={busy}
                placeholder="https://github.com/owner/repository"
                autoComplete="off"
              />
            </FormGroup>
            <FormGroup
              label="Custom GitHub labels"
              fieldId="github-labels"
              isRequired={provider === "github"}
            >
              <TextInput
                id="github-labels"
                name="github_labels"
                defaultValue="soda-local"
                isRequired={provider === "github"}
                isDisabled={busy}
                pattern="[A-Za-z0-9][A-Za-z0-9._-]{0,63}(,[A-Za-z0-9][A-Za-z0-9._-]{0,63})*"
                autoComplete="off"
              />
              <p>
                GitHub also adds its native <code>self-hosted</code>, <code>linux</code>, and
                architecture labels.
              </p>
            </FormGroup>
          </div>
          <FormGroup label="Provider registration token" fieldId="registration-token" isRequired>
            <TextInput
              ref={tokenRef}
              id="registration-token"
              name="registration_token"
              type="password"
              isRequired
              isDisabled={busy}
              autoComplete="off"
            />
            <p>
              Used only through the provider's native registration input. Soda descriptors, command
              arguments, environment, and logs never contain it. The provider runner retains the
              credential it needs to reconnect.
            </p>
          </FormGroup>
          {error && <Alert isInline variant="danger" title={error} className="soda-diagnostic" />}
        </Form>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" isDisabled={busy} onClick={onClose}>
          Cancel
        </Button>
        <Button type="submit" form="register-runner" isDisabled={busy} isLoading={busy}>
          Register and start
        </Button>
      </ModalFooter>
    </Modal>
  );
}
