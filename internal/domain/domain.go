package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

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

type HostStatus struct {
	SampledAt            time.Time
	Overall              RuntimeState
	Services             []ServiceStatus
	SSHFirewallReady     bool
	CockpitFirewallReady bool
	Interfaces           []NetworkInterface
	CPUPercent           *float64
	LoadAverage          [3]float64
	UptimeSeconds        uint64
	MemoryTotalBytes     uint64
	MemoryAvailableBytes uint64
	Filesystems          []FilesystemStatus
	SSHObserver          RuntimeState
	GitObserver          RuntimeState
}

type WorktreeState string

const (
	WorktreeClean       WorktreeState = "clean"
	WorktreeDirty       WorktreeState = "dirty"
	WorktreeUnavailable WorktreeState = "unavailable"
)

type WorktreeStatus struct {
	WorktreeID  string
	Branch      string
	ShortCommit string
	Upstream    *string
	Ahead       uint64
	Behind      uint64
	Staged      uint64
	Modified    uint64
	Untracked   uint64
	Conflicted  uint64
	State       WorktreeState
	Error       *string
}

type SSHChannelKind string

const (
	SSHChannelInteractive SSHChannelKind = "interactive"
	SSHChannelCommand     SSHChannelKind = "command"
	SSHChannelSFTP        SSHChannelKind = "sftp"
)

type SSHChannel struct {
	Kind       SSHChannelKind
	WorktreeID string
}

type ActiveSSHConnection struct {
	ID            string
	ProjectID     string
	PersonID      string
	ConnectedAt   time.Time
	ClientAddress string
	ClientPort    uint16
	ServerAddress string
	ServerPort    uint16
	Channels      []SSHChannel
}

type EventKind string

const (
	EventHostChanged         EventKind = "host_changed"
	EventPeopleChanged       EventKind = "people_changed"
	EventProjectsChanged     EventKind = "projects_changed"
	EventWorktreesChanged    EventKind = "worktrees_changed"
	EventProvisioningChanged EventKind = "provisioning_changed"
	EventGitChanged          EventKind = "git_changed"
	EventSessionsChanged     EventKind = "sessions_changed"
	EventAccessChanged       EventKind = "access_changed"
)

type Event struct {
	Kind      EventKind
	ProjectID *string
	Sequence  uint64
}

func ParseID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid UUID %q: %w", value, err)
	}
	return id, nil
}
