use std::{path::Path, sync::Mutex};

use rusqlite::{Connection, OptionalExtension, params};
use soda_core::{
    JobState, Membership, Person, Project, ProjectSource, ProvisioningJob, Role,
    ToolchainInstallation, ToolchainProfile, Worktree,
};
use uuid::Uuid;

use crate::error::{AppError, Result};

const MIGRATION: &str = r"
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS people (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    email TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin', 'developer')),
    ssh_public_key TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    unix_user TEXT NOT NULL UNIQUE,
    profile TEXT NOT NULL CHECK (profile IN ('web', 'python', 'rust', 'go')),
    source_json TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS memberships (
    project_id TEXT NOT NULL REFERENCES projects(id),
    person_id TEXT NOT NULL REFERENCES people(id),
    PRIMARY KEY (project_id, person_id)
);

CREATE TABLE IF NOT EXISTS worktrees (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id),
    person_id TEXT NOT NULL REFERENCES people(id),
    name TEXT NOT NULL,
    branch TEXT NOT NULL,
    path TEXT NOT NULL UNIQUE,
    UNIQUE (project_id, person_id, name)
);

CREATE TABLE IF NOT EXISTS toolchain_installations (
    id TEXT PRIMARY KEY,
    profile TEXT NOT NULL,
    version TEXT NOT NULL,
    path TEXT NOT NULL,
    checksum TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('installing', 'ready', 'failed')),
    UNIQUE (profile, version)
);

CREATE TABLE IF NOT EXISTS project_toolchains (
    project_id TEXT PRIMARY KEY REFERENCES projects(id),
    installation_id TEXT NOT NULL REFERENCES toolchain_installations(id)
);

CREATE TABLE IF NOT EXISTS provisioning_jobs (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id),
    state TEXT NOT NULL CHECK (state IN ('installing', 'ready', 'failed')),
    error TEXT
);
";

pub struct Store {
    connection: Mutex<Connection>,
}

impl Store {
    /// Opens the Soda database and applies the embedded schema migration.
    ///
    /// # Errors
    ///
    /// Returns an error if `SQLite` cannot open or migrate the database.
    pub fn open(path: impl AsRef<Path>) -> Result<Self> {
        let connection = Connection::open(path)?;
        connection.execute_batch(MIGRATION)?;
        Ok(Self {
            connection: Mutex::new(connection),
        })
    }

    pub(crate) fn create_person(&self, person: &Person) -> Result<()> {
        let connection = self.connection.lock().expect("database mutex poisoned");
        connection
            .execute(
                "INSERT INTO people (id, username, display_name, email, role, ssh_public_key)
                 VALUES (?1, ?2, ?3, ?4, ?5, ?6)",
                params![
                    person.id.to_string(),
                    person.username,
                    person.display_name,
                    person.email,
                    role_name(person.role),
                    person.ssh_public_key,
                ],
            )
            .map_err(map_constraint("person", &person.username))?;
        Ok(())
    }

    pub(crate) fn list_people(&self) -> Result<Vec<Person>> {
        let connection = self.connection.lock().expect("database mutex poisoned");
        let mut statement = connection.prepare(
            "SELECT id, username, display_name, email, role, ssh_public_key
             FROM people ORDER BY username",
        )?;
        let rows = statement.query_map([], |row| {
            Ok((
                row.get::<_, String>(0)?,
                row.get::<_, String>(1)?,
                row.get::<_, String>(2)?,
                row.get::<_, String>(3)?,
                row.get::<_, String>(4)?,
                row.get::<_, String>(5)?,
            ))
        })?;
        rows.map(|row| {
            let (id, username, display_name, email, role, ssh_public_key) = row?;
            Ok(Person {
                id: parse_uuid(&id)?,
                username,
                display_name,
                email,
                role: parse_role(&role)?,
                ssh_public_key,
            })
        })
        .collect()
    }

    pub(crate) fn person(&self, id: Uuid) -> Result<Person> {
        self.list_people()?
            .into_iter()
            .find(|person| person.id == id)
            .ok_or_else(|| AppError::NotFound(format!("person {id}")))
    }

    pub(crate) fn create_project(&self, project: &Project) -> Result<()> {
        let source_json = serde_json::to_string(&project.source)?;
        let connection = self.connection.lock().expect("database mutex poisoned");
        connection
            .execute(
                "INSERT INTO projects (id, slug, name, unix_user, profile, source_json)
                 VALUES (?1, ?2, ?3, ?4, ?5, ?6)",
                params![
                    project.id.to_string(),
                    project.slug,
                    project.name,
                    project.unix_user,
                    project.profile.as_str(),
                    source_json,
                ],
            )
            .map_err(map_constraint("project", &project.slug))?;
        Ok(())
    }

