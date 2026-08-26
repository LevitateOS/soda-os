use std::{
    env, fs,
    path::{Path, PathBuf},
    process::Command as Process,
};

use anyhow::{Context, ensure};
use clap::{Parser, Subcommand};
use soda_core::DistroSpec;

const BUILDER_IMAGE: &str = "soda-os-builder:0.1.0";

#[derive(Debug, Parser)]
#[command(name = "soda-image", version, about = "Build Soda OS artifacts")]
struct Cli {
    #[arg(long, default_value = "distro/soda.toml")]
    spec: PathBuf,
    #[command(subcommand)]
    command: Command,
}

#[derive(Debug, Subcommand)]
enum Command {
    Check,
    Verify,
    Rpm,
    Iso {
        #[arg(long)]
        automated: bool,
    },
}

struct Builder {
    root: PathBuf,
    spec: DistroSpec,
}

fn main() -> anyhow::Result<()> {
    let cli = Cli::parse();
    let root = env::current_dir()?.canonicalize()?;
    let spec = DistroSpec::load(root.join(&cli.spec))?;
    let builder = Builder { root, spec };
    builder.check()?;
    match cli.command {
        Command::Check => println!(
            "{} {} spec is valid",
            builder.spec.identity.name, builder.spec.identity.version
        ),
        Command::Verify => builder.verify_source()?,
        Command::Rpm => builder.build_rpms()?,
        Command::Iso { automated } => builder.build_iso(automated)?,
    }
    Ok(())
}

impl Builder {
    fn check(&self) -> anyhow::Result<()> {
        ensure!(
            self.spec.identity.architecture == "aarch64",
            "only AArch64 image builds are supported"
        );
        ensure!(
            self.spec.base.distribution == "rocky" && self.spec.base.version == "10.2",
            "Soda OS 0.1.0 requires Rocky Linux 10.2"
        );
        for path in [
            &self.spec.base.source_iso,
            &self.spec.base.checksum_file,
            &self.spec.base.signature_file,
        ] {
            ensure!(
                self.root.join(path).is_file(),
                "required input {path} is missing"
            );
        }
        Ok(())
    }

    fn verify_source(&self) -> anyhow::Result<()> {
        self.build_container()?;
        let iso = self.container_path(&self.spec.base.source_iso)?;
        let checksum = self.container_path(&self.spec.base.checksum_file)?;
        let signature = self.container_path(&self.spec.base.signature_file)?;
        self.docker([
            "sq",
            "verify",
            "--signer-file",
            "/etc/pki/rpm-gpg/RPM-GPG-KEY-Rocky-10",
            "--signature-file",
            &signature,
            &checksum,
        ])?;

        let signed_checksums = fs::read_to_string(self.root.join(&self.spec.base.checksum_file))?;
        ensure!(
            signed_checksums.contains(&self.spec.base.source_iso_sha256),
            "signed checksum file does not contain the configured ISO digest"
        );
        let output = self.docker_output(["sha256sum", &iso])?;
        let actual = output.split_whitespace().next().unwrap_or_default();
        ensure!(
            actual == self.spec.base.source_iso_sha256,
            "source ISO checksum mismatch: expected {}, got {actual}",
            self.spec.base.source_iso_sha256
        );
        println!(
            "Verified Rocky {} {} source ISO and release signature",
            self.spec.base.version, self.spec.identity.architecture
        );
        Ok(())
    }

