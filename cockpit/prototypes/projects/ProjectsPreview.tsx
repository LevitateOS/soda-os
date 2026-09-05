import { useEffect, useState } from "react";
import {
  Alert,
  Button,
  Dropdown,
  DropdownItem,
  DropdownList,
  EmptyState,
  EmptyStateActions,
  EmptyStateBody,
  EmptyStateFooter,
  Flex,
  FlexItem,
  FormSelect,
  FormSelectOption,
  Label,
  MenuToggle,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  Page,
  PageSection,
  Stack,
} from "@patternfly/react-core";
import { Table, Thead, Tbody, Tr, Th, Td } from "@patternfly/react-table";
import { PageHeading } from "../../src/molecules/PageHeading";
import { RepositoryForm, exampleProject } from "./RepositoryForm";
import { RemovalDialog } from "./RemovalDialog";
import { WorkspaceTask, workspaceLabels, type WorkspaceState } from "./WorkspaceTask";

export const previewMarker = "SODA_PROJECTS_DESIGN_PREVIEW";
export type Scenario = WorkspaceState | "empty" | "partial-removal";
const scenarios: Record<Scenario, string> = {
  empty: "Empty catalog",
  absent: "Not set up",
  working: "Working",
  "personal-key": "Personal key needed",
  "git-key": "Git access needed",
  unconfirmed: "Account exists; clone unconfirmed",
  ready: "Ready to connect",
  unknown: "Outcome unknown",
  "partial-removal": "Partial project removal",
};
type Dialog = "add" | "edit" | "workspace" | "remove-workspace" | "remove-project" | null;

