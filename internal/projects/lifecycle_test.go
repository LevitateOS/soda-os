package projects

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLifecyclePreparesAndCompletesOneDerivedWorkspace(t *testing.T) {
	catalog := testCatalog(t)
	entry := CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:alice/site.git"}
	require.NoError(t, catalog.Add(entry))
	platform := newFakePlatform()
	platform.accounts["alice"] = primaryAccount("alice", primaryRoleUser)
	lifecycle := Lifecycle{Catalog: catalog, Platform: platform}

	preparation, err := lifecycle.PrepareWorkspace(context.Background(), "alice", HelperWorkspaceRequest{ID: "site", CanonicalURL: entry.CanonicalURL})
	require.NoError(t, err)
	require.NotEmpty(t, preparation.Username)
	require.NotEmpty(t, preparation.PublicKey)
	username, err := lifecycle.CompleteWorkspace(context.Background(), "alice", HelperWorkspaceRequest{ID: "site", CanonicalURL: entry.CanonicalURL})
	require.NoError(t, err)
	require.Equal(t, preparation.Username, username)
	require.True(t, platform.ready[preparation.Username+":site"])

	againPreparation, err := lifecycle.PrepareWorkspace(context.Background(), "alice", HelperWorkspaceRequest{ID: "site", CanonicalURL: entry.CanonicalURL})
	require.NoError(t, err)
	require.Equal(t, username, againPreparation.Username)
	require.Empty(t, platform.calls.deleted)
}

func TestLifecycleRetainsPreparedWorkspaceWhenCloneFails(t *testing.T) {
	catalog := testCatalog(t)
	entry := CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:alice/site.git"}
	require.NoError(t, catalog.Add(entry))
	platform := newFakePlatform()
	platform.accounts["alice"] = primaryAccount("alice", primaryRoleUser)
	platform.failures.cloneErr = errors.New("clone failed")
	lifecycle := Lifecycle{Catalog: catalog, Platform: platform}

	request := HelperWorkspaceRequest{ID: "site", CanonicalURL: entry.CanonicalURL}
	_, err := lifecycle.PrepareWorkspace(context.Background(), "alice", request)
	require.NoError(t, err)
	_, err = lifecycle.CompleteWorkspace(context.Background(), "alice", request)
	require.ErrorContains(t, err, "outbound Git key were retained; clone can be retried")
	require.Empty(t, platform.calls.deleted)
}

func TestLifecycleRetainsWorkspaceWhenAuthorizedKeysProvenanceIsAmbiguous(t *testing.T) {
	catalog := testCatalog(t)
	entry := CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:alice/site.git"}
	require.NoError(t, catalog.Add(entry))
	platform := newFakePlatform()
	platform.accounts["alice"] = primaryAccount("alice", primaryRoleUser)
	platform.failures.installErr = errors.Join(ErrAuthorizedKeysPublished, errors.New("authorized_keys already exists"))
	lifecycle := Lifecycle{Catalog: catalog, Platform: platform}

	_, err := lifecycle.PrepareWorkspace(context.Background(), "alice", HelperWorkspaceRequest{ID: entry.ID, CanonicalURL: entry.CanonicalURL})
	require.ErrorIs(t, err, ErrAuthorizedKeysPublished)
	require.ErrorContains(t, err, "was retained because inbound SSH keys may be incomplete")
	require.Empty(t, platform.calls.deleted)
}

func TestLifecycleRejectsUnlockedExistingWorkspaceBeforePreparation(t *testing.T) {
	catalog := testCatalog(t)
	entry := CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:alice/site.git"}
	require.NoError(t, catalog.Add(entry))
	platform := newFakePlatform()
	platform.accounts["alice"] = primaryAccount("alice", primaryRoleUser)
	workspace, err := platform.CreateWorkspace(context.Background(), platform.accounts["alice"], entry.ID)
	require.NoError(t, err)
	platform.ready[workspace.Username+":"+entry.ID] = true
	platform.failures.passwordErr = errors.New("workspace password is not locked")
	lifecycle := Lifecycle{Catalog: catalog, Platform: platform}

	_, err = lifecycle.PrepareWorkspace(context.Background(), "alice", HelperWorkspaceRequest{ID: entry.ID, CanonicalURL: entry.CanonicalURL})
	require.ErrorContains(t, err, "password is not locked")
	require.Equal(t, []string{workspace.Username}, platform.calls.passwordChecks)
	require.Empty(t, platform.calls.deleted)
}