    fn build_rpms(&self) -> anyhow::Result<()> {
        self.verify_source()?;
        let artifacts = self.root.join(".artifacts");
        let build = artifacts.join("build");
        let topdir = artifacts.join("rpmbuild");
        let repo = artifacts.join("soda");
        recreate(&build)?;
        recreate(&topdir)?;
        recreate(&repo)?;
        for directory in ["BUILD", "BUILDROOT", "RPMS", "SOURCES", "SPECS", "SRPMS"] {
            fs::create_dir_all(topdir.join(directory))?;
        }

        self.docker_env(
            [("CARGO_TARGET_DIR", "/src/.artifacts/build/target")],
            [
                "cargo",
                "build",
                "--locked",
                "--release",
                "-p",
                "sodad",
                "-p",
                "sodactl",
                "-p",
                "soda-ssh",
            ],
        )?;
        self.docker_env(
            [("CGO_ENABLED", "1")],
            [
                "go",
                "build",
                "-trimpath",
                "-ldflags=-s -w",
                "-o",
                "/src/.artifacts/build/soda-cockpit",
                "./cockpit/cmd/soda-cockpit",
            ],
        )?;
        self.docker_env(
            [("CGO_ENABLED", "1")],
            [
                "go",
                "build",
                "-trimpath",
                "-ldflags=-s -w",
                "-o",
                "/src/.artifacts/build/soda-authd",
                "./cockpit/cmd/soda-authd",
            ],
        )?;

        let sources = topdir.join("SOURCES");
        copy(build.join("target/release/sodad"), sources.join("sodad"))?;
        copy(
            build.join("target/release/sodactl"),
            sources.join("sodactl"),
        )?;
        copy(
            build.join("target/release/soda-ssh"),
            sources.join("soda-ssh"),
        )?;
        copy(build.join("soda-cockpit"), sources.join("soda-cockpit"))?;
        copy(build.join("soda-authd"), sources.join("soda-authd"))?;
        copy(
            self.root.join("packaging/systemd/sodad.service"),
            sources.join("sodad.service"),
        )?;
        copy(
            self.root.join("packaging/systemd/soda-cockpit.service"),
            sources.join("soda-cockpit.service"),
        )?;
        copy(
            self.root.join("packaging/systemd/soda-authd.service"),
            sources.join("soda-authd.service"),
        )?;
        copy(
            self.root.join("packaging/avahi/soda-cockpit.service"),
            sources.join("soda-cockpit.avahi.service"),
        )?;
        copy(
            self.root.join("packaging/pam/soda-cockpit"),
            sources.join("soda-cockpit.pam"),
        )?;

        for spec in ["soda-release", "soda-runtime", "soda-cockpit"] {
            self.docker([
                "rpmbuild",
                "-bb",
                "--define",
                "_topdir /src/.artifacts/rpmbuild",
                &format!("packaging/rpm/{spec}.spec"),
            ])?;
        }
        collect_rpms(&topdir.join("RPMS"), &repo)?;
        self.docker(["createrepo_c", "--update", "/src/.artifacts/soda"])?;
        self.validate_rpms(&repo)?;
        println!("Built Soda RPM repository at {}", repo.display());
        Ok(())
    }

    fn build_iso(&self, automated: bool) -> anyhow::Result<()> {
        self.build_rpms()?;
        let images = self.root.join(".artifacts/images");
        fs::create_dir_all(&images)?;
        let suffix = if automated { "-test" } else { "" };
        let output = images.join(format!(
            "SodaOS-{}-aarch64{suffix}.iso",
            self.spec.identity.version
        ));
        if output.exists() {
            fs::remove_file(&output)?;
        }
        let kickstart = if automated {
            "packaging/kickstart/automated.ks"
        } else {
            "packaging/kickstart/interactive.ks"
        };
        self.docker(["ksvalidator", "-v", "RHEL10", &format!("/src/{kickstart}")])?;
        let input = self.container_path(&self.spec.base.source_iso)?;
        let output_container = self.container_path(&output)?;
        let mut arguments = vec![
            "mkksiso".to_owned(),
            "--add".to_owned(),
            "/src/.artifacts/soda".to_owned(),
            "--ks".to_owned(),
            format!("/src/{kickstart}"),
            "--volid".to_owned(),
            "SodaOS-0.1.0-aarch64".to_owned(),
            "--replace".to_owned(),
            "Rocky Linux 10.2".to_owned(),
            "Soda OS 0.1.0".to_owned(),
        ];
        if automated {
            arguments.extend(["--rm-args".to_owned(), "rd.live.check".to_owned()]);
        }
        arguments.extend([input, output_container]);
        self.docker_privileged_owned(arguments)?;
        let output_container = self.container_path(&output)?;
        let digest = self.docker_output(["sha256sum", &output_container])?;
        let checksum = digest
            .split_whitespace()
            .next()
            .context("sha256sum did not return a digest")?;
        fs::write(output.with_extension("iso.sha256"), format!("{checksum}\n"))?;
        println!("Built {} ({checksum})", output.display());
        Ok(())
    }

