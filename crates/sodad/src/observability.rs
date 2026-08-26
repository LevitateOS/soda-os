use std::{
    collections::{HashMap, HashSet},
    fs,
    io::Write,
    path::Path,
    process::{Command, Stdio},
    sync::{Arc, Mutex, RwLock},
    time::{SystemTime, UNIX_EPOCH},
};

use serde_json::Value;
use soda_core::{
    ActiveSshConnection, EventKind, FilesystemStatus, HostStatus, NetworkInterface, Person,
    Project, RuntimeState, ServiceStatus, SshChannel, SshChannelKind, Worktree, WorktreeState,
    WorktreeStatus,
};
use uuid::Uuid;

use crate::{events::EventBroker, service::Service};

#[derive(Clone)]
pub struct Observability {
    service: Arc<Service>,
    events: EventBroker,
    host: Arc<RwLock<HostStatus>>,
    git: Arc<RwLock<HashMap<Uuid, Vec<WorktreeStatus>>>>,
    sessions: Arc<RwLock<Vec<ActiveSshConnection>>>,
    cpu: Arc<Mutex<Option<CpuSample>>>,
    ssh_observer: Arc<RwLock<RuntimeState>>,
    git_observer: Arc<RwLock<RuntimeState>>,
    project_interests: Arc<Mutex<HashMap<Uuid, usize>>>,
}

pub struct ProjectInterest {
    project_id: Option<Uuid>,
    interests: Arc<Mutex<HashMap<Uuid, usize>>>,
}

impl Drop for ProjectInterest {
    fn drop(&mut self) {
        let Some(project_id) = self.project_id else {
            return;
        };
        let mut interests = self.interests.lock().expect("interest lock poisoned");
        if let Some(count) = interests.get_mut(&project_id) {
            *count -= 1;
            if *count == 0 {
                interests.remove(&project_id);
            }
        }
    }
}

#[derive(Debug, Clone, Copy)]
struct CpuSample {
    total: u64,
    idle: u64,
}

#[derive(Debug, Clone)]
struct JournalConnection {
    project_id: Uuid,
    person_id: Uuid,
    project_user: String,
    connected_at: u64,
    client_address: String,
    client_port: u16,
}

impl Observability {
    pub fn start(service: Arc<Service>) -> Arc<Self> {
        let events = service.events();
        let this = Arc::new(Self {
            service,
            events,
            host: Arc::new(RwLock::new(empty_host_status())),
            git: Arc::new(RwLock::new(HashMap::new())),
            sessions: Arc::new(RwLock::new(Vec::new())),
            cpu: Arc::new(Mutex::new(None)),
            ssh_observer: Arc::new(RwLock::new(RuntimeState::Unavailable)),
            git_observer: Arc::new(RwLock::new(RuntimeState::Unavailable)),
            project_interests: Arc::new(Mutex::new(HashMap::new())),
        });
        this.refresh_host();
        this.refresh_git();
        this.refresh_sessions();
        spawn_observers(this.clone());
        this
    }

    pub fn host(&self) -> HostStatus {
        self.host.read().expect("host status lock poisoned").clone()
    }

    pub fn worktrees(&self, project_id: Uuid) -> Vec<WorktreeStatus> {
        if let Some(statuses) = self
            .git
            .read()
            .expect("git status lock poisoned")
            .get(&project_id)
            .cloned()
        {
            return statuses;
        }
        let Some(project) = self.service.list_projects().ok().and_then(|projects| {
            projects
                .into_iter()
                .find(|project| project.id == project_id)
        }) else {
            return Vec::new();
        };
        self.service
            .list_worktrees(project_id)
            .unwrap_or_default()
            .iter()
            .map(|worktree| inspect_worktree(&project, worktree))
            .collect()
    }

    pub fn sessions(&self) -> Vec<ActiveSshConnection> {
        self.sessions.read().expect("session lock poisoned").clone()
    }

    pub fn interest(&self, project_id: Option<Uuid>) -> ProjectInterest {
        if let Some(project_id) = project_id {
            *self
                .project_interests
                .lock()
                .expect("interest lock poisoned")
                .entry(project_id)
                .or_default() += 1;
        }
        ProjectInterest {
            project_id,
            interests: self.project_interests.clone(),
        }
    }

