use std::{
    fs,
    io::Write,
    os::unix::fs::PermissionsExt,
    path::{Path, PathBuf},
    process::Command,
};

use fs2::FileExt;
use reqwest::blocking::Client;
use serde::Deserialize;
use sha2::{Digest, Sha256};
use soda_core::{JobState, ToolchainProfile};

use crate::error::{AppError, Result};

#[derive(Debug, Clone, Copy)]
enum ArchiveKind {
    TarGz,
    TarXz,
    Zip,
    Executable,
}

#[derive(Debug, Clone)]
struct Artifact {
    tool: &'static str,
    version: String,
    url: String,
    sha256: String,
    archive: ArchiveKind,
}

#[derive(Debug, Clone)]
pub struct InstalledProfile {
    pub version: String,
    pub path: PathBuf,
    pub checksum: String,
    pub state: JobState,
}

pub struct ToolchainManager {
    client: Client,
    root: PathBuf,
}

impl ToolchainManager {
    pub fn new(root: PathBuf) -> Result<Self> {
        let client = Client::builder()
            .user_agent("SodaOS/0.1 toolchain resolver")
            .build()
            .map_err(http_error)?;
        Ok(Self { client, root })
    }

    pub fn install(&self, profile: ToolchainProfile) -> Result<InstalledProfile> {
        fs::create_dir_all(&self.root)?;
        let lock = fs::File::create(self.root.join(format!(".{}.lock", profile.as_str())))?;
        lock.lock_exclusive()?;
        let artifacts = self.resolve(profile)?;
        let mut version = artifacts
            .iter()
            .map(|artifact| format!("{}={}", artifact.tool, artifact.version))
            .collect::<Vec<_>>()
            .join(",");
        let mut paths = Vec::new();
        for artifact in &artifacts {
            paths.extend(self.install_artifact(artifact)?);
        }
        if profile == ToolchainProfile::Python {
            let (python_paths, python_version) = self.install_python(&artifacts)?;
            paths.extend(python_paths);
            version.push_str(",python=");
            version.push_str(&python_version);
        }
        let digest = Sha256::digest(version.as_bytes());
        let profile_root = self
            .root
            .join("profiles")
            .join(profile.as_str())
            .join(hex::encode(digest));
        fs::create_dir_all(&profile_root)?;
        let mut env = fs::File::create(profile_root.join("env"))?;
        writeln!(env, "export SODA_PROFILE={}", profile.as_str())?;
        if profile == ToolchainProfile::Rust {
            let rust = artifacts
                .iter()
                .find(|artifact| artifact.tool == "rust")
                .ok_or_else(|| AppError::System("Rust profile is missing rustup".to_owned()))?;
            let rust_root = self.root.join("rust").join(&rust.version);
            writeln!(
                env,
                "export RUSTUP_HOME={}",
                rust_root.join("rustup").display()
            )?;
            writeln!(
                env,
                "export CARGO_HOME={}",
                rust_root.join("cargo").display()
            )?;
        }
        writeln!(env, "export PATH={}:$PATH", paths.join(":"))?;

        Ok(InstalledProfile {
            version,
            checksum: aggregate_checksum(&artifacts),
            path: profile_root,
            state: JobState::Ready,
        })
    }

    fn resolve(&self, profile: ToolchainProfile) -> Result<Vec<Artifact>> {
        match profile {
            ToolchainProfile::Web => Ok(vec![
                self.resolve_node()?,
                self.resolve_github(
                    "oven-sh/bun",
                    "bun-linux-aarch64.zip",
                    "bun",
                    ArchiveKind::Zip,
                )?,
            ]),
            ToolchainProfile::Python => Ok(vec![self.resolve_github(
                "astral-sh/uv",
                "uv-aarch64-unknown-linux-gnu.tar.gz",
                "uv",
                ArchiveKind::TarGz,
            )?]),
            ToolchainProfile::Rust => Ok(vec![self.resolve_rustup()?]),
            ToolchainProfile::Go => Ok(vec![self.resolve_go()?]),
        }
    }

    fn resolve_node(&self) -> Result<Artifact> {
        #[derive(Deserialize)]
        struct Release {
            version: String,
            lts: serde_json::Value,
            files: Vec<String>,
        }

        let releases: Vec<Release> = self.get_json("https://nodejs.org/dist/index.json")?;
        let release = releases
            .into_iter()
            .find(|release| {
                release.lts.is_string() && release.files.iter().any(|f| f == "linux-arm64")
            })
            .ok_or_else(|| {
                AppError::System("Node active LTS AArch64 release not found".to_owned())
            })?;
        let filename = format!("node-{}-linux-arm64.tar.xz", release.version);
        let base = format!("https://nodejs.org/dist/{}/", release.version);
        let sums = self.get_text(&format!("{base}SHASUMS256.txt"))?;
        let sha256 = checksum_line(&sums, &filename)?;
        Ok(Artifact {
            tool: "node",
            version: release.version,
            url: format!("{base}{filename}"),
            sha256,
            archive: ArchiveKind::TarXz,
        })
    }

