use std::{
    env, fs,
    os::unix::process::CommandExt,
    path::{Path, PathBuf},
    process::Command,
};

use anyhow::Context;
use clap::Parser;

#[derive(Debug, Parser)]
#[command(name = "soda-ssh", version, about = "Enter a Soda project worktree")]
struct Cli {
    #[arg(long)]
    actor: String,
    #[arg(long)]
    worktree: String,
}

fn main() -> anyhow::Result<()> {
    let cli = Cli::parse();
    validate_actor(&cli.actor)?;
    let worktree = PathBuf::from(&cli.worktree)
        .canonicalize()
        .with_context(|| format!("worktree {} is unavailable", cli.worktree))?;
    validate_worktree(&worktree)?;

    let mut command = session_command(&worktree);
    command
        .current_dir(&worktree)
        .env("SODA_ACTOR", &cli.actor)
        .env("SODA_WORKTREE", &worktree);
    if let Some(environment) = project_environment(&worktree)? {
        for (name, value) in environment {
            if name == "PATH" {
                let current = env::var_os("PATH").unwrap_or_default();
                let mut joined = std::ffi::OsString::from(value);
                joined.push(":");
                joined.push(current);
                command.env(name, joined);
            } else {
                command.env(name, value);
            }
        }
    }

    let error = command.exec();
    Err(error).context("failed to enter Soda project session")
}

fn session_command(worktree: &Path) -> Command {
    let original = env::var("SSH_ORIGINAL_COMMAND").unwrap_or_default();
    if original == "internal-sftp" {
        Command::new("/usr/libexec/openssh/sftp-server")
    } else if original.is_empty() {
        let shell = env::var("SHELL").unwrap_or_else(|_| "/bin/bash".to_owned());
        let mut command = Command::new(shell);
        command.arg("-l");
        command
    } else {
        let mut command = Command::new("/bin/bash");
        command.args(["-lc", &original]).env("PWD", worktree);
        command
    }
}

fn validate_actor(actor: &str) -> anyhow::Result<()> {
    if actor.is_empty()
        || !actor
            .bytes()
            .all(|byte| byte.is_ascii_lowercase() || byte.is_ascii_digit() || byte == b'-')
    {
        anyhow::bail!("invalid Soda actor")
    }
    Ok(())
}

fn validate_worktree(path: &Path) -> anyhow::Result<()> {
    let projects_root = env::var_os("SODA_PROJECTS_ROOT")
        .map_or_else(|| PathBuf::from("/srv/soda/projects"), PathBuf::from);
    let projects_root = projects_root
        .canonicalize()
        .with_context(|| format!("projects root {} is unavailable", projects_root.display()))?;
    if !path.starts_with(&projects_root) || !path.join(".git").exists() {
        anyhow::bail!("worktree is outside the Soda projects root or is not a Git worktree")
    }
    Ok(())
}

fn project_environment(worktree: &Path) -> anyhow::Result<Option<Vec<(String, String)>>> {
    let Some(project_root) = worktree
        .ancestors()
        .find(|ancestor| ancestor.join(".soda/env").is_file())
    else {
        return Ok(None);
    };
    let link = fs::read_to_string(project_root.join(".soda/env"))?;
    let profile_env = link
        .lines()
        .find_map(|line| line.strip_prefix("source "))
        .map(str::trim)
        .ok_or_else(|| anyhow::anyhow!("project environment does not name a profile"))?;
    let contents = fs::read_to_string(profile_env)?;
    let mut environment = Vec::new();
    for line in contents.lines() {
        let Some(assignment) = line.strip_prefix("export ") else {
            continue;
        };
        let Some((name, value)) = assignment.split_once('=') else {
            continue;
        };
        if matches!(name, "SODA_PROFILE" | "RUSTUP_HOME" | "CARGO_HOME") {
            environment.push((name.to_owned(), value.to_owned()));
        } else if name == "PATH" {
            let value = value
                .strip_suffix(":$PATH")
                .ok_or_else(|| anyhow::anyhow!("profile PATH has an unsupported form"))?;
            environment.push((name.to_owned(), value.to_owned()));
        }
    }
    if !environment.iter().any(|(name, _)| name == "SODA_PROFILE")
        || !environment.iter().any(|(name, _)| name == "PATH")
    {
        anyhow::bail!("profile environment is incomplete")
    }
    Ok(Some(environment))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn validates_actor_names() {
        assert!(validate_actor("alice-2").is_ok());
        assert!(validate_actor("Alice").is_err());
        assert!(validate_actor("alice\nbob").is_err());
    }
}
