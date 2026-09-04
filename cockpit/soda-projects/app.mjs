import {
  coordinatorCommand,
  decodeResponse,
  encodeRequest,
} from "./protocol.mjs";
import {
  clearPayloadSecrets,
  clearSecrets,
  errorMessage,
  humanDeletionHidden,
  payloadFor,
  projectRemovalHidden,
  successMessage,
} from "./ui.mjs";
import { initializeSetup } from "./setup.mjs";

const cockpit = window.cockpit;
const state = {
  data: null,
  busy: false,
};

const elements = {
  notice: document.querySelector("#notice"),
  summary: document.querySelector("#catalog-summary"),
  tableWrap: document.querySelector("#projects-table-wrap"),
  rows: document.querySelector("#project-rows"),
  empty: document.querySelector("#empty-projects"),
  humanPanel: document.querySelector("#human-deletion-panel"),
  forgejoDescription: document.querySelector("#forgejo-description"),
};

document.querySelector("#refresh-projects").addEventListener("click", loadProjects);
document.querySelector("#open-add-existing").addEventListener("click", () => openDialog("add-existing-dialog"));
document.querySelector("#open-create-forgejo").addEventListener("click", () => openDialog("create-forgejo-dialog"));
document.querySelector("#open-delete-human").addEventListener("click", () => openDialog("delete-human-dialog"));
document.querySelector("#open-add-person").addEventListener("click", () => openDialog("add-person-dialog"));
document.querySelectorAll("[data-action-form]").forEach(form => form.addEventListener("submit", submitAction));
document.querySelectorAll("[data-dialog-close]").forEach(button => button.addEventListener("click", () => {
  button.closest("dialog").close();
}));
document.querySelectorAll("dialog").forEach(dialog => dialog.addEventListener("close", () => {
  clearSecrets(dialog.querySelector("form"));
}));

elements.rows.addEventListener("click", event => {
  const button = event.target.closest("button[data-project-action]");
  if (!button || state.busy) {
    return;
  }
  const project = state.data?.projects.find(candidate => candidate.id === button.dataset.projectId);
  if (!project) {
    showNotice("The project is no longer present. Refresh the catalog and try again.", "error");
    return;
  }
  prepareProjectDialog(button.dataset.projectAction, project);
});

loadProjects();
initializeSetup({ cockpit, showNotice, setBusy });

async function invoke(action, payload) {
  const process = cockpit.spawn(coordinatorCommand(action), { err: "message" });
  process.input(encodeRequest(action, payload));
  const output = await process;
  return decodeResponse(action, output);
}

async function loadProjects() {
  setBusy(true);
  elements.summary.textContent = "Loading projects…";
  try {
    state.data = await invoke("list", {});
    render();
  } catch (error) {
    state.data = null;
    elements.tableWrap.hidden = true;
    elements.empty.hidden = true;
    elements.humanPanel.hidden = true;
    elements.summary.textContent = "The project catalog could not be loaded.";
    showNotice(errorMessage(error), "error");
  } finally {
    setBusy(false);
  }
}

function render() {
  const { projects, current_user: currentUser, forgejo_url: forgejoURL } = state.data;
  elements.summary.textContent = `${projects.length} ${projects.length === 1 ? "project" : "projects"} available to ${currentUser.username}.`;
  elements.rows.replaceChildren(...projects.map(projectRow));
  elements.tableWrap.hidden = projects.length === 0;
  elements.empty.hidden = projects.length !== 0;
  elements.humanPanel.hidden = humanDeletionHidden(currentUser);
  elements.forgejoDescription.textContent = forgejoURL
    ? `Creates an empty repository in your native Forgejo namespace at ${forgejoURL}.`
    : "Creates an empty repository in your native Forgejo namespace.";
}