    fn resolve_go(&self) -> Result<Artifact> {
        #[derive(Deserialize)]
        struct File {
            filename: String,
            os: String,
            arch: String,
            sha256: String,
            kind: String,
        }
        #[derive(Deserialize)]
        struct Release {
            version: String,
            stable: bool,
            files: Vec<File>,
        }

        let releases: Vec<Release> = self.get_json("https://go.dev/dl/?mode=json")?;
        let release = releases
            .into_iter()
            .find(|release| release.stable)
            .ok_or_else(|| AppError::System("stable Go release not found".to_owned()))?;
        let file = release
            .files
            .into_iter()
            .find(|file| file.os == "linux" && file.arch == "arm64" && file.kind == "archive")
            .ok_or_else(|| AppError::System("Go Linux AArch64 archive not found".to_owned()))?;
        Ok(Artifact {
            tool: "go",
            version: release.version,
            url: format!("https://go.dev/dl/{}", file.filename),
            sha256: file.sha256,
            archive: ArchiveKind::TarGz,
        })
    }

    fn resolve_rustup(&self) -> Result<Artifact> {
        #[derive(Deserialize)]
        struct Channel {
            pkg: Packages,
        }
        #[derive(Deserialize)]
        struct Packages {
            rust: RustPackage,
        }
        #[derive(Deserialize)]
        struct RustPackage {
            version: String,
        }

        let channel: Channel = toml::from_str(
            &self.get_text("https://static.rust-lang.org/dist/channel-rust-stable.toml")?,
        )
        .map_err(|error| AppError::System(format!("invalid Rust channel manifest: {error}")))?;
        let url = "https://static.rust-lang.org/rustup/dist/aarch64-unknown-linux-gnu/rustup-init";
        let sha256 = self.get_text(&format!("{url}.sha256"))?;
        Ok(Artifact {
            tool: "rust",
            version: channel.pkg.rust.version,
            url: url.to_owned(),
            sha256: sha256
                .split_whitespace()
                .next()
                .unwrap_or_default()
                .to_owned(),
            archive: ArchiveKind::Executable,
        })
    }

    fn resolve_github(
        &self,
        repository: &str,
        asset_name: &str,
        tool: &'static str,
        archive: ArchiveKind,
    ) -> Result<Artifact> {
        #[derive(Deserialize)]
        struct Release {
            tag_name: String,
            assets: Vec<Asset>,
        }
        #[derive(Deserialize)]
        struct Asset {
            name: String,
            browser_download_url: String,
            digest: Option<String>,
        }

        let release: Release = self.get_json(&format!(
            "https://api.github.com/repos/{repository}/releases/latest"
        ))?;
        let asset = release
            .assets
            .into_iter()
            .find(|asset| asset.name == asset_name)
            .ok_or_else(|| {
                AppError::System(format!("{repository} asset {asset_name} not found"))
            })?;
        let digest = asset.digest.ok_or_else(|| {
            AppError::System(format!(
                "{repository} asset {asset_name} has no publisher digest"
            ))
        })?;
        let sha256 = digest
            .strip_prefix("sha256:")
            .ok_or_else(|| AppError::System(format!("unsupported asset digest {digest}")))?;
        Ok(Artifact {
            tool,
            version: release.tag_name,
            url: asset.browser_download_url,
            sha256: sha256.to_owned(),
            archive,
        })
    }

