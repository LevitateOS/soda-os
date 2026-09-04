package people

import (
	"testing"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/stretchr/testify/require"
)

func TestPrimaryAndAdministratorClassificationUsesOnlyLinuxFacts(t *testing.T) {
	primary := linuxhost.Account{
		Username: "alice", UID: 1000, Shell: "/bin/bash", Groups: map[string]bool{"alice": true},
	}
	require.True(t, IsPrimary(primary, 1000))
	require.False(t, IsAdministrator(primary, 1000))

	administrator := primary
	administrator.Groups = map[string]bool{"alice": true, linuxhost.AdministratorGroup: true}
	require.True(t, IsAdministrator(administrator, 1000))

	for name, account := range map[string]linuxhost.Account{
		"system UID":   {Username: "alice", UID: 999, Shell: "/bin/bash", Groups: map[string]bool{}},
		"invalid name": {Username: "Alice", UID: 1000, Shell: "/bin/bash", Groups: map[string]bool{}},
		"no login":     {Username: "alice", UID: 1000, Shell: "/usr/sbin/nologin", Groups: map[string]bool{}},
		"workspace":    {Username: "alice", UID: 1000, Shell: "/bin/bash", Groups: map[string]bool{"soda-workspaces": true}},
	} {
		t.Run(name, func(t *testing.T) {
			require.False(t, IsPrimary(account, 1000))
		})
	}
}

func TestValidateUsernameUsesTheEstablishedPrimaryAccountShape(t *testing.T) {
	for _, username := range []string{"alice", "a1", "team-admin"} {
		require.NoError(t, ValidateUsername(username))
	}
	for _, username := range []string{"", "Alice", "1alice", "alice_", "alice@example", "this-username-is-far-too-long"} {
		require.Error(t, ValidateUsername(username))
	}
}
