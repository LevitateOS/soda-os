import { Alert, PageSection } from "@patternfly/react-core";
export function RunnerExecutionNotice() {
  return (
    <PageSection>
      <Alert isInline variant="warning" title="Runner jobs execute repository code">
        Each local runner has its own unprivileged Linux account, one job slot, no sudo access, and
        no access to human home directories. Its files persist between jobs and it can use the
        network. Run jobs only for repositories and contributors you trust.
      </Alert>
    </PageSection>
  );
}
