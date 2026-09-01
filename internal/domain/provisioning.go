package domain

type ToolchainProfile string

const (
	ToolchainWeb    ToolchainProfile = "web"
	ToolchainPython ToolchainProfile = "python"
	ToolchainRust   ToolchainProfile = "rust"
	ToolchainGo     ToolchainProfile = "go"
)

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

type ProjectEnvironment struct {
	Profile   string            `json:"profile"`
	Path      []string          `json:"path"`
	Variables map[string]string `json:"variables,omitempty"`
}
