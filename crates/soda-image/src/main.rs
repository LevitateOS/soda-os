use std::{
    env, fs,
    path::{Path, PathBuf},
    process::Command as Process,
};

use anyhow::{Context, bail, ensure};
use clap::{Parser, Subcommand};
use serde::Deserialize;
use sha2::{Digest, Sha256};
use soda_core::DistroSpec;

const BUILDER_IMAGE: &str = "soda-os-builder:0.1.0";
const TARGET_RPMS: [&str; 3] = ["soda-release", "soda-runtime", "soda-cockpit"];
const BASEOS_MIRRORLIST: &str =
    "https://mirrors.rockylinux.org/mirrorlist?arch=aarch64&repo=BaseOS-10";
const APPSTREAM_MIRRORLIST: &str =
    "https://mirrors.rockylinux.org/mirrorlist?arch=aarch64&repo=AppStream-10";
const PAYLOAD_PACKAGES: [&str; 7] = [
    "avahi",
    "git",
    "openssh-server",
    "soda-release",
    "soda-runtime",
    "soda-cockpit",
    "sudo",
];
const AUTOMATED_EXTRA_PACKAGES: [&str; 1] = ["curl"];
const ANACONDA_REQUIRED_PACKAGES: [&str; 6] = [
    "kernel",
    "grub2",
    "grub2-tools",
    "grub2-efi-aa64",
    "grub2-efi-aa64-cdboot",
    "shim-aa64",
];
const REQUIRED_FIRMWARE_PACKAGES: [&str; 4] = [
    "linux-firmware",
    "amd-gpu-firmware",
    "intel-gpu-firmware",
    "nvidia-gpu-firmware",
];
const ISO_ROOT_ALLOWLIST: [&str; 11] = [
    ".discinfo",
    ".treeinfo",
    "COMMUNITY-CHARTER",
    "EFI",
    "EULA",
    "LICENSE",
    "RPM-GPG-KEY-Rocky-10",
    "boot.catalog",
    "images",
    "ks.cfg",
    "soda",
];

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

#[derive(Debug, Deserialize)]
struct BrandingManifest {
    schema_version: u32,
    asset: Vec<BrandAsset>,
}

#[derive(Debug, Deserialize)]
struct BrandAsset {
    source: String,
    output: String,
    width: u32,
    height: u32,
    sha256: String,
}

#[derive(Debug, Deserialize)]
struct UpstreamManifest {
    schema_version: u32,
    anaconda_gui_nevra: String,
    anaconda_gui_rpm: String,
    spokes: SpokeContract,
    glade: Vec<GladeContract>,
}

#[derive(Debug, Deserialize)]
struct SpokeContract {
    visible: Vec<String>,
    hidden: Vec<String>,
}

#[derive(Debug, Deserialize)]
struct GladeContract {
    path: String,
    sha256: String,
    #[serde(rename = "override")]
    overrides: Vec<GladeOverride>,
}

#[derive(Debug, Deserialize)]
struct GladeOverride {
    object_id: String,
    property: String,
    value: String,
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
            "{} {} installer contract is valid",
            builder.spec.identity.name, builder.spec.identity.version
        ),
        Command::Verify => builder.verify_source()?,
        Command::Rpm => builder.build_rpms()?,
        Command::Iso { automated } => builder.build_iso(automated)?,
    }
    Ok(())
}

