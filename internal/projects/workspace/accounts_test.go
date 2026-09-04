package workspace

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/LevitateOS/soda-os/internal/projects/catalog"
	"github.com/stretchr/testify/require"
)

type fakeAccountHost struct {
	uidMin        int
	accounts      map[string]linuxhost.Account
	candidates    []linuxhost.Account
	keys          []byte
	installedKeys map[string][]byte
	passwords     map[string]linuxhost.PasswordStatus
	failures      fakeAccountFailures
	calls         fakeAccountCalls
}

type fakeAccountFailures struct {
	password  error
	install   error
	preflight map[string]error
	deletion  map[string]error
}

type fakeAccountCalls struct {
	keyReads   int
	preflights []string
	deleted    []string
}

func newFakeAccountHost() *fakeAccountHost {
	return &fakeAccountHost{
		uidMin:        1000,
		accounts:      map[string]linuxhost.Account{},
		keys:          []byte("ssh-ed25519 AAAA test\n"),
		installedKeys: map[string][]byte{},
		passwords:     map[string]linuxhost.PasswordStatus{},
		failures: fakeAccountFailures{
			preflight: map[string]error{},
			deletion:  map[string]error{},
		},
	}
}

func (host *fakeAccountHost) UIDMin() (int, error) { return host.uidMin, nil }

func (host *fakeAccountHost) LookupAccount(_ context.Context, username string) (linuxhost.Account, error) {
	account, found := host.accounts[username]
	if !found {
		return linuxhost.Account{}, fmt.Errorf("%w: %s", linuxhost.ErrAccountNotFound, username)
	}
	return account, nil
}

func (host *fakeAccountHost) CandidateAccounts(context.Context, string, string) ([]linuxhost.Account, error) {
	return append([]linuxhost.Account(nil), host.candidates...), nil
}

func (host *fakeAccountHost) PasswordStatus(_ context.Context, account linuxhost.Account) (linuxhost.PasswordStatus, error) {
	if host.failures.password != nil {
		return 0, host.failures.password
	}
	if status := host.passwords[account.Username]; status != 0 {
		return status, nil
	}
	return linuxhost.PasswordLocked, nil
}

func (host *fakeAccountHost) ReadAuthorizedKeys(linuxhost.Account) ([]byte, error) {
	host.calls.keyReads++
	if host.keys == nil {
		return nil, errors.New("authorized_keys does not contain a public key")
	}
	return append([]byte(nil), host.keys...), nil
}

func (host *fakeAccountHost) InstallAuthorizedKeys(account linuxhost.Account, keys []byte) error {
	host.installedKeys[account.Username] = append([]byte(nil), keys...)
	return host.failures.install
}

func (host *fakeAccountHost) PreflightDeleteAccount(_ context.Context, account linuxhost.Account) error {
	host.calls.preflights = append(host.calls.preflights, account.Username)
	return host.failures.preflight[account.Username]
}

func (host *fakeAccountHost) DeleteAccount(_ context.Context, account linuxhost.Account) error {
	if err := host.failures.deletion[account.Username]; err != nil {
		return err
	}
	host.calls.deleted = append(host.calls.deleted, account.Username)
	delete(host.accounts, account.Username)
	return nil
}

type commandRunnerFunc func(context.Context, linuxhost.Command) (linuxhost.CommandResult, error)

func (run commandRunnerFunc) Run(ctx context.Context, command linuxhost.Command) (linuxhost.CommandResult, error) {
	return run(ctx, command)
}

type fakeRepository struct {
	publicKey  string
	keyErr     error
	cloned     map[string]bool
	publishErr error
	keyCalls   []string
	clones     []string
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		publicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKJpV7x5Ay34Nh0wiB89JgVG5ZrOxz2SeNUylLBzmrkS",
		cloned:    map[string]bool{},
	}
}

func (repository *fakeRepository) GenerateOutboundKey(_ context.Context, account linuxhost.Account) (string, error) {
	repository.keyCalls = append(repository.keyCalls, account.Username)
	return repository.publicKey, repository.keyErr
}

func (repository *fakeRepository) CloneExists(account linuxhost.Account, entry catalog.Entry) (bool, error) {
	return repository.cloned[account.Username+":"+entry.ID], nil
}

func (repository *fakeRepository) Publish(_ context.Context, account linuxhost.Account, entry catalog.Entry) error {
	repository.clones = append(repository.clones, account.Username+":"+entry.ID+":"+entry.CanonicalURL)
	if repository.publishErr != nil {
		return repository.publishErr
	}
	repository.cloned[account.Username+":"+entry.ID] = true
	return nil
}

