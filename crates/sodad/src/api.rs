use std::{convert::Infallible, sync::Arc, time::Duration};

use axum::{
    Json, Router,
    extract::{Path, Query, State},
    response::sse::{Event, KeepAlive, Sse},
    routing::{get, post},
};
use serde::{Deserialize, Serialize};
use soda_core::{
    ActiveSshConnection, AddCollaboratorRequest, CreatePersonRequest, CreateProjectRequest,
    CreateWorktreeRequest, DeployKey, EventKind, HostStatus, ImportPersonRequest, Person, Project,
    ProvisioningJob, SodaEvent, ToolchainInstallation, Worktree, WorktreeStatus,
};
use tokio_stream::{StreamExt, wrappers::BroadcastStream};
use uuid::Uuid;

use crate::{error::Result, observability::Observability, service::Service};

#[derive(Clone)]
struct AppState {
    service: Arc<Service>,
    observability: Arc<Observability>,
}

#[derive(Serialize)]
struct Health {
    status: &'static str,
    service: &'static str,
    version: &'static str,
}

#[derive(Deserialize)]
struct EventQuery {
    project_id: Option<Uuid>,
}

pub fn router(service: Arc<Service>) -> Router {
    let observability = Observability::start(service.clone());
    Router::new()
        .route("/v1/health", get(health))
        .route("/v1/events", get(events))
        .route("/v1/host-status", get(host_status))
        .route("/v1/ssh-sessions", get(active_sessions))
        .route("/v1/people", get(list_people).post(create_person))
        .route("/v1/people/import", post(import_person))
        .route(
            "/v1/people/{person_id}/projects",
            get(list_projects_for_person),
        )
        .route("/v1/projects", get(list_projects).post(create_project))
        .route("/v1/projects/{project_id}/deploy-key", get(get_deploy_key))
        .route(
            "/v1/projects/{project_id}/toolchain",
            get(get_project_toolchain),
        )
        .route("/v1/projects/{project_id}/clone", post(start_provisioning))
        .route(
            "/v1/projects/{project_id}/collaborators",
            post(add_collaborator),
        )
        .route(
            "/v1/projects/{project_id}/worktrees",
            get(list_worktrees).post(create_worktree),
        )
        .route(
            "/v1/projects/{project_id}/provisioning",
            get(list_jobs).post(start_provisioning),
        )
        .route(
            "/v1/projects/{project_id}/worktree-status",
            get(worktree_status),
        )
        .with_state(AppState {
            service,
            observability,
        })
}

async fn health() -> Json<Health> {
    Json(Health {
        status: "ok",
        service: "sodad",
        version: env!("CARGO_PKG_VERSION"),
    })
}

async fn events(
    State(state): State<AppState>,
    Query(query): Query<EventQuery>,
) -> Sse<impl tokio_stream::Stream<Item = std::result::Result<Event, Infallible>>> {
    let interest = state.observability.interest(query.project_id);
    let initial = tokio_stream::once(Ok(Event::default().event("refresh").data("refresh")));
    let stream =
        BroadcastStream::new(state.service.events().subscribe()).filter_map(move |result| {
            let _keep_interest_alive = &interest;
            Some(Ok(match result {
                Ok(event) => event_for(event),
                Err(_) => Event::default().event("refresh").data("refresh"),
            }))
        });
    Sse::new(initial.chain(stream)).keep_alive(
        KeepAlive::new()
            .interval(Duration::from_secs(15))
            .text("keepalive"),
    )
}

fn event_for(event: SodaEvent) -> Event {
    Event::default()
        .event(event_name(event.kind))
        .id(event.sequence.to_string())
        .json_data(event)
        .expect("Soda events serialize")
}

const fn event_name(kind: EventKind) -> &'static str {
    match kind {
        EventKind::HostChanged => "host_changed",
        EventKind::PeopleChanged => "people_changed",
        EventKind::ProjectsChanged => "projects_changed",
        EventKind::WorktreesChanged => "worktrees_changed",
        EventKind::ProvisioningChanged => "provisioning_changed",
        EventKind::GitChanged => "git_changed",
        EventKind::SessionsChanged => "sessions_changed",
    }
}