    fn refresh_host(&self) {
        let mut status = sample_host(&self.cpu);
        status.ssh_observer = *self
            .ssh_observer
            .read()
            .expect("SSH observer lock poisoned");
        status.git_observer = *self
            .git_observer
            .read()
            .expect("Git observer lock poisoned");
        if status.ssh_observer != RuntimeState::Ready || status.git_observer != RuntimeState::Ready
        {
            status.overall = RuntimeState::Degraded;
        }
        let changed = comparable_host(&status)
            != comparable_host(&self.host.read().expect("host lock poisoned"));
        *self.host.write().expect("host lock poisoned") = status;
        if changed {
            self.events.publish(EventKind::HostChanged, None);
        }
    }

    fn refresh_git(&self) {
        let projects = match self.service.list_projects() {
            Ok(projects) => projects,
            Err(error) => {
                tracing::warn!(%error, "Git observer could not list projects");
                *self
                    .git_observer
                    .write()
                    .expect("Git observer lock poisoned") = RuntimeState::Unavailable;
                return;
            }
        };
        let interested = self
            .project_interests
            .lock()
            .expect("interest lock poisoned")
            .keys()
            .copied()
            .collect::<HashSet<_>>();
        if interested.is_empty() {
            *self
                .git_observer
                .write()
                .expect("Git observer lock poisoned") = RuntimeState::Ready;
            return;
        }
        let mut next = HashMap::new();
        let mut observer_state = RuntimeState::Ready;
        for project in projects
            .into_iter()
            .filter(|project| interested.contains(&project.id))
        {
            let worktrees = match self.service.list_worktrees(project.id) {
                Ok(worktrees) => worktrees,
                Err(error) => {
                    tracing::warn!(%error, project_id = %project.id, "Git observer could not list worktrees");
                    observer_state = RuntimeState::Degraded;
                    continue;
                }
            };
            let statuses = worktrees
                .iter()
                .map(|worktree| inspect_worktree(&project, worktree))
                .collect::<Vec<_>>();
            if statuses
                .iter()
                .any(|status| status.state == WorktreeState::Unavailable)
            {
                observer_state = RuntimeState::Degraded;
            }
            next.insert(project.id, statuses);
        }
        let mut current = self.git.write().expect("Git status lock poisoned");
        for (project_id, statuses) in &next {
            if current.get(project_id) != Some(statuses) {
                self.events
                    .publish(EventKind::GitChanged, Some(*project_id));
            }
        }
        *current = next;
        *self
            .git_observer
            .write()
            .expect("Git observer lock poisoned") = observer_state;
    }

    fn refresh_sessions(&self) {
        match observe_sessions(&self.service) {
            Ok(next) => {
                let mut current = self.sessions.write().expect("session lock poisoned");
                if *current != next {
                    *current = next;
                    self.events.publish(EventKind::SessionsChanged, None);
                }
                *self
                    .ssh_observer
                    .write()
                    .expect("SSH observer lock poisoned") = RuntimeState::Ready;
            }
            Err(error) => {
                tracing::warn!(%error, "SSH observer unavailable");
                *self
                    .ssh_observer
                    .write()
                    .expect("SSH observer lock poisoned") = RuntimeState::Degraded;
            }
        }
    }
}

fn spawn_observers(observability: Arc<Observability>) {
    let host = observability.clone();
    tokio::spawn(async move {
        let mut interval = tokio::time::interval(std::time::Duration::from_secs(5));
        loop {
            interval.tick().await;
            host.refresh_host();
        }
    });
    let git = observability.clone();
    tokio::spawn(async move {
        let mut interval = tokio::time::interval(std::time::Duration::from_secs(3));
        loop {
            interval.tick().await;
            git.refresh_git();
        }
    });
    tokio::spawn(async move {
        let mut interval = tokio::time::interval(std::time::Duration::from_secs(1));
        loop {
            interval.tick().await;
            observability.refresh_sessions();
        }
    });
}