    pub(crate) fn list_projects(&self) -> Result<Vec<Project>> {
        let connection = self.connection.lock().expect("database mutex poisoned");
        let mut statement = connection.prepare(
            "SELECT id, slug, name, unix_user, profile, source_json
             FROM projects ORDER BY slug",
        )?;
        let rows = statement.query_map([], |row| {
            Ok((
                row.get::<_, String>(0)?,
                row.get::<_, String>(1)?,
                row.get::<_, String>(2)?,
                row.get::<_, String>(3)?,
                row.get::<_, String>(4)?,
                row.get::<_, String>(5)?,
            ))
        })?;
        rows.map(|row| {
            let (id, slug, name, unix_user, profile, source_json) = row?;
            Ok(Project {
                id: parse_uuid(&id)?,
                slug,
                name,
                unix_user,
                profile: parse_profile(&profile)?,
                source: serde_json::from_str::<ProjectSource>(&source_json)?,
            })
        })
        .collect()
    }

    pub(crate) fn project(&self, id: Uuid) -> Result<Project> {
        self.list_projects()?
            .into_iter()
            .find(|project| project.id == id)
            .ok_or_else(|| AppError::NotFound(format!("project {id}")))
    }

    pub(crate) fn list_projects_for_person(&self, person_id: Uuid) -> Result<Vec<Project>> {
        let projects = self.list_projects()?;
        let connection = self.connection.lock().expect("database mutex poisoned");
        let mut statement = connection.prepare(
            "SELECT project_id FROM memberships WHERE person_id = ?1 ORDER BY project_id",
        )?;
        let rows = statement.query_map([person_id.to_string()], |row| row.get::<_, String>(0))?;
        let project_ids = rows
            .map(|row| row.map_err(AppError::from).and_then(|id| parse_uuid(&id)))
            .collect::<Result<Vec<_>>>()?;
        Ok(projects
            .into_iter()
            .filter(|project| project_ids.contains(&project.id))
            .collect())
    }

    pub(crate) fn add_membership(&self, membership: &Membership) -> Result<()> {
        let connection = self.connection.lock().expect("database mutex poisoned");
        connection
            .execute(
                "INSERT INTO memberships (project_id, person_id) VALUES (?1, ?2)",
                params![
                    membership.project_id.to_string(),
                    membership.person_id.to_string()
                ],
            )
            .map_err(map_constraint("membership", "already exists"))?;
        Ok(())
    }

    pub(crate) fn membership_exists(&self, project_id: Uuid, person_id: Uuid) -> Result<bool> {
        let connection = self.connection.lock().expect("database mutex poisoned");
        let found = connection
            .query_row(
                "SELECT 1 FROM memberships WHERE project_id = ?1 AND person_id = ?2",
                params![project_id.to_string(), person_id.to_string()],
                |_| Ok(()),
            )
            .optional()?
            .is_some();
        Ok(found)
    }

    pub(crate) fn create_worktree(&self, worktree: &Worktree) -> Result<()> {
        let connection = self.connection.lock().expect("database mutex poisoned");
        connection
            .execute(
                "INSERT INTO worktrees (id, project_id, person_id, name, branch, path)
                 VALUES (?1, ?2, ?3, ?4, ?5, ?6)",
                params![
                    worktree.id.to_string(),
                    worktree.project_id.to_string(),
                    worktree.person_id.to_string(),
                    worktree.name,
                    worktree.branch,
                    worktree.path,
                ],
            )
            .map_err(map_constraint("worktree", &worktree.name))?;
        Ok(())
    }

    pub(crate) fn list_worktrees(&self, project_id: Uuid) -> Result<Vec<Worktree>> {
        let connection = self.connection.lock().expect("database mutex poisoned");
        let mut statement = connection.prepare(
            "SELECT id, project_id, person_id, name, branch, path
             FROM worktrees WHERE project_id = ?1 ORDER BY path",
        )?;
        let rows = statement.query_map([project_id.to_string()], |row| {
            Ok((
                row.get::<_, String>(0)?,
                row.get::<_, String>(1)?,
                row.get::<_, String>(2)?,
                row.get::<_, String>(3)?,
                row.get::<_, String>(4)?,
                row.get::<_, String>(5)?,
            ))
        })?;
        rows.map(|row| {
            let (id, project_id, person_id, name, branch, path) = row?;
            Ok(Worktree {
                id: parse_uuid(&id)?,
                project_id: parse_uuid(&project_id)?,
                person_id: parse_uuid(&person_id)?,
                name,
                branch,
                path,
            })
        })
        .collect()
    }

    pub(crate) fn create_job(&self, job: &ProvisioningJob) -> Result<()> {
        let connection = self.connection.lock().expect("database mutex poisoned");
        connection.execute(
            "INSERT INTO provisioning_jobs (id, project_id, state, error) VALUES (?1, ?2, ?3, ?4)",
            params![
                job.id.to_string(),
                job.project_id.to_string(),
                state_name(job.state),
                job.error,
            ],
        )?;
        Ok(())
    }

    pub(crate) fn update_job(&self, job: &ProvisioningJob) -> Result<()> {
        let connection = self.connection.lock().expect("database mutex poisoned");
        connection.execute(
            "UPDATE provisioning_jobs SET state = ?2, error = ?3 WHERE id = ?1",
            params![job.id.to_string(), state_name(job.state), job.error],
        )?;
        Ok(())
    }

