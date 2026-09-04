package projects

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLifecyclePublishesOneDerivedWorkspace(t *testing.T) {
	catalog := testCatalog(t)
	entry := CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "https://git.example.test/alice/site.git"}
	require.NoError(t, catalog.Add(entry))
	platform := newFakePlatform()
	platform.accounts["alice"] = primaryAccount("alice", primaryRoleUser)
	lifecycle := Lifecycle{Catalog: catalog, Platform: platform}

	username, err := lifecycle.Publish(context.Background(), "alice", HelperWorkspaceRequest{ID: "site", CanonicalURL: entry.CanonicalURL})
	require.NoError(t, err)
	require.NotEmpty(t, username)
	require.True(t, platform.ready[username+":site"])

	again, err := lifecycle.Publish(context.Background(), "alice", HelperWorkspaceRequest{ID: "site", CanonicalURL: entry.CanonicalURL})
	require.NoError(t, err)
	require.Equal(t, username, again)
	require.Empty(t, platform.calls.deleted)
}

func TestLifecycleDoesNotCleanupAmbiguousFailedPublication(t *testing.T) {
	catalog := testCatalog(t)
	entry := CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "https://git.example.test/alice/site.git"}
	require.NoError(t, catalog.Add(entry))
	platform := newFakePlatform()
	platform.accounts["alice"] = primaryAccount("alice", primaryRoleUser)
	platform.failures.publishErr = errors.New("copy failed")
	platform.failures.unsafeCleanup = true
	lifecycle := Lifecycle{Catalog: catalog, Platform: platform}

	_, err := lifecycle.Publish(context.Background(), "alice", HelperWorkspaceRequest{ID: "site", CanonicalURL: entry.CanonicalURL})
	require.ErrorContains(t, err, "incomplete workspace was retained")
	require.Empty(t, platform.calls.deleted)
}

func TestLifecycleRetainsWorkspaceWhenAuthorizedKeysProvenanceIsAmbiguous(t *testing.T) {
	catalog := testCatalog(t)
	entry := CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "https://git.example.test/alice/site.git"}
	require.NoError(t, catalog.Add(entry))
	platform := newFakePlatform()
	platform.accounts["alice"] = primaryAccount("alice", primaryRoleUser)
	platform.failures.installErr = errors.Join(ErrAuthorizedKeysPublished, errors.New("authorized_keys already exists"))
	lifecycle := Lifecycle{Catalog: catalog, Platform: platform}

	_, err := lifecycle.Publish(context.Background(), "alice", HelperWorkspaceRequest{ID: entry.ID, CanonicalURL: entry.CanonicalURL})
	require.ErrorIs(t, err, ErrAuthorizedKeysPublished)
	require.ErrorContains(t, err, "workspace was retained")
	require.Empty(t, platform.calls.deleted)
}

func TestLifecycleRejectsUnlockedExistingWorkspaceBeforeReadiness(t *testing.T) {
	catalog := testCatalog(t)
	entry := CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "https://git.example.test/alice/site.git"}
	require.NoError(t, catalog.Add(entry))
	platform := newFakePlatform()
	platform.accounts["alice"] = primaryAccount("alice", primaryRoleUser)
	workspace, err := platform.CreateWorkspace(context.Background(), platform.accounts["alice"], entry.ID)
	require.NoError(t, err)
	platform.ready[workspace.Username+":"+entry.ID] = true
	platform.failures.passwordErr = errors.New("workspace password is not locked")
	lifecycle := Lifecycle{Catalog: catalog, Platform: platform}

	_, err = lifecycle.Publish(context.Background(), "alice", HelperWorkspaceRequest{ID: entry.ID, CanonicalURL: entry.CanonicalURL})
	require.ErrorContains(t, err, "password is not locked")
	require.Equal(t, []string{workspace.Username}, platform.calls.passwordChecks)
	require.Empty(t, platform.calls.deleted)
}

func TestLifecycleChecksExistingReadyWorkspaceUnderLockBeforeReadingKeys(t *testing.T) {
	catalog := testCatalog(t)
	entry := CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "https://git.example.test/alice/site.git"}
	require.NoError(t, catalog.Add(entry))
	platform := newFakePlatform()
	platform.accounts["alice"] = primaryAccount("alice", primaryRoleUser)
	workspace, err := platform.CreateWorkspace(context.Background(), platform.accounts["alice"], entry.ID)
	require.NoError(t, err)
	platform.ready[workspace.Username+":"+entry.ID] = true
	platform.keys = nil
	lifecycle := Lifecycle{Catalog: catalog, Platform: platform}

	username, err := lifecycle.Publish(context.Background(), "alice", HelperWorkspaceRequest{ID: entry.ID, CanonicalURL: entry.CanonicalURL})
	require.NoError(t, err)
	require.Equal(t, workspace.Username, username)
	require.Equal(t, []string{workspace.Username}, platform.calls.passwordChecks)
	require.Zero(t, platform.calls.keyReads)
}