    fn install_artifact(&self, artifact: &Artifact) -> Result<Vec<String>> {
        let destination = self.root.join(artifact.tool).join(&artifact.version);
        if destination.join(".ready").is_file() {
            return Ok(vec![
                artifact_bin_path(artifact, &destination)
                    .display()
                    .to_string(),
            ]);
        }
        if destination.exists() {
            fs::remove_dir_all(&destination)?;
        }
        fs::create_dir_all(&destination)?;
        let temporary = tempfile::Builder::new()
            .prefix(".soda-download-")
            .tempdir_in(&self.root)?;
        let archive = temporary.path().join("payload");
        let payload = self.get_bytes(&artifact.url)?;
        verify_checksum(&payload, &artifact.sha256)?;
        fs::write(&archive, payload)?;

        match artifact.archive {
            ArchiveKind::TarGz => {
                run(Command::new("tar").args([
                    "-xzf",
                    &archive.display().to_string(),
                    "-C",
                    &destination.display().to_string(),
                    "--strip-components=1",
                ]))?;
                if artifact.tool == "uv" {
                    normalize_single_binary(&destination, artifact.tool)?;
                }
            }
            ArchiveKind::TarXz => run(Command::new("tar").args([
                "-xJf",
                &archive.display().to_string(),
                "-C",
                &destination.display().to_string(),
                "--strip-components=1",
            ]))?,
            ArchiveKind::Zip => {
                run(Command::new("unzip").args([
                    "-q",
                    &archive.display().to_string(),
                    "-d",
                    &destination.display().to_string(),
                ]))?;
                normalize_single_binary(&destination, artifact.tool)?;
            }
            ArchiveKind::Executable => {
                let executable = temporary.path().join("rustup-init");
                fs::copy(&archive, &executable)?;
                fs::set_permissions(&executable, fs::Permissions::from_mode(0o755))?;
                let rustup_home = destination.join("rustup");
                let cargo_home = destination.join("cargo");
                run(Command::new(&executable)
                    .args([
                        "-y",
                        "--no-modify-path",
                        "--profile",
                        "default",
                        "--default-toolchain",
                        "stable",
                    ])
                    .env("RUSTUP_HOME", &rustup_home)
                    .env("CARGO_HOME", &cargo_home))?;
                fs::write(destination.join(".ready"), b"ready\n")?;
                return Ok(vec![cargo_home.join("bin").display().to_string()]);
            }
        }
        fs::write(destination.join(".ready"), b"ready\n")?;
        Ok(vec![destination.join("bin").display().to_string()])
    }

    fn install_python(&self, artifacts: &[Artifact]) -> Result<(Vec<String>, String)> {
        let uv = artifacts
            .iter()
            .find(|artifact| artifact.tool == "uv")
            .ok_or_else(|| AppError::System("Python profile is missing uv".to_owned()))?;
        let uv_binary = self.root.join("uv").join(&uv.version).join("bin/uv");
        let python_root = self.root.join("python");
        run(Command::new(&uv_binary)
            .args(["python", "install"])
            .env("UV_PYTHON_INSTALL_DIR", &python_root))?;
        let bin = first_python_bin(&python_root)?;
        let version = command_output(Command::new(bin.join("python3")).arg("--version"))?;
        Ok((vec![bin.display().to_string()], version.trim().to_owned()))
    }

    fn get_text(&self, url: &str) -> Result<String> {
        self.client
            .get(url)
            .send()
            .and_then(reqwest::blocking::Response::error_for_status)
            .and_then(reqwest::blocking::Response::text)
            .map_err(http_error)
    }

    fn get_bytes(&self, url: &str) -> Result<Vec<u8>> {
        self.client
            .get(url)
            .send()
            .and_then(reqwest::blocking::Response::error_for_status)
            .and_then(reqwest::blocking::Response::bytes)
            .map(|bytes| bytes.to_vec())
            .map_err(http_error)
    }

    fn get_json<T: serde::de::DeserializeOwned>(&self, url: &str) -> Result<T> {
        self.client
            .get(url)
            .send()
            .and_then(reqwest::blocking::Response::error_for_status)
            .and_then(reqwest::blocking::Response::json)
            .map_err(http_error)
    }
}

fn aggregate_checksum(artifacts: &[Artifact]) -> String {
    let mut hasher = Sha256::new();
    for artifact in artifacts {
        hasher.update(artifact.sha256.as_bytes());
    }
    hex::encode(hasher.finalize())
}

fn artifact_bin_path(artifact: &Artifact, destination: &Path) -> PathBuf {
    match artifact.archive {
        ArchiveKind::Executable => destination.join("cargo/bin"),
        ArchiveKind::TarGz | ArchiveKind::TarXz | ArchiveKind::Zip => destination.join("bin"),
    }
}

fn checksum_line(contents: &str, filename: &str) -> Result<String> {
    contents
        .lines()
        .find_map(|line| {
            let mut fields = line.split_whitespace();
            let checksum = fields.next()?;
            let name = fields.next()?.trim_start_matches('*');
            (name == filename).then(|| checksum.to_owned())
        })
        .ok_or_else(|| AppError::System(format!("checksum for {filename} not found")))
}

fn verify_checksum(payload: &[u8], expected: &str) -> Result<()> {
    let actual = hex::encode(Sha256::digest(payload));
    if actual == expected.to_ascii_lowercase() {
        Ok(())
    } else {
        Err(AppError::System(format!(
            "toolchain checksum mismatch: expected {expected}, got {actual}"
        )))
    }
}

fn normalize_single_binary(destination: &Path, tool: &str) -> Result<()> {
    let binary = find_file(destination, tool)?;
    let bin_dir = destination.join("bin");
    fs::create_dir_all(&bin_dir)?;
    let target = bin_dir.join(tool);
    if binary != target {
        fs::copy(binary, &target)?;
    }
    fs::set_permissions(target, fs::Permissions::from_mode(0o755))?;
    Ok(())
}

