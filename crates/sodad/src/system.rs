use std::{
    fs::{self, OpenOptions},
    io::Write,
    os::unix::fs::PermissionsExt,
    path::{Path, PathBuf},
    process::{Command, Stdio},
};

use soda_core::{Person, Project, ProjectSource, Role, Worktree};

use crate::error::{AppError, Result};

pub trait SystemOps: Send + Sync {
    fn create_person(&self, person: &Person, password: &str) -> Result<()>;
    fn import_person(&self, person: &Person) -> Result<()>;
    fn create_project(&self, project: &Project) -> Result<()>;
    fn ensure_repository(&self, project: &Project) -> Result<()>;
    fn create_worktree(
        &self,
        project: &Project,
        person: &Person,
        worktree: &Worktree,
        base_ref: &str,
    ) -> Result<()>;
    fn write_project_environment(&self, project: &Project, contents: &str) -> Result<()>;
}

#[derive(Debug, Clone)]
pub struct HostSystem {
    projects_root: PathBuf,
    manage_accounts: bool,
}

impl HostSystem {
    pub fn production(projects_root: impl Into<PathBuf>) -> Self {
        Self {
            projects_root: projects_root.into(),
            manage_accounts: true,
        }
    }

    #[cfg(test)]
    pub fn test(projects_root: impl Into<PathBuf>) -> Self {
        Self {
            projects_root: projects_root.into(),
            manage_accounts: false,
        }
    }

    fn project_root(&self, project: &Project) -> PathBuf {
        self.projects_root.join(&project.slug)
    }

    fn repository(&self, project: &Project) -> PathBuf {
        self.project_root(project).join("repository.git")
    }

    fn ensure_groups(&self) -> Result<()> {
        if !self.manage_accounts {
            return Ok(());
        }
        run(Command::new("groupadd").args(["--force", "--system", "soda-admins"]))?;
        run(Command::new("groupadd").args(["--force", "--system", "soda-developers"]))?;
        Ok(())
    }

    fn chown_project(&self, project: &Project, path: &Path) -> Result<()> {
        if self.manage_accounts {
            run(Command::new("chown").args([
                "--recursive",
                &format!("{}:{}", project.unix_user, project.unix_user),
                &path.display().to_string(),
            ]))?;
        }
        Ok(())
    }
}

impl SystemOps for HostSystem {
    fn create_person(&self, person: &Person, password: &str) -> Result<()> {
        validate_key(&person.ssh_public_key)?;
        if !self.manage_accounts {
            return Ok(());
        }
        self.ensure_groups()?;
        let role_group = match person.role {
            Role::Admin => "soda-admins",
            Role::Developer => "soda-developers",
        };
        run(Command::new("useradd").args([
            "--create-home",
            "--groups",
            role_group,
            "--shell",
            "/sbin/nologin",
            &person.username,
        ]))?;
        run_with_input(
            &mut Command::new("chpasswd"),
            &format!("{}:{password}\n", person.username),
        )?;
        Ok(())
    }

    fn import_person(&self, person: &Person) -> Result<()> {
        validate_key(&person.ssh_public_key)?;
        if !self.manage_accounts {
            return Ok(());
        }
        let status = Command::new("getent")
            .args(["passwd", &person.username])
            .status()?;
        if !status.success() {
            return Err(AppError::NotFound(format!(
                "Linux account {}",
                person.username
            )));
        }
        self.ensure_groups()?;
        let role_group = match person.role {
            Role::Admin => "soda-admins",
            Role::Developer => "soda-developers",
        };
        run(Command::new("usermod").args(["--append", "--groups", role_group, &person.username]))
    }