function projectRow(project) {
  const row = document.createElement("tr");

  const identity = document.createElement("td");
  const title = document.createElement("strong");
  title.textContent = project.display_name;
  const id = document.createElement("code");
  id.textContent = project.id;
  identity.append(title, id);

  const repository = document.createElement("td");
  const repositoryURL = document.createElement("code");
  repositoryURL.className = "repository-url";
  repositoryURL.textContent = project.canonical_url;
  repository.append(repositoryURL);

  const workspace = document.createElement("td");
  const guidance = document.createElement("span");
  guidance.className = "status";
  guidance.textContent = project.workspace_ready ? "Ready" : "After setup";
  const command = document.createElement("code");
  command.className = "ssh-command";
  command.textContent = state.data.ssh_host
    ? `ssh ${project.workspace_username}@${state.data.ssh_host}`
    : project.workspace_username;
  workspace.append(guidance, command);

  const actionsCell = document.createElement("td");
  actionsCell.className = "row-actions";
  actionsCell.append(projectButton("Set up for me", "setup", project.id, "primary"));
  if (project.workspace_ready) {
	actionsCell.append(projectButton("Add tools", "install-tools", project.id, "secondary"));
    actionsCell.append(projectButton("Remove my workspace", "remove-workspace", project.id, "danger-link"));
  }
  actionsCell.append(projectButton("Edit", "edit", project.id, "secondary"));
  if (!projectRemovalHidden(state.data.current_user)) {
    actionsCell.append(projectButton("Remove project", "remove", project.id, "danger-link"));
  }

  row.append(identity, repository, workspace, actionsCell);
  return row;
}

function projectButton(label, action, projectID, style) {
  const button = document.createElement("button");
  button.type = "button";
  button.className = `button compact ${style}`;
  button.dataset.projectAction = action;
  button.dataset.projectId = projectID;
  button.textContent = label;
  return button;
}

function prepareProjectDialog(action, project) {
  const dialog = document.querySelector(`#${action}-project-dialog`);
  const form = dialog.querySelector("form");
  form.reset();
  form.elements.id.value = project.id;

  if (action === "edit") {
    dialog.querySelector("[data-project-id]").textContent = project.id;
    form.elements.display_name.value = project.display_name;
    form.elements.canonical_url.value = project.canonical_url;
    form.elements.additional_metadata.value = JSON.stringify(project.catalog_metadata, null, 2);
  } else if (action === "setup") {
    dialog.querySelector("[data-project-name]").textContent = project.display_name;
  } else if (action === "install-tools") {
	dialog.querySelector("[data-project-name]").textContent = project.display_name;
  } else if (action === "remove" || action === "remove-workspace") {
    dialog.querySelector("[data-confirmation-value]").textContent = project.id;
  }
  dialog.showModal();
}

function openDialog(id) {
  if (state.busy) {
    return;
  }
  const dialog = document.querySelector(`#${id}`);
  const form = dialog.querySelector("form");
  form.reset();
  dialog.showModal();
}

async function submitAction(event) {
  event.preventDefault();
  const form = event.currentTarget;
  if (state.busy) {
    return;
  }

  const action = form.dataset.actionForm;
  if (!form.reportValidity()) {
    return;
  }

  const payload = payloadFor(action, new FormData(form), message => showNotice(message, "error"));
  if (!payload) {
    return;
  }

  setBusy(true);
  try {
    const operation = invoke(action, payload);
    clearSecrets(form);
    clearPayloadSecrets(payload);
    const result = await operation;
    form.closest("dialog").close();
    const message = successMessage(action, payload, result);
    await loadProjects();
    showNotice(message, "success");
  } catch (error) {
    clearSecrets(form);
    showNotice(errorMessage(error), "error");
  } finally {
    setBusy(false);
  }
}

function setBusy(busy) {
  state.busy = busy;
  document.querySelectorAll("button, input, select, textarea").forEach(element => {
    element.disabled = busy || element.dataset.disabled === "true";
  });
  document.body.setAttribute("aria-busy", String(busy));
}

function showNotice(message, kind) {
  elements.notice.textContent = message;
  elements.notice.className = `notice ${kind}`;
  elements.notice.hidden = false;
}
