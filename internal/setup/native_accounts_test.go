package setup

import (
	"context"
	"testing"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/stretchr/testify/require"
)

type nativeAccountsHost struct {
	accounts map[string]linuxhost.Account
}

func (host nativeAccountsHost) UIDMin() (int, error) { return 1000, nil }
func (host nativeAccountsHost) LookupGroup(context.Context, string) (linuxhost.Group, error) {
	members := map[string]bool{}
	for name := range host.accounts {
		members[name] = true
	}
	return linuxhost.Group{Members: members}, nil
}
func (host nativeAccountsHost) LookupAccount(_ context.Context, name string) (linuxhost.Account, error) {
	return host.accounts[name], nil
}
func TestNativeRecognizesInstallerAndCloudAdministratorsWithoutMutation(t *testing.T) {
	host := nativeAccountsHost{accounts: map[string]linuxhost.Account{
		"owner_name": {Username: "owner_name", UID: 1000, Home: "/var/home/owner_name", Shell: "/bin/bash", Groups: map[string]bool{"wheel": true}},
		"workspace":  {Username: "workspace", UID: 1001, Shell: "/bin/bash", Groups: map[string]bool{"wheel": true, "soda-workspaces": true}},
		"root":       {Username: "root", UID: 0, Shell: "/bin/bash", Groups: map[string]bool{"wheel": true}},
	}}
	administrators, err := (NativeAccounts{Host: host}).Administrators(context.Background())
	require.NoError(t, err)
	require.Equal(t, []Administrator{{Username: "owner_name"}}, administrators)
}