fn find_file(root: &Path, name: &str) -> Result<PathBuf> {
    for entry in fs::read_dir(root)? {
        let path = entry?.path();
        if path.is_dir() {
            if let Ok(found) = find_file(&path, name) {
                return Ok(found);
            }
        } else if path.file_name().is_some_and(|candidate| candidate == name) {
            return Ok(path);
        }
    }
    Err(AppError::System(format!(
        "downloaded archive does not contain {name}"
    )))
}

fn first_python_bin(root: &Path) -> Result<PathBuf> {
    for entry in fs::read_dir(root)? {
        let bin = entry?.path().join("bin");
        if bin.join("python").exists() || bin.join("python3").exists() {
            return Ok(bin);
        }
    }
    Err(AppError::System(
        "uv did not install a Python interpreter".to_owned(),
    ))
}

fn run(command: &mut Command) -> Result<()> {
    let display = format!("{command:?}");
    let output = command.output()?;
    if output.status.success() {
        Ok(())
    } else {
        Err(AppError::System(format!(
            "{display} exited with {}: {}",
            output.status,
            String::from_utf8_lossy(&output.stderr).trim()
        )))
    }
}

fn command_output(command: &mut Command) -> Result<String> {
    let display = format!("{command:?}");
    let output = command.output()?;
    if !output.status.success() {
        return Err(AppError::System(format!(
            "{display} exited with {}: {}",
            output.status,
            String::from_utf8_lossy(&output.stderr).trim()
        )));
    }
    let bytes = if output.stdout.is_empty() {
        output.stderr
    } else {
        output.stdout
    };
    String::from_utf8(bytes)
        .map_err(|error| AppError::System(format!("{display} returned invalid UTF-8: {error}")))
}

#[allow(clippy::needless_pass_by_value)]
fn http_error(error: reqwest::Error) -> AppError {
    AppError::System(format!("toolchain metadata request failed: {error}"))
}

#[cfg(test)]
mod tests {
    use tempfile::TempDir;

    use super::*;

    #[test]
    fn finds_named_checksum() {
        let sums = "abc  other.tar.gz\ndef *node-v1-linux-arm64.tar.xz\n";
        assert_eq!(
            checksum_line(sums, "node-v1-linux-arm64.tar.xz").expect("checksum"),
            "def"
        );
    }

    #[test]
    fn rejects_checksum_mismatch() {
        assert!(verify_checksum(b"payload", "incorrect").is_err());
    }

    #[test]
    #[ignore = "queries live publisher metadata"]
    fn resolves_all_live_profiles() {
        let temp = TempDir::new().expect("temp dir");
        let manager = ToolchainManager::new(temp.path().to_path_buf()).expect("manager");
        for profile in [
            ToolchainProfile::Web,
            ToolchainProfile::Python,
            ToolchainProfile::Rust,
            ToolchainProfile::Go,
        ] {
            let artifacts = manager.resolve(profile).expect("resolve profile");
            assert!(!artifacts.is_empty());
            assert!(artifacts.iter().all(|artifact| artifact.sha256.len() == 64));
        }
    }

    #[test]
    #[cfg(target_os = "linux")]
    #[ignore = "downloads and executes all live AArch64 toolchains"]
    fn installs_and_executes_all_live_profiles() {
        let temp = TempDir::new().expect("temp dir");
        let manager = ToolchainManager::new(temp.path().to_path_buf()).expect("manager");
        for (profile, commands) in [
            (ToolchainProfile::Web, vec!["node", "bun"]),
            (ToolchainProfile::Python, vec!["python3", "uv"]),
            (ToolchainProfile::Rust, vec!["rustc", "cargo"]),
            (ToolchainProfile::Go, vec!["go"]),
        ] {
            let installed = manager.install(profile).expect("install profile");
            let env = fs::read_to_string(installed.path.join("env")).expect("profile env");
            let path = env
                .lines()
                .find_map(|line| line.strip_prefix("export PATH="))
                .and_then(|value| value.strip_suffix(":$PATH"))
                .expect("profile PATH");
            for command in commands {
                let mut process = Command::new(command);
                process
                    .arg(if command == "go" {
                        "version"
                    } else {
                        "--version"
                    })
                    .env("PATH", path);
                for variable in ["RUSTUP_HOME", "CARGO_HOME"] {
                    if let Some(value) = env
                        .lines()
                        .find_map(|line| line.strip_prefix(&format!("export {variable}=")))
                    {
                        process.env(variable, value);
                    }
                }
                let status = process.status().expect("run tool");
                assert!(status.success(), "{command} did not execute");
            }
        }
    }
}