fn empty_host_status() -> HostStatus {
    HostStatus {
        sampled_at: now(),
        overall: RuntimeState::Unavailable,
        services: Vec::new(),
        ssh_firewall_ready: false,
        cockpit_firewall_ready: false,
        interfaces: Vec::new(),
        cpu_percent: None,
        load_average: [0.0; 3],
        uptime_seconds: 0,
        memory_total_bytes: 0,
        memory_available_bytes: 0,
        filesystems: Vec::new(),
        ssh_observer: RuntimeState::Unavailable,
        git_observer: RuntimeState::Unavailable,
    }
}

fn comparable_host(status: &HostStatus) -> HostStatus {
    let mut comparable = status.clone();
    comparable.sampled_at = 0;
    comparable
}

fn sample_host(cpu_state: &Mutex<Option<CpuSample>>) -> HostStatus {
    let service_names = [
        "sodad",
        "soda-authd",
        "soda-cockpit",
        "sshd",
        "avahi-daemon",
        "NetworkManager",
        "firewalld",
    ];
    let services = service_names
        .into_iter()
        .map(|name| ServiceStatus {
            name: name.to_owned(),
            state: if name == "sodad" {
                RuntimeState::Ready
            } else {
                service_state(name)
            },
        })
        .collect::<Vec<_>>();
    let interfaces = network_interfaces();
    let (memory_total_bytes, memory_available_bytes) = memory_status();
    let load_average = load_average();
    let uptime_seconds = fs::read_to_string("/proc/uptime")
        .ok()
        .and_then(|value| {
            value
                .split_whitespace()
                .next()?
                .split('.')
                .next()?
                .parse::<u64>()
                .ok()
        })
        .unwrap_or(0);
    let cpu_percent = cpu_percent(cpu_state);
    let filesystems = ["/", "/srv/soda/projects", "/opt/soda/toolchains"]
        .into_iter()
        .filter_map(filesystem_status)
        .collect::<Vec<_>>();
    let ssh_firewall_ready = firewall_ready("--query-service", "ssh");
    let cockpit_firewall_ready = firewall_ready("--query-port", "9090/tcp");
    let services_ready = services
        .iter()
        .all(|service| service.state == RuntimeState::Ready);
    let overall =
        if services_ready && ssh_firewall_ready && cockpit_firewall_ready && !interfaces.is_empty()
        {
            RuntimeState::Ready
        } else {
            RuntimeState::Degraded
        };
    HostStatus {
        sampled_at: now(),
        overall,
        services,
        ssh_firewall_ready,
        cockpit_firewall_ready,
        interfaces,
        cpu_percent,
        load_average,
        uptime_seconds,
        memory_total_bytes,
        memory_available_bytes,
        filesystems,
        ssh_observer: RuntimeState::Unavailable,
        git_observer: RuntimeState::Unavailable,
    }
}

fn service_state(name: &str) -> RuntimeState {
    Command::new("systemctl")
        .args(["is-active", "--quiet", name])
        .status()
        .map_or(RuntimeState::Unavailable, |status| {
            if status.success() {
                RuntimeState::Ready
            } else {
                RuntimeState::Degraded
            }
        })
}

fn firewall_ready(flag: &str, value: &str) -> bool {
    Command::new("firewall-cmd")
        .args(["--quiet", flag, value])
        .status()
        .is_ok_and(|status| status.success())
}

fn network_interfaces() -> Vec<NetworkInterface> {
    let Ok(output) = Command::new("ip")
        .args(["-json", "address", "show", "up"])
        .output()
    else {
        return Vec::new();
    };
    let Ok(value) = serde_json::from_slice::<Value>(&output.stdout) else {
        return Vec::new();
    };
    value
        .as_array()
        .into_iter()
        .flatten()
        .filter_map(|interface| {
            let name = interface.get("ifname")?.as_str()?;
            if name == "lo" {
                return None;
            }
            let addresses = interface
                .get("addr_info")?
                .as_array()?
                .iter()
                .filter(|address| address.get("scope").and_then(Value::as_str) == Some("global"))
                .filter_map(|address| address.get("local")?.as_str().map(str::to_owned))
                .collect::<Vec<_>>();
            (!addresses.is_empty()).then(|| NetworkInterface {
                name: name.to_owned(),
                addresses,
            })
        })
        .collect()
}

