import { Alert, Spinner, Stack, StackItem } from "@patternfly/react-core";
import { DiagnosticAlert } from "../DiagnosticAlert";

export function UpdateFeedback({
  operation,
  error,
  notice,
  diagnostic,
}: {
  operation: string | null;
  error: string | null;
  notice: string | null;
  diagnostic: string | null;
}) {
  return (
    <Stack hasGutter>
      {operation && (
        <StackItem>
          <div role="status">
            <Spinner size="sm" /> {operation}
          </div>
        </StackItem>
      )}
      {error && (
        <StackItem>
          <Alert isInline variant="danger" title="Operation could not be confirmed" role="alert">
            <pre className="soda-diagnostic">{error}</pre>
          </Alert>
        </StackItem>
      )}
      {notice && (
        <StackItem>
          <DiagnosticAlert variant="info" message={notice} />
        </StackItem>
      )}
      {diagnostic && (
        <StackItem>
          <DiagnosticAlert variant="warning" message={diagnostic} />
        </StackItem>
      )}
    </Stack>
  );
}