impl Builder {
    #[allow(clippy::too_many_lines)]
    fn check(&self) -> anyhow::Result<()> {
        ensure!(
            self.spec.identity.architecture == "aarch64",
            "only AArch64 image builds are supported"
        );
        ensure!(
            self.spec.base.distribution == "rocky"
                && self.spec.base.installer_source_version == "10.2"
                && self.spec.base.package_stream == "10",
            "Soda OS requires the Rocky 10.2 installer runtime and Rocky 10 package stream"
        );
        ensure!(
            self.spec.installer.profile_id == "sodaos",
            "installer profile must be sodaos"
        );
        ensure!(
            self.spec.installer.volume_id == "SodaOS-0-1-0-aarch64",
            "unexpected installer volume ID"
        );
        ensure!(
            self.spec.installer.boot_timeout_seconds == 10,
            "installer boot timeout must be 10 seconds"
        );
        let payload = &self.spec.installer.payload;
        ensure!(
            payload.mode == "network",
            "installer payload must be network based"
        );
        ensure!(
            payload.baseos_mirrorlist == BASEOS_MIRRORLIST
                && payload.appstream_mirrorlist == APPSTREAM_MIRRORLIST,
            "Rocky network sources differ from the approved mirrorlists"
        );
        ensure!(
            !payload.install_weak_dependencies,
            "weak RPM dependencies must remain disabled"
        );
        ensure!(
            payload.max_iso_size_bytes == 1_342_177_280,
            "compact ISO size limit must be 1.25 GiB"
        );
        ensure!(
            payload.environment == "minimal-environment"
                && string_slice_eq(&payload.packages, &PAYLOAD_PACKAGES)
                && string_slice_eq(&payload.automated_extra_packages, &AUTOMATED_EXTRA_PACKAGES)
                && string_slice_eq(
                    &payload.anaconda_required_packages,
                    &ANACONDA_REQUIRED_PACKAGES
                ),
            "network payload roots differ from the approved package contract"
        );
        for path in [
            &self.spec.base.source_iso,
            &self.spec.base.checksum_file,
            &self.spec.base.signature_file,
            &self.spec.installer.branding_manifest,
            &self.spec.installer.upstream_manifest,
        ] {
            ensure!(
                self.root.join(path).is_file(),
                "required input {path} is missing"
            );
        }

        let branding = self.branding_manifest()?;
        ensure!(
            branding.schema_version == 1,
            "unsupported branding manifest"
        );
        for asset in &branding.asset {
            ensure!(
                self.root.join(&asset.source).is_file(),
                "branding source {} is missing",
                asset.source
            );
            let output = self.root.join(&asset.output);
            ensure!(
                output.is_file(),
                "branding output {} is missing",
                asset.output
            );
            let (width, height) = png_dimensions(&output)?;
            ensure!(
                (width, height) == (asset.width, asset.height),
                "{} is {width}x{height}; expected {}x{}",
                asset.output,
                asset.width,
                asset.height
            );
            ensure!(
                sha256_file(&output)? == asset.sha256,
                "{} does not match its recorded SHA-256",
                asset.output
            );
        }

        let upstream = self.upstream_manifest()?;
        ensure!(
            upstream.schema_version == 1,
            "unsupported upstream manifest"
        );
        ensure!(
            upstream.anaconda_gui_nevra == self.spec.installer.anaconda_gui_nevra,
            "Anaconda package pin differs between distro and upstream manifests"
        );
        let expected_visible = [
            "WelcomeLanguageSpoke",
            "KeyboardSpoke",
            "DatetimeSpoke",
            "StorageSpoke",
            "NetworkSpoke",
            "UserSpoke",
        ];
        let expected_hidden = [
            "LangsupportSpoke",
            "SourceSpoke",
            "SoftwareSelectionSpoke",
            "KdumpSpoke",
            "PasswordSpoke",
            "CustomPartitioningSpoke",
            "BlivetGuiSpoke",
            "FilterSpoke",
        ];
        ensure!(
            upstream.spokes.visible == expected_visible,
            "visible spoke contract differs from the approved allowlist"
        );
        ensure!(
            upstream.spokes.hidden == expected_hidden,
            "hidden spoke contract differs from the approved list"
        );
        let profile = fs::read_to_string(
            self.root
                .join("packaging/anaconda/product/etc/anaconda/profile.d/sodaos.conf"),
        )?;
        ensure!(
            profile.contains("profile_id = sodaos")
                && profile.contains("base_profile = rocky")
                && profile.contains("efi_dir = rocky")
                && profile.contains("custom_stylesheet = /usr/share/anaconda/pixmaps/soda.css")
                && profile.contains("user (quality 1, length 6)")
                && !profile.contains("strict"),
            "Soda Anaconda profile is incomplete"
        );
        for spoke in &upstream.spokes.hidden {
            ensure!(profile.contains(spoke), "profile does not hide {spoke}");
        }
        ensure!(upstream.glade.len() == 4, "expected four Glade overlays");
        for glade in &upstream.glade {
            ensure!(
                glade.path.starts_with("usr/share/anaconda/ui/spokes/")
                    && Path::new(&glade.path)
                        .extension()
                        .is_some_and(|extension| extension.eq_ignore_ascii_case("glade")),
                "invalid Glade runtime path {}",
                glade.path
            );
            ensure!(
                glade.sha256.len() == 64
                    && glade.sha256.bytes().all(|byte| byte.is_ascii_hexdigit()),
                "invalid Glade digest for {}",
                glade.path
            );
            ensure!(
                !glade.overrides.is_empty(),
                "{} has no overlays",
                glade.path
            );
        }
        for path in [
            "packaging/anaconda/product/.buildstamp",
            "packaging/anaconda/product/etc/os-release",
            "packaging/anaconda/product/usr/lib/os-release",
            "packaging/anaconda/product/usr/share/anaconda/pixmaps/soda.css",
            "packaging/anaconda/grub.cfg",
            "packaging/rpm/soda-installer-branding.spec",
        ] {
            ensure!(
                self.root.join(path).is_file(),
                "required overlay {path} is missing"
            );
        }
        for (path, mode) in [
            ("packaging/kickstart/interactive.ks", "graphical"),
            ("packaging/kickstart/automated.ks", "text"),
        ] {
            let kickstart = fs::read_to_string(self.root.join(path))?;
            ensure!(
                kickstart.contains(mode)
                    && kickstart.contains(&format!(
                        "url --mirrorlist=\"{}\"",
                        payload.baseos_mirrorlist
                    ))
                    && kickstart.contains(&format!(
                        "repo --name=AppStream --mirrorlist=\"{}\"",
                        payload.appstream_mirrorlist
                    ))
                    && kickstart.contains("%packages --exclude-weakdeps")
                    && kickstart.contains("file:///run/install/repo/soda/")
                    && !kickstart.lines().any(|line| line.trim() == "cdrom"),
                "{path} does not match the compact network payload contract"
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
            self.spec.base.installer_source_version, self.spec.identity.architecture
        );
        Ok(())
    }

    #[allow(clippy::too_many_lines)]
    fn build_rpms(&self) -> anyhow::Result<()> {
        self.verify_source()?;
        let artifacts = self.root.join(".artifacts");
        let build = artifacts.join("build");
        let topdir = artifacts.join("rpmbuild");
        let repo = artifacts.join("soda");
        let installer = artifacts.join("installer");
        fs::create_dir_all(&build)?;
        recreate(&topdir)?;
        recreate(&repo)?;
        recreate(&installer)?;
        for directory in ["BUILD", "BUILDROOT", "RPMS", "SOURCES", "SPECS", "SRPMS"] {
            fs::create_dir_all(topdir.join(directory))?;
        }

        let product_root = self.prepare_product_image(&installer)?;

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
        for (source, destination) in [
            (build.join("target/release/sodad"), sources.join("sodad")),
            (
                build.join("target/release/sodactl"),
                sources.join("sodactl"),
            ),
            (
                build.join("target/release/soda-ssh"),
                sources.join("soda-ssh"),
            ),
            (build.join("soda-cockpit"), sources.join("soda-cockpit")),
            (build.join("soda-authd"), sources.join("soda-authd")),
            (
                self.root.join("packaging/systemd/sodad.service"),
                sources.join("sodad.service"),
            ),
            (
                self.root.join("packaging/sshd/40-soda-observability.conf"),
                sources.join("40-soda-observability.conf"),
            ),
            (
                self.root.join("packaging/systemd/soda-cockpit.service"),
                sources.join("soda-cockpit.service"),
            ),
            (
                self.root.join("packaging/systemd/soda-authd.service"),
                sources.join("soda-authd.service"),
            ),
            (
                self.root.join("packaging/avahi/soda-cockpit.service"),
                sources.join("soda-cockpit.avahi.service"),
            ),
            (
                self.root.join("packaging/pam/soda-cockpit"),
                sources.join("soda-cockpit.pam"),
            ),
            (
                self.root
                    .join("packaging/anaconda/product/usr/share/doc/soda-installer/BASE_SYSTEM.md"),
                sources.join("BASE_SYSTEM.md"),
            ),
            (
                self.root.join("assets/branding/source/soda-symbol.svg"),
                sources.join("soda-symbol.svg"),
            ),
            (
                self.root
                    .join("assets/branding/installer/soda-symbol-256.png"),
                sources.join("soda-symbol-256.png"),
            ),
            (
                self.root
                    .join("assets/branding/icons/hicolor/16x16/apps/soda-os.png"),
                sources.join("soda-os-16.png"),
            ),
            (
                self.root
                    .join("assets/branding/icons/hicolor/24x24/apps/soda-os.png"),
                sources.join("soda-os-24.png"),
            ),
            (
                self.root
                    .join("assets/branding/icons/hicolor/32x32/apps/soda-os.png"),
                sources.join("soda-os-32.png"),
            ),
            (
                self.root
                    .join("assets/branding/icons/hicolor/48x48/apps/soda-os.png"),
                sources.join("soda-os-48.png"),
            ),
            (
                self.root
                    .join("assets/branding/icons/hicolor/64x64/apps/soda-os.png"),
                sources.join("soda-os-64.png"),
            ),
            (
                self.root
                    .join("assets/branding/icons/hicolor/128x128/apps/soda-os.png"),
                sources.join("soda-os-128.png"),
            ),
            (
                self.root
                    .join("assets/branding/icons/hicolor/256x256/apps/soda-os.png"),
                sources.join("soda-os-256.png"),
            ),
            (
                self.root
                    .join("assets/branding/icons/hicolor/512x512/apps/soda-os.png"),
                sources.join("soda-os-512.png"),
            ),
        ] {
            copy(source, destination)?;
        }
        copy_tree(&product_root, &sources.join("soda-installer-product"))?;
        copy(
            self.root.join(&self.spec.installer.branding_manifest),
            sources.join("branding.toml"),
        )?;
        copy(
            self.root.join(&self.spec.installer.upstream_manifest),
            sources.join("upstream.toml"),
        )?;

        for spec in TARGET_RPMS {
            self.rpmbuild(spec)?;
        }
        for name in TARGET_RPMS {
            let rpm = find_single_rpm(&topdir.join("RPMS"), name)?;
            let filename = rpm.file_name().context("RPM has no filename")?;
            copy(&rpm, repo.join(filename))?;
        }
        self.docker(["createrepo_c", "--update", "/src/.artifacts/soda"])?;
        self.validate_target_rpms(&repo)?;

        self.rpmbuild("soda-installer-branding")?;
        let branding_rpm = find_single_rpm(&topdir.join("RPMS"), "soda-installer-branding")?;
        let branding_dir = installer.join("rpms");
        fs::create_dir_all(&branding_dir)?;
        let branding_output =
            branding_dir.join(branding_rpm.file_name().context("RPM has no filename")?);
        copy(&branding_rpm, &branding_output)?;
        let listing = self.docker_output_owned(vec![
            "rpm".to_owned(),
            "-qpl".to_owned(),
            self.container_path(&branding_output)?,
        ])?;
        ensure!(
            listing
                .contains("/usr/share/soda-installer/product/etc/anaconda/profile.d/sodaos.conf")
                && listing.contains("/usr/share/soda-installer/manifests/upstream.toml"),
            "installer-branding RPM payload is incomplete"
        );
        ensure!(
            fs::read_dir(&repo)?.filter_map(Result::ok).all(|entry| {
                !entry
                    .file_name()
                    .to_string_lossy()
                    .starts_with("soda-installer-branding-")
            }),
            "build-only installer RPM leaked into the target repository"
        );
        println!("Built Soda target RPM repository at {}", repo.display());
        println!(
            "Built build-only installer RPM at {}",
            branding_output.display()
        );
        Ok(())
    }

    #[allow(clippy::too_many_lines)]
    fn build_iso(&self, automated: bool) -> anyhow::Result<()> {
        self.build_rpms()?;
        let artifacts = self.root.join(".artifacts");
        let images = artifacts.join("images");
        let overlay = artifacts.join("iso-overlay");
        fs::create_dir_all(&images)?;
        recreate(&overlay)?;
        fs::create_dir_all(overlay.join("EFI/BOOT"))?;
        fs::create_dir_all(overlay.join("images"))?;

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
        self.resolve_network_payload(automated)?;

        let source_iso = self.container_path(&self.spec.base.source_iso)?;
        let source_metadata = artifacts.join("source-metadata");
        recreate(&source_metadata)?;
        self.extract_iso_file(&source_iso, "/.treeinfo", &source_metadata.join("treeinfo"))?;
        self.extract_iso_file(&source_iso, "/.discinfo", &source_metadata.join("discinfo"))?;

        let efi_stage = artifacts.join("efi-stage");
        recreate(&efi_stage)?;
        self.docker_owned(vec![
            "xorriso".to_owned(),
            "-osirrox".to_owned(),
            "on".to_owned(),
            "-indev".to_owned(),
            source_iso.clone(),
            "-extract".to_owned(),
            "/EFI".to_owned(),
            self.container_path(efi_stage.join("EFI"))?,
        ])?;

        let grub = self.render_grub()?;
        let staged_grub = efi_stage.join("EFI/BOOT/grub.cfg");
        fs::remove_file(&staged_grub)?;
        fs::write(&staged_grub, &grub)?;
        fs::write(overlay.join("EFI/BOOT/grub.cfg"), &grub)?;
        copy(
            self.root
                .join("assets/branding/installer/grub-background.png"),
            efi_stage.join("EFI/BOOT/soda-grub-background.png"),
        )?;
        copy(
            self.root
                .join("assets/branding/installer/grub-background.png"),
            overlay.join("EFI/BOOT/soda-grub-background.png"),
        )?;

        let efiboot = overlay.join("images/efiboot.img");
        self.docker_privileged_owned(vec![
            "mkefiboot".to_owned(),
            "--label=SODAOS".to_owned(),
            self.container_path(efi_stage.join("EFI/BOOT"))?,
            self.container_path(&efiboot)?,
        ])?;
        copy(
            artifacts.join("installer/product.img"),
            overlay.join("images/product.img"),
        )?;
        copy(self.root.join(kickstart), overlay.join("ks.cfg"))?;
        copy_tree(&artifacts.join("soda"), &overlay.join("soda"))?;

        let treeinfo = rewrite_treeinfo(
            &fs::read_to_string(source_metadata.join("treeinfo"))?,
            &sha256_file(&efiboot)?,
            &sha256_file(&overlay.join("images/product.img"))?,
            &self.spec.identity.version,
        );
        fs::write(overlay.join(".treeinfo"), treeinfo)?;
        let source_discinfo = fs::read_to_string(source_metadata.join("discinfo"))?;
        let timestamp = source_discinfo.lines().next().unwrap_or("0");
        fs::write(
            overlay.join(".discinfo"),
            format!(
                "{timestamp}\nSoda OS {}\naarch64\nALL\n",
                self.spec.identity.version
            ),
        )?;
        let output_container = self.container_path(&output)?;
        let mut xorriso = vec![
            "xorriso".to_owned(),
            "-indev".to_owned(),
            source_iso,
            "-outdev".to_owned(),
            output_container.clone(),
            "-boot_image".to_owned(),
            "any".to_owned(),
            "replay".to_owned(),
            "-volid".to_owned(),
            self.spec.installer.volume_id.clone(),
            "-rm_r".to_owned(),
            "/BaseOS".to_owned(),
            "/AppStream".to_owned(),
            "--".to_owned(),
            "-rm".to_owned(),
            "/media.repo".to_owned(),
            "/extra_files.json".to_owned(),
            "/RPM-GPG-KEY-Rocky-10-Testing".to_owned(),
            "--".to_owned(),
        ];
        for (disk, iso) in [
            (overlay.join(".treeinfo"), "/.treeinfo"),
            (overlay.join(".discinfo"), "/.discinfo"),
            (overlay.join("EFI/BOOT/grub.cfg"), "/EFI/BOOT/grub.cfg"),
            (overlay.join("images/efiboot.img"), "/images/efiboot.img"),
        ] {
            xorriso.extend([
                "-update".to_owned(),
                self.container_path(disk)?,
                iso.to_owned(),
            ]);
        }
        for (disk, iso) in [
            (
                overlay.join("EFI/BOOT/soda-grub-background.png"),
                "/EFI/BOOT/soda-grub-background.png",
            ),
            (overlay.join("images/product.img"), "/images/product.img"),
            (overlay.join("ks.cfg"), "/ks.cfg"),
            (overlay.join("soda"), "/soda"),
        ] {
            xorriso.extend([
                "-map".to_owned(),
                self.container_path(disk)?,
                iso.to_owned(),
            ]);
        }
        self.docker_privileged_owned(xorriso)?;
        self.docker_privileged_owned(vec![
            "implantisomd5".to_owned(),
            "--force".to_owned(),
            output_container.clone(),
        ])?;
        let iso_size = fs::metadata(&output)?.len();
        ensure!(
            iso_size <= self.spec.installer.payload.max_iso_size_bytes,
            "compact ISO is {iso_size} bytes; maximum is {} bytes",
            self.spec.installer.payload.max_iso_size_bytes
        );
        self.inspect_iso(&output, automated)?;

        let digest = self.docker_output(["sha256sum", &output_container])?;
        let checksum = digest
            .split_whitespace()
            .next()
            .context("sha256sum did not return a digest")?;
        fs::write(output.with_extension("iso.sha256"), format!("{checksum}\n"))?;
        println!("Built {} ({checksum})", output.display());
        Ok(())
    }

    fn prepare_product_image(&self, installer: &Path) -> anyhow::Result<PathBuf> {
        let product_root = installer.join("product-root");
        let upstream_root = installer.join("upstream-root");
        recreate(&product_root)?;
        recreate(&upstream_root)?;
        copy_tree(&self.root.join("packaging/anaconda/product"), &product_root)?;

        let manifest = self.upstream_manifest()?;
        let upstream_rpm = installer.join("anaconda-gui.rpm");
        let source_iso = self.container_path(&self.spec.base.source_iso)?;
        self.extract_iso_file(
            &source_iso,
            &format!("/{}", manifest.anaconda_gui_rpm),
            &upstream_rpm,
        )?;
        let actual_nevra = self
            .docker_output_owned(vec![
                "rpm".to_owned(),
                "-qp".to_owned(),
                "--qf".to_owned(),
                "%{NAME}-%{VERSION}-%{RELEASE}.%{ARCH}".to_owned(),
                self.container_path(&upstream_rpm)?,
            ])?
            .trim()
            .to_owned();
        ensure!(
            actual_nevra == manifest.anaconda_gui_nevra,
            "expected {}, extracted {actual_nevra}",
            manifest.anaconda_gui_nevra
        );
        self.docker_owned(vec![
            "bash".to_owned(),
            "-c".to_owned(),
            "cd \"$1\" && rpm2cpio \"$2\" | cpio -idm --quiet".to_owned(),
            "soda-extract".to_owned(),
            self.container_path(&upstream_root)?,
            self.container_path(&upstream_rpm)?,
        ])?;

        for contract in &manifest.glade {
            let source = upstream_root.join(&contract.path);
            ensure!(
                source.is_file(),
                "upstream Glade {} is missing",
                contract.path
            );
            ensure!(
                sha256_file(&source)? == contract.sha256,
                "upstream Glade {} changed; refusing to apply an unreviewed overlay",
                contract.path
            );
            let mut xml = fs::read_to_string(&source)?;
            for change in &contract.overrides {
                xml = apply_glade_override(&xml, change).with_context(|| {
                    format!(
                        "apply {}.{} in {}",
                        change.object_id, change.property, contract.path
                    )
                })?;
            }
            let destination = product_root.join(&contract.path);
            if let Some(parent) = destination.parent() {
                fs::create_dir_all(parent)?;
            }
            fs::write(&destination, xml)?;
            self.docker_owned(vec![
                "xmllint".to_owned(),
                "--noout".to_owned(),
                self.container_path(&destination)?,
            ])?;
        }

        let product_image = installer.join("product.img");
        self.docker_owned(vec![
            "bash".to_owned(), "-c".to_owned(),
            "cd \"$1\" && find . -exec touch -h -d @0 {} + && find . -print0 | LC_ALL=C sort -z | cpio --null -o --format=newc --owner=0:0 --reproducible --quiet | xz -9e > \"$2\"".to_owned(),
            "soda-product-image".to_owned(), self.container_path(&product_root)?, self.container_path(&product_image)?,
        ])?;
        let listing = self.docker_output_owned(vec![
            "bash".to_owned(),
            "-c".to_owned(),
            "xz -dc \"$1\" | cpio -it --quiet".to_owned(),
            "soda-product-inspect".to_owned(),
            self.container_path(&product_image)?,
        ])?;
        for required in [
            ".buildstamp",
            "etc/anaconda/profile.d/sodaos.conf",
            "etc/os-release",
            "usr/lib/os-release",
            "usr/share/anaconda/pixmaps/soda.css",
            "usr/share/anaconda/ui/spokes/welcome.glade",
            "usr/share/anaconda/ui/spokes/storage.glade",
            "usr/share/anaconda/ui/spokes/network.glade",
            "usr/share/anaconda/ui/spokes/user.glade",
        ] {
            ensure!(
                listing.contains(required),
                "product.img is missing {required}"
            );
        }
        println!("Built and inspected {}", product_image.display());
        Ok(product_root)
    }

    fn render_grub(&self) -> anyhow::Result<String> {
        let template = fs::read_to_string(self.root.join("packaging/anaconda/grub.cfg"))?;
        let rendered = template
            .replace(
                "@BOOT_TIMEOUT@",
                &self.spec.installer.boot_timeout_seconds.to_string(),
            )
            .replace("@VOLUME_ID@", &self.spec.installer.volume_id)
            .replace("@VOLUME_ID_ESCAPED@", &self.spec.installer.volume_id)
            .replace("@PROFILE_ID@", &self.spec.installer.profile_id)
            .replace("@VERSION@", &self.spec.identity.version);
        ensure!(!rendered.contains('@'), "unresolved GRUB template marker");
        ensure!(
            !rendered.contains("Rocky") && !rendered.contains("FIPS"),
            "forbidden boot menu identity"
        );
        Ok(rendered)
    }

    #[allow(clippy::too_many_lines)]
    fn inspect_iso(&self, iso: &Path, automated: bool) -> anyhow::Result<()> {
        let inspect = self.root.join(".artifacts/iso-inspect");
        recreate(&inspect)?;
        let iso_container = self.container_path(iso)?;
        for (source, destination) in [
            ("/.treeinfo", "treeinfo"),
            ("/.discinfo", "discinfo"),
            ("/EFI/BOOT/grub.cfg", "grub.cfg"),
            ("/ks.cfg", "ks.cfg"),
            ("/images/product.img", "product.img"),
        ] {
            self.extract_iso_file(&iso_container, source, &inspect.join(destination))?;
        }
        let report = self.docker_output_combined_owned(vec![
            "xorriso".to_owned(),
            "-indev".to_owned(),
            iso_container.clone(),
            "-pvd_info".to_owned(),
            "-report_el_torito".to_owned(),
            "plain".to_owned(),
            "-report_system_area".to_owned(),
            "plain".to_owned(),
        ])?;
        ensure!(
            report.contains(&format!(
                "Volume id    : '{}'",
                self.spec.installer.volume_id
            )),
            "ISO volume ID differs from the installer contract"
        );
        ensure!(
            report.contains("EFI") || report.contains("UEFI"),
            "ISO does not report an EFI boot image"
        );
        let grub = fs::read_to_string(inspect.join("grub.cfg"))?;
        ensure!(
            grub.matches("menuentry '").count() == 4,
            "unexpected Soda boot menu entry count"
        );
        ensure!(
            grub.contains("inst.profile=sodaos")
                && grub.contains("Install Soda OS 0.1.0")
                && !grub.contains("Rocky")
                && !grub.contains("FIPS"),
            "boot menu contract failed"
        );
        let treeinfo = fs::read_to_string(inspect.join("treeinfo"))?;
        ensure!(
            !treeinfo.contains("BaseOS")
                && !treeinfo.contains("AppStream")
                && !treeinfo.contains("[variant-"),
            "boot-only treeinfo still advertises a local Rocky package payload"
        );
        let customer_visible = ["treeinfo", "discinfo", "grub.cfg"]
            .into_iter()
            .map(|name| fs::read_to_string(inspect.join(name)))
            .collect::<Result<Vec<_>, _>>()?
            .join("\n");
        ensure!(
            !customer_visible.contains("Rocky"),
            "customer-visible ISO metadata still contains Rocky branding"
        );
        let kickstart = fs::read_to_string(inspect.join("ks.cfg"))?;
        ensure!(
            kickstart.contains(if automated { "text" } else { "graphical" })
                && kickstart.contains(BASEOS_MIRRORLIST)
                && kickstart.contains(APPSTREAM_MIRRORLIST)
                && kickstart.contains("%packages --exclude-weakdeps")
                && !kickstart.lines().any(|line| line.trim() == "cdrom"),
            "ISO contains the wrong Kickstart mode"
        );
        let product_listing = self.docker_output_owned(vec![
            "bash".to_owned(),
            "-c".to_owned(),
            "xz -dc \"$1\" | cpio -it --quiet".to_owned(),
            "soda-product-inspect".to_owned(),
            self.container_path(inspect.join("product.img"))?,
        ])?;
        ensure!(
            product_listing.contains("etc/anaconda/profile.d/sodaos.conf"),
            "ISO product image is missing the Soda profile"
        );
        let root_report = self.docker_output_combined_owned(vec![
            "xorriso".to_owned(),
            "-indev".to_owned(),
            iso_container.clone(),
            "-ls".to_owned(),
            "/".to_owned(),
        ])?;
        let mut roots = quoted_listing_entries(&root_report);
        roots.sort();
        let mut expected_roots = ISO_ROOT_ALLOWLIST.map(str::to_owned).to_vec();
        expected_roots.sort();
        ensure!(
            roots == expected_roots,
            "compact ISO root differs from the allowlist: found {roots:?}"
        );
        let rpm_report = self.docker_output_combined_owned(vec![
            "xorriso".to_owned(),
            "-indev".to_owned(),
            iso_container.clone(),
            "-find".to_owned(),
            "/soda".to_owned(),
            "-type".to_owned(),
            "f".to_owned(),
            "-name".to_owned(),
            "*.rpm".to_owned(),
        ])?;
        let rpm_entries = quoted_listing_entries(&rpm_report);
        ensure!(
            rpm_entries.len() == TARGET_RPMS.len()
                && TARGET_RPMS.iter().all(|package| {
                    rpm_entries.iter().any(|entry| {
                        Path::new(entry)
                            .file_name()
                            .and_then(|filename| filename.to_str())
                            .is_some_and(|filename| filename.starts_with(&format!("{package}-")))
                    })
                }),
            "compact ISO does not contain exactly the three target Soda RPMs: {rpm_entries:?}"
        );
        ensure!(
            fs::metadata(iso)?.len() <= self.spec.installer.payload.max_iso_size_bytes,
            "compact ISO exceeds its size contract"
        );
        self.docker_owned(vec!["checkisomd5".to_owned(), iso_container])?;
        println!(
            "Inspected UEFI layout and Soda identity in {}",
            iso.display()
        );
        Ok(())
    }

    fn resolve_network_payload(&self, automated: bool) -> anyhow::Result<()> {
        let artifacts = self.root.join(".artifacts");
        let repo_dir = artifacts.join("network-repos");
        let manifest_dir = artifacts.join("manifests");
        recreate(&repo_dir)?;
        fs::create_dir_all(&manifest_dir)?;
        fs::write(
            repo_dir.join("soda-network.repo"),
            format!(
                "[soda-baseos]\nname=Rocky Linux 10 BaseOS\nmirrorlist={}\nenabled=1\ngpgcheck=0\n\n[soda-appstream]\nname=Rocky Linux 10 AppStream\nmirrorlist={}\nenabled=1\ngpgcheck=0\n\n[soda-local]\nname=Soda OS local packages\nbaseurl=file:///src/.artifacts/soda\nenabled=1\ngpgcheck=0\n",
                self.spec.installer.payload.baseos_mirrorlist,
                self.spec.installer.payload.appstream_mirrorlist
            ),
        )?;

        let mut roots = vec![format!("@{}", self.spec.installer.payload.environment)];
        roots.extend(self.spec.installer.payload.packages.iter().cloned());
        if automated {
            roots.extend(
                self.spec
                    .installer
                    .payload
                    .automated_extra_packages
                    .iter()
                    .cloned(),
            );
        }
        roots.extend(
            self.spec
                .installer
                .payload
                .anaconda_required_packages
                .iter()
                .cloned(),
        );

        let script = "set -euo pipefail; rm -rf /tmp/soda-network-root /tmp/soda-network-payload; mkdir -p /tmp/soda-network-root /tmp/soda-network-payload; dnf -q -y --installroot /tmp/soda-network-root --releasever \"$2\" --setopt=\"reposdir=$1\" --setopt=install_weak_deps=False --downloadonly --destdir /tmp/soda-network-payload install \"${@:3}\" >/dev/null; find /tmp/soda-network-payload -type f -name '*.rpm' -print0 | xargs -0 rpm -qp --qf '%{NAME}-%{VERSION}-%{RELEASE}.%{ARCH}\\n' | LC_ALL=C sort -u";
        let mut command = vec![
            "bash".to_owned(),
            "-c".to_owned(),
            script.to_owned(),
            "soda-network-resolve".to_owned(),
            self.container_path(&repo_dir)?,
            self.spec.base.package_stream.clone(),
        ];
        command.extend(roots);
        let resolved = self.docker_output_owned(command)?;
        ensure!(
            !resolved.trim().is_empty(),
            "Rocky network payload resolved no RPMs"
        );
        for package in ANACONDA_REQUIRED_PACKAGES
            .iter()
            .chain(REQUIRED_FIRMWARE_PACKAGES.iter())
            .chain(TARGET_RPMS.iter())
        {
            ensure!(
                manifest_contains_package(&resolved, package),
                "resolved network payload is missing {package}"
            );
        }
        let suffix = if automated { "-test" } else { "" };
        let manifest = manifest_dir.join(format!("rocky-network-payload{suffix}.txt"));
        fs::write(
            &manifest,
            format!(
                "baseos_mirrorlist={}\nappstream_mirrorlist={}\npackage_stream={}\nweak_dependencies=false\n\n{}",
                self.spec.installer.payload.baseos_mirrorlist,
                self.spec.installer.payload.appstream_mirrorlist,
                self.spec.base.package_stream,
                resolved
            ),
        )?;
        println!(
            "Resolved current Rocky {} network payload at {}",
            self.spec.base.package_stream,
            manifest.display()
        );
        Ok(())
    }

    fn validate_target_rpms(&self, repo: &Path) -> anyhow::Result<()> {
        let mut rpms = fs::read_dir(repo)?
            .filter_map(Result::ok)
            .map(|entry| entry.path())
            .filter(|path| path.extension().is_some_and(|extension| extension == "rpm"))
            .collect::<Vec<_>>();
        rpms.sort();
        ensure!(
            rpms.len() == TARGET_RPMS.len(),
            "expected three target Soda RPMs, found {}",
            rpms.len()
        );
        let mut command = vec!["dnf".to_owned(), "-y".to_owned(), "install".to_owned()];
        for rpm in rpms {
            command.push(self.container_path(&rpm)?);
        }
        self.docker_owned(command)
    }

    fn rpmbuild(&self, name: &str) -> anyhow::Result<()> {
        self.docker_owned(vec![
            "rpmbuild".to_owned(),
            "-bb".to_owned(),
            "--define".to_owned(),
            "_topdir /src/.artifacts/rpmbuild".to_owned(),
            format!("packaging/rpm/{name}.spec"),
        ])
    }

    fn extract_iso_file(&self, iso: &str, source: &str, destination: &Path) -> anyhow::Result<()> {
        if let Some(parent) = destination.parent() {
            fs::create_dir_all(parent)?;
        }
        if destination.exists() {
            fs::remove_file(destination)?;
        }
        self.docker_owned(vec![
            "xorriso".to_owned(),
            "-osirrox".to_owned(),
            "on".to_owned(),
            "-indev".to_owned(),
            iso.to_owned(),
            "-extract".to_owned(),
            source.to_owned(),
            self.container_path(destination)?,
        ])
    }

    fn branding_manifest(&self) -> anyhow::Result<BrandingManifest> {
        load_toml(&self.root.join(&self.spec.installer.branding_manifest))
    }

    fn upstream_manifest(&self) -> anyhow::Result<UpstreamManifest> {
        load_toml(&self.root.join(&self.spec.installer.upstream_manifest))
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

    fn docker_output_owned(&self, arguments: Vec<String>) -> anyhow::Result<String> {
        let mut command = self.docker_command();
        command.arg(BUILDER_IMAGE).args(arguments);
        output(&mut command)
    }

    fn docker_output_combined_owned(&self, arguments: Vec<String>) -> anyhow::Result<String> {
        let mut command = self.docker_command();
        command.arg(BUILDER_IMAGE).args(arguments);
        output_combined(&mut command)
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

fn load_toml<T: for<'de> Deserialize<'de>>(path: &Path) -> anyhow::Result<T> {
    let text = fs::read_to_string(path).with_context(|| format!("read {}", path.display()))?;
    toml::from_str(&text).with_context(|| format!("parse {}", path.display()))
}

fn png_dimensions(path: &Path) -> anyhow::Result<(u32, u32)> {
    let bytes = fs::read(path)?;
    ensure!(
        bytes.len() >= 24 && &bytes[..8] == b"\x89PNG\r\n\x1a\n" && &bytes[12..16] == b"IHDR",
        "{} is not a PNG",
        path.display()
    );
    Ok((
        u32::from_be_bytes(bytes[16..20].try_into()?),
        u32::from_be_bytes(bytes[20..24].try_into()?),
    ))
}

fn sha256_file(path: &Path) -> anyhow::Result<String> {
    let mut hasher = Sha256::new();
    hasher.update(fs::read(path)?);
    Ok(hex::encode(hasher.finalize()))
}

fn apply_glade_override(xml: &str, change: &GladeOverride) -> anyhow::Result<String> {
    let id_double = format!("id=\"{}\"", change.object_id);
    let id_single = format!("id='{}'", change.object_id);
    let id_position = xml
        .find(&id_double)
        .or_else(|| xml.find(&id_single))
        .with_context(|| format!("object {} is missing", change.object_id))?;
    let object_start = xml[..id_position]
        .rfind("<object")
        .with_context(|| format!("object {} has no opening tag", change.object_id))?;
    let opening_end = xml[id_position..]
        .find('>')
        .map(|offset| id_position + offset + 1)
        .context("object opening tag is incomplete")?;
    let object_end = matching_object_end(xml, object_start)?;
    let first_child = xml[opening_end..object_end]
        .find("<child")
        .map_or(object_end, |offset| opening_end + offset);
    let direct = &xml[opening_end..first_child];
    let property_double = format!("<property name=\"{}\"", change.property);
    let property_single = format!("<property name='{}'", change.property);
    if let Some(relative) = direct
        .find(&property_double)
        .or_else(|| direct.find(&property_single))
    {
        let property_start = opening_end + relative;
        let value_start = xml[property_start..]
            .find('>')
            .map(|offset| property_start + offset + 1)
            .context("property opening tag is incomplete")?;
        let value_end = xml[value_start..]
            .find("</property>")
            .map(|offset| value_start + offset)
            .context("property closing tag is missing")?;
        return Ok(format!(
            "{}{}{}",
            &xml[..value_start],
            escape_xml(&change.value),
            &xml[value_end..]
        ));
    }
    let indentation = line_indentation(xml, opening_end);
    let property = format!(
        "\n{indentation}  <property name=\"{}\">{}</property>",
        change.property,
        escape_xml(&change.value)
    );
    Ok(format!(
        "{}{}{}",
        &xml[..opening_end],
        property,
        &xml[opening_end..]
    ))
}

fn matching_object_end(xml: &str, object_start: usize) -> anyhow::Result<usize> {
    let mut position = object_start;
    let mut depth = 0_u32;
    loop {
        let next_open = xml[position..]
            .find("<object")
            .map(|offset| position + offset);
        let next_close = xml[position..]
            .find("</object>")
            .map(|offset| position + offset);
        match (next_open, next_close) {
            (Some(open), Some(close)) if open < close => {
                let opening_end = xml[open..]
                    .find('>')
                    .map(|offset| open + offset)
                    .context("object opening tag is incomplete")?;
                if !xml[open..=opening_end].trim_end().ends_with("/>") {
                    depth += 1;
                }
                position = opening_end + 1;
            }
            (_, Some(close)) => {
                ensure!(depth > 0, "unexpected object closing tag");
                depth -= 1;
                if depth == 0 {
                    return Ok(close);
                }
                position = close + "</object>".len();
            }
            _ => bail!("object closing tag is missing"),
        }
    }
}

fn line_indentation(text: &str, position: usize) -> &str {
    let line_start = text[..position].rfind('\n').map_or(0, |offset| offset + 1);
    let line = &text[line_start..position];
    &line[..line.len() - line.trim_start().len()]
}

fn escape_xml(value: &str) -> String {
    value
        .replace('&', "&amp;")
        .replace('<', "&lt;")
        .replace('>', "&gt;")
}

fn rewrite_treeinfo(
    source: &str,
    efiboot_sha256: &str,
    product_sha256: &str,
    version: &str,
) -> String {
    let mut section = "";
    let mut output = Vec::new();
    let mut product_checksum_added = false;
    let mut omit_section = false;
    for line in source.lines() {
        if line.starts_with('[') && line.ends_with(']') {
            if section == "checksums" && !product_checksum_added && !omit_section {
                output.push(format!("images/product.img = sha256:{product_sha256}"));
                product_checksum_added = true;
            }
            section = &line[1..line.len() - 1];
            omit_section = section.starts_with("variant-");
            if !omit_section {
                output.push(line.to_owned());
            }
            continue;
        }
        if omit_section {
            continue;
        }
        let rewritten = match (section, line.split_once('=').map(|(key, _)| key.trim())) {
            ("checksums", Some("images/efiboot.img")) => {
                format!("images/efiboot.img = sha256:{efiboot_sha256}")
            }
            ("general", Some("family")) => "family = Soda OS".to_owned(),
            ("general", Some("name")) => format!("name = Soda OS {version}"),
            ("general" | "release", Some("version")) => format!("version = {version}"),
            ("release", Some("name")) => "name = Soda OS".to_owned(),
            ("release", Some("short")) => "short = SodaOS".to_owned(),
            ("general", Some("packagedir" | "repository" | "variant" | "variants"))
            | ("tree", Some("variants")) => continue,
            _ => line.to_owned(),
        };
        output.push(rewritten);
    }
    if section == "checksums" && !product_checksum_added {
        output.push(format!("images/product.img = sha256:{product_sha256}"));
    }
    format!("{}\n", output.join("\n"))
}

fn recreate(path: &Path) -> anyhow::Result<()> {
    if path.exists() {
        fs::remove_dir_all(path)?;
    }
    fs::create_dir_all(path)?;
    Ok(())
}

fn string_slice_eq<const N: usize>(actual: &[String], expected: &[&str; N]) -> bool {
    actual
        .iter()
        .map(String::as_str)
        .eq(expected.iter().copied())
}

fn manifest_contains_package(manifest: &str, package: &str) -> bool {
    manifest.lines().any(|line| {
        line.strip_prefix(package)
            .is_some_and(|rest| rest.starts_with('-'))
    })
}

fn quoted_listing_entries(report: &str) -> Vec<String> {
    report
        .lines()
        .filter_map(|line| {
            let line = line.trim();
            line.strip_prefix('\'')
                .and_then(|entry| entry.strip_suffix('\''))
                .map(str::to_owned)
        })
        .collect()
}

fn copy(source: impl AsRef<Path>, destination: impl AsRef<Path>) -> anyhow::Result<()> {
    let source = source.as_ref();
    let destination = destination.as_ref();
    if let Some(parent) = destination.parent() {
        fs::create_dir_all(parent)?;
    }
    fs::copy(source, destination).with_context(|| format!("copy {}", source.display()))?;
    Ok(())
}

fn copy_tree(source: &Path, destination: &Path) -> anyhow::Result<()> {
    fs::create_dir_all(destination)?;
    for entry in fs::read_dir(source)? {
        let entry = entry?;
        let path = entry.path();
        let target = destination.join(entry.file_name());
        if path.is_dir() {
            copy_tree(&path, &target)?;
        } else {
            copy(&path, &target)?;
        }
    }
    Ok(())
}

fn find_single_rpm(root: &Path, name: &str) -> anyhow::Result<PathBuf> {
    let mut matches = Vec::new();
    collect_matching_rpms(root, name, &mut matches)?;
    ensure!(
        matches.len() == 1,
        "expected one {name} RPM, found {}",
        matches.len()
    );
    Ok(matches.remove(0))
}

fn collect_matching_rpms(
    root: &Path,
    name: &str,
    matches: &mut Vec<PathBuf>,
) -> anyhow::Result<()> {
    for entry in fs::read_dir(root)? {
        let path = entry?.path();
        if path.is_dir() {
            collect_matching_rpms(&path, name, matches)?;
        } else if path.extension().is_some_and(|extension| extension == "rpm")
            && path
                .file_name()
                .is_some_and(|filename| filename.to_string_lossy().starts_with(&format!("{name}-")))
        {
            matches.push(path);
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

fn output_combined(command: &mut Process) -> anyhow::Result<String> {
    let display = format!("{command:?}");
    println!("+ {display}");
    let result = command.output()?;
    ensure!(
        result.status.success(),
        "{display} exited with {}: {}",
        result.status,
        String::from_utf8_lossy(&result.stderr).trim()
    );
    let mut combined = String::from_utf8(result.stdout).context("command stdout is not UTF-8")?;
    combined.push_str(&String::from_utf8(result.stderr).context("command stderr is not UTF-8")?);
    Ok(combined)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn overrides_existing_direct_glade_property() {
        let xml = r#"<interface><object class="GtkWindow" id="window"><property name="title">Old</property><child><object class="GtkLabel" id="child"><property name="title">Nested</property></object></child></object></interface>"#;
        let changed = apply_glade_override(
            xml,
            &GladeOverride {
                object_id: "window".to_owned(),
                property: "title".to_owned(),
                value: "Soda & More".to_owned(),
            },
        )
        .unwrap();
        assert!(changed.contains(">Soda &amp; More</property>"));
        assert!(changed.contains(">Nested</property>"));
    }

    #[test]
    fn inserts_missing_direct_glade_property() {
        let xml = "<interface>\n  <object class=\"GtkBox\" id=\"box\">\n  </object>\n</interface>";
        let changed = apply_glade_override(
            xml,
            &GladeOverride {
                object_id: "box".to_owned(),
                property: "visible".to_owned(),
                value: "False".to_owned(),
            },
        )
        .unwrap();
        assert!(changed.contains("<property name=\"visible\">False</property>"));
    }

    #[test]
    fn handles_self_closing_nested_objects() {
        let xml = r#"<interface><object class="GtkWindow" id="window"><property name="title">Old</property><child><object class="GtkSizeGroup" id="size"/></child></object></interface>"#;
        let changed = apply_glade_override(
            xml,
            &GladeOverride {
                object_id: "window".to_owned(),
                property: "title".to_owned(),
                value: "Soda".to_owned(),
            },
        )
        .unwrap();
        assert!(changed.contains(">Soda</property>"));
    }

    #[test]
    fn rewrites_treeinfo_for_boot_only_media() {
        let source = "[checksums]\nimages/efiboot.img = sha256:old\n\n[general]\nfamily = Rocky Linux\nname = Rocky Linux 10.2\npackagedir = AppStream/Packages\nrepository = AppStream\nvariants = AppStream,BaseOS\nversion = 10.2\n\n[release]\nname = Rocky Linux\nshort = Rocky\nversion = 10.2\n\n[tree]\narch = aarch64\nvariants = AppStream,BaseOS\n\n[variant-BaseOS]\nname = BaseOS\n";
        let result = rewrite_treeinfo(source, "efi", "product", "0.1.0");
        assert!(result.contains("images/product.img = sha256:product"));
        assert!(result.contains("family = Soda OS"));
        assert!(result.contains("short = SodaOS"));
        assert!(!result.contains("variant-BaseOS"));
        assert!(!result.contains("AppStream"));
        assert!(!result.contains("BaseOS"));
        assert!(!result.contains("Rocky"));
    }

    #[test]
    fn finds_exact_package_names_in_payload_manifest() {
        let manifest = "kernel-6.12.aarch64\nkernel-core-6.12.aarch64\n";
        assert!(manifest_contains_package(manifest, "kernel"));
        assert!(manifest_contains_package(manifest, "kernel-core"));
        assert!(!manifest_contains_package(manifest, "kern"));
    }
}
