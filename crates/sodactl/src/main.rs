use std::{path::PathBuf, str::FromStr};

use bytes::Bytes;
use clap::{Args, Parser, Subcommand, ValueEnum};
use http_body_util::{BodyExt, Full};
use hyper::{Method, Request, client::conn::http1};
use hyper_util::rt::TokioIo;
use serde::Serialize;
use soda_core::{
    AddCollaboratorRequest, CreatePersonRequest, CreateProjectRequest, CreateWorktreeRequest,
    ProjectSource, Role, ToolchainProfile,
};
use tokio::net::UnixStream;
use uuid::Uuid;

#[derive(Debug, Parser)]
#[command(name = "sodactl", version, about = "Administer Soda OS")]
struct Cli {
    #[arg(long, default_value = "/run/soda/sodad.sock")]
    socket: PathBuf,
    #[command(subcommand)]
    command: Command,
}

#[derive(Debug, Subcommand)]
enum Command {
    Health,
    People {
        #[command(subcommand)]
        command: PeopleCommand,
    },
    Projects {
        #[command(subcommand)]
        command: ProjectCommand,
    },
    Collaborator(AddCollaborator),
    Worktrees {
        #[command(subcommand)]
        command: WorktreeCommand,
    },
}

#[derive(Debug, Subcommand)]
enum PeopleCommand {
    List,
    Add(AddPerson),
}

#[derive(Debug, Args)]
struct AddPerson {
    #[arg(long)]
    username: String,
    #[arg(long)]
    display_name: String,
    #[arg(long)]
    email: String,
    #[arg(long, value_enum, default_value = "developer")]
    role: RoleArg,
    #[arg(long)]
    ssh_key: PathBuf,
}

#[derive(Debug, Clone, Copy, ValueEnum)]
enum RoleArg {
    Admin,
    Developer,
}

#[derive(Debug, Subcommand)]
enum ProjectCommand {
    List,
    Create(CreateProject),
}

#[derive(Debug, Args)]
struct CreateProject {
    #[arg(long)]
    slug: String,
    #[arg(long)]
    name: String,
    #[arg(long, value_enum)]
    profile: ProfileArg,
    #[arg(long)]
    git: Option<String>,
}

#[derive(Debug, Clone, Copy, ValueEnum)]
enum ProfileArg {
    Web,
    Python,
    Rust,
    Go,
}

#[derive(Debug, Args)]
struct AddCollaborator {
    #[arg(long)]
    project: Uuid,
    #[arg(long)]
    person: Uuid,
}

#[derive(Debug, Subcommand)]
enum WorktreeCommand {
    List {
        #[arg(long)]
        project: Uuid,
    },
    Add(AddWorktree),
}

#[derive(Debug, Args)]
struct AddWorktree {
    #[arg(long)]
    project: Uuid,
    #[arg(long)]
    person: Uuid,
    #[arg(long)]
    name: String,
    #[arg(long, default_value = "main")]
    base: String,
}

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    let cli = Cli::parse();
    let response = match cli.command {
        Command::Health => request(&cli.socket, Method::GET, "/v1/health", None).await?,
        Command::People { command } => match command {
            PeopleCommand::List => request(&cli.socket, Method::GET, "/v1/people", None).await?,
            PeopleCommand::Add(args) => {
                let password = std::env::var("SODA_PERSON_PASSWORD")
                    .map_err(|_| anyhow::anyhow!("SODA_PERSON_PASSWORD is required"))?;
                let body = CreatePersonRequest {
                    username: args.username,
                    display_name: args.display_name,
                    email: args.email,
                    role: match args.role {
                        RoleArg::Admin => Role::Admin,
                        RoleArg::Developer => Role::Developer,
                    },
                    ssh_public_key: std::fs::read_to_string(args.ssh_key)?.trim().to_owned(),
                    password,
                };
                request_json(&cli.socket, Method::POST, "/v1/people", &body).await?
            }
        },
        Command::Projects { command } => match command {
            ProjectCommand::List => request(&cli.socket, Method::GET, "/v1/projects", None).await?,
            ProjectCommand::Create(args) => {
                let source =
                    args.git
                        .map_or(ProjectSource::Empty, |remote_url| ProjectSource::Git {
                            remote_url,
                        });
                let body = CreateProjectRequest {
                    slug: args.slug,
                    name: args.name,
                    profile: match args.profile {
                        ProfileArg::Web => ToolchainProfile::Web,
                        ProfileArg::Python => ToolchainProfile::Python,
                        ProfileArg::Rust => ToolchainProfile::Rust,
                        ProfileArg::Go => ToolchainProfile::Go,
                    },
                    source,
                };
                request_json(&cli.socket, Method::POST, "/v1/projects", &body).await?
            }
        },
        Command::Collaborator(args) => {
            let body = AddCollaboratorRequest {
                person_id: args.person,
            };
            request_json(
                &cli.socket,
                Method::POST,
                &format!("/v1/projects/{}/collaborators", args.project),
                &body,
            )
            .await?
        }
        Command::Worktrees { command } => match command {
            WorktreeCommand::List { project } => {
                request(
                    &cli.socket,
                    Method::GET,
                    &format!("/v1/projects/{project}/worktrees"),
                    None,
                )
                .await?
            }
            WorktreeCommand::Add(args) => {
                let body = CreateWorktreeRequest {
                    person_id: args.person,
                    name: args.name,
                    base_ref: args.base,
                };
                request_json(
                    &cli.socket,
                    Method::POST,
                    &format!("/v1/projects/{}/worktrees", args.project),
                    &body,
                )
                .await?
            }
        },
    };
    let json = serde_json::Value::from_str(&response)?;
    println!("{}", serde_json::to_string_pretty(&json)?);
    Ok(())
}

async fn request_json<T: Serialize>(
    socket: &PathBuf,
    method: Method,
    path: &str,
    body: &T,
) -> anyhow::Result<String> {
    request(socket, method, path, Some(serde_json::to_vec(body)?)).await
}

async fn request(
    socket: &PathBuf,
    method: Method,
    path: &str,
    body: Option<Vec<u8>>,
) -> anyhow::Result<String> {
    let stream = UnixStream::connect(socket).await?;
    let (mut sender, connection) = http1::handshake(TokioIo::new(stream)).await?;
    tokio::spawn(async move {
        if let Err(error) = connection.await {
            eprintln!("daemon connection failed: {error}");
        }
    });
    let mut builder = Request::builder()
        .method(method)
        .uri(path)
        .header("Host", "sodad");
    if body.is_some() {
        builder = builder.header("Content-Type", "application/json");
    }
    let request = builder.body(Full::new(Bytes::from(body.unwrap_or_default())))?;
    let response = sender.send_request(request).await?;
    let status = response.status();
    let bytes = response.into_body().collect().await?.to_bytes();
    let text = String::from_utf8(bytes.to_vec())?;
    if !status.is_success() {
        anyhow::bail!("sodad returned {status}: {text}");
    }
    Ok(text)
}
