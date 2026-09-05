import { useCallback, useEffect, useRef, useState } from "react";
import {
  Alert,
  Button,
  Checkbox,
  DescriptionList,
  DescriptionListDescription,
  DescriptionListGroup,
  DescriptionListTerm,
  EmptyState,
  EmptyStateBody,
  Form,
  FormGroup,
  FormSelect,
  FormSelectOption,
  PageSection,
  Spinner,
  Title,
} from "@patternfly/react-core";
import { Table, Thead, Tbody, Tr, Th, Td } from "@patternfly/react-table";
import type { Cockpit } from "../cockpit/types";
import type { AuthenticationMessage, Snapshot } from "./types";
import type { NativeTailscale } from "./native";
import { adminURL, cliURL } from "./native";
import {
  advertisesExitNode,
  authenticationURL,
  connectionState,
  deviceName,
  exitNodeApproval,
  exitNodeChoices,
} from "./status";

const reload = () => window.location.reload();
const diagnostic = (error: unknown) => {
  if (
    error &&
    typeof error === "object" &&
    "message" in error &&
    typeof error.message === "string" &&
    error.message
  )
    return error.message;
  return String(error);
};
export function Tailscale({
  native,
  cockpit = window.cockpit,
  onReopen = reload,
}: {
  native: NativeTailscale;
  cockpit?: Pick<Cockpit, "hidden" | "addEventListener" | "removeEventListener">;
  onReopen?: () => void;
}) {
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null);
  const [busy, setBusy] = useState(false),
    [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");
  const [authURL, setAuthURL] = useState<string | null>(null),
    [streamState, setStreamState] = useState<string | undefined>();
  const [exitNode, setExitNode] = useState(""),
    [allowLAN, setAllowLAN] = useState(false),
    [advertise, setAdvertise] = useState(false);
  const closed = useRef(false),
    reading = useRef(false),
    pending = useRef(false);
  const dirty = useRef(new Set<"exit" | "advertise">()),
    refreshedIdentity = useRef("");
  const timer = useRef<ReturnType<typeof setTimeout>>();
  const showAuthentication = useCallback((message: AuthenticationMessage) => {
    if (closed.current) return;
    setAuthURL(authenticationURL(message.AuthURL));
    if (message.BackendState) setStreamState(connectionState(message));
  }, []);
  const load = useCallback(async () => {
    if (closed.current || reading.current) return;
    reading.current = true;
    try {
      const next = await native.read();
      if (closed.current) return;
      setSnapshot(next);
      setStreamState(undefined);
      setAuthURL(
        authenticationURL(next.status.BackendState === "NeedsLogin" ? next.status.AuthURL : ""),
      );
      if (!dirty.current.has("exit")) {
        setExitNode(exitSelection(next));
        setAllowLAN(Boolean(next.prefs.ExitNodeAllowLANAccess));
      }
      if (!dirty.current.has("advertise")) setAdvertise(advertisesExitNode(next.prefs));
      const connected = next.status.BackendState === "Running" && !next.status.Self?.Expired;
      if (!connected) {
        refreshedIdentity.current = "";
        return;
      }
      const identity = `${next.status.Self?.DNSName || ""}/${(next.status.TailscaleIPs || []).join(",")}`;
      if (identity === refreshedIdentity.current) return;
      refreshedIdentity.current = identity;
      try {
        await native.refreshForgejo();
      } catch (error) {
        if (!closed.current)
          setNotice(
            `Tailscale connected, but Forgejo could not refresh its Tailnet address: ${diagnostic(error)}`,
          );
      }
    } catch (error) {
      if (!closed.current) {
        setSnapshot(null);
        setAuthURL(null);
        setStreamState(undefined);
        setNotice(diagnostic(error));
      }
    } finally {
      reading.current = false;
      if (!closed.current) setLoading(false);
    }
  }, [native]);
  useEffect(() => {
    closed.current = false;
    async function observe() {
      await load();
      if (!closed.current) timer.current = setTimeout(() => void observe(), 3000);
    }
    function close() {
      closed.current = true;
      clearTimeout(timer.current);
      native.close();
    }
    function visibility() {
      if (cockpit.hidden) close();
      else if (closed.current) onReopen();
    }
    window.addEventListener("pagehide", close);
    cockpit.addEventListener("visibilitychange", visibility);
    if (cockpit.hidden) close();
    else void observe();
    return () => {
      close();
      window.removeEventListener("pagehide", close);
      cockpit.removeEventListener("visibilitychange", visibility);
    };
  }, [cockpit, load, native, onReopen]);
  async function mutate(operation: () => Promise<void>, form?: "exit" | "advertise") {
    if (pending.current || closed.current) return;
    pending.current = true;
    setBusy(true);
    setNotice("");
    try {
      await operation();
      if (form) dirty.current.delete(form);
    } catch (error) {
      if (!closed.current) setNotice(diagnostic(error));
    } finally {
      pending.current = false;
      if (!closed.current) setBusy(false);
      await load();
    }
  }
  const status = snapshot?.status,
    prefs = snapshot?.prefs;
  const connected = status?.BackendState === "Running" && !status.Self?.Expired;
  const peers = Object.values(status?.Peer || {}).sort((a, b) =>
    deviceName(a).localeCompare(deviceName(b)),
  );
  const choices = status ? exitNodeChoices(status).filter((peer) => peer.TailscaleIPs?.[0]) : [];
  const selected = snapshot ? exitSelection(snapshot) : "";
  const missingSelection = Boolean(
    prefs?.ExitNodeID && selected && !choices.some((peer) => peer.TailscaleIPs?.[0] === selected),
  );
  return (
    <main className="soda-page" aria-labelledby="page-title" aria-busy={busy}>
      <PageSection>
        <p className="soda-eyebrow">Soda OS</p>
        <Title headingLevel="h1" id="page-title">
          Tailscale
        </Title>
        {notice && (
          <Alert
            isInline
            variant="danger"
            title={notice}
            role="status"
            aria-live="polite"
            className="soda-diagnostic"
          />
        )}
      </PageSection>
      <PageSection aria-labelledby="connection-title">
        <Title headingLevel="h2" id="connection-title">
          Connection
        </Title>
        <p role="status" aria-live="polite">
          {streamState ||
            (status
              ? connectionState(status)
              : loading
                ? "Reading Tailscale state…"
                : "Tailscale state unavailable")}
        </p>
        {loading && <Spinner aria-label="Reading Tailscale state" size="lg" />}
        {!!status?.Health?.length && (
          <Alert
            isInline
            variant="warning"
            title={status.Health.join("\n")}
            className="soda-diagnostic"
          />
        )}
        {!connected && status?.BackendState !== "NeedsMachineAuth" && (
          <Button
            isDisabled={
              busy || !status || ["Starting", "NoState"].includes(status.BackendState ?? "")
            }
            onClick={() => {
              if (status) void mutate(() => native.signIn(status, showAuthentication));
            }}
          >
            {status?.BackendState === "Stopped"
              ? "Connect"
              : status?.HaveNodeKey
                ? "Sign in again"
                : "Sign in"}
          </Button>
        )}
        {authURL && (
          <Alert isInline variant="info" title="Continue in Tailscale">
            <a href={authURL} target="_blank" rel="noreferrer" className="soda-diagnostic">
              {authURL}
            </a>
          </Alert>
        )}
        {status?.BackendState === "NeedsMachineAuth" && (
          <p>
            Ask a Tailnet administrator to approve this device in{" "}
            <a href={adminURL} target="_blank" rel="noreferrer">
              Tailscale administration
            </a>
            .
          </p>
        )}
        <DescriptionList>
          <DescriptionListGroup>
            <DescriptionListTerm>This device</DescriptionListTerm>
            <DescriptionListDescription>{deviceName(status?.Self)}</DescriptionListDescription>
          </DescriptionListGroup>
          <DescriptionListGroup>
            <DescriptionListTerm>Tailscale addresses</DescriptionListTerm>
            <DescriptionListDescription>
              {(status?.TailscaleIPs || status?.Self?.TailscaleIPs || []).join(", ") ||
                "Unavailable"}
            </DescriptionListDescription>
          </DescriptionListGroup>
        </DescriptionList>
      </PageSection>
      <PageSection aria-labelledby="devices-title">
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
            titleText={status || loading ? "No devices reported." : "Device list unavailable."}
            headingLevel="h3"
          >
            <EmptyStateBody />
          </EmptyState>
        )}
      </PageSection>
      <PageSection aria-labelledby="exit-title">
        <Title headingLevel="h2" id="exit-title">
          Use an exit node
        </Title>
        <Form
          onSubmit={(event) => {
            event.preventDefault();
            void mutate(() => native.selectExitNode(exitNode, allowLAN), "exit");
          }}
        >
          <FormGroup label="Exit node" fieldId="exit-node">
            <FormSelect
              id="exit-node"
              value={exitNode}
              isDisabled={busy || !connected}
              onChange={(_, value) => {
                dirty.current.add("exit");
                setExitNode(value);
              }}
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
            onChange={(_, checked) => {
              dirty.current.add("exit");
              setAllowLAN(checked);
            }}
          />
          <div>
            <Button type="submit" isDisabled={busy || !connected}>
              Apply
            </Button>
          </div>
        </Form>
      </PageSection>
      <PageSection aria-labelledby="advertise-title">
        <Title headingLevel="h2" id="advertise-title">
          Advertise this device as an exit node
        </Title>
        <Form
          onSubmit={(event) => {
            event.preventDefault();
            void mutate(() => native.advertiseExitNode(advertise), "advertise");
          }}
        >
          <Checkbox
            id="advertise"
            label="Advertise as an exit node"
            isChecked={advertise}
            isDisabled={busy || !connected}
            onChange={(_, checked) => {
              dirty.current.add("advertise");
              setAdvertise(checked);
            }}
          />
          <div>
            <Button type="submit" isDisabled={busy || !connected}>
              Apply
            </Button>
          </div>
        </Form>
        <p role="status">
          {snapshot
            ? exitNodeApproval(snapshot.status, snapshot.prefs)
            : loading
              ? "Not advertised"
              : "Approval status unavailable"}
        </p>
        {prefs &&
          advertisesExitNode(prefs) &&
          status?.Self?.InNetworkMap &&
          !status.Self.ExitNodeOption && (
            <p>
              A Tailnet administrator can approve this exit node in{" "}
              <a href={adminURL} target="_blank" rel="noreferrer">
                Tailscale administration
              </a>
              .
            </p>
          )}
      </PageSection>
      <PageSection>
        <a href={cliURL} target="_blank" rel="noreferrer">
          Tailscale CLI documentation
        </a>
      </PageSection>
    </main>
  );
}

function exitSelection({ status, prefs }: Snapshot) {
  let selected = prefs.ExitNodeIP || "";
  const choices = exitNodeChoices(status).filter((peer) => peer.TailscaleIPs?.[0]);
  for (const peer of choices) if (peer.ID === prefs.ExitNodeID) selected = peer.TailscaleIPs![0];
  if (prefs.ExitNodeID && !choices.some((peer) => selected && peer.TailscaleIPs![0] === selected))
    selected = status.ExitNodeStatus?.TailscaleIPs?.[0] || selected || "unavailable";
  return selected;
}