fn memory_status() -> (u64, u64) {
    let contents = fs::read_to_string("/proc/meminfo").unwrap_or_default();
    let value = |name: &str| {
        contents
            .lines()
            .find_map(|line| line.strip_prefix(name))
            .and_then(|line| line.split_whitespace().next())
            .and_then(|value| value.parse::<u64>().ok())
            .unwrap_or(0)
            * 1024
    };
    (value("MemTotal:"), value("MemAvailable:"))
}

fn load_average() -> [f64; 3] {
    let values = fs::read_to_string("/proc/loadavg").unwrap_or_default();
    let mut fields = values
        .split_whitespace()
        .take(3)
        .filter_map(|value| value.parse().ok());
    [
        fields.next().unwrap_or(0.0),
        fields.next().unwrap_or(0.0),
        fields.next().unwrap_or(0.0),
    ]
}

#[allow(clippy::cast_precision_loss)]
fn cpu_percent(state: &Mutex<Option<CpuSample>>) -> Option<f64> {
    let line = fs::read_to_string("/proc/stat").ok()?;
    let fields = line.lines().next()?.split_whitespace().skip(1);
    let values = fields
        .filter_map(|value| value.parse::<u64>().ok())
        .collect::<Vec<_>>();
    if values.len() < 4 {
        return None;
    }
    let sample = CpuSample {
        total: values.iter().sum(),
        idle: values[3] + values.get(4).copied().unwrap_or(0),
    };
    let mut previous = state.lock().expect("CPU sample lock poisoned");
    let percent = previous.and_then(|old| {
        let total = sample.total.saturating_sub(old.total);
        let idle = sample.idle.saturating_sub(old.idle);
        (total > 0).then(|| 100.0 * (total - idle) as f64 / total as f64)
    });
    *previous = Some(sample);
    percent
}

fn filesystem_status(path: &str) -> Option<FilesystemStatus> {
    let path_ref = Path::new(path);
    if !path_ref.exists() {
        return None;
    }
    Some(FilesystemStatus {
        path: path.to_owned(),
        total_bytes: fs2::total_space(path_ref).ok()?,
        available_bytes: fs2::available_space(path_ref).ok()?,
    })
}

fn inspect_worktree(project: &Project, worktree: &Worktree) -> WorktreeStatus {
    let running_as_root = Command::new("id")
        .arg("-u")
        .output()
        .is_ok_and(|output| output.status.success() && output.stdout == b"0\n");
    let mut command = if running_as_root
        && Command::new("id")
            .args(["-u", &project.unix_user])
            .status()
            .is_ok_and(|status| status.success())
    {
        let mut command = Command::new("runuser");
        command.args(["--user", &project.unix_user, "--", "git"]);
        command
    } else {
        Command::new("git")
    };
    let output = command
        .args([
            "-C",
            &worktree.path,
            "status",
            "--porcelain=v2",
            "--branch",
            "--untracked-files=normal",
        ])
        .env("GIT_OPTIONAL_LOCKS", "0")
        .env("LC_ALL", "C")
        .output();
    match output {
        Ok(output) if output.status.success() => parse_git_status(worktree, &output.stdout),
        Ok(output) => unavailable_worktree(
            worktree,
            String::from_utf8_lossy(&output.stderr).trim().to_owned(),
        ),
        Err(error) => unavailable_worktree(worktree, error.to_string()),
    }
}

fn unavailable_worktree(worktree: &Worktree, error: String) -> WorktreeStatus {
    WorktreeStatus {
        worktree_id: worktree.id,
        branch: worktree.branch.clone(),
        head: String::new(),
        upstream: None,
        ahead: 0,
        behind: 0,
        staged: 0,
        modified: 0,
        untracked: 0,
        conflicted: 0,
        state: WorktreeState::Unavailable,
        error: Some(if error.is_empty() {
            "Git status unavailable".to_owned()
        } else {
            error
        }),
    }
}