func TestProjectRemovalDeletesWorkspacesBeforeCatalog(t *testing.T) {
	catalog := testCatalog(t)
	entry := CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "https://git.example.test/site.git"}
	require.NoError(t, catalog.Add(entry))
	platform := newFakePlatform()
	platform.accounts["alice"] = primaryAccount("alice", primaryRoleUser)
	for _, primary := range []string{"alice", "bob"} {
		workspace, err := platform.CreateWorkspace(context.Background(), primaryAccount(primary, primaryRoleUser), "site")
		require.NoError(t, err)
		platform.accounts[workspace.Username] = workspace
	}
	platform.onDelete = func(Account) {
		_, err := catalog.Get("site")
		require.NoError(t, err, "catalog entry must remain until every account is deleted")
	}
	lifecycle := Lifecycle{Catalog: catalog, Platform: platform}
	require.NoError(t, lifecycle.RemoveProject(context.Background(), "alice", "site"))
	require.Len(t, platform.calls.deleted, 2)
	_, err := catalog.Get("site")
	require.Error(t, err)
}

func TestProjectRemovalRetainsCatalogWhenWorkspacePasswordIsNotLocked(t *testing.T) {
	catalog := testCatalog(t)
	entry := CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "https://git.example.test/site.git"}
	require.NoError(t, catalog.Add(entry))
	platform := newFakePlatform()
	platform.accounts["alice"] = primaryAccount("alice", primaryRoleUser)
	workspace, err := platform.CreateWorkspace(context.Background(), platform.accounts["alice"], entry.ID)
	require.NoError(t, err)
	platform.failures.passwordErr = errors.New("workspace password is not locked")
	lifecycle := Lifecycle{Catalog: catalog, Platform: platform}

	err = lifecycle.RemoveProject(context.Background(), "alice", entry.ID)
	require.ErrorContains(t, err, "password is not locked")
	require.Equal(t, []string{workspace.Username}, platform.calls.passwordChecks)
	require.Empty(t, platform.calls.deleted)
	_, err = catalog.Get(entry.ID)
	require.NoError(t, err)
}

func TestHumanDeletionDeletesDerivedAccountsAndPrimaryLast(t *testing.T) {
	catalog := testCatalog(t)
	platform := newFakePlatform()
	platform.accounts["admin"] = primaryAccount("admin", primaryRoleAdministrator)
	platform.accounts["alice"] = primaryAccount("alice", primaryRoleUser)
	workspace, err := platform.CreateWorkspace(context.Background(), platform.accounts["alice"], "site")
	require.NoError(t, err)
	lifecycle := Lifecycle{Catalog: catalog, Platform: platform}

	require.NoError(t, lifecycle.DeleteHuman(context.Background(), "admin", "alice"))
	require.Equal(t, []string{workspace.Username, "alice"}, platform.calls.deleted)
	require.Equal(t, []string{"linux:" + workspace.Username, "forgejo:alice", "linux:alice"}, platform.calls.deletionEvents)
}

func TestHumanDeletionStopsAfterForgejoFailureAndReportsRemainingAccounts(t *testing.T) {
	catalog := testCatalog(t)
	platform := newFakePlatform()
	platform.accounts["admin"] = primaryAccount("admin", primaryRoleAdministrator)
	platform.accounts["alice"] = primaryAccount("alice", primaryRoleUser)
	workspace, err := platform.CreateWorkspace(context.Background(), platform.accounts["alice"], "site")
	require.NoError(t, err)
	platform.failures.forgejoErr = errors.New("Forgejo refuses users that own repositories")
	lifecycle := Lifecycle{Catalog: catalog, Platform: platform}

	err = lifecycle.DeleteHuman(context.Background(), "admin", "alice")
	require.ErrorContains(t, err, "removed Soda workspaces "+workspace.Username+"; Forgejo account and primary Linux account alice remain")
	require.Equal(t, []string{"linux:" + workspace.Username, "forgejo:alice"}, platform.calls.deletionEvents)
	require.Contains(t, platform.accounts, "alice")
}

