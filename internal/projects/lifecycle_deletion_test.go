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

	err = (Lifecycle{Catalog: testCatalog(t), Platform: platform}).DeleteHuman(context.Background(), "admin", "alice")
	require.ErrorContains(t, err, "removed Soda workspaces "+removed.Username+"; workspace "+retained.Username+", Forgejo account, and primary Linux account remain")
	require.Equal(t, []string{"linux:" + removed.Username}, platform.calls.deletionEvents)
	require.Contains(t, platform.accounts, "alice")
}
