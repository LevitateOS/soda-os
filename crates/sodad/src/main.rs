use axum::{Json, Router, routing::get};
use serde::Serialize;
use tokio::net::UnixListener;
use tracing::info;

#[derive(Serialize)]
struct Health {
    status: &'static str,
    service: &'static str,
    version: &'static str,
}

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    tracing_subscriber::fmt()
        .with_env_filter(tracing_subscriber::EnvFilter::from_default_env())
        .init();

    let socket_path =
        std::env::var("SODA_SOCKET").unwrap_or_else(|_| "/run/soda/sodad.sock".to_owned());
    if let Some(parent) = std::path::Path::new(&socket_path).parent() {
        tokio::fs::create_dir_all(parent).await?;
    }
    if tokio::fs::try_exists(&socket_path).await? {
        tokio::fs::remove_file(&socket_path).await?;
    }

    let listener = UnixListener::bind(&socket_path)?;
    let app = Router::new().route(
        "/v1/health",
        get(|| async {
            Json(Health {
                status: "ok",
                service: "sodad",
                version: env!("CARGO_PKG_VERSION"),
            })
        }),
    );

    info!(%socket_path, "sodad listening");
    axum::serve(listener, app).await?;
    Ok(())
}