func TestHumanDeletionRetriesAfterForgejoAccountWasAlreadyRemoved(t *testing.T) {
	platform := newFakePlatform()
	platform.accounts["admin"] = primaryAccount("admin", primaryRoleAdministrator)
	platform.accounts["alice"] = primaryAccount("alice", primaryRoleUser)
	platform.failures.forgejoErr = ErrForgejoUserNotFound
	lifecycle := Lifecycle{Catalog: testCatalog(t), Platform: platform}

	require.NoError(t, lifecycle.DeleteHuman(context.Background(), "admin", "alice"))
	require.Equal(t, []string{"forgejo:alice", "linux:alice"}, platform.calls.deletionEvents)
	require.NotContains(t, platform.accounts, "alice")
}

func TestHumanDeletionRetainsPrimaryWhenWorkspacePasswordIsNotLocked(t *testing.T) {
	catalog := testCatalog(t)
	platform := newFakePlatform()
	platform.accounts["admin"] = primaryAccount("admin", primaryRoleAdministrator)
	platform.accounts["alice"] = primaryAccount("alice", primaryRoleUser)
	workspace, err := platform.CreateWorkspace(context.Background(), platform.accounts["alice"], "site")
	require.NoError(t, err)
	platform.failures.passwordErr = errors.New("workspace password is not locked")
	lifecycle := Lifecycle{Catalog: catalog, Platform: platform}

	err = lifecycle.DeleteHuman(context.Background(), "admin", "alice")
	require.ErrorContains(t, err, "password is not locked")
	require.Equal(t, []string{workspace.Username}, platform.calls.passwordChecks)
	require.Empty(t, platform.calls.deleted)
	_, found := platform.accounts["alice"]
	require.True(t, found)
}

type orderedPreflightPlatform struct {
	*fakePlatform
	workspaces       []Account
	preflightFailure map[string]error
}

func (platform *orderedPreflightPlatform) WorkspaceAccounts(context.Context) ([]Account, error) {
	return append([]Account(nil), platform.workspaces...), nil
}

func (platform *orderedPreflightPlatform) PreflightDeleteAccount(_ context.Context, account Account) error {
	platform.calls.preflights = append(platform.calls.preflights, account.Username)
	return platform.preflightFailure[account.Username]
}

func TestProjectRemovalPreflightsEveryWorkspaceBeforeDeletingAny(t *testing.T) {
	catalog := testCatalog(t)
	entry := CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "https://git.example.test/site.git"}
	require.NoError(t, catalog.Add(entry))
	base := newFakePlatform()
	base.accounts["admin"] = primaryAccount("admin", primaryRoleAdministrator)
	first, err := base.CreateWorkspace(context.Background(), primaryAccount("alice", primaryRoleUser), entry.ID)
	require.NoError(t, err)
	second, err := base.CreateWorkspace(context.Background(), primaryAccount("bob", primaryRoleUser), entry.ID)
	require.NoError(t, err)
	platform := &orderedPreflightPlatform{
		fakePlatform:     base,
		workspaces:       []Account{first, second},
		preflightFailure: map[string]error{second.Username: errors.New("second workspace failed preflight")},
	}
	lifecycle := Lifecycle{Catalog: catalog, Platform: platform}

	err = lifecycle.RemoveProject(context.Background(), "admin", entry.ID)
	require.ErrorContains(t, err, "second workspace failed preflight")
	require.Equal(t, []string{first.Username, second.Username}, platform.calls.preflights)
	require.Empty(t, platform.calls.deleted)
	require.Contains(t, platform.accounts, first.Username)
	require.Contains(t, platform.accounts, second.Username)
	_, err = catalog.Get(entry.ID)
	require.NoError(t, err, "catalog must remain when any deletion preflight fails")
}

func TestHumanDeletionPreflightsPrimaryBeforeDeletingWorkspaces(t *testing.T) {
	base := newFakePlatform()
	base.accounts["admin"] = primaryAccount("admin", primaryRoleAdministrator)
	base.accounts["alice"] = primaryAccount("alice", primaryRoleUser)
	workspace, err := base.CreateWorkspace(context.Background(), base.accounts["alice"], "site")
	require.NoError(t, err)
	platform := &orderedPreflightPlatform{
		fakePlatform:     base,
		workspaces:       []Account{workspace},
		preflightFailure: map[string]error{"alice": errors.New("primary failed preflight")},
	}
	lifecycle := Lifecycle{Catalog: testCatalog(t), Platform: platform}

	err = lifecycle.DeleteHuman(context.Background(), "admin", "alice")
	require.ErrorContains(t, err, "primary failed preflight")
	require.Equal(t, []string{"alice"}, platform.calls.preflights)
	require.Empty(t, platform.calls.deleted)
	require.Contains(t, platform.accounts, workspace.Username)
	require.Contains(t, platform.accounts, "alice")
}