func TestAccountsPrepareCreatesRepresentableLinuxStateAndCopiesKeysOnce(t *testing.T) {
	host := newFakeAccountHost()
	primary := primaryAccount("alice")
	host.accounts[primary.Username] = primary
	entry := projectEntry("site")
	var calls []linuxhost.Command
	runner := commandRunnerFunc(func(_ context.Context, command linuxhost.Command) (linuxhost.CommandResult, error) {
		calls = append(calls, command)
		username, err := DerivedUsername(primary.Username, entry.ID)
		require.NoError(t, err)
		host.accounts[username] = workspaceAccount(t, primary.Username, entry.ID, 2000)
		return linuxhost.CommandResult{}, nil
	})
	accounts := NewAccounts(host, host, host, runner)
	repository := newFakeRepository()

	prepared, err := accounts.Prepare(context.Background(), repository, primary, entry)
	require.NoError(t, err)
	require.NotEmpty(t, prepared.PublicKey)
	require.Len(t, calls, 1)
	require.Equal(t, "/usr/sbin/useradd", calls[0].Name)
	require.Equal(t, []string{
		"--create-home", "--user-group", "--groups", Group, "--shell", Shell,
		"--home-dir", "/home/" + prepared.Username, "--comment", "soda-workspace=alice/site", "--", prepared.Username,
	}, calls[0].Args)
	require.NotContains(t, calls[0].Args, "--password")
	require.Equal(t, host.keys, host.installedKeys[prepared.Username])
	require.Equal(t, 1, host.calls.keyReads)

	again, err := accounts.Prepare(context.Background(), repository, primary, entry)
	require.NoError(t, err)
	require.Equal(t, prepared.Username, again.Username)
	require.Len(t, calls, 1)
	require.Equal(t, 1, host.calls.keyReads, "an existing workspace must retain its copied inbound keys")
}

func TestAssociationReportsOnlyTheDerivedLinuxAccount(t *testing.T) {
	host := newFakeAccountHost()
	primary := primaryAccount("alice")
	entry := projectEntry("site")
	accounts := NewAccounts(host, host, host, commandRunnerFunc(func(context.Context, linuxhost.Command) (linuxhost.CommandResult, error) {
		return linuxhost.CommandResult{}, errors.New("association must not run a command")
	}))

	username, exists, err := accounts.Association(context.Background(), primary, entry)
	require.NoError(t, err)
	require.False(t, exists)
	host.accounts[username] = workspaceAccount(t, primary.Username, entry.ID, 2000)
	username, exists, err = accounts.Association(context.Background(), primary, entry)
	require.NoError(t, err)
	require.True(t, exists)
	require.NotEmpty(t, username)
}

func TestAccountsRetainsCreatedWorkspaceWhenInboundKeyPublicationIsAmbiguous(t *testing.T) {
	host := newFakeAccountHost()
	primary := primaryAccount("alice")
	host.failures.install = errors.Join(linuxhost.ErrAuthorizedKeysPublished, errors.New("authorized_keys already exists"))
	entry := projectEntry("site")
	runner := commandRunnerFunc(func(_ context.Context, _ linuxhost.Command) (linuxhost.CommandResult, error) {
		workspace := workspaceAccount(t, primary.Username, entry.ID, 2000)
		host.accounts[workspace.Username] = workspace
		return linuxhost.CommandResult{}, nil
	})

	_, err := NewAccounts(host, host, host, runner).Prepare(context.Background(), newFakeRepository(), primary, entry)
	require.ErrorIs(t, err, linuxhost.ErrAuthorizedKeysPublished)
	require.ErrorContains(t, err, "was retained because inbound SSH keys may be incomplete")
	username, _ := DerivedUsername(primary.Username, entry.ID)
	require.Contains(t, host.accounts, username)
}

func TestAccountsRejectsAnUnlockedExistingWorkspace(t *testing.T) {
	host := newFakeAccountHost()
	primary := primaryAccount("alice")
	entry := projectEntry("site")
	workspace := workspaceAccount(t, primary.Username, entry.ID, 2000)
	host.accounts[workspace.Username] = workspace
	host.passwords[workspace.Username] = linuxhost.PasswordSet
	accounts := NewAccounts(host, host, host, commandRunnerFunc(func(context.Context, linuxhost.Command) (linuxhost.CommandResult, error) {
		return linuxhost.CommandResult{}, errors.New("must not create an existing workspace")
	}))

	_, err := accounts.Prepare(context.Background(), newFakeRepository(), primary, entry)
	require.ErrorContains(t, err, "does not have a locked password")
	require.Zero(t, host.calls.keyReads)
}

func TestAccountsPublishRetainsNativeFactsForRetry(t *testing.T) {
	host := newFakeAccountHost()
	primary := primaryAccount("alice")
	entry := projectEntry("site")
	workspace := workspaceAccount(t, primary.Username, entry.ID, 2000)
	host.accounts[workspace.Username] = workspace
	accounts := NewAccounts(host, host, host, commandRunnerFunc(func(context.Context, linuxhost.Command) (linuxhost.CommandResult, error) {
		return linuxhost.CommandResult{}, errors.New("unexpected useradd")
	}))
	repository := newFakeRepository()
	repository.publishErr = errors.New("native SSH authentication failed")

	_, err := accounts.Publish(context.Background(), repository, primary, entry)
	require.ErrorContains(t, err, "SSH keys, and outbound Git key were retained; clone can be retried")
	require.Contains(t, host.accounts, workspace.Username)

	repository.publishErr = nil
	username, err := accounts.Publish(context.Background(), repository, primary, entry)
	require.NoError(t, err)
	require.Equal(t, workspace.Username, username)
	require.Len(t, repository.clones, 2)
	username, err = accounts.Publish(context.Background(), repository, primary, entry)
	require.NoError(t, err)
	require.Len(t, repository.clones, 2, "an already complete clone must not be replaced")
}

func primaryAccount(username string) linuxhost.Account {
	return linuxhost.Account{
		Username:     username,
		UID:          1000,
		GID:          1000,
		PrimaryGroup: username,
		Home:         "/home/" + username,
		Shell:        "/bin/bash",
		Groups:       map[string]bool{},
	}
}

func projectEntry(id string) catalog.Entry {
	return catalog.Entry{ID: id, DisplayName: "Project " + id, CanonicalURL: "git@git.example.test:team/" + id + ".git"}
}