fn parse_git_status(worktree: &Worktree, bytes: &[u8]) -> WorktreeStatus {
    let contents = String::from_utf8_lossy(bytes);
    let mut status = WorktreeStatus {
        worktree_id: worktree.id,
        branch: worktree.branch.clone(),
        head: String::new(),
        upstream: None,
        ahead: 0,
        behind: 0,
        staged: 0,
        modified: 0,
        untracked: 0,
        conflicted: 0,
        state: WorktreeState::Clean,
        error: None,
    };
    for line in contents.lines() {
        if let Some(value) = line.strip_prefix("# branch.head ") {
            value.clone_into(&mut status.branch);
        } else if let Some(value) = line.strip_prefix("# branch.oid ") {
            status.head = value.chars().take(12).collect();
        } else if let Some(value) = line.strip_prefix("# branch.upstream ") {
            status.upstream = Some(value.to_owned());
        } else if let Some(value) = line.strip_prefix("# branch.ab ") {
            for field in value.split_whitespace() {
                if let Some(ahead) = field.strip_prefix('+') {
                    status.ahead = ahead.parse().unwrap_or(0);
                } else if let Some(behind) = field.strip_prefix('-') {
                    status.behind = behind.parse().unwrap_or(0);
                }
            }
        } else if line.starts_with("u ") {
            status.conflicted += 1;
        } else if line.starts_with("? ") {
            status.untracked += 1;
        } else if line.starts_with("1 ") || line.starts_with("2 ") {
            let xy = line.split_whitespace().nth(1).unwrap_or("..").as_bytes();
            if xy.first().is_some_and(|value| *value != b'.') {
                status.staged += 1;
            }
            if xy.get(1).is_some_and(|value| *value != b'.') {
                status.modified += 1;
            }
        }
    }
    if status.staged + status.modified + status.untracked + status.conflicted > 0 {
        status.state = WorktreeState::Dirty;
    }
    status
}

#[allow(clippy::too_many_lines)]
fn observe_sessions(service: &Service) -> Result<Vec<ActiveSshConnection>, String> {
    let projects = service.list_projects().map_err(|error| error.to_string())?;
    let people = service.list_people().map_err(|error| error.to_string())?;
    let output = Command::new("journalctl")
        .args([
            "--boot",
            "--unit=sshd.service",
            "--output=json",
            "--no-pager",
        ])
        .output()
        .map_err(|error| error.to_string())?;
    if !output.status.success() {
        return Err(String::from_utf8_lossy(&output.stderr).trim().to_owned());
    }
    let fingerprints = people
        .iter()
        .filter_map(|person| key_fingerprint(&person.ssh_public_key).map(|value| (value, person)))
        .collect::<HashMap<_, _>>();
    let project_users = projects
        .iter()
        .map(|project| (project.unix_user.as_str(), project))
        .collect::<HashMap<_, _>>();
    let mut active = HashMap::<String, JournalConnection>::new();
    let mut unrecognized_project_event = false;
    for line in String::from_utf8_lossy(&output.stdout).lines() {
        let Ok(value) = serde_json::from_str::<Value>(line) else {
            continue;
        };
        let Some(message) = value.get("MESSAGE").and_then(Value::as_str) else {
            continue;
        };
        if message.starts_with("Accepted publickey for soda-p-")
            && parse_accepted(message).is_none()
        {
            unrecognized_project_event = true;
            continue;
        }
        if message.starts_with("Disconnected from user soda-p-")
            && parse_disconnected(message).is_none()
        {
            unrecognized_project_event = true;
            continue;
        }
        if let Some((user, address, port, fingerprint)) = parse_accepted(message) {
            let Some(project) = project_users.get(user.as_str()) else {
                continue;
            };
            let Some(person) = fingerprints.get(fingerprint.as_str()) else {
                continue;
            };
            let connected_at = value
                .get("__REALTIME_TIMESTAMP")
                .and_then(Value::as_str)
                .and_then(|value| value.parse::<u64>().ok())
                .map_or_else(now, |micros| micros / 1_000_000);
            active.insert(
                connection_key(&user, &address, port),
                JournalConnection {
                    project_id: project.id,
                    person_id: person.id,
                    project_user: user,
                    connected_at,
                    client_address: address,
                    client_port: port,
                },
            );
        } else if let Some((user, address, port)) = parse_disconnected(message) {
            active.remove(&connection_key(&user, &address, port));
        }
    }
    if unrecognized_project_event {
        return Err("OpenSSH journal format is not recognized".to_owned());
    }
    let sockets = established_ssh_sockets();
    if let Some(sockets) = &sockets {
        active.retain(|_, connection| {
            sockets.contains_key(&(connection.client_address.clone(), connection.client_port))
        });
    }
    let worktrees = projects
        .iter()
        .flat_map(|project| service.list_worktrees(project.id).unwrap_or_default())
        .collect::<Vec<_>>();
    let channels = process_channels(&people, &worktrees);
    let mut result = active
        .into_values()
        .map(|connection| {
            let endpoint = (connection.client_address.clone(), connection.client_port);
            let mut channel_list = channels.get(&endpoint).cloned().unwrap_or_default();
            channel_list.sort_by_key(|channel| (channel.worktree_id, channel.kind as u8));
            ActiveSshConnection {
                id: connection_key(
                    &connection.project_user,
                    &connection.client_address,
                    connection.client_port,
                ),
                project_id: connection.project_id,
                person_id: connection.person_id,
                connected_at: connection.connected_at,
                client_address: connection.client_address,
                client_port: connection.client_port,
                server_address: sockets
                    .as_ref()
                    .and_then(|sockets| sockets.get(&endpoint))
                    .map_or_else(|| "soda".to_owned(), |endpoint| endpoint.0.clone()),
                server_port: sockets
                    .as_ref()
                    .and_then(|sockets| sockets.get(&endpoint))
                    .map_or(22, |endpoint| endpoint.1),
                channels: channel_list,
            }
        })
        .collect::<Vec<_>>();
    result.sort_by_key(|connection| (connection.project_id, connection.connected_at));
    Ok(result)
}

