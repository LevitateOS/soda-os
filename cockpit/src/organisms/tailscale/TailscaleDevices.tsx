import { EmptyState, EmptyStateBody, PageSection, Stack, Title } from "@patternfly/react-core";
import { Table, Thead, Tbody, Tr, Th, Td } from "@patternfly/react-table";
import type { Device } from "../../tailscale/types";
import { deviceName } from "../../tailscale/status";
export function TailscaleDevices({
  peers,
  available,
  loading,
}: {
  peers: Device[];
  available: boolean;
  loading: boolean;
}) {
  return (
    <PageSection aria-labelledby="devices-title">
      <Stack hasGutter>
        <Title headingLevel="h2" id="devices-title">
          Devices
        </Title>
        {peers.length ? (
          <Table aria-label="Devices">
            <Thead>
              <Tr>
                <Th>Device</Th>
                <Th>Addresses</Th>
                <Th>Connection</Th>
              </Tr>
            </Thead>
            <Tbody>
              {peers.map((peer, index) => (
                <Tr key={peer.ID ?? index}>
                  <Td dataLabel="Device">{deviceName(peer)}</Td>
                  <Td dataLabel="Addresses">{(peer.TailscaleIPs || []).join(", ")}</Td>
                  <Td dataLabel="Connection">
                    {peer.Expired ? "Expired" : peer.Online ? "Online" : "Offline"}
                  </Td>
                </Tr>
              ))}
            </Tbody>
          </Table>
        ) : (
          <EmptyState
            titleText={available || loading ? "No devices reported." : "Device list unavailable."}
            headingLevel="h3"
          >
            <EmptyStateBody />
          </EmptyState>
        )}
      </Stack>
    </PageSection>
  );
}
