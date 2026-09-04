package runners

import (
	"context"
	"errors"
	"testing"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/stretchr/testify/require"
)

type fakeLinuxAccounts struct {
	uidMin     int
	uidMinErr  error
	account    linuxhost.Account
	accountErr error
}

func (accounts fakeLinuxAccounts) UIDMin() (int, error) {
	return accounts.uidMin, accounts.uidMinErr
}

func (accounts fakeLinuxAccounts) LookupAccount(context.Context, string) (linuxhost.Account, error) {
	return accounts.account, accounts.accountErr
}

func TestLinuxAuthorizerAcceptsOnlyResolvedRegularInteractiveAdministrator(t *testing.T) {
	actor := linuxhost.PKExecIdentity{Username: "alice", UID: 1000}
	administrator := linuxhost.Account{
		Username: "alice", UID: 1000, Shell: "/bin/bash", Groups: map[string]bool{linuxhost.AdministratorGroup: true},
	}
	tests := map[string]struct {
		account linuxhost.Account
		actor   linuxhost.PKExecIdentity
	}{
		"different username": {account: linuxhost.Account{Username: "bob", UID: 1000, Shell: "/bin/bash", Groups: administrator.Groups}, actor: actor},
		"different UID":      {account: linuxhost.Account{Username: "alice", UID: 1001, Shell: "/bin/bash", Groups: administrator.Groups}, actor: actor},
		"system account":     {account: linuxhost.Account{Username: "alice", UID: 999, Shell: "/bin/bash", Groups: administrator.Groups}, actor: actor},
		"noninteractive":     {account: linuxhost.Account{Username: "alice", UID: 1000, Shell: "/usr/sbin/nologin", Groups: administrator.Groups}, actor: actor},
		"not administrator":  {account: linuxhost.Account{Username: "alice", UID: 1000, Shell: "/bin/bash", Groups: map[string]bool{}}, actor: actor},
	}

	authorizer := LinuxAuthorizer{Accounts: fakeLinuxAccounts{uidMin: 1000, account: administrator}}
	require.NoError(t, authorizer.RequireAdministrator(context.Background(), actor))
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			authorizer := LinuxAuthorizer{Accounts: fakeLinuxAccounts{uidMin: 1000, account: test.account}}
			require.Error(t, authorizer.RequireAdministrator(context.Background(), test.actor))
		})
	}
}

func TestLinuxAuthorizerPropagatesNativeLinuxEvidenceFailures(t *testing.T) {
	actor := linuxhost.PKExecIdentity{Username: "alice", UID: 1000}
	authorizer := LinuxAuthorizer{Accounts: fakeLinuxAccounts{uidMinErr: errors.New("UID_MIN unavailable")}}
	require.ErrorContains(t, authorizer.RequireAdministrator(context.Background(), actor), "UID_MIN unavailable")

	authorizer = LinuxAuthorizer{Accounts: fakeLinuxAccounts{uidMin: 1000, accountErr: errors.New("account unavailable")}}
	require.ErrorContains(t, authorizer.RequireAdministrator(context.Background(), actor), "account unavailable")
}
