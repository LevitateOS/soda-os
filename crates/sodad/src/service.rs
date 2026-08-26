use std::{fs, path::PathBuf, sync::Arc};

use soda_core::{
    CreatePersonRequest, CreateProjectRequest, DeployKey, ImportPersonRequest, JobState,
    Membership, Person, Project, ProvisioningJob, ToolchainInstallation, Worktree,
};
use uuid::Uuid;

use crate::{
    error::{AppError, Result},
    store::Store,
    system::SystemOps,
    toolchains::ToolchainManager,
};

pub struct Service {
    store: Arc<Store>,
    system: Arc<dyn SystemOps>,
    toolchains: Arc<ToolchainManager>,
    projects_root: PathBuf,
}

impl Service {
    pub fn new(
        store: Arc<Store>,
        system: Arc<dyn SystemOps>,
        toolchains: Arc<ToolchainManager>,
        projects_root: PathBuf,
    ) -> Self {
        Self {
            store,
            system,
            toolchains,
            projects_root,
        }
    }

    pub fn create_person(&self, request: CreatePersonRequest) -> Result<Person> {
        validate_person(&request.username, &request.display_name, &request.email)?;
        if request.password.is_empty() {
            return Err(AppError::Invalid("password is required".to_owned()));
        }
        let person = Person {
            id: Uuid::new_v4(),
            username: request.username,
            display_name: request.display_name,
            email: request.email,
            role: request.role,
            ssh_public_key: request.ssh_public_key,
        };
        self.system.create_person(&person, &request.password)?;
        self.store.create_person(&person)?;
        Ok(person)
    }

    pub fn import_person(&self, request: ImportPersonRequest) -> Result<Person> {
        validate_person(&request.username, &request.display_name, &request.email)?;
        let person = Person {
            id: Uuid::new_v4(),
            username: request.username,
            display_name: request.display_name,
            email: request.email,
            role: request.role,
            ssh_public_key: request.ssh_public_key,
        };
        self.system.import_person(&person)?;
        self.store.create_person(&person)?;
        Ok(person)
    }

    pub fn list_people(&self) -> Result<Vec<Person>> {
        self.store.list_people()
    }

    pub fn create_project(&self, request: CreateProjectRequest) -> Result<Project> {
        validate_slug(&request.slug)?;
        if request.name.trim().is_empty() {
            return Err(AppError::Invalid("project name is required".to_owned()));
        }
        let project = Project {
            id: Uuid::new_v4(),
            unix_user: format!("soda-p-{}", request.slug),
            slug: request.slug,
            name: request.name,
            profile: request.profile,
            source: request.source,
        };
        self.system.create_project(&project)?;
        self.store.create_project(&project)?;
        Ok(project)
    }

    pub fn list_projects(&self) -> Result<Vec<Project>> {
        self.store.list_projects()
    }

    pub fn list_projects_for_person(&self, person_id: Uuid) -> Result<Vec<Project>> {
        self.store.person(person_id)?;
        self.store.list_projects_for_person(person_id)
    }

    pub fn deploy_key(&self, project_id: Uuid) -> Result<DeployKey> {
        let project = self.store.project(project_id)?;
        let public_key = fs::read_to_string(
            self.projects_root
                .join(project.slug)
                .join(".ssh/deploy_key.pub"),
        )?;
        Ok(DeployKey {
            project_id,
            public_key: public_key.trim().to_owned(),
        })
    }

    pub fn project_installation(&self, project_id: Uuid) -> Result<ToolchainInstallation> {
        self.store.project(project_id)?;
        self.store.project_installation(project_id)
    }

    pub fn add_collaborator(&self, project_id: Uuid, person_id: Uuid) -> Result<Worktree> {
        if self.store.membership_exists(project_id, person_id)? {
            return Err(AppError::Conflict("membership already exists".to_owned()));
        }
        let project = self.store.project(project_id)?;
        let person = self.store.person(person_id)?;
        let worktree = self.worktree(&project, &person, "default", "people", None)?;
        self.system
            .create_worktree(&project, &person, &worktree, "main")?;
        self.store.add_membership(&Membership {
            project_id,
            person_id,
        })?;
        self.store.create_worktree(&worktree)?;
        Ok(worktree)
    }

