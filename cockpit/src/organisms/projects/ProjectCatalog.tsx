import { EmptyState, EmptyStateBody, PageSection, Spinner, Stack } from "@patternfly/react-core";
import { Table, Thead, Tbody, Tr, Th, Td } from "@patternfly/react-table";
import { ProjectActions } from "../../molecules/projects/ProjectActions";
import { WorkspaceSummary } from "../../molecules/projects/WorkspaceSummary";
import type {
  ProjectAction,
  ListResponse,
  Project,
  WorkspaceInspection,
} from "../../projects/types";

export function ProjectCatalog({
  data,
  inspections,
  loading,
  busy,
  onAction,
}: {
  data: ListResponse | null;
  inspections: Record<string, WorkspaceInspection>;
  loading: boolean;
  busy: boolean;
  onAction: (action: ProjectAction, project: Project) => void;
}) {
  return (
    <PageSection aria-label="Project list">
      <Stack hasGutter>
        <p role="status" className="pf-v6-screen-reader">
          {loading
            ? "Loading projects…"
            : data
              ? `${data.projects.length} ${data.projects.length === 1 ? "project" : "projects"} available to ${data.current_user.username}.`
              : "The project catalog could not be loaded."}
        </p>
        {loading && <Spinner aria-label="Loading projects" size="lg" />}
        {data && data.projects.length === 0 && (
          <EmptyState titleText="No projects yet" headingLevel="h2">
            <EmptyStateBody>
              Create a repository on your Git host, then choose Add repository.
            </EmptyStateBody>
          </EmptyState>
        )}
        {!!data?.projects.length && (
          <Table aria-label="Projects">
            <Thead>
              <Tr>
                <Th>Project</Th>
                <Th>Your workspace</Th>
                <Th>Actions</Th>
              </Tr>
            </Thead>
            <Tbody>
              {data.projects.map((project) => (
                <Tr key={project.id}>
                  <Td dataLabel="Project">
                    <strong>{project.display_name}</strong>
                  </Td>
                  <Td dataLabel="Your workspace">
                    <WorkspaceSummary project={project} inspection={inspections[project.id]} />
                  </Td>
                  <Td dataLabel="Actions">
                    <ProjectActions
                      project={project}
                      inspection={inspections[project.id]}
                      currentUser={data.current_user}
                      busy={busy}
                      onAction={onAction}
                    />
                  </Td>
                </Tr>
              ))}
            </Tbody>
          </Table>
        )}
      </Stack>
    </PageSection>
  );
}