async fn host_status(State(state): State<AppState>) -> Json<HostStatus> {
    Json(state.observability.host())
}

async fn active_sessions(State(state): State<AppState>) -> Json<Vec<ActiveSshConnection>> {
    Json(state.observability.sessions())
}

async fn worktree_status(
    State(state): State<AppState>,
    Path(project_id): Path<Uuid>,
) -> Result<Json<Vec<WorktreeStatus>>> {
    state.service.list_worktrees(project_id)?;
    Ok(Json(state.observability.worktrees(project_id)))
}

async fn create_person(
    State(state): State<AppState>,
    Json(request): Json<CreatePersonRequest>,
) -> Result<Json<Person>> {
    state.service.create_person(request).map(Json)
}

async fn list_people(State(state): State<AppState>) -> Result<Json<Vec<Person>>> {
    state.service.list_people().map(Json)
}

async fn import_person(
    State(state): State<AppState>,
    Json(request): Json<ImportPersonRequest>,
) -> Result<Json<Person>> {
    state.service.import_person(request).map(Json)
}

async fn list_projects_for_person(
    State(state): State<AppState>,
    Path(person_id): Path<Uuid>,
) -> Result<Json<Vec<Project>>> {
    state.service.list_projects_for_person(person_id).map(Json)
}

async fn create_project(
    State(state): State<AppState>,
    Json(request): Json<CreateProjectRequest>,
) -> Result<Json<Project>> {
    let project = state.service.create_project(request)?;
    let job = state.service.start_provisioning(project.id)?;
    spawn_provisioning(state.service, job.project_id, job.id);
    Ok(Json(project))
}

async fn list_projects(State(state): State<AppState>) -> Result<Json<Vec<Project>>> {
    state.service.list_projects().map(Json)
}

async fn get_deploy_key(
    State(state): State<AppState>,
    Path(project_id): Path<Uuid>,
) -> Result<Json<DeployKey>> {
    state.service.deploy_key(project_id).map(Json)
}

async fn get_project_toolchain(
    State(state): State<AppState>,
    Path(project_id): Path<Uuid>,
) -> Result<Json<ToolchainInstallation>> {
    state.service.project_installation(project_id).map(Json)
}

async fn add_collaborator(
    State(state): State<AppState>,
    Path(project_id): Path<Uuid>,
    Json(request): Json<AddCollaboratorRequest>,
) -> Result<Json<Worktree>> {
    state
        .service
        .add_collaborator(project_id, request.person_id)
        .map(Json)
}

async fn create_worktree(
    State(state): State<AppState>,
    Path(project_id): Path<Uuid>,
    Json(request): Json<CreateWorktreeRequest>,
) -> Result<Json<Worktree>> {
    state
        .service
        .create_worktree(
            project_id,
            request.person_id,
            &request.name,
            &request.base_ref,
        )
        .map(Json)
}

async fn list_worktrees(
    State(state): State<AppState>,
    Path(project_id): Path<Uuid>,
) -> Result<Json<Vec<Worktree>>> {
    state.service.list_worktrees(project_id).map(Json)
}

async fn start_provisioning(
    State(state): State<AppState>,
    Path(project_id): Path<Uuid>,
) -> Result<Json<ProvisioningJob>> {
    let job = state.service.start_provisioning(project_id)?;
    spawn_provisioning(state.service, job.project_id, job.id);
    Ok(Json(job))
}

async fn list_jobs(
    State(state): State<AppState>,
    Path(project_id): Path<Uuid>,
) -> Result<Json<Vec<ProvisioningJob>>> {
    state.service.list_jobs(project_id).map(Json)
}

fn spawn_provisioning(service: Arc<Service>, project_id: Uuid, job_id: Uuid) {
    tokio::task::spawn_blocking(move || service.run_provisioning(project_id, job_id));
}
