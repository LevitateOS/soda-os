use std::sync::Arc;

use axum::{
    Json, Router,
    extract::{Path, State},
    routing::{get, post},
};
use serde::Serialize;
use soda_core::{
    AddCollaboratorRequest, CreatePersonRequest, CreateProjectRequest, CreateWorktreeRequest,
    Person, Project, Worktree,
};
use uuid::Uuid;

use crate::{error::Result, service::Service};

#[derive(Clone)]
struct AppState {
    service: Arc<Service>,
}

#[derive(Serialize)]
struct Health {
    status: &'static str,
    service: &'static str,
    version: &'static str,
}

pub fn router(service: Arc<Service>) -> Router {
    Router::new()
        .route("/v1/health", get(health))
        .route("/v1/people", get(list_people).post(create_person))
        .route("/v1/projects", get(list_projects).post(create_project))
        .route(
            "/v1/projects/{project_id}/collaborators",
            post(add_collaborator),
        )
        .route(
            "/v1/projects/{project_id}/worktrees",
            get(list_worktrees).post(create_worktree),
        )
        .with_state(AppState { service })
}

async fn health() -> Json<Health> {
    Json(Health {
        status: "ok",
        service: "sodad",
        version: env!("CARGO_PKG_VERSION"),
    })
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

async fn create_project(
    State(state): State<AppState>,
    Json(request): Json<CreateProjectRequest>,
) -> Result<Json<Project>> {
    state.service.create_project(request).map(Json)
}

async fn list_projects(State(state): State<AppState>) -> Result<Json<Vec<Project>>> {
    state.service.list_projects().map(Json)
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
