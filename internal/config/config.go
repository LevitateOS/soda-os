package config

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

const (
	DefaultStateDir      = "/var/lib/soda"
	DefaultProjectsDir   = "/srv/soda/projects"
	DefaultToolchainsDir = "/opt/soda/toolchains"
	DefaultDaemonSocket  = "/run/soda/sodad.sock"
)

type DistroSpec struct {
	SchemaVersion uint32       `toml:"schema_version"`
	Identity      IdentitySpec `toml:"identity"`
	Base          BaseSpec     `toml:"base"`
	Image         ImageSpec    `toml:"image"`
	Build         BuildSpec    `toml:"build"`
	Network       NetworkSpec  `toml:"network"`
	Paths         PathSpec     `toml:"paths"`
}

type IdentitySpec struct {
	Name         string `toml:"name"`
	ID           string `toml:"id"`
	Version      string `toml:"version"`
	Hostname     string `toml:"hostname"`
	Architecture string `toml:"architecture"`
}

type BaseSpec struct {
	Reference string `toml:"reference"`
	Platform  string `toml:"platform"`
}

type ImageSpec struct {
	Registry    string `toml:"registry"`
	StateSchema uint32 `toml:"state_schema"`
	PackageLock string `toml:"package_lock"`
}

type BuildSpec struct {
	SourceDateEpoch int64 `toml:"source_date_epoch"`
}

type NetworkSpec struct {
	CockpitPort uint16 `toml:"cockpit_port"`
	MDNSName    string `toml:"mdns_name"`
}

type PathSpec struct {
	StateDir      string `toml:"state_dir"`
	ProjectsDir   string `toml:"projects_dir"`
	ToolchainsDir string `toml:"toolchains_dir"`
	DaemonSocket  string `toml:"daemon_socket"`
}

func LoadDistro(path string) (DistroSpec, error) {
	var spec DistroSpec
	if _, err := toml.DecodeFile(path, &spec); err != nil {
		return DistroSpec{}, fmt.Errorf("decode distro specification %q: %w", path, err)
	}
	if spec.SchemaVersion != 2 {
		return DistroSpec{}, fmt.Errorf("unsupported distro schema version %d; expected 2", spec.SchemaVersion)
	}
	return spec, nil
}
