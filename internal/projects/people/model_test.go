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
		"system UID": {Username: "alice", UID: 999, Shell: "/bin/bash", Groups: map[string]bool{}},
		"empty name": {Username: "", UID: 1000, Shell: "/bin/bash", Groups: map[string]bool{}},
		"no login":   {Username: "alice", UID: 1000, Shell: "/usr/sbin/nologin", Groups: map[string]bool{}},
		"workspace":  {Username: "alice", UID: 1000, Shell: "/bin/bash", Groups: map[string]bool{"soda-workspaces": true}},
	} {
		t.Run(name, func(t *testing.T) {
			require.False(t, IsPrimary(account, 1000))
		})
	}
}

func TestPrimaryClassificationAcceptsExistingLinuxUsername(t *testing.T) {
	account := linuxhost.Account{Username: "alice_dev", UID: 1000, Shell: "/bin/bash", Groups: map[string]bool{"alice_dev": true}}
	require.True(t, IsPrimary(account, 1000))
}
