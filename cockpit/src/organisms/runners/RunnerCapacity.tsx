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
import { ExternalLink } from "../../atoms/ExternalLink";
import { RunnerActions } from "../../molecules/runners/RunnerActions";
import { RunnerServiceStatus } from "../../molecules/runners/RunnerServiceStatus";
import { providerURL, providerName } from "../../runners/ui";
import type { LifecycleAction, ListResponse } from "../../runners/types";
export function RunnerCapacity({
  data,
  loading,
  busy,
  hostname,
  onAction,
  onRemove,
}: {
  data: ListResponse | null;
  loading: boolean;
  busy: boolean;
  hostname: string;
  onAction: (action: LifecycleAction, id: string) => void;
  onRemove: (id: string) => void;
}) {
  return (
    <PageSection aria-labelledby="capacity-title">
      <Stack hasGutter>
        <Title headingLevel="h2" id="capacity-title">
          Local capacity
        </Title>
        <p>
          {loading
            ? "Loading local runner capacity…"
            : data
              ? `${data.runner_count} local ${data.runner_count === 1 ? "runner" : "runners"}; ${data.active_listeners} listening; ${data.total_capacity} configured ${data.total_capacity === 1 ? "slot" : "slots"}.`
              : "Local runner status is unavailable. Refresh to try again."}
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
                    <CodeValue>{runner.account}</CodeValue>
                  </Td>
                  <Td dataLabel="Provider">
                    <ExternalLink
                      href={providerURL(runner.provider, runner.registration_url, hostname)}
                    >
                      {providerName(runner.provider)}
                    </ExternalLink>
                    <CodeValue>{runner.version}</CodeValue>
                  </Td>
                  <Td dataLabel="Local listener">
                    <RunnerServiceStatus service={runner.service} />{" "}
                  </Td>
                  <Td dataLabel="Capacity">
                    {runner.capacity} slot · {runner.architecture}
                  </Td>
                  <Td dataLabel="Actions">
                    <RunnerActions
                      runner={runner}
                      busy={busy}
                      onAction={onAction}
                      onRemove={onRemove}
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
