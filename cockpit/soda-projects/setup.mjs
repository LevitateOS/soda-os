import {
  clearSetupSecrets,
  decodeSetupResponse,
  encodeSetupRequest,
  setupCommand,
} from "./setup-protocol.mjs";

export function initializeSetup({ cockpit, showNotice, setBusy }) {
  const summary = document.querySelector("#soda-setup-summary");
  const body = document.querySelector("#soda-setup-body");
  const facts = document.querySelector("#soda-setup-facts");
  const connection = document.querySelector("#setup-connection");
  const localNetworkForm = document.querySelector("#allow-local-network-form");
  const tailscaleForm = document.querySelector("#connect-tailscale-form");
  const dismiss = document.querySelector("#dismiss-setup");

  document.querySelector("#refresh-setup").addEventListener("click", load);
  localNetworkForm.addEventListener("submit", event => mutateFromForm(event, "allow-local-network", form => ({
    connection: form.elements.connection.value,
  })));
  tailscaleForm.addEventListener("submit", event => mutateFromForm(event, "connect-tailscale", form => ({
    auth_key: form.elements.auth_key.value,
  })));
  dismiss.addEventListener("click", () => mutate("dismiss", null, null));

  load();

  async function invoke(action, payload = null) {
    const process = cockpit.spawn(setupCommand(action), { superuser: "require", err: "message" });
    if (payload !== null) {
      process.input(encodeSetupRequest(action, payload));
    }
    const output = await process;
    return decodeSetupResponse(action, output);
  }

  async function load() {
    summary.textContent = "Loading machine setup…";
    try {
      render(await invoke("status"));
    } catch (error) {
      body.hidden = true;
      summary.textContent = "Machine setup could not be loaded.";
      showNotice(error.message || "Soda Setup could not be loaded.", "error");
    }
  }

  async function mutateFromForm(event, action, payloadFor) {
    event.preventDefault();
    const form = event.currentTarget;
    if (!form.reportValidity()) {
      return;
    }
    await mutate(action, payloadFor(form), form);
  }

  async function mutate(action, payload, form) {
    setBusy(true);
    try {
      const operation = invoke(action, payload);
      clearSetupSecrets(form, payload);
      const status = await operation;
      render(status);
      showNotice(setupSuccessMessage(action), "success");
    } catch (error) {
      clearSetupSecrets(form, payload);
      showNotice(error.message || "The setup action failed.", "error");
      await load();
    } finally {
      setBusy(false);
    }
  }

  function render(status) {
    summary.textContent = status.ready
      ? status.dismissed
        ? "Automatic console Setup is disabled. You can still configure networking here."
        : "Network access is configured. Choose Don't show Setup automatically to stop its console prompt."
      : "Choose a trusted local network or connect Tailscale.";
    facts.replaceChildren(
      fact("Administrator", administratorFact(status.administrators)),
      fact("Tailscale connected", yesNo(status.tailscale_connected)),
      fact("Access from the local network", yesNo(status.local_network_allowed)),
    );
    connection.replaceChildren(...status.connections.map(item => {
      const option = document.createElement("option");
      option.value = item.name;
      option.textContent = `${item.name}${item.local_network_allowed ? " (allowed)" : ""}`;
      return option;
    }));
    setDisabled(localNetworkForm.querySelector("button"), status.connections.length === 0);
    dismiss.hidden = status.dismissed;
    setDisabled(dismiss, !status.can_dismiss);
    body.hidden = false;
  }
}

function setDisabled(element, disabled) {
  element.dataset.disabled = String(disabled);
  element.disabled = disabled;
}

function fact(label, value) {
  const group = document.createElement("div");
  const term = document.createElement("dt");
  const description = document.createElement("dd");
  term.textContent = label;
  description.textContent = value;
  group.append(term, description);
  return group;
}

function administratorFact(administrators) {
  if (administrators.length === 0) {
    return "missing";
  }
  return administrators.map(administrator => administrator.username).join("; ");
}

function yesNo(value) {
  return value ? "yes" : "no";
}

function setupSuccessMessage(action) {
  if (action === "allow-local-network") return "Access from the local network is allowed on the selected connection.";
  if (action === "connect-tailscale") return "Tailscale is connected. Any selected local-network access remains enabled.";
  if (action === "dismiss") return "Soda Setup will no longer open automatically on console login.";
  return "Network configuration updated.";
}