    fn create_project(&self, project: &Project) -> Result<()> {
        let root = self.project_root(project);
        if self.manage_accounts {
            run(Command::new("useradd").args([
                "--system",
                "--create-home",
                "--home-dir",
                &root.display().to_string(),
                "--shell",
                "/bin/bash",
                &project.unix_user,
            ]))?;
        } else {
            fs::create_dir_all(&root)?;
        }

        let ssh_dir = root.join(".ssh");
        fs::create_dir_all(&ssh_dir)?;
        fs::set_permissions(&ssh_dir, fs::Permissions::from_mode(0o700))?;
        run(Command::new("ssh-keygen").args([
            "-q",
            "-t",
            "ed25519",
            "-N",
            "",
            "-C",
            &format!("soda-project-{}", project.slug),
            "-f",
            &ssh_dir.join("deploy_key").display().to_string(),
        ]))?;
        let authorized_keys = ssh_dir.join("authorized_keys");
        OpenOptions::new()
            .create(true)
            .append(true)
            .open(&authorized_keys)?;
        fs::set_permissions(&authorized_keys, fs::Permissions::from_mode(0o600))?;
        if project.source == ProjectSource::Empty {
            initialize_empty_repository(&self.repository(project))?;
        }
        if self.manage_accounts {
            run(Command::new("restorecon").args(["-R", &root.display().to_string()]))?;
        }
        self.chown_project(project, &root)?;
        Ok(())
    }

    fn ensure_repository(&self, project: &Project) -> Result<()> {
        let repository = self.repository(project);
        if repository.is_dir() {
            return Ok(());
        }
        let ProjectSource::Git { remote_url } = &project.source else {
            return Err(AppError::System(format!(
                "project {} repository is missing",
                project.slug
            )));
        };
        let deploy_key = self.project_root(project).join(".ssh/deploy_key");
        let ssh_command = format!(
            "ssh -i {} -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new",
            deploy_key.display()
        );
        run(Command::new("git")
            .args([
                "clone",
                "--bare",
                remote_url,
                &repository.display().to_string(),
            ])
            .env("GIT_SSH_COMMAND", ssh_command))?;
        self.chown_project(project, &repository)
    }

    fn create_worktree(
        &self,
        project: &Project,
        person: &Person,
        worktree: &Worktree,
        base_ref: &str,
    ) -> Result<()> {
        validate_key(&person.ssh_public_key)?;
        let repository = self.repository(project);
        let path = Path::new(&worktree.path);
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent)?;
            self.chown_project(project, parent)?;
        }
        run(Command::new("git").args([
            "--git-dir",
            &repository.display().to_string(),
            "worktree",
            "add",
            "-b",
            &worktree.branch,
            &worktree.path,
            base_ref,
        ]))?;
        run(Command::new("git").args([
            "--git-dir",
            &repository.display().to_string(),
            "config",
            "extensions.worktreeConfig",
            "true",
        ]))?;
        run(Command::new("git").args([
            "-C",
            &worktree.path,
            "config",
            "--worktree",
            "core.bare",
            "false",
        ]))?;
        run(Command::new("git").args([
            "-C",
            &worktree.path,
            "config",
            "--worktree",
            "user.name",
            &person.display_name,
        ]))?;
        run(Command::new("git").args([
            "-C",
            &worktree.path,
            "config",
            "--worktree",
            "user.email",
            &person.email,
        ]))?;

        let authorized_keys = self.project_root(project).join(".ssh/authorized_keys");
        let mut file = OpenOptions::new()
            .create(true)
            .append(true)
            .open(&authorized_keys)?;
        writeln!(
            file,
            "command=\"/usr/libexec/soda/soda-ssh --actor {} --worktree {}\" {}",
            person.username, worktree.path, person.ssh_public_key
        )?;
        self.chown_project(project, &repository)?;
        self.chown_project(project, path)?;
        self.chown_project(project, &authorized_keys)?;
        Ok(())
    }

    fn write_project_environment(&self, project: &Project, contents: &str) -> Result<()> {
        let soda_dir = self.project_root(project).join(".soda");
        fs::create_dir_all(&soda_dir)?;
        fs::write(soda_dir.join("env"), contents)?;
        self.chown_project(project, &soda_dir)
    }
}

