package domain

type ProjectSource interface {
	isProjectSource()
}

type EmptyProjectSource struct{}

func (EmptyProjectSource) isProjectSource() {}

type GitProjectSource struct {
	RemoteURL string
}

func (GitProjectSource) isProjectSource() {}

type Project struct {
	ID                string
	Slug              string
	Name              string
	UnixUser          string
	Profile           ToolchainProfile
	Source            ProjectSource
	BootstrapPersonID string
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

type DeployKey struct {
	ProjectID string
	PublicKey string
}