    pub(crate) fn list_jobs(&self, project_id: Uuid) -> Result<Vec<ProvisioningJob>> {
        let connection = self.connection.lock().expect("database mutex poisoned");
        let mut statement = connection.prepare(
            "SELECT id, project_id, state, error FROM provisioning_jobs
             WHERE project_id = ?1 ORDER BY rowid DESC",
        )?;
        let rows = statement.query_map([project_id.to_string()], |row| {
            Ok((
                row.get::<_, String>(0)?,
                row.get::<_, String>(1)?,
                row.get::<_, String>(2)?,
                row.get::<_, Option<String>>(3)?,
            ))
        })?;
        rows.map(|row| {
            let (id, project_id, state, error) = row?;
            Ok(ProvisioningJob {
                id: parse_uuid(&id)?,
                project_id: parse_uuid(&project_id)?,
                state: parse_state(&state)?,
                error,
            })
        })
        .collect()
    }

    pub(crate) fn save_installation(
        &self,
        project_id: Uuid,
        installation: &ToolchainInstallation,
    ) -> Result<()> {
        let mut connection = self.connection.lock().expect("database mutex poisoned");
        let transaction = connection.transaction()?;
        transaction.execute(
            "INSERT INTO toolchain_installations (id, profile, version, path, checksum, state)
             VALUES (?1, ?2, ?3, ?4, ?5, ?6)
             ON CONFLICT(profile, version) DO UPDATE SET
               path = excluded.path, checksum = excluded.checksum, state = excluded.state",
            params![
                installation.id.to_string(),
                installation.profile.as_str(),
                installation.version,
                installation.path,
                installation.checksum,
                state_name(installation.state),
            ],
        )?;
        let installation_id: String = transaction.query_row(
            "SELECT id FROM toolchain_installations WHERE profile = ?1 AND version = ?2",
            params![installation.profile.as_str(), installation.version],
            |row| row.get(0),
        )?;
        transaction.execute(
            "INSERT INTO project_toolchains (project_id, installation_id) VALUES (?1, ?2)
             ON CONFLICT(project_id) DO UPDATE SET installation_id = excluded.installation_id",
            params![project_id.to_string(), installation_id],
        )?;
        transaction.commit()?;
        Ok(())
    }

    pub(crate) fn project_installation(&self, project_id: Uuid) -> Result<ToolchainInstallation> {
        let connection = self.connection.lock().expect("database mutex poisoned");
        let row = connection
            .query_row(
                "SELECT i.id, i.profile, i.version, i.path, i.checksum, i.state
                 FROM project_toolchains p
                 JOIN toolchain_installations i ON i.id = p.installation_id
                 WHERE p.project_id = ?1",
                [project_id.to_string()],
                |row| {
                    Ok((
                        row.get::<_, String>(0)?,
                        row.get::<_, String>(1)?,
                        row.get::<_, String>(2)?,
                        row.get::<_, String>(3)?,
                        row.get::<_, String>(4)?,
                        row.get::<_, String>(5)?,
                    ))
                },
            )
            .optional()?
            .ok_or_else(|| AppError::NotFound(format!("project {project_id} toolchain")))?;
        Ok(ToolchainInstallation {
            id: parse_uuid(&row.0)?,
            profile: parse_profile(&row.1)?,
            version: row.2,
            path: row.3,
            checksum: row.4,
            state: parse_state(&row.5)?,
        })
    }
}

fn role_name(role: Role) -> &'static str {
    match role {
        Role::Admin => "admin",
        Role::Developer => "developer",
    }
}

fn parse_role(value: &str) -> Result<Role> {
    match value {
        "admin" => Ok(Role::Admin),
        "developer" => Ok(Role::Developer),
        other => Err(AppError::Invalid(format!("unknown role {other}"))),
    }
}

fn parse_profile(value: &str) -> Result<ToolchainProfile> {
    match value {
        "web" => Ok(ToolchainProfile::Web),
        "python" => Ok(ToolchainProfile::Python),
        "rust" => Ok(ToolchainProfile::Rust),
        "go" => Ok(ToolchainProfile::Go),
        other => Err(AppError::Invalid(format!("unknown profile {other}"))),
    }
}

fn state_name(state: JobState) -> &'static str {
    match state {
        JobState::Installing => "installing",
        JobState::Ready => "ready",
        JobState::Failed => "failed",
    }
}

fn parse_state(value: &str) -> Result<JobState> {
    match value {
        "installing" => Ok(JobState::Installing),
        "ready" => Ok(JobState::Ready),
        "failed" => Ok(JobState::Failed),
        other => Err(AppError::Invalid(format!("unknown job state {other}"))),
    }
}

fn parse_uuid(value: &str) -> Result<Uuid> {
    Uuid::parse_str(value).map_err(|error| AppError::Invalid(error.to_string()))
}

fn map_constraint<'a>(
    kind: &'a str,
    name: &'a str,
) -> impl FnOnce(rusqlite::Error) -> AppError + 'a {
    move |error| match error {
        rusqlite::Error::SqliteFailure(ref failure, _)
            if failure.code == rusqlite::ErrorCode::ConstraintViolation =>
        {
            AppError::Conflict(format!("{kind} {name}"))
        }
        other => AppError::Database(other),
    }
}
