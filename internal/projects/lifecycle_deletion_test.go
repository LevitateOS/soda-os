package projects

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHumanDeletionReportsEveryRetainedIdentityAfterWorkspaceFailure(t *testing.T) {
	platform := newFakePlatform()
	platform.accounts["admin"] = primaryAccount("admin", primaryRoleAdministrator)
	platform.accounts["alice"] = primaryAccount("alice", primaryRoleUser)
	first, err := platform.CreateWorkspace(context.Background(), platform.accounts["alice"], "first")
	require.NoError(t, err)
	second, err := platform.CreateWorkspace(context.Background(), platform.accounts["alice"], "second")
	require.NoError(t, err)
	removed, retained := first, second
	if retained.Username < removed.Username {
		removed, retained = retained, removed
	}
	platform.deleteErr[retained.Username] = errors.New("workspace process cannot terminate")

	err = (Lifecycle{Catalog: testCatalog(t), Host: platform, Platform: platform}).DeleteHuman(context.Background(), "admin", "alice")
	require.ErrorContains(t, err, "removed Soda workspaces "+removed.Username+"; workspace "+retained.Username+", Forgejo account, and primary Linux account remain")
	require.Equal(t, []string{"linux:" + removed.Username}, platform.calls.deletionEvents)
	require.Contains(t, platform.accounts, "alice")
}

func TestProjectRemovalReportsPartialProgressAndSupportsRetry(t *testing.T) {
	catalog := testCatalog(t)
	entry := CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:site.git"}
	require.NoError(t, catalog.Add(entry))
	platform := newFakePlatform()
	platform.accounts["admin"] = primaryAccount("admin", primaryRoleAdministrator)
	first, err := platform.CreateWorkspace(context.Background(), primaryAccount("alice", primaryRoleUser), entry.ID)
	require.NoError(t, err)
	second, err := platform.CreateWorkspace(context.Background(), primaryAccount("bob", primaryRoleUser), entry.ID)
	require.NoError(t, err)
	removed, retained := first, second
	if retained.Username < removed.Username {
		removed, retained = retained, removed
	}
	platform.deleteErr[retained.Username] = errors.New("workspace process cannot terminate")
	lifecycle := Lifecycle{Catalog: catalog, Host: platform, Platform: platform}

	err = lifecycle.RemoveProject(context.Background(), "admin", entry.ID)
	require.ErrorContains(t, err, "removed local workspaces "+removed.Username)
	require.ErrorContains(t, err, "local workspaces "+retained.Username+", shared catalog entry, and canonical repository remain")
	_, err = catalog.Get(entry.ID)
	require.NoError(t, err)

	delete(platform.deleteErr, retained.Username)
	require.NoError(t, lifecycle.RemoveProject(context.Background(), "admin", entry.ID))
	require.NotContains(t, platform.accounts, retained.Username)
	_, err = catalog.Get(entry.ID)
	require.Error(t, err)
}

func TestOwnWorkspaceRemovalReportsEverythingRetainedOnFailure(t *testing.T) {
	catalog := testCatalog(t)
	entry := CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:site.git"}
	require.NoError(t, catalog.Add(entry))
	platform := newFakePlatform()
	platform.accounts["alice"] = primaryAccount("alice", primaryRoleUser)
	workspace, err := platform.CreateWorkspace(context.Background(), platform.accounts["alice"], entry.ID)
	require.NoError(t, err)
	platform.deleteErr[workspace.Username] = errors.New("workspace process cannot terminate")

	err = (Lifecycle{Catalog: catalog, Host: platform, Platform: platform}).RemoveWorkspace(context.Background(), "alice", entry.ID)
	require.ErrorContains(t, err, "no workspace was removed; workspace "+workspace.Username+", shared catalog entry, other local workspaces, and canonical repository remain")
	require.Contains(t, platform.accounts, workspace.Username)
}