func TestHumanDeletionPreflightsEveryWorkspaceBeforeDeletingAnyAccount(t *testing.T) {
	base := newFakePlatform()
	base.accounts["admin"] = primaryAccount("admin", primaryRoleAdministrator)
	base.accounts["alice"] = primaryAccount("alice", primaryRoleUser)
	first, err := base.CreateWorkspace(context.Background(), base.accounts["alice"], "site")
	require.NoError(t, err)
	second, err := base.CreateWorkspace(context.Background(), base.accounts["alice"], "tools")
	require.NoError(t, err)
	platform := &orderedPreflightPlatform{
		fakePlatform:     base,
		workspaces:       []Account{first, second},
		preflightFailure: map[string]error{second.Username: errors.New("second workspace failed preflight")},
	}
	lifecycle := Lifecycle{Catalog: testCatalog(t), Platform: platform}

	err = lifecycle.DeleteHuman(context.Background(), "admin", "alice")
	require.ErrorContains(t, err, "second workspace failed preflight")
	require.Equal(t, []string{"alice", first.Username, second.Username}, platform.calls.preflights)
	require.Empty(t, platform.calls.deleted)
	require.Contains(t, platform.accounts, first.Username)
	require.Contains(t, platform.accounts, second.Username)
	require.Contains(t, platform.accounts, "alice")
}

type recordedCommand struct {
	directory  string
	name       string
	args       []string
	extraFiles int
}

type recordingRunner struct {
	calls []recordedCommand
	onRun func(string, string, []string, []*os.File) error
}

func (runner *recordingRunner) Run(_ context.Context, request Command) (CommandResult, error) {
	runner.calls = append(runner.calls, recordedCommand{
		directory:  request.Directory,
		name:       request.Name,
		args:       append([]string(nil), request.Args...),
		extraFiles: len(request.ExtraFiles),
	})
	if runner.onRun != nil {
		if err := runner.onRun(request.Directory, request.Name, request.Args, request.ExtraFiles); err != nil {
			return CommandResult{}, err
		}
	}
	return CommandResult{}, nil
}

func TestNativePublicationUsesValidatedStagingCWDAndLabelsBeforeRename(t *testing.T) {
	root := t.TempDir()
	primary := primaryAccount("alice", primaryRoleUser)
	primary.UID = os.Getuid()
	workspace := Account{Username: "soda-w-example", UID: os.Getuid(), GID: os.Getgid(), PrimaryGroup: "soda-w-example", Home: filepath.Join(root, "soda-w-example")}
	runner := &recordingRunner{}
	runner.onRun = func(_ string, name string, _ []string, _ []*os.File) error {
		if name == "/usr/sbin/runuser" {
			return os.Mkdir(filepath.Join(workspace.Home, "Projects", ".soda-site.tmp", ".git"), 0o705)
		}
		return nil
	}
	platform := &NativePlatform{Runner: runner, HomeRoot: root, RuntimeRoot: filepath.Join(root, "run")}
	staging := platform.StagingPath(primary, "site")
	require.NoError(t, os.MkdirAll(filepath.Join(staging, ".git"), 0o700))
	require.NoError(t, os.MkdirAll(workspace.Home, 0o700))
	require.NoError(t, platform.PrepareStaging(primary, "site"))

	require.NoError(t, platform.PublishWorkspace(context.Background(), primary, workspace, "site"))
	calls := runner.calls
	require.Len(t, calls, 2)
	require.Empty(t, calls[0].directory)
	require.Equal(t, 2, calls[0].extraFiles)
	require.Equal(t, []string{"--user", workspace.Username, "--", "/usr/bin/cp", "--archive", "--", "/proc/self/fd/3/.", "/proc/self/fd/4/"}, calls[0].args)
	require.Equal(t, "/usr/sbin/restorecon", calls[1].name, "relabeling must happen before descriptor-anchored rename")
	require.Equal(t, []string{"-R", filepath.Join(workspace.Home, "Projects", ".soda-site.tmp")}, calls[1].args)
	require.Zero(t, calls[1].extraFiles)
	_, err := os.Stat(staging)
	require.NoError(t, err, "the privileged publisher must not remove caller-owned staging")
	_, err = os.Stat(filepath.Join(workspace.Home, "Projects", "site", ".git"))
	require.NoError(t, err, "the validated temporary clone must be atomically published")
}

