package projects

import (
	"context"
	"errors"
	"fmt"
	"io"
)

type fakePlatform struct {
	uidMin    int
	accounts  map[string]Account
	ready     map[string]bool
	keys      []byte
	published map[string]string
	deleteErr map[string]error
	failures  fakePlatformFailures
	calls     fakePlatformCalls
	onDelete  func(Account)
}

type fakePlatformFailures struct {
	fakeWorkspaceFailures
	fakeAccountFailures
	fakeMiseFailures
}

type fakeWorkspaceFailures struct {
	passwordErr  error
	setupLockErr error
	unlockErr    error
	installErr   error
	gitKeyErr    error
	cloneErr     error
}

type fakeAccountFailures struct {
	preflightErr error
	forgejoErr   error
}

type fakeMiseFailures struct {
	miseErr       error
	removeMiseErr error
}

type fakePlatformCalls struct {
	fakeWorkspaceCalls
	fakeAccountCalls
	fakeMiseCalls
}

type fakeWorkspaceCalls struct {
	keyReads       int
	passwordChecks []string
	locks          []string
	installedKeys  map[string][]byte
	gitKeys        []string
	clones         []string
}

type fakeAccountCalls struct {
	deleted        []string
	preflights     []string
	createdPrimary []string
	publishedHuman []string
	deletionEvents []string
}

type fakeMiseCalls struct {
	miseInstalls []string
	miseRemovals []string
}

type fakeSetupLock struct {
	platform *fakePlatform
	id       string
}

type primaryRole uint8

const (
	primaryRoleUser primaryRole = iota
	primaryRoleAdministrator
)

func newFakePlatform() *fakePlatform {
	return &fakePlatform{
		uidMin:    1000,
		accounts:  map[string]Account{},
		ready:     map[string]bool{},
		keys:      []byte("ssh-ed25519 AAAA test\n"),
		published: map[string]string{},
		deleteErr: map[string]error{},
		calls: fakePlatformCalls{
			installedKeys: map[string][]byte{},
		},
	}
}

func (platform *fakePlatform) UIDMin() (int, error) { return platform.uidMin, nil }

func (platform *fakePlatform) LookupAccount(_ context.Context, username string) (Account, error) {
	account, found := platform.accounts[username]
	if !found {
		return Account{}, fmt.Errorf("%w: %s", ErrAccountNotFound, username)
	}
	return account, nil
}

func (platform *fakePlatform) WorkspaceAccounts(context.Context) ([]Account, error) {
	accounts := []Account{}
	for _, account := range platform.accounts {
		if account.Groups[WorkspaceGroup] {
			accounts = append(accounts, account)
		}
	}
	return accounts, nil
}

func (platform *fakePlatform) ReadAuthorizedKeys(Account) ([]byte, error) {
	platform.calls.keyReads++
	if len(platform.keys) == 0 {
		return nil, errors.New("authorized_keys does not contain a public key")
	}
	return platform.keys, nil
}

func (platform *fakePlatform) WorkspaceOperationSharedLock() (io.Closer, error) {
	return platform.fakeLock("operations-shared")
}

func (platform *fakePlatform) WorkspaceOperationExclusiveLock() (io.Closer, error) {
	return platform.fakeLock("operations-exclusive")
}

func (platform *fakePlatform) fakeLock(id string) (io.Closer, error) {
	platform.calls.locks = append(platform.calls.locks, "lock:"+id)
	if platform.failures.setupLockErr != nil {
		return nil, platform.failures.setupLockErr
	}
	return &fakeSetupLock{platform: platform, id: id}, nil
}

func (platform *fakePlatform) SetupLock(_ Account, id string) (io.Closer, error) {
	return platform.fakeLock(id)
}

func (lock *fakeSetupLock) Close() error {
	lock.platform.calls.locks = append(lock.platform.calls.locks, "unlock:"+lock.id)
	return lock.platform.failures.unlockErr
}

func (platform *fakePlatform) WorkspaceReady(account Account, id string) (bool, error) {
	return platform.ready[account.Username+":"+id], nil
}

func (platform *fakePlatform) ValidatePasswordLocked(_ context.Context, account Account) error {
	platform.calls.passwordChecks = append(platform.calls.passwordChecks, account.Username)
	return platform.failures.passwordErr
}

