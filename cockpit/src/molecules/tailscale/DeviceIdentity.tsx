import {
  DescriptionList,
  DescriptionListGroup,
  DescriptionListTerm,
  DescriptionListDescription,
} from "@patternfly/react-core";
import type { Status } from "../../tailscale/types";
import { deviceName } from "../../tailscale/status";
export function DeviceIdentity({ status }: { status?: Status }) {
  return (
    <DescriptionList>
      <DescriptionListGroup>
        <DescriptionListTerm>This device</DescriptionListTerm>
        <DescriptionListDescription>{deviceName(status?.Self)}</DescriptionListDescription>
      </DescriptionListGroup>
      <DescriptionListGroup>
        <DescriptionListTerm>Tailscale addresses</DescriptionListTerm>
        <DescriptionListDescription>
          {(status?.TailscaleIPs || status?.Self?.TailscaleIPs || []).join(", ") || "Unavailable"}
        </DescriptionListDescription>
      </DescriptionListGroup>
    </DescriptionList>
  );
}