    pub fn create_worktree(
        &self,
        project_id: Uuid,
        person_id: Uuid,
        name: &str,
        base_ref: &str,
    ) -> Result<Worktree> {
        validate_slug(name)?;
        if !self.store.membership_exists(project_id, person_id)? {
            return Err(AppError::NotFound("project membership".to_owned()));
        }
        if base_ref.trim().is_empty() {
            return Err(AppError::Invalid("base ref is required".to_owned()));
        }
        let project = self.store.project(project_id)?;
        let person = self.store.person(person_id)?;
        let worktree = self.worktree(&project, &person, name, "work", Some(name))?;
        self.system
            .create_worktree(&project, &person, &worktree, base_ref)?;
        self.store.create_worktree(&worktree)?;
        Ok(worktree)
    }

    pub fn list_worktrees(&self, project_id: Uuid) -> Result<Vec<Worktree>> {
        self.store.list_worktrees(project_id)
    }

    pub fn start_provisioning(&self, project_id: Uuid) -> Result<ProvisioningJob> {
        self.store.project(project_id)?;
        let job = ProvisioningJob {
            id: Uuid::new_v4(),
            project_id,
            state: JobState::Installing,
            error: None,
        };
        self.store.create_job(&job)?;
        Ok(job)
    }

    pub fn run_provisioning(&self, project_id: Uuid, job_id: Uuid) {
        let result = self.install_project_profile(project_id);
        let job = match result {
            Ok(()) => ProvisioningJob {
                id: job_id,
                project_id,
                state: JobState::Ready,
                error: None,
            },
            Err(error) => ProvisioningJob {
                id: job_id,
                project_id,
                state: JobState::Failed,
                error: Some(error.to_string()),
            },
        };
        if let Err(error) = self.store.update_job(&job) {
            tracing::error!(%error, %job_id, "failed to update provisioning job");
        }
    }

    pub fn list_jobs(&self, project_id: Uuid) -> Result<Vec<ProvisioningJob>> {
        self.store.project(project_id)?;
        self.store.list_jobs(project_id)
    }

    fn install_project_profile(&self, project_id: Uuid) -> Result<()> {
        let project = self.store.project(project_id)?;
        self.system.ensure_repository(&project)?;
        match self.store.project_installation(project_id) {
            Ok(installation) => return self.link_project_environment(&project, &installation),
            Err(AppError::NotFound(_)) => {}
            Err(error) => return Err(error),
        }
        let installed = self.toolchains.install(project.profile)?;
        let installation = ToolchainInstallation {
            id: Uuid::new_v4(),
            profile: project.profile,
            version: installed.version,
            path: installed.path.display().to_string(),
            checksum: installed.checksum,
            state: installed.state,
        };
        self.store.save_installation(project_id, &installation)?;
        self.link_project_environment(&project, &installation)
    }

    fn link_project_environment(
        &self,
        project: &Project,
        installation: &ToolchainInstallation,
    ) -> Result<()> {
        self.system
            .write_project_environment(project, &format!("source {}/env\n", installation.path))
    }

    fn worktree(
        &self,
        project: &Project,
        person: &Person,
        name: &str,
        branch_prefix: &str,
        branch_suffix: Option<&str>,
    ) -> Result<Worktree> {
        let branch = branch_suffix.map_or_else(
            || format!("{branch_prefix}/{}", person.username),
            |suffix| format!("{branch_prefix}/{}/{suffix}", person.username),
        );
        let path = if name == "default" {
            self.projects_root
                .join(&project.slug)
                .join("worktrees")
                .join(&person.username)
        } else {
            self.projects_root
                .join(&project.slug)
                .join("worktrees")
                .join(&person.username)
                .join(name)
        };
        let path = path
            .to_str()
            .ok_or_else(|| AppError::Invalid("worktree path is not UTF-8".to_owned()))?;
        Ok(Worktree {
            id: Uuid::new_v4(),
            project_id: project.id,
            person_id: person.id,
            name: name.to_owned(),
            branch,
            path: path.to_owned(),
        })
    }
}

fn validate_username(value: &str) -> Result<()> {
    if value.is_empty()
        || value.len() > 24
        || !value
            .bytes()
            .all(|byte| byte.is_ascii_lowercase() || byte.is_ascii_digit() || byte == b'-')
        || !value.as_bytes()[0].is_ascii_lowercase()
    {
        return Err(AppError::Invalid(
            "username must start with a lowercase letter and contain at most 24 lowercase letters, digits, or hyphens"
                .to_owned(),
        ));
    }
    Ok(())
}

