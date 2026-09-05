import {
  EmptyState,
  EmptyStateBody,
  PageSection,
  Spinner,
  Stack,
  Title,
} from "@patternfly/react-core";
import { Table, Thead, Tbody, Tr, Th, Td } from "@patternfly/react-table";
import { CodeValue } from "../../atoms/CodeValue";
import { ProjectActions } from "../../molecules/projects/ProjectActions";
import { WorkspaceSummary } from "../../molecules/projects/WorkspaceSummary";
import type { FormAction, ListResponse, Project } from "../../projects/types";
export function ProjectCatalog({
  data,
  loading,
  busy,
  hostname,
  onAction,
}: {
  data: ListResponse | null;
  loading: boolean;
  busy: boolean;
  hostname: string;
  onAction: (action: FormAction, project: Project) => void;
}) {
  return (
    <PageSection aria-labelledby="catalog-title">
      <Stack hasGutter>
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
                    <CodeValue>{project.id}</CodeValue>
                  </Td>
                  <Td dataLabel="Canonical repository">
                    <CodeValue>{project.canonical_url}</CodeValue>
                  </Td>
                  <Td dataLabel="Your workspace">
                    <WorkspaceSummary project={project} hostname={hostname} />{" "}
                  </Td>
                  <Td dataLabel="Actions">
                    <ProjectActions
                      project={project}
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