fn initialize_empty_repository(repository: &Path) -> Result<()> {
    run(Command::new("git").args([
        "init",
        "--bare",
        "--initial-branch=main",
        &repository.display().to_string(),
    ]))?;
    let tree = output_with_input(
        Command::new("git").args(["--git-dir", &repository.display().to_string(), "mktree"]),
        "",
    )?;
    let mut commit = Command::new("git");
    commit
        .args([
            "--git-dir",
            &repository.display().to_string(),
            "commit-tree",
            tree.trim(),
            "-m",
            "Initialize Soda project",
        ])
        .env("GIT_AUTHOR_NAME", "Soda OS")
        .env("GIT_AUTHOR_EMAIL", "soda@soda.local")
        .env("GIT_COMMITTER_NAME", "Soda OS")
        .env("GIT_COMMITTER_EMAIL", "soda@soda.local");
    let commit_id = output(&mut commit)?;
    run(Command::new("git").args([
        "--git-dir",
        &repository.display().to_string(),
        "update-ref",
        "refs/heads/main",
        commit_id.trim(),
    ]))?;
    Ok(())
}

fn validate_key(key: &str) -> Result<()> {
    if key.contains(['\r', '\n'])
        || !(key.starts_with("ssh-ed25519 ") || key.starts_with("ssh-rsa "))
    {
        return Err(AppError::Invalid(
            "SSH public key is not a supported single-line key".to_owned(),
        ));
    }
    Ok(())
}

fn run(command: &mut Command) -> Result<()> {
    let display = format!("{command:?}");
    let status = command.status()?;
    if status.success() {
        Ok(())
    } else {
        Err(AppError::System(format!("{display} exited with {status}")))
    }
}

fn output(command: &mut Command) -> Result<String> {
    let display = format!("{command:?}");
    let result = command.output()?;
    if !result.status.success() {
        return Err(AppError::System(format!(
            "{display} exited with {}: {}",
            result.status,
            String::from_utf8_lossy(&result.stderr).trim()
        )));
    }
    String::from_utf8(result.stdout)
        .map_err(|error| AppError::System(format!("{display} returned invalid UTF-8: {error}")))
}

fn run_with_input(command: &mut Command, input: &str) -> Result<()> {
    output_with_input(command, input).map(drop)
}

fn output_with_input(command: &mut Command, input: &str) -> Result<String> {
    let display = format!("{command:?}");
    let mut child = command
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()?;
    child
        .stdin
        .take()
        .expect("piped stdin exists")
        .write_all(input.as_bytes())?;
    let result = child.wait_with_output()?;
    if !result.status.success() {
        return Err(AppError::System(format!(
            "{display} exited with {}: {}",
            result.status,
            String::from_utf8_lossy(&result.stderr).trim()
        )));
    }
    String::from_utf8(result.stdout)
        .map_err(|error| AppError::System(format!("{display} returned invalid UTF-8: {error}")))
}

#[cfg(test)]
mod tests {
    use soda_core::{Project, ProjectSource, ToolchainProfile};
    use tempfile::TempDir;
    use uuid::Uuid;

    use super::*;

    #[test]
    fn prepares_deploy_key_before_continuing_git_clone() {
        let temp = TempDir::new().expect("temp dir");
        let projects = temp.path().join("projects");
        let remote = temp.path().join("remote.git");
        let project = Project {
            id: Uuid::new_v4(),
            slug: "private-project".to_owned(),
            name: "Private project".to_owned(),
            unix_user: "soda-p-private-project".to_owned(),
            profile: ToolchainProfile::Go,
            source: ProjectSource::Git {
                remote_url: remote.display().to_string(),
            },
        };
        let system = HostSystem::test(&projects);

        system.create_project(&project).expect("prepare project");
        assert!(
            projects
                .join("private-project/.ssh/deploy_key.pub")
                .is_file()
        );
        assert!(!projects.join("private-project/repository.git").exists());

        initialize_empty_repository(&remote).expect("initialize remote");
        system
            .ensure_repository(&project)
            .expect("continue clone after remote is ready");
        assert!(projects.join("private-project/repository.git").is_dir());
    }
}
