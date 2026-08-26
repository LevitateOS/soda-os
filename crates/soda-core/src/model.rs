use serde::{Deserialize, Serialize};
use uuid::Uuid;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum Role {
    Admin,
    Developer,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Person {
    pub id: Uuid,
    pub username: String,
    pub display_name: String,
    pub email: String,
    pub role: Role,
    pub ssh_public_key: String,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum ProjectSource {
    Empty,
    Git { remote_url: String },
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ToolchainProfile {
    Web,
    Python,
    Rust,
    Go,
}

impl ToolchainProfile {
    #[must_use]
    pub const fn as_str(self) -> &'static str {
        match self {
            Self::Web => "web",
            Self::Python => "python",
            Self::Rust => "rust",
            Self::Go => "go",
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Project {
    pub id: Uuid,
    pub slug: String,
    pub name: String,
    pub unix_user: String,
    pub profile: ToolchainProfile,
    pub source: ProjectSource,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Membership {
    pub project_id: Uuid,
    pub person_id: Uuid,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Worktree {
    pub id: Uuid,
    pub project_id: Uuid,
    pub person_id: Uuid,
    pub name: String,
    pub branch: String,
    pub path: String,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum JobState {
    Installing,
    Ready,
    Failed,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ToolchainInstallation {
    pub id: Uuid,
    pub profile: ToolchainProfile,
    pub version: String,
    pub path: String,
    pub checksum: String,
    pub state: JobState,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ProvisioningJob {
    pub id: Uuid,
    pub project_id: Uuid,
    pub state: JobState,
    pub error: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct DeployKey {
    pub project_id: Uuid,
    pub public_key: String,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum RuntimeState {
    Ready,
    Degraded,
    Unavailable,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct ServiceStatus {
    pub name: String,
    pub state: RuntimeState,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct NetworkInterface {
    pub name: String,
    pub addresses: Vec<String>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct FilesystemStatus {
    pub path: String,
    pub total_bytes: u64,
    pub available_bytes: u64,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct HostStatus {
    pub sampled_at: u64,
    pub overall: RuntimeState,
    pub services: Vec<ServiceStatus>,
    pub ssh_firewall_ready: bool,
    pub cockpit_firewall_ready: bool,
    pub interfaces: Vec<NetworkInterface>,
    pub cpu_percent: Option<f64>,
    pub load_average: [f64; 3],
    pub uptime_seconds: u64,
    pub memory_total_bytes: u64,
    pub memory_available_bytes: u64,
    pub filesystems: Vec<FilesystemStatus>,
    pub ssh_observer: RuntimeState,
    pub git_observer: RuntimeState,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum WorktreeState {
    Clean,
    Dirty,
    Unavailable,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct WorktreeStatus {
    pub worktree_id: Uuid,
    pub branch: String,
    pub head: String,
    pub upstream: Option<String>,
    pub ahead: u64,
    pub behind: u64,
    pub staged: u64,
    pub modified: u64,
    pub untracked: u64,
    pub conflicted: u64,
    pub state: WorktreeState,
    pub error: Option<String>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum SshChannelKind {
    Interactive,
    Command,
    Sftp,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct SshChannel {
    pub kind: SshChannelKind,
    pub worktree_id: Uuid,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ActiveSshConnection {
    pub id: String,
    pub project_id: Uuid,
    pub person_id: Uuid,
    pub connected_at: u64,
    pub client_address: String,
    pub client_port: u16,
    pub server_address: String,
    pub server_port: u16,
    pub channels: Vec<SshChannel>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum EventKind {
    HostChanged,
    PeopleChanged,
    ProjectsChanged,
    WorktreesChanged,
    ProvisioningChanged,
    GitChanged,
    SessionsChanged,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct SodaEvent {
    pub kind: EventKind,
    pub project_id: Option<Uuid>,
    pub sequence: u64,
}