fn key_fingerprint(key: &str) -> Option<String> {
    if key.trim().is_empty() {
        return None;
    }
    let mut child = Command::new("ssh-keygen")
        .args(["-lf", "-"])
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .spawn()
        .ok()?;
    child.stdin.as_mut()?.write_all(key.as_bytes()).ok()?;
    let output = child.wait_with_output().ok()?;
    output
        .status
        .success()
        .then(|| {
            String::from_utf8_lossy(&output.stdout)
                .split_whitespace()
                .nth(1)
                .map(str::to_owned)
        })
        .flatten()
}

fn parse_accepted(message: &str) -> Option<(String, String, u16, String)> {
    let value = message.strip_prefix("Accepted publickey for ")?;
    let (user, value) = value.split_once(" from ")?;
    let (address, value) = value.split_once(" port ")?;
    let (port, details) = value.split_once(" ssh2: ")?;
    let fingerprint = details.split_whitespace().last()?;
    Some((
        user.to_owned(),
        address.to_owned(),
        port.parse().ok()?,
        fingerprint.to_owned(),
    ))
}

fn parse_disconnected(message: &str) -> Option<(String, String, u16)> {
    let value = message.strip_prefix("Disconnected from user ")?;
    let mut fields = value.split_whitespace();
    let user = fields.next()?.to_owned();
    let address = fields.next()?.to_owned();
    (fields.next()? == "port").then_some(())?;
    Some((user, address, fields.next()?.parse().ok()?))
}

fn connection_key(user: &str, address: &str, port: u16) -> String {
    format!("{user}|{address}|{port}")
}

fn established_ssh_sockets() -> Option<HashMap<(String, u16), (String, u16)>> {
    let output = Command::new("ss")
        .args(["-Htn", "state", "established"])
        .output()
        .ok()?;
    if !output.status.success() {
        return None;
    }
    Some(
        String::from_utf8_lossy(&output.stdout)
            .lines()
            .filter_map(|line| {
                let fields = line.split_whitespace().collect::<Vec<_>>();
                let local = parse_endpoint(fields.get(fields.len().checked_sub(2)?)?)?;
                let peer = parse_endpoint(fields.last()?)?;
                (local.1 == 22).then_some((peer, local))
            })
            .collect(),
    )
}

fn parse_endpoint(value: &str) -> Option<(String, u16)> {
    let (address, port) = value.rsplit_once(':')?;
    Some((
        address.trim_matches(['[', ']']).to_owned(),
        port.parse().ok()?,
    ))
}

