package domain

import (
	"fmt"
	"regexp"
	"time"
)

var unixIdentifier = regexp.MustCompile(`^[a-z][a-z0-9-]{0,23}$`)

func ValidateUnixIdentifier(value string) error {
	if unixIdentifier.MatchString(value) {
		return nil
	}
	return fmt.Errorf("must start with a lowercase letter and contain at most 24 lowercase letters, digits, or hyphens")
}

type Role string

const (
	RoleAdmin     Role = "admin"
	RoleDeveloper Role = "developer"
)

type Person struct {
	ID          string
	Username    string
	DisplayName string
	Email       string
	Role        Role
}

type SSHDeviceKey struct {
	ID               string
	PersonID         string
	Label            string
	PublicKey        string
	Fingerprint      string
	IdentityFileHint string
	CreatedAt        time.Time
}

type ProjectSource interface {
	isProjectSource()
}

type EmptyProjectSource struct{}

func (EmptyProjectSource) isProjectSource() {}

type GitProjectSource struct {
	RemoteURL string
}

func (GitProjectSource) isProjectSource() {}

type ToolchainProfile string

const (
	ToolchainWeb    ToolchainProfile = "web"
	ToolchainPython ToolchainProfile = "python"
	ToolchainRust   ToolchainProfile = "rust"
	ToolchainGo     ToolchainProfile = "go"
)

type Project struct {
	ID       string
	Slug     string
	Name     string
	UnixUser string
	Profile  ToolchainProfile
	Source   ProjectSource
}

type Membership struct {
	ProjectID string
	PersonID  string
}

type Worktree struct {
	ID        string
	ProjectID string
	PersonID  string
	Name      string
	Branch    string
	Path      string
}

type ProjectAccess struct {
	Person   Person
	Worktree Worktree
	Keys     []SSHDeviceKey
}

type JobState string

const (
	JobInstalling JobState = "installing"
	JobReady      JobState = "ready"
	JobFailed     JobState = "failed"
)

type ToolchainInstallation struct {
	ID       string
	Profile  ToolchainProfile
	Version  string
	Path     string
	Checksum string
	State    JobState
}

// ProjectEnvironment is the generated environment contract copied from a
// resolved toolchain installation into a project for SSH sessions.
type ProjectEnvironment struct {
	Profile   string            `json:"profile"`
	Path      []string          `json:"path"`
	Variables map[string]string `json:"variables,omitempty"`
}

type ProjectToolchainResolution struct {
	ProjectID               string
	ToolchainInstallationID string
}

type ProvisioningJob struct {
	ID        string
	ProjectID string
	State     JobState
	Error     *string
}

type DeployKey struct {
	ProjectID string
	PublicKey string
}

type RuntimeState string

const (
	RuntimeReady       RuntimeState = "ready"
	RuntimeDegraded    RuntimeState = "degraded"
	RuntimeUnavailable RuntimeState = "unavailable"
)

type ServiceStatus struct {
	Name  string
	State RuntimeState
}

type NetworkInterface struct {
	Name      string
	Addresses []string
}

type FilesystemStatus struct {
	Path           string
	TotalBytes     uint64
	AvailableBytes uint64
}

type HostHealth struct {
	Overall  RuntimeState
	Services []ServiceStatus
}

type HostNetwork struct {
	Interfaces []NetworkInterface
}

type FirewallStatus struct {
	SSHReady     bool
	CockpitReady bool
}

type HostResources struct {
	CPUPercent           *float64
	LoadAverage          [3]float64
	UptimeSeconds        uint64
	MemoryTotalBytes     uint64
	MemoryAvailableBytes uint64
	Filesystems          []FilesystemStatus
}

type HostStatus struct {
	SampledAt time.Time
	Health    HostHealth
	Network   HostNetwork
	Firewall  FirewallStatus
	Resources HostResources
}
