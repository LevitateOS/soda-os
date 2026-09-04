import { coordinatorCommand, decodeResponse, encodeRequest } from "./protocol.mjs";
import { clearRegistrationSecret, createPayload, errorMessage, providerName, statusClass, statusText, successMessage } from "./ui.mjs";

const cockpit = window.cockpit;
const state = { data: null, busy: false };
const elements = {
  notice: document.querySelector("#notice"),
  summary: document.querySelector("#runner-summary"),
  tableWrap: document.querySelector("#runners-table-wrap"),
  rows: document.querySelector("#runner-rows"),
  empty: document.querySelector("#empty-runners"),
  create: document.querySelector("#create-runner-dialog"),
  forgejoFields: document.querySelector("#forgejo-fields"),
  githubFields: document.querySelector("#github-fields"),
  forgejoLink: document.querySelector("#forgejo-runners-link"),
};

document.querySelector("#refresh-runners").addEventListener("click", loadRunners);
document.querySelector("#open-create-runner").addEventListener("click", openCreateDialog);
document.querySelectorAll("[data-dialog-close]").forEach(button => button.addEventListener("click", () => button.closest("dialog").close()));
document.querySelectorAll("input[name=provider]").forEach(input => input.addEventListener("change", renderProviderFields));
document.querySelector("#create-runner-form").addEventListener("submit", createRunner);
document.querySelector("#remove-runner-form").addEventListener("submit", removeRunner);
elements.rows.addEventListener("click", handleRunnerAction);
elements.create.addEventListener("close", () => clearRegistrationSecret(elements.create.querySelector("form")));

loadRunners();

async function invoke(action, payload) {
  const process = cockpit.spawn(coordinatorCommand(action), { err: "message" });
  process.input(encodeRequest(action, payload));
  return decodeResponse(action, await process);
}

async function loadRunners() {
  setBusy(true);
  elements.summary.textContent = "Loading local runner capacity…";
  try {
    state.data = await invoke("list", {});
    render();
  } catch (error) {
    state.data = null;
    elements.tableWrap.hidden = true;
    elements.empty.hidden = true;
    elements.summary.textContent = "Local runner capacity is available only to Soda OS administrators.";
    showNotice(errorMessage(error), "error");
  } finally {
    setBusy(false);
  }
}

function render() {
  const data = state.data;
  elements.summary.textContent = `${data.runner_count} local ${data.runner_count === 1 ? "runner" : "runners"}; ${data.active_listeners} listening; ${data.total_capacity} configured ${data.total_capacity === 1 ? "slot" : "slots"}.`;
  elements.rows.replaceChildren(...data.runners.map(runnerRow));
  elements.tableWrap.hidden = data.runners.length === 0;
  elements.empty.hidden = data.runners.length !== 0;
  elements.forgejoLink.href = `${data.forgejo_url.replace(/\/$/, "")}/admin/actions/runners`;
}

function runnerRow(runner) {
  const row = document.createElement("tr");
  const identity = document.createElement("td");
  const title = document.createElement("strong");
  title.textContent = runner.id;
  const account = document.createElement("code");
  account.textContent = runner.account;
  identity.append(title, account);

  const provider = document.createElement("td");
  const providerLink = document.createElement("a");
  providerLink.href = runner.registration_url;
  providerLink.target = "_blank";
  providerLink.rel = "noreferrer";
  providerLink.textContent = providerName(runner.provider);
  const version = document.createElement("code");
  version.textContent = runner.version;
  provider.append(providerLink, version);

  const listener = document.createElement("td");
  const status = document.createElement("span");
  status.className = `status ${statusClass(runner.service)}`;
  status.textContent = statusText(runner.service);
  const nativeState = document.createElement("code");
  nativeState.textContent = `${runner.service.active}/${runner.service.sub}; ${runner.service.enabled}`;
  listener.append(status, nativeState);

  const capacity = document.createElement("td");
  capacity.textContent = `${runner.capacity} slot · ${runner.architecture}`;

  const actions = document.createElement("td");
  actions.className = "row-actions";
  if (runner.service.active === "active") {
    actions.append(actionButton("Stop", "stop", runner.id, "secondary"));
  } else {
    actions.append(actionButton("Start", "start", runner.id, "primary"));
  }
  actions.append(actionButton("Restart", "restart", runner.id, "secondary"), actionButton("Remove", "remove", runner.id, "danger-link"));
  row.append(identity, provider, listener, capacity, actions);
  return row;
}

function actionButton(label, action, id, style) {
  const button = document.createElement("button");
  button.type = "button";
  button.className = `button compact ${style}`;
  button.dataset.runnerAction = action;
  button.dataset.runnerId = id;
  button.textContent = label;
  return button;
}

function openCreateDialog() {
  if (state.busy) return;
  const form = elements.create.querySelector("form");
  form.reset();
  renderProviderFields();
  elements.create.showModal();
}

function renderProviderFields() {
  const provider = elements.create.querySelector("input[name=provider]:checked").value;
  elements.forgejoFields.hidden = provider !== "forgejo";
  elements.githubFields.hidden = provider !== "github";
  elements.forgejoFields.querySelectorAll("input").forEach(input => { input.required = provider === "forgejo"; });
  elements.githubFields.querySelector("input[name=registration_url]").required = provider === "github";
}

async function createRunner(event) {
  event.preventDefault();
  const form = event.currentTarget;
  if (state.busy || !form.reportValidity()) return;
  const payload = createPayload(new FormData(form));
  const id = payload.id;
  setBusy(true);
  try {
    const operation = invoke("create", payload);
    clearRegistrationSecret(form, payload);
    await operation;
    elements.create.close();
    await loadRunners();
    showNotice(successMessage("create", id), "success");
  } catch (error) {
    clearRegistrationSecret(form, payload);
    showNotice(errorMessage(error), "error");
  } finally {
    setBusy(false);
  }
}

async function handleRunnerAction(event) {
  const button = event.target.closest("button[data-runner-action]");
  if (!button || state.busy) return;
  const action = button.dataset.runnerAction;
  const id = button.dataset.runnerId;
  if (action === "remove") {
    const dialog = document.querySelector("#remove-runner-dialog");
    const form = dialog.querySelector("form");
    form.reset();
    form.elements.id.value = id;
    dialog.querySelector("[data-runner-id]").textContent = id;
    dialog.showModal();
    return;
  }
  await mutate(action, id);
}

async function removeRunner(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const id = form.elements.id.value;
  if (form.elements.confirmation.value !== id) {
    showNotice(`Type ${id} exactly to confirm local runner removal.`, "error");
    return;
  }
  await mutate("remove", id);
  if (state.data?.runners.every(runner => runner.id !== id)) {
    form.closest("dialog").close();
  }
}

async function mutate(action, id) {
  setBusy(true);
  try {
    await invoke(action, { id });
    await loadRunners();
    showNotice(successMessage(action, id), "success");
  } catch (error) {
    showNotice(errorMessage(error), "error");
  } finally {
    setBusy(false);
  }
}

function setBusy(busy) {
  state.busy = busy;
  document.querySelectorAll("button, input").forEach(element => { element.disabled = busy; });
  document.body.setAttribute("aria-busy", String(busy));
}

function showNotice(message, kind) {
  elements.notice.textContent = message;
  elements.notice.className = `notice ${kind}`;
  elements.notice.hidden = false;
}
