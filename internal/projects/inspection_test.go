package projects

import (
	"context"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/LevitateOS/soda-os/internal/projects/catalog"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceInspectionUsesTheExistingHelperWithoutSetup(t *testing.T) {
	fixture := newCoordinatorFixture(t, "")
	response, err := fixture.coordinator.Execute(context.Background(), "alice", "inspect", strings.NewReader(`{"id":"site"}`))
	require.NoError(t, err)
	require.Equal(t, []string{"workspace-inspect"}, fixture.pkexec.actions(t))
	require.JSONEq(t, `{"id":"site"}`, fixture.pkexec.requests(t)[0])
	inspection := response.(WorkspaceInspectionResponse)
	require.True(t, inspection.OK)
	require.Equal(t, "soda-w-example", inspection.Workspace.Username)
	require.False(t, inspection.Workspace.Exists)
}

func TestWorkspaceInspectionCannotSelectAnotherHumanOrAnArbitraryPath(t *testing.T) {
	for _, input := range []string{`{"id":"site","username":"bob"}`, `{"id":"site","path":"/etc"}`, `{"id":"../site"}`} {
		t.Run(input, func(t *testing.T) {
			fixture := newCoordinatorFixture(t, "")
			_, err := fixture.coordinator.Execute(context.Background(), "alice", "inspect", strings.NewReader(input))
			require.Error(t, err)
			require.Empty(t, fixture.pkexec.actions(t))
			_, err = newHelperFixture(t).helper.Execute(context.Background(), helperAlice(), "workspace-inspect", strings.NewReader(input))
			require.Error(t, err)
		})
	}
}

func TestHelperInspectionWaitsForDestructiveOperations(t *testing.T) {
	fixture := newHelperFixture(t)
	addSite(t, &fixture)
	held, err := fixture.helper.operationLocks.Exclusive()
	require.NoError(t, err)
	assertHelperActionBlocks(t, fixture, held, helperLockCase{action: "workspace-inspect", request: `{"id":"site"}`}, "")
}

func TestHelperInspectionAllowsPrimaryHumansWithoutCreatingAWorkspace(t *testing.T) {
	fixture := newHelperFixture(t)
	alice := fixture.host.accounts["alice"]
	delete(alice.Groups, linuxhost.AdministratorGroup)
	fixture.host.accounts["alice"] = alice
	require.NoError(t, fixture.store.Add(catalog.Entry{ID: "site", DisplayName: "Site", CanonicalURL: "git@example.test:team/site.git"}))
	response, err := fixture.helper.Execute(context.Background(), helperAlice(), "workspace-inspect", strings.NewReader(`{"id":"site"}`))
	require.NoError(t, err)
	inspection := response.(WorkspaceInspectionResponse)
	require.True(t, inspection.OK)
	require.False(t, inspection.Workspace.Exists)
	require.Empty(t, inspection.Workspace.PrimaryKeyProblem)
	require.Len(t, fixture.host.accounts, 1)
	require.Empty(t, fixture.host.events)
	entry, err := fixture.store.Get("site")
	require.NoError(t, err)
	require.Equal(t, "git@example.test:team/site.git", entry.CanonicalURL)
}