export function ProjectsPreview({ initialScenario = "absent" }: { initialScenario?: Scenario }) {
  const [scenario, setScenario] = useState<Scenario>(initialScenario);
  const [project, setProject] = useState(exampleProject);
  const [dialog, setDialog] = useState<Dialog>(null);
  const [menuOpen, setMenuOpen] = useState(false);
  const [destination, setDestination] = useState("");
  const [nextResult, setNextResult] = useState<WorkspaceState>("git-key");
  const [notice, setNotice] = useState("");
  const [checking, setChecking] = useState(false);
  const busy = scenario === "working";
  const state: WorkspaceState =
    scenario === "empty" || scenario === "partial-removal" ? "absent" : scenario;
  const statusLabel =
    scenario === "partial-removal"
      ? "Your workspace removed"
      : busy && checking
        ? "Checking setup…"
        : workspaceLabels[state];

  useEffect(() => {
    if (!busy) return;
    const timer = setTimeout(() => setScenario(nextResult), 1800);
    return () => clearTimeout(timer);
  }, [busy, nextResult]);

  function selectScenario(value: Scenario) {
    setChecking(false);
    setNextResult("git-key");
    setScenario(value);
    setDialog(null);
    setMenuOpen(false);
    setNotice("");
    setDestination("");
  }
  function startSetup() {
    setChecking(state === "unconfirmed" || state === "unknown");
    setDestination("");
    setNextResult(
      state === "absent" ? "personal-key" : state === "personal-key" ? "git-key" : "ready",
    );
    setNotice("");
    setScenario("working");
    setDialog("workspace");
  }
  function openTask() {
    setDestination("");
    if (state === "absent") startSetup();
    else setDialog("workspace");
  }
  function manage(action: Exclude<Dialog, null>) {
    setDestination("");
    setMenuOpen(false);
    setDialog(action);
  }
  const taskLabel =
    state === "ready"
      ? "Connection details"
      : state === "absent"
        ? "Set up for me"
        : "Review setup";
  return (
    <Page
      sidebar={null}
      className="soda-page"
      mainAriaLabel="Projects design preview"
      data-preview={previewMarker}
    >
      <PageSection aria-label="Design preview controls" variant="secondary">
        <Stack hasGutter>
          <Flex alignItems={{ default: "alignItemsCenter" }}>
            <FlexItem>
              <strong>Design preview</strong> · Simulated data only
            </FlexItem>
            <FlexItem>
              <FormSelect
                aria-label="Preview scenario"
                value={scenario}
                onChange={(_, value) => selectScenario(value as Scenario)}
              >
                {Object.entries(scenarios).map(([value, label]) => (
                  <FormSelectOption key={value} value={value} label={label} />
                ))}
              </FormSelect>
            </FlexItem>
            <FlexItem>
              <Button
                variant="link"
                onClick={() => {
                  setProject(exampleProject);
                  selectScenario("absent");
                }}
              >
                Reset preview
              </Button>
            </FlexItem>
          </Flex>
          <small>
            No real accounts, repositories, or keys are changed. Destination buttons stay in this
            preview.
          </small>
          {destination && !dialog && (
            <p role="status">
              Preview destination: {destination}. No external action was performed.
            </p>
          )}
        </Stack>
      </PageSection>
      <PageSection>
        <PageHeading
          title="Projects"
          description="Choose a project and set up your own workspace."
        />
      </PageSection>
      <PageSection aria-label="Project list">
        <Stack hasGutter>
          {notice && <Alert isInline variant="success" title={notice} role="status" />}
          {scenario === "partial-removal" && dialog !== "remove-project" && (
            <Alert isInline variant="warning" title="Project removal is incomplete">
              Your workspace was deleted. Bob’s workspace and the project entry remain.{" "}
              <Button variant="link" isInline onClick={() => setDialog("remove-project")}>
                Review remaining work
              </Button>
            </Alert>
          )}
          {scenario === "empty" ? (
            <EmptyState titleText="No projects yet" headingLevel="h2">
              <EmptyStateBody>
                Add a repository from GitHub, Forgejo, or another Git host.
              </EmptyStateBody>
              <EmptyStateFooter>
                <EmptyStateActions>
                  <Button onClick={() => setDialog("add")}>Add repository</Button>
                </EmptyStateActions>
              </EmptyStateFooter>
            </EmptyState>
          ) : (
            <>
              <Flex>
                <Button isDisabled={busy} onClick={() => setDialog("add")}>
                  Add repository
                </Button>
              </Flex>
              <Table aria-label="Projects">
                <Thead>
                  <Tr>
                    <Th>Project</Th>
                    <Th>Your workspace</Th>
                    <Th>Actions</Th>
                  </Tr>
                </Thead>
                <Tbody>
                  <Tr>
                    <Td dataLabel="Project">
                      <strong>{project.name}</strong>
                    </Td>
                    <Td dataLabel="Your workspace">
                      <Label
                        color={
                          state === "ready" ? "green" : state === "unknown" ? "orange" : "grey"
                        }
                      >
                        {statusLabel}
                      </Label>
                    </Td>
                    <Td dataLabel="Actions">
                      <Flex>
                        <Button
                          variant="secondary"
                          isDisabled={busy}
                          onClick={openTask}
                          aria-label={`${taskLabel} — ${project.name}`}
                        >
                          {taskLabel}
                        </Button>
                        <Dropdown
                          isOpen={menuOpen}
                          onOpenChange={setMenuOpen}
                          toggle={(ref) => (
                            <MenuToggle
                              ref={ref}
                              variant="plainText"
                              isExpanded={menuOpen}
                              isDisabled={busy}
                              aria-label={`Actions — ${project.name}`}
                              onClick={() => setMenuOpen(!menuOpen)}
                            >
                              Actions
                            </MenuToggle>
                          )}
                        >
                          <DropdownList>
                            <DropdownItem onClick={() => manage("edit")}>Edit project</DropdownItem>
                            {!["absent", "personal-key"].includes(state) && (
                              <DropdownItem onClick={() => manage("remove-workspace")}>
                                Remove my workspace
                              </DropdownItem>
                            )}
                            <DropdownItem onClick={() => manage("remove-project")}>
                              Remove project
                            </DropdownItem>
                          </DropdownList>
                        </Dropdown>
                      </Flex>
                    </Td>
                  </Tr>
                </Tbody>
              </Table>
            </>
          )}
        </Stack>
      </PageSection>
      <PageSection>
        <Button variant="link" isInline onClick={() => setDestination("Cockpit Accounts")}>
          Manage people in Accounts
        </Button>
      </PageSection>
      {(dialog === "add" || dialog === "edit") && (
        <RepositoryForm
          project={dialog === "edit" ? project : undefined}
          onClose={() => setDialog(null)}
          onSave={(value) => {
            setProject(value);
            if (dialog === "add") setScenario("absent");
            setDialog(null);
            setNotice(
              dialog === "add"
                ? "Repository added. Choose Set up for me to start working."
                : "Project details saved.",
            );
          }}
        />
      )}
      {dialog === "workspace" && (
        <Modal
          isOpen
          variant="medium"
          aria-labelledby="workspace-title"
          onClose={busy ? undefined : () => setDialog(null)}
        >
          <ModalHeader
            title={state === "ready" ? `Connect to ${project.name}` : `Set up ${project.name}`}
            labelId="workspace-title"
          />
          {/* Keep a stable focus target when the working view is replaced by its outcome. */}
          <ModalBody tabIndex={0} role="group" aria-label={statusLabel}>
            <WorkspaceTask
              state={state}
              checking={checking}
              projectName={project.name}
              onRetry={startSetup}
              onDestination={setDestination}
            />
            {destination && (
              <p role="status">
                Preview destination: {destination}. No external action was performed.
              </p>
            )}
          </ModalBody>
          <ModalFooter>
            <Button variant="link" isDisabled={busy} onClick={() => setDialog(null)}>
              Close
            </Button>
          </ModalFooter>
        </Modal>
      )}
      {(dialog === "remove-project" || dialog === "remove-workspace") && (
        <RemovalDialog
          project={project}
          scope={dialog === "remove-workspace" ? "workspace" : "project"}
          partial={scenario === "partial-removal"}
          hasWorkspace={!["absent", "personal-key"].includes(state)}
          destination={destination}
          onClose={() => setDialog(null)}
          onDestination={setDestination}
          onRemove={() => {
            if (dialog === "remove-project" && !["absent", "personal-key"].includes(state)) {
              setScenario("partial-removal");
            } else if (dialog === "remove-project") {
              setScenario("empty");
              setDialog(null);
              setNotice("Project and local workspaces removed. The Git-host repository stays.");
            } else {
              setScenario("absent");
              setDialog(null);
              setNotice("Your workspace was removed. The project and Git-host repository stay.");
            }
          }}
        />
      )}
    </Page>
  );
}
