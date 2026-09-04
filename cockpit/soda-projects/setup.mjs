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
  const administratorButton = document.querySelector("#open-setup-administrator");
  const dismissButton = document.querySelector("#dismiss-setup");
  const administratorDialog = document.querySelector("#setup-administrator-dialog");
  const administratorForm = document.querySelector("#setup-administrator-form");
  const localNetworkForm = document.querySelector("#allow-local-network-form");
  const tailscaleForm = document.querySelector("#connect-tailscale-form");

  document.querySelector("#refresh-setup").addEventListener("click", load);
  administratorButton.addEventListener("click", () => {
    administratorForm.reset();
    administratorDialog.showModal();
  });
  administratorForm.addEventListener("submit", event => mutateFromForm(event, "create-administrator", form => ({
    username: form.elements.username.value,
    password: form.elements.password.value,
    authorized_key: form.elements.authorized_key.value,
  }), true));
  localNetworkForm.addEventListener("submit", event => mutateFromForm(event, "allow-local-network", form => ({
    connection: form.elements.connection.value,
  })));
  tailscaleForm.addEventListener("submit", event => mutateFromForm(event, "connect-tailscale", form => ({
    auth_key: form.elements.auth_key.value,
  })));
  dismissButton.addEventListener("click", () => mutate("dismiss", {}, null));
  administratorDialog.addEventListener("close", () => clearSetupSecrets(administratorForm));

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

  async function mutateFromForm(event, action, payloadFor, close = false) {
    event.preventDefault();
    const form = event.currentTarget;
    if (!form.reportValidity()) {
      return;
    }
    if (action === "create-administrator" && form.elements.password.value !== form.elements.password_confirmation.value) {
      showNotice("The password confirmation does not match.", "error");
      return;
    }
    await mutate(action, payloadFor(form), form, close);
  }

  async function mutate(action, payload, form, close = false) {
    setBusy(true);
    try {
      const operation = invoke(action, payload);
      clearSetupSecrets(form, payload);
      const status = await operation;
      if (close) {
        administratorDialog.close();
      }
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
    summary.textContent = status.dismissed
      ? "Machine-wide setup is dismissed. Native settings remain available below."
      : status.can_dismiss
        ? "Every required fact is complete. Soda Setup can now be dismissed."
        : "Complete an administrator and at least one approved network path before dismissal.";
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
    setDisabled(administratorButton, status.administrators.length !== 0);
    setDisabled(localNetworkForm.querySelector("button"), status.connections.length === 0);
    setDisabled(dismissButton, !status.can_dismiss || status.dismissed);
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
  return administrators.map(administrator => `${administrator.username}: password ${yesNo(administrator.password_set)}, SSH key ${yesNo(administrator.ssh_public_key)}, Forgejo ${yesNo(administrator.forgejo_ready)}`).join("; ");
}

function yesNo(value) {
  return value ? "yes" : "no";
}

function setupSuccessMessage(action) {
  if (action === "create-administrator") return "The primary administrator is ready in Linux and Forgejo.";
  if (action === "allow-local-network") return "Access from the local network is allowed on the selected connection.";
  if (action === "connect-tailscale") return "Tailscale is connected. Any selected local-network access remains enabled.";
  return "Soda Setup was dismissed for this machine.";
}
