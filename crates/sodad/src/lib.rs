mod api;
mod error;
mod service;
mod store;
mod system;
mod toolchains;

use std::{path::PathBuf, sync::Arc};

use axum::Router;
use service::Service;
use store::Store;
use system::HostSystem;

/// Builds the production daemon router and initializes persistent state.
///
/// # Errors
///
/// Returns an error when the database cannot be opened or migrated.
pub fn production_router(
    database: PathBuf,
    projects_root: PathBuf,
    toolchains_root: PathBuf,
) -> anyhow::Result<Router> {
    let store = Arc::new(Store::open(database)?);
    let system = Arc::new(HostSystem::production(&projects_root));
    let installer_administrator = if store.list_people()?.is_empty() {
        system.installer_administrator()?
    } else {
        None
    };
    let toolchains = Arc::new(toolchains::ToolchainManager::new(toolchains_root)?);
    let service = Arc::new(Service::new(store, system, toolchains, projects_root));
    if let Some(administrator) = installer_administrator {
        service.import_person(administrator)?;
    }
    Ok(api::router(service))
}