func TestLifecycleChecksExistingWorkspaceUnderLockBeforeReadingKeys(t *testing.T) {
	catalog := testCatalog(t)
	entry := CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:alice/site.git"}
	require.NoError(t, catalog.Add(entry))
	platform := newFakePlatform()
	platform.accounts["alice"] = primaryAccount("alice", primaryRoleUser)
	workspace, err := platform.CreateWorkspace(context.Background(), platform.accounts["alice"], entry.ID)
	require.NoError(t, err)
	platform.ready[workspace.Username+":"+entry.ID] = true
	platform.keys = nil
	lifecycle := Lifecycle{Catalog: catalog, Platform: platform}

	preparation, err := lifecycle.PrepareWorkspace(context.Background(), "alice", HelperWorkspaceRequest{ID: entry.ID, CanonicalURL: entry.CanonicalURL})
	require.NoError(t, err)
	require.Equal(t, workspace.Username, preparation.Username)
	require.Equal(t, []string{workspace.Username}, platform.calls.passwordChecks)
	require.Zero(t, platform.calls.keyReads)
}

func TestProjectRemovalDeletesWorkspacesBeforeCatalog(t *testing.T) {
	catalog := testCatalog(t)
	entry := CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:site.git"}
	require.NoError(t, catalog.Add(entry))
	platform := newFakePlatform()
	platform.accounts["alice"] = primaryAccount("alice", primaryRoleAdministrator)
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
	entry := CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:site.git"}
	require.NoError(t, catalog.Add(entry))
	platform := newFakePlatform()
	platform.accounts["alice"] = primaryAccount("alice", primaryRoleAdministrator)
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

func TestProjectRemovalRequiresAdministratorBeforeMutation(t *testing.T) {
	catalog := testCatalog(t)
	entry := CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:site.git"}
	require.NoError(t, catalog.Add(entry))
	platform := newFakePlatform()
	platform.accounts["alice"] = primaryAccount("alice", primaryRoleUser)
	lifecycle := Lifecycle{Catalog: catalog, Platform: platform}

	err := lifecycle.RemoveProject(context.Background(), "alice", entry.ID)
	require.ErrorContains(t, err, "administrator status is required")
	_, err = catalog.Get(entry.ID)
	require.NoError(t, err)
}

func TestPersonRemovesOnlyTheirOwnWorkspace(t *testing.T) {
	catalog := testCatalog(t)
	entry := CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:site.git"}
	require.NoError(t, catalog.Add(entry))
	platform := newFakePlatform()
	platform.accounts["alice"] = primaryAccount("alice", primaryRoleUser)
	platform.accounts["bob"] = primaryAccount("bob", primaryRoleUser)
	aliceWorkspace, err := platform.CreateWorkspace(context.Background(), platform.accounts["alice"], entry.ID)
	require.NoError(t, err)
	bobWorkspace, err := platform.CreateWorkspace(context.Background(), platform.accounts["bob"], entry.ID)
	require.NoError(t, err)
	lifecycle := Lifecycle{Catalog: catalog, Platform: platform}

	require.NoError(t, lifecycle.RemoveWorkspace(context.Background(), "alice", entry.ID))
	require.Equal(t, []string{aliceWorkspace.Username}, platform.calls.deleted)
	require.Contains(t, platform.accounts, bobWorkspace.Username)
	_, err = catalog.Get(entry.ID)
	require.NoError(t, err)
	require.NoError(t, lifecycle.RemoveWorkspace(context.Background(), "alice", entry.ID), "retry after success is idempotent")
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
	entry := CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:site.git"}
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