func (platform *fakePlatform) CreateWorkspace(_ context.Context, primary Account, id string) (Account, error) {
	username, _ := DerivedUsername(primary.Username, id)
	marker, _ := WorkspaceMarker(primary.Username, id)
	account := Account{
		Username:     username,
		UID:          2000 + len(platform.accounts),
		GID:          2000 + len(platform.accounts),
		PrimaryGroup: username,
		GECOS:        marker,
		Home:         "/home/" + username,
		Shell:        WorkspaceShell,
		Groups:       map[string]bool{WorkspaceGroup: true},
	}
	platform.accounts[username] = account
	return account, nil
}

func (platform *fakePlatform) CreatePrimary(_ context.Context, username, _ string) (Account, error) {
	if account, found := platform.accounts[username]; found {
		return account, nil
	}
	account := primaryAccount(username, primaryRoleUser)
	account.UID = 1000 + len(platform.accounts)
	account.GID = account.UID
	platform.accounts[username] = account
	platform.calls.createdPrimary = append(platform.calls.createdPrimary, username)
	return account, nil
}

func (platform *fakePlatform) PublishHuman(_ context.Context, username string, _ []byte) error {
	platform.calls.publishedHuman = append(platform.calls.publishedHuman, username)
	return nil
}

func (platform *fakePlatform) InstallAuthorizedKeys(workspace Account, keys []byte) error {
	platform.calls.installedKeys[workspace.Username] = append([]byte(nil), keys...)
	if platform.failures.installErr != nil {
		return platform.failures.installErr
	}
	return nil
}

func (platform *fakePlatform) GenerateWorkspaceGitKey(_ context.Context, workspace Account) (string, error) {
	platform.calls.gitKeys = append(platform.calls.gitKeys, workspace.Username)
	if platform.failures.gitKeyErr != nil {
		return "", platform.failures.gitKeyErr
	}
	return "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKJpV7x5Ay34Nh0wiB89JgVG5ZrOxz2SeNUylLBzmrkS soda-workspace=" + workspace.Username, nil
}

func (platform *fakePlatform) CloneWorkspace(_ context.Context, workspace Account, id, remote string) error {
	platform.calls.clones = append(platform.calls.clones, workspace.Username+":"+id+":"+remote)
	if platform.failures.cloneErr != nil {
		return platform.failures.cloneErr
	}
	platform.ready[workspace.Username+":"+id] = true
	return nil
}

func (platform *fakePlatform) InstallMiseTools(_ context.Context, account Account, projectID string, workspaceTools, projectTools []string) error {
	platform.calls.miseInstalls = append(platform.calls.miseInstalls, fmt.Sprintf("%s:%s:workspace=%v:project=%v", account.Username, projectID, workspaceTools, projectTools))
	return platform.failures.miseErr
}

func (platform *fakePlatform) RemoveMiseProject(projectID string) error {
	platform.calls.miseRemovals = append(platform.calls.miseRemovals, projectID)
	return platform.failures.removeMiseErr
}

func (platform *fakePlatform) PreflightDeleteAccount(_ context.Context, account Account) error {
	platform.calls.preflights = append(platform.calls.preflights, account.Username)
	return platform.failures.preflightErr
}

func (platform *fakePlatform) DeleteForgejoUser(_ context.Context, username string) error {
	platform.calls.deletionEvents = append(platform.calls.deletionEvents, "forgejo:"+username)
	return platform.failures.forgejoErr
}

func (platform *fakePlatform) DeleteAccount(_ context.Context, account Account) error {
	if err := platform.deleteErr[account.Username]; err != nil {
		return err
	}
	if platform.onDelete != nil {
		platform.onDelete(account)
	}
	platform.calls.deleted = append(platform.calls.deleted, account.Username)
	platform.calls.deletionEvents = append(platform.calls.deletionEvents, "linux:"+account.Username)
	delete(platform.accounts, account.Username)
	delete(platform.published, account.Username)
	return nil
}

func primaryAccount(username string, role primaryRole) Account {
	groups := map[string]bool{}
	if role == primaryRoleAdministrator {
		groups["wheel"] = true
	}
	return Account{
		Username:     username,
		UID:          1000,
		GID:          1000,
		PrimaryGroup: username,
		Home:         "/home/" + username,
		Shell:        "/bin/bash",
		Groups:       groups,
	}
}
