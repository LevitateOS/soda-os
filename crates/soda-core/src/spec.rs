use std::{fs, path::Path};

use serde::{Deserialize, Serialize};
use thiserror::Error;

#[derive(Debug, Error)]
pub enum SpecError {
    #[error("failed to read spec {path}: {source}")]
    Read {
        path: String,
        source: std::io::Error,
    },
    #[error("failed to parse spec {path}: {source}")]
    Parse {
        path: String,
        source: toml::de::Error,
    },
    #[error("unsupported spec schema version {0}; expected 1")]
    Schema(u32),
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct DistroSpec {
    pub schema_version: u32,
    pub identity: IdentitySpec,
    pub base: BaseSpec,
    pub network: NetworkSpec,
    pub paths: PathSpec,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct IdentitySpec {
    pub name: String,
    pub id: String,
    pub version: String,
    pub hostname: String,
    pub architecture: String,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct BaseSpec {
    pub distribution: String,
    pub version: String,
    pub source_iso: String,
    pub source_iso_sha256: String,
    pub checksum_file: String,
    pub signature_file: String,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct NetworkSpec {
    pub cockpit_port: u16,
    pub mdns_name: String,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct PathSpec {
    pub state_dir: String,
    pub projects_dir: String,
    pub toolchains_dir: String,
    pub daemon_socket: String,
}

impl DistroSpec {
    /// Loads and validates a distro specification from disk.
    ///
    /// # Errors
    ///
    /// Returns [`SpecError`] when the file cannot be read, parsed, or uses an
    /// unsupported schema version.
    pub fn load(path: impl AsRef<Path>) -> Result<Self, SpecError> {
        let path = path.as_ref();
        let text = fs::read_to_string(path).map_err(|source| SpecError::Read {
            path: path.display().to_string(),
            source,
        })?;
        let spec: Self = toml::from_str(&text).map_err(|source| SpecError::Parse {
            path: path.display().to_string(),
            source,
        })?;
        if spec.schema_version != 1 {
            return Err(SpecError::Schema(spec.schema_version));
        }
        Ok(spec)
    }
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ProfileSpec {
    pub schema_version: u32,
    pub profile: ProfileIdentity,
    pub tools: Vec<ToolSpec>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ProfileIdentity {
    pub id: String,
    pub display_name: String,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ToolSpec {
    pub id: String,
    pub resolver: String,
    pub channel: String,
    pub bin_paths: Vec<String>,
}

impl ProfileSpec {
    /// Loads and validates a toolchain profile specification from disk.
    ///
    /// # Errors
    ///
    /// Returns [`SpecError`] when the file cannot be read, parsed, or uses an
    /// unsupported schema version.
    pub fn load(path: impl AsRef<Path>) -> Result<Self, SpecError> {
        let path = path.as_ref();
        let text = fs::read_to_string(path).map_err(|source| SpecError::Read {
            path: path.display().to_string(),
            source,
        })?;
        let spec: Self = toml::from_str(&text).map_err(|source| SpecError::Parse {
            path: path.display().to_string(),
            source,
        })?;
        if spec.schema_version != 1 {
            return Err(SpecError::Schema(spec.schema_version));
        }
        Ok(spec)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn rejects_unknown_schema() {
        let error = toml::from_str::<DistroSpec>(
            r#"
schema_version = 2
[identity]
name = "Soda OS"
id = "sodaos"
version = "0.1.0"
hostname = "soda"
architecture = "aarch64"
[base]
distribution = "rocky"
version = "10.2"
source_iso = "iso"
source_iso_sha256 = "hash"
checksum_file = "checksum"
signature_file = "signature"
[network]
cockpit_port = 9090
mdns_name = "soda.local"
[paths]
state_dir = "/var/lib/soda"
projects_dir = "/srv/soda/projects"
toolchains_dir = "/opt/soda/toolchains"
daemon_socket = "/run/soda/sodad.sock"
"#,
        )
        .expect("valid TOML");
        assert_eq!(error.schema_version, 2);
    }
}
