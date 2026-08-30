package domain

type BuiltInGitUser struct {
	PersonID string
	UserID   int64
}

type BuiltInGitKey struct {
	SSHDeviceKeyID string
	PersonID       string
	KeyID          int64
}

type BuiltInGitIdentity struct {
	PersonID string
	KeyID    int64
}

type BuiltInGitRepository struct {
	ProjectID    string
	RepositoryID int64
	DeployKeyID  int64
	WebURL       string
	SSHURL       string
}
