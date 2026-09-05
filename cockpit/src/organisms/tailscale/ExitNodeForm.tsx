import {
  ActionGroup,
  Button,
  Checkbox,
  Form,
  FormGroup,
  FormSelect,
  FormSelectOption,
  PageSection,
  Stack,
  Title,
} from "@patternfly/react-core";
import type { Device } from "../../tailscale/types";
import { deviceName } from "../../tailscale/status";
export function ExitNodeForm({
  exitNode,
  allowLAN,
  busy,
  connected,
  choices,
  selected,
  missingSelection,
  onExitNodeChange,
  onAllowLANChange,
  onApply,
}: {
  exitNode: string;
  allowLAN: boolean;
  busy: boolean;
  connected: boolean;
  choices: Device[];
  selected: string;
  missingSelection: boolean;
  onExitNodeChange: (value: string) => void;
  onAllowLANChange: (value: boolean) => void;
  onApply: () => void;
}) {
  return (
    <PageSection aria-labelledby="exit-title">
      <Stack hasGutter>
        <Title headingLevel="h2" id="exit-title">
          Use an exit node
        </Title>
        <Form
          onSubmit={(event) => {
            event.preventDefault();
            onApply();
          }}
        >
          <FormGroup label="Exit node" fieldId="exit-node">
            <FormSelect
              id="exit-node"
              value={exitNode}
              isDisabled={busy || !connected}
              onChange={(_, value) => onExitNodeChange(value)}
            >
              <FormSelectOption value="" label="None" />
              {choices.map((peer) => (
                <FormSelectOption
                  key={peer.ID ?? peer.TailscaleIPs![0]}
                  value={peer.TailscaleIPs![0]}
                  label={`${deviceName(peer)}${peer.Online ? "" : " (offline)"}`}
                />
              ))}
              {missingSelection && (
                <FormSelectOption
                  value={selected}
                  label="Selected exit node unavailable"
                  isDisabled
                />
              )}
            </FormSelect>
          </FormGroup>
          <Checkbox
            id="allow-lan"
            label="Allow local network access while using an exit node"
            isChecked={allowLAN}
            isDisabled={busy || !connected}
            onChange={(_, checked) => onAllowLANChange(checked)}
          />
          <ActionGroup>
            <Button type="submit" isDisabled={busy || !connected}>
              Apply
            </Button>
          </ActionGroup>
        </Form>
      </Stack>
    </PageSection>
  );
}