func TestNativePublicationRequiresGitDirectoriesAtBothBoundaries(t *testing.T) {
	root := t.TempDir()
	primary := primaryAccount("alice", primaryRoleUser)
	primary.UID = os.Getuid()
	workspace := Account{Username: "soda-w-example", UID: os.Getuid(), GID: os.Getgid(), PrimaryGroup: "soda-w-example", Home: filepath.Join(root, "soda-w-example")}
	runner := &recordingRunner{}
	platform := &NativePlatform{Runner: runner, HomeRoot: root, RuntimeRoot: filepath.Join(root, "run")}
	staging := platform.StagingPath(primary, "site")
	require.NoError(t, os.MkdirAll(staging, 0o700))
	require.NoError(t, os.MkdirAll(workspace.Home, 0o700))
	require.NoError(t, platform.PrepareStaging(primary, "site"))

	err := platform.PublishWorkspace(context.Background(), primary, workspace, "site")
	require.ErrorContains(t, err, "completed clone .git directory")
	require.Empty(t, runner.calls)

	require.NoError(t, os.Mkdir(filepath.Join(staging, ".git"), 0o705))
	require.NoError(t, platform.PrepareStaging(primary, "site"))
	runner.onRun = func(_ string, _ string, _ []string, _ []*os.File) error { return nil }
	err = platform.PublishWorkspace(context.Background(), primary, workspace, "site")
	require.ErrorContains(t, err, "temporary workspace .git directory")
	require.Len(t, runner.calls, 1, "restorecon must not run for a copied tree without .git")
}

func TestNativePublicationReservesTemporaryAndNeverReplacesDestination(t *testing.T) {
	root := t.TempDir()
	primary := primaryAccount("alice", primaryRoleUser)
	primary.UID = os.Getuid()
	workspace := Account{Username: "soda-w-example", UID: os.Getuid(), GID: os.Getgid(), PrimaryGroup: "soda-w-example", Home: filepath.Join(root, "soda-w-example")}
	runner := &recordingRunner{}
	platform := &NativePlatform{Runner: runner, HomeRoot: root, RuntimeRoot: filepath.Join(root, "run")}
	staging := platform.StagingPath(primary, "site")
	require.NoError(t, os.MkdirAll(filepath.Join(staging, ".git"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(workspace.Home, "Projects", ".soda-site.tmp"), 0o700))
	require.NoError(t, platform.PrepareStaging(primary, "site"))

	err := platform.PublishWorkspace(context.Background(), primary, workspace, "site")
	require.ErrorContains(t, err, "already exists")
	require.Empty(t, runner.calls)
	require.NoError(t, os.Remove(filepath.Join(workspace.Home, "Projects", ".soda-site.tmp")))

	runner.onRun = func(_ string, name string, _ []string, _ []*os.File) error {
		switch name {
		case "/usr/sbin/runuser":
			return os.Mkdir(filepath.Join(workspace.Home, "Projects", ".soda-site.tmp", ".git"), 0o705)
		case "/usr/sbin/restorecon":
			destination := filepath.Join(workspace.Home, "Projects", "site")
			if err := os.Mkdir(destination, 0o700); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(destination, "existing"), []byte("keep"), 0o600)
		}
		return nil
	}
	err = platform.PublishWorkspace(context.Background(), primary, workspace, "site")
	require.ErrorContains(t, err, "file exists")
	require.FileExists(t, filepath.Join(workspace.Home, "Projects", "site", "existing"))
	require.DirExists(t, filepath.Join(workspace.Home, "Projects", ".soda-site.tmp"))
}

func TestResetStagingRefusesUnexpectedOwnership(t *testing.T) {
	root := t.TempDir()
	platform := &NativePlatform{RuntimeRoot: root}
	primary := primaryAccount("alice", primaryRoleUser)
	primary.UID = os.Getuid() + 1
	staging := platform.StagingPath(primary, "site")
	require.NoError(t, os.MkdirAll(staging, 0o700))
	require.ErrorContains(t, platform.ResetStaging(primary, "site"), "unexpected ownership")
	_, err := os.Stat(staging)
	require.NoError(t, err, "ambiguous staging state must remain untouched")
}