fn validate_person(username: &str, display_name: &str, email: &str) -> Result<()> {
    validate_username(username)?;
    if display_name.trim().is_empty() {
        return Err(AppError::Invalid("display name is required".to_owned()));
    }
    if !email.contains('@') {
        return Err(AppError::Invalid("email address is invalid".to_owned()));
    }
    Ok(())
}

fn validate_slug(value: &str) -> Result<()> {
    validate_username(value)
}

#[cfg(test)]
mod tests {
    use std::sync::Arc;

    use soda_core::{
        CreatePersonRequest, CreateProjectRequest, ImportPersonRequest, ProjectSource, Role,
        ToolchainProfile,
    };
    use tempfile::TempDir;

    use super::*;
    use crate::{store::Store, system::HostSystem, toolchains::ToolchainManager};

    #[test]
    fn creates_two_collaborator_worktrees_with_git_identity() {
        let temp = TempDir::new().expect("temp dir");
        let projects = temp.path().join("projects");
        let store = Arc::new(Store::open(temp.path().join("soda.db")).expect("open store"));
        let system = Arc::new(HostSystem::test(&projects));
        let toolchains = Arc::new(
            ToolchainManager::new(temp.path().join("toolchains")).expect("toolchain manager"),
        );
        let service = Service::new(store, system, toolchains, projects.clone());

        let alice = service
            .create_person(person("alice", "Alice Example", "alice@example.test"))
            .expect("create Alice");
        let bob = service
            .create_person(person("bob", "Bob Example", "bob@example.test"))
            .expect("create Bob");
        let project = service
            .create_project(CreateProjectRequest {
                slug: "demo".to_owned(),
                name: "Demo".to_owned(),
                profile: ToolchainProfile::Rust,
                source: ProjectSource::Empty,
            })
            .expect("create project");
        let alice_tree = service
            .add_collaborator(project.id, alice.id)
            .expect("add Alice");
        let bob_tree = service
            .add_collaborator(project.id, bob.id)
            .expect("add Bob");

        assert_ne!(alice_tree.path, bob_tree.path);
        assert_eq!(git_config(&alice_tree.path, "core.bare"), "false");
        assert_eq!(git_config(&bob_tree.path, "core.bare"), "false");
        assert_eq!(git_config(&alice_tree.path, "user.name"), "Alice Example");
        assert_eq!(git_config(&bob_tree.path, "user.email"), "bob@example.test");
        assert_eq!(service.list_worktrees(project.id).expect("list").len(), 2);
    }

    #[test]
    fn imports_existing_administrator_without_replacing_credentials() {
        let temp = TempDir::new().expect("temp dir");
        let projects = temp.path().join("projects");
        let store = Arc::new(Store::open(temp.path().join("soda.db")).expect("open store"));
        let system = Arc::new(HostSystem::test(&projects));
        let toolchains = Arc::new(
            ToolchainManager::new(temp.path().join("toolchains")).expect("toolchain manager"),
        );
        let service = Service::new(store, system, toolchains, projects);

        let admin = service
            .import_person(ImportPersonRequest {
                username: "installer-admin".to_owned(),
                display_name: "Installer Admin".to_owned(),
                email: "admin@example.test".to_owned(),
                role: Role::Admin,
                ssh_public_key: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest admin".to_owned(),
            })
            .expect("import administrator");
        assert_eq!(admin.role, Role::Admin);
        assert_eq!(service.list_people().expect("list people"), vec![admin]);
    }

    fn person(username: &str, display_name: &str, email: &str) -> CreatePersonRequest {
        CreatePersonRequest {
            username: username.to_owned(),
            display_name: display_name.to_owned(),
            email: email.to_owned(),
            role: Role::Developer,
            ssh_public_key: format!("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest {username}"),
            password: "test-only".to_owned(),
        }
    }

    fn git_config(path: &str, key: &str) -> String {
        let output = std::process::Command::new("git")
            .args(["-C", path, "config", "--worktree", "--get", key])
            .output()
            .expect("run git config");
        assert!(output.status.success());
        String::from_utf8(output.stdout)
            .expect("UTF-8 output")
            .trim()
            .to_owned()
    }
}