    fn validate_rpms(&self, repo: &Path) -> anyhow::Result<()> {
        let mut rpms = fs::read_dir(repo)?
            .filter_map(Result::ok)
            .map(|entry| entry.path())
            .filter(|path| path.extension().is_some_and(|extension| extension == "rpm"))
            .collect::<Vec<_>>();
        rpms.sort();
        ensure!(
            rpms.len() == 3,
            "expected three Soda RPMs, found {}",
            rpms.len()
        );
        let mut command = vec!["dnf".to_owned(), "-y".to_owned(), "install".to_owned()];
        for rpm in rpms {
            command.push(self.container_path(&rpm)?);
        }
        self.docker_owned(command)
    }

    fn build_container(&self) -> anyhow::Result<()> {
        run(Process::new("docker").current_dir(&self.root).args([
            "build",
            "--platform",
            "linux/arm64",
            "--file",
            "packaging/builder/Containerfile",
            "--tag",
            BUILDER_IMAGE,
            ".",
        ]))
    }

    fn docker<const N: usize>(&self, arguments: [&str; N]) -> anyhow::Result<()> {
        self.docker_env([], arguments)
    }

    fn docker_env<const E: usize, const N: usize>(
        &self,
        environment: [(&str, &str); E],
        arguments: [&str; N],
    ) -> anyhow::Result<()> {
        let mut command = self.docker_command();
        for (name, value) in environment {
            command.args(["--env", &format!("{name}={value}")]);
        }
        command.arg(BUILDER_IMAGE).args(arguments);
        run(&mut command)
    }

    fn docker_owned(&self, arguments: Vec<String>) -> anyhow::Result<()> {
        let mut command = self.docker_command();
        command.arg(BUILDER_IMAGE).args(arguments);
        run(&mut command)
    }

    fn docker_privileged_owned(&self, arguments: Vec<String>) -> anyhow::Result<()> {
        let mut command = self.docker_command();
        command
            .arg("--privileged")
            .arg(BUILDER_IMAGE)
            .args(arguments);
        run(&mut command)
    }

    fn docker_output<const N: usize>(&self, arguments: [&str; N]) -> anyhow::Result<String> {
        let mut command = self.docker_command();
        command.arg(BUILDER_IMAGE).args(arguments);
        output(&mut command)
    }

    fn docker_command(&self) -> Process {
        let mut command = Process::new("docker");
        command.args([
            "run",
            "--rm",
            "--platform",
            "linux/arm64",
            "--volume",
            &format!("{}:/src", self.root.display()),
            "--workdir",
            "/src",
        ]);
        command
    }

    fn container_path(&self, path: impl AsRef<Path>) -> anyhow::Result<String> {
        let path = path.as_ref();
        let absolute = if path.is_absolute() {
            path.to_path_buf()
        } else {
            self.root.join(path)
        };
        let relative = absolute
            .strip_prefix(&self.root)
            .with_context(|| format!("{} is outside the workspace", absolute.display()))?;
        Ok(format!("/src/{}", relative.display()))
    }
}

fn recreate(path: &Path) -> anyhow::Result<()> {
    if path.exists() {
        fs::remove_dir_all(path)?;
    }
    fs::create_dir_all(path)?;
    Ok(())
}

fn copy(source: impl AsRef<Path>, destination: impl AsRef<Path>) -> anyhow::Result<()> {
    let source = source.as_ref();
    fs::copy(source, destination.as_ref()).with_context(|| format!("copy {}", source.display()))?;
    Ok(())
}

fn collect_rpms(source: &Path, destination: &Path) -> anyhow::Result<()> {
    for entry in fs::read_dir(source)? {
        let path = entry?.path();
        if path.is_dir() {
            collect_rpms(&path, destination)?;
        } else if path.extension().is_some_and(|extension| extension == "rpm") {
            let filename = path.file_name().context("RPM has no filename")?;
            copy(&path, destination.join(filename))?;
        }
    }
    Ok(())
}

fn run(command: &mut Process) -> anyhow::Result<()> {
    let display = format!("{command:?}");
    println!("+ {display}");
    let status = command.status()?;
    ensure!(status.success(), "{display} exited with {status}");
    Ok(())
}

fn output(command: &mut Process) -> anyhow::Result<String> {
    let display = format!("{command:?}");
    println!("+ {display}");
    let result = command.output()?;
    ensure!(
        result.status.success(),
        "{display} exited with {}: {}",
        result.status,
        String::from_utf8_lossy(&result.stderr).trim()
    );
    String::from_utf8(result.stdout).context("command output is not UTF-8")
}