fn process_channels(
    people: &[Person],
    worktrees: &[Worktree],
) -> HashMap<(String, u16), Vec<SshChannel>> {
    let people = people
        .iter()
        .map(|person| (person.username.as_str(), person.id))
        .collect::<HashMap<_, _>>();
    let worktrees = worktrees
        .iter()
        .map(|worktree| (worktree.path.as_str(), worktree))
        .collect::<HashMap<_, _>>();
    let mut channels = HashMap::<(String, u16), HashSet<(Uuid, SshChannelKind)>>::new();
    let Ok(entries) = fs::read_dir("/proc") else {
        return HashMap::new();
    };
    for entry in entries.flatten() {
        if !entry
            .file_name()
            .to_string_lossy()
            .bytes()
            .all(|byte| byte.is_ascii_digit())
        {
            continue;
        }
        let Ok(contents) = fs::read(entry.path().join("environ")) else {
            continue;
        };
        let environment = contents
            .split(|byte| *byte == 0)
            .filter_map(|field| String::from_utf8(field.to_vec()).ok())
            .filter_map(|field| {
                field
                    .split_once('=')
                    .map(|(name, value)| (name.to_owned(), value.to_owned()))
            })
            .collect::<HashMap<_, _>>();
        let (Some(actor), Some(path), Some(connection)) = (
            environment.get("SODA_ACTOR"),
            environment.get("SODA_WORKTREE"),
            environment.get("SSH_CONNECTION"),
        ) else {
            continue;
        };
        let Some(worktree) = worktrees.get(path.as_str()) else {
            continue;
        };
        if people.get(actor.as_str()).copied() != Some(worktree.person_id) {
            continue;
        }
        let mut fields = connection.split_whitespace();
        let (Some(address), Some(port)) = (
            fields.next(),
            fields.next().and_then(|value| value.parse().ok()),
        ) else {
            continue;
        };
        let kind = match environment.get("SSH_ORIGINAL_COMMAND").map(String::as_str) {
            Some("internal-sftp") => SshChannelKind::Sftp,
            Some(command) if !command.is_empty() => SshChannelKind::Command,
            _ => SshChannelKind::Interactive,
        };
        channels
            .entry((address.to_owned(), port))
            .or_default()
            .insert((worktree.id, kind));
    }
    channels
        .into_iter()
        .map(|(endpoint, values)| {
            (
                endpoint,
                values
                    .into_iter()
                    .map(|(worktree_id, kind)| SshChannel { kind, worktree_id })
                    .collect(),
            )
        })
        .collect()
}

fn now() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs()
}

#[cfg(test)]
mod tests {
    use super::*;

    fn worktree() -> Worktree {
        Worktree {
            id: Uuid::nil(),
            project_id: Uuid::nil(),
            person_id: Uuid::nil(),
            name: "default".to_owned(),
            branch: "people/alice".to_owned(),
            path: "/tmp/example".to_owned(),
        }
    }

    #[test]
    fn parses_git_summary_counts() {
        let status = parse_git_status(
            &worktree(),
            b"# branch.oid 1234567890abcdef\n# branch.head main\n# branch.upstream origin/main\n# branch.ab +2 -1\n1 M. N... 100644 100644 100644 a b staged\n1 .M N... 100644 100644 100644 a b modified\n? new.txt\nu UU N... 100644 100644 100644 100644 a b c conflict\n",
        );
        assert_eq!(status.head, "1234567890ab");
        assert_eq!((status.ahead, status.behind), (2, 1));
        assert_eq!(
            (
                status.staged,
                status.modified,
                status.untracked,
                status.conflicted
            ),
            (1, 1, 1, 1)
        );
        assert_eq!(status.state, WorktreeState::Dirty);
    }

    #[test]
    fn parses_ssh_lifecycle_messages() {
        assert_eq!(
            parse_accepted(
                "Accepted publickey for soda-p-demo from 192.0.2.4 port 54321 ssh2: ED25519 SHA256:key"
            ),
            Some((
                "soda-p-demo".to_owned(),
                "192.0.2.4".to_owned(),
                54321,
                "SHA256:key".to_owned()
            ))
        );
        assert_eq!(
            parse_disconnected("Disconnected from user soda-p-demo 2001:db8::4 port 54321"),
            Some(("soda-p-demo".to_owned(), "2001:db8::4".to_owned(), 54321))
        );
    }
}
