package workspace

import (
	"testing"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/stretchr/testify/require"
)

func TestDerivedAccountConvention(t *testing.T) {
	t.Parallel()
	username, err := DerivedUsername("alice", "website")
	require.NoError(t, err)
	require.Equal(t, "soda-w-9bf62dc7a4af46c08d5730ed", username)
	marker, err := Marker("alice", "website")
	require.NoError(t, err)
	require.Equal(t, "soda-workspace=alice/website", marker)
	primary, project, err := ParseMarker(marker)
	require.NoError(t, err)
	require.Equal(t, "alice", primary)
	require.Equal(t, "website", project)
}

func TestDerivedUsernameAcceptsLinuxOwnedPrimaryName(t *testing.T) {
	t.Parallel()
	username, err := DerivedUsername("alice_dev", "website")
	require.NoError(t, err)
	require.Regexp(t, `^soda-w-[0-9a-f]{24}$`, username)
}

func TestValidateAccountRequiresTheExactLinuxWorkspaceEvidence(t *testing.T) {
	t.Parallel()
	account := workspaceAccount(t, "alice", "website", 1001)
	require.NoError(t, ValidateAccount(account, "alice", "website", 1000))

	account.Groups[linuxhost.AdministratorGroup] = true
	require.ErrorContains(t, ValidateAccount(account, "alice", "website", 1000), "must not be an administrator")
	delete(account.Groups, linuxhost.AdministratorGroup)
	account.GECOS = "soda-workspace=alice/other"
	require.ErrorContains(t, ValidateAccount(account, "alice", "website", 1000), "marker does not match")
}

func workspaceAccount(t *testing.T, primary, project string, uid int) linuxhost.Account {
	t.Helper()
	username, err := DerivedUsername(primary, project)
	require.NoError(t, err)
	marker, err := Marker(primary, project)
	require.NoError(t, err)
	return linuxhost.Account{
		Username:     username,
		UID:          uid,
		GID:          uid,
		PrimaryGroup: username,
		GECOS:        marker,
		Home:         "/home/" + username,
		Shell:        Shell,
		Groups:       map[string]bool{Group: true},
	}
}
