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
  saving,
  saved,
  snapshot,
  loading,
  busy,
  connected,
  advertise,
  onChange,
  onApply,
}: {
  saving: boolean;
  saved: boolean;
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
            <Button type="submit" isDisabled={busy || !connected} isLoading={saving}>
              Apply
            </Button>
          </ActionGroup>
        </Form>
        {saving && <p role="status">Saving exit-node advertisement…</p>}
        {saved && <p role="status">Exit-node advertisement saved. Tailnet approval is separate.</p>}
        <ExitNodeApprovalGuidance snapshot={snapshot} loading={loading} />{" "}
      </Stack>
    </PageSection>
  );
}
