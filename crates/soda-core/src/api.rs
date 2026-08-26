use serde::{Deserialize, Serialize};
use uuid::Uuid;

use crate::{ProjectSource, Role, ToolchainProfile};

#[derive(Clone, Serialize, Deserialize)]
pub struct CreatePersonRequest {
    pub username: String,
    pub display_name: String,
    pub email: String,
    pub role: Role,
    pub ssh_public_key: String,
    pub password: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CreateProjectRequest {
    pub slug: String,
    pub name: String,
    pub profile: ToolchainProfile,
    pub source: ProjectSource,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AddCollaboratorRequest {
    pub person_id: Uuid,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CreateWorktreeRequest {
    pub person_id: Uuid,
    pub name: String,
    pub base_ref: String,
}
