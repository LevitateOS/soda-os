use std::{os::unix::fs::PermissionsExt, path::PathBuf};

use tokio::net::UnixListener;
use tracing::info;

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    tracing_subscriber::fmt()
        .with_env_filter(tracing_subscriber::EnvFilter::from_default_env())
        .init();

    let socket_path = env_path("SODA_SOCKET", "/run/soda/sodad.sock");
    let database_path = env_path("SODA_DATABASE", "/var/lib/soda/soda.db");
    let projects_root = env_path("SODA_PROJECTS_ROOT", "/srv/soda/projects");
    let toolchains_root = env_path("SODA_TOOLCHAINS_ROOT", "/opt/soda/toolchains");

    if let Some(parent) = socket_path.parent() {
        tokio::fs::create_dir_all(parent).await?;
    }
    if tokio::fs::try_exists(&socket_path).await? {
        tokio::fs::remove_file(&socket_path).await?;
    }
    if let Some(parent) = database_path.parent() {
        tokio::fs::create_dir_all(parent).await?;
    }

    let listener = UnixListener::bind(&socket_path)?;
    std::fs::set_permissions(&socket_path, std::fs::Permissions::from_mode(0o660))?;
    let app = sodad::production_router(database_path, projects_root, toolchains_root)?;

    info!(socket_path = %socket_path.display(), "sodad listening");
    axum::serve(listener, app).await?;
    Ok(())
}

fn env_path(name: &str, fallback: &str) -> PathBuf {
    std::env::var_os(name).map_or_else(|| PathBuf::from(fallback), PathBuf::from)
}
