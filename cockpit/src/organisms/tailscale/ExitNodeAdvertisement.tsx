import {
  ActionGroup,
  Button,
  Checkbox,
  Form,
  PageSection,
  Stack,
  Title,
} from "@patternfly/react-core";
import { ExitNodeApprovalGuidance } from "../../molecules/tailscale/ExitNodeApprovalGuidance";
import type { Snapshot } from "../../tailscale/types";
export function ExitNodeAdvertisement({
  snapshot,
  loading,
  busy,
  connected,
  advertise,
  onChange,
  onApply,
}: {
  snapshot: Snapshot | null;
  loading: boolean;
  busy: boolean;
  connected: boolean;
  advertise: boolean;
  onChange: (value: boolean) => void;
  onApply: () => void;
}) {
  return (
    <PageSection aria-labelledby="advertise-title">
      <Stack hasGutter>
        <Title headingLevel="h2" id="advertise-title">
          Advertise this device as an exit node
        </Title>
        <Form
          onSubmit={(event) => {
            event.preventDefault();
            onApply();
          }}
        >
          <Checkbox
            id="advertise"
            label="Advertise as an exit node"
            isChecked={advertise}
            isDisabled={busy || !connected}
            onChange={(_, checked) => onChange(checked)}
          />
          <ActionGroup>
            <Button type="submit" isDisabled={busy || !connected}>
              Apply
            </Button>
          </ActionGroup>
        </Form>
        <ExitNodeApprovalGuidance snapshot={snapshot} loading={loading} />{" "}
      </Stack>
    </PageSection>
  );
}
