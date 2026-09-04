package setup

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/stretchr/testify/require"
)

type nativeAccountsHost struct {
	uidMin    int
	wheel     linuxhost.Group
	accounts  map[string]linuxhost.Account
	passwords map[string]linuxhost.PasswordStatus
	keys      map[string][]byte
	calls     []string
}

func newNativeAccountsHost() *nativeAccountsHost {
	return &nativeAccountsHost{
		uidMin:    1000,
		wheel:     linuxhost.Group{Name: linuxhost.AdministratorGroup, Members: map[string]bool{}},
		accounts:  map[string]linuxhost.Account{},
		passwords: map[string]linuxhost.PasswordStatus{},
		keys:      map[string][]byte{},
	}
}

func (host *nativeAccountsHost) UIDMin() (int, error) {
	host.calls = append(host.calls, "uid-min")
	return host.uidMin, nil
}

func (host *nativeAccountsHost) LookupGroup(_ context.Context, name string) (linuxhost.Group, error) {
	host.calls = append(host.calls, "lookup-group:"+name)
	return host.wheel, nil
}

func (host *nativeAccountsHost) LookupAccount(_ context.Context, username string) (linuxhost.Account, error) {
	host.calls = append(host.calls, "lookup:"+username)
	account, found := host.accounts[username]
	if !found {
		return linuxhost.Account{}, linuxhost.ErrAccountNotFound
	}
	return account, nil
}

func (host *nativeAccountsHost) PasswordStatus(_ context.Context, account linuxhost.Account) (linuxhost.PasswordStatus, error) {
	host.calls = append(host.calls, "password-status:"+account.Username)
	status, found := host.passwords[account.Username]
	if !found {
		return 0, errors.New("password status unavailable")
	}
	return status, nil
}

func (host *nativeAccountsHost) ReadAuthorizedKeys(account linuxhost.Account) ([]byte, error) {
	host.calls = append(host.calls, "read-key:"+account.Username)
	key, found := host.keys[account.Username]
	if !found {
		return nil, errors.New("authorized_keys is absent")
	}
	return append([]byte(nil), key...), nil
}

func (host *nativeAccountsHost) InstallAuthorizedKeys(account linuxhost.Account, contents []byte) error {
	host.calls = append(host.calls, "install-key:"+account.Username)
	host.keys[account.Username] = append([]byte(nil), contents...)
	return nil
}

type nativeAccountsRunner struct {
	host          *nativeAccountsHost
	calls         []linuxhost.Command
	passwordValue string
}

func (runner *nativeAccountsRunner) Run(_ context.Context, request linuxhost.Command) (linuxhost.CommandResult, error) {
	runner.calls = append(runner.calls, request)
	switch request.Name {
	case "/usr/sbin/useradd":
		username := request.Args[len(request.Args)-1]
		runner.host.accounts[username] = ordinaryPrimary(username, runner.host.uidMin)
	case "/usr/bin/passwd":
		password, err := io.ReadAll(request.Input)
		if err != nil {
			return linuxhost.CommandResult{}, err
		}
		username := request.Args[len(request.Args)-1]
		runner.passwordValue = string(password)
		runner.host.passwords[username] = linuxhost.PasswordSet
	default:
		return linuxhost.CommandResult{}, errors.New("unexpected initial administrator command")
	}
	return linuxhost.CommandResult{}, nil
}

func ordinaryPrimary(username string, uid int) linuxhost.Account {
	return linuxhost.Account{
		Username:     username,
		UID:          uid,
		GID:          uid,
		PrimaryGroup: username,
		Home:         "/home/" + username,
		Shell:        administratorShell,
		Groups:       map[string]bool{username: true},
	}
}

func TestNativeAccountsPreparesInitialAdministrator(t *testing.T) {
	host := newNativeAccountsHost()
	runner := &nativeAccountsRunner{host: host}
	key := testAdministratorKey(t)
	request := AdministratorRequest{Username: "ada", Password: "secret", AuthorizedKey: key}

	err := (NativeAccounts{Host: host, Runner: runner}).Prepare(context.Background(), request)
	require.NoError(t, err)
	require.Len(t, runner.calls, 2)
	require.Equal(t, "/usr/sbin/useradd", runner.calls[0].Name)
	require.Equal(t, []string{
		"--create-home", "--user-group", "--shell", administratorShell,
		"--home-dir", "/home/ada", "--", "ada",
	}, runner.calls[0].Args)
	require.Equal(t, "/usr/bin/passwd", runner.calls[1].Name)
	require.Equal(t, []string{"--stdin", "--", "ada"}, runner.calls[1].Args)
	require.Empty(t, runner.calls[1].Environment)
	require.Equal(t, "secret\n", runner.passwordValue)
	require.Equal(t, key+"\n", string(host.keys["ada"]))
	require.Equal(t, []string{
		"uid-min", "lookup:ada", "lookup:ada", "password-status:ada",
		"uid-min", "lookup:ada", "read-key:ada", "install-key:ada",
	}, host.calls)
}

func TestNativeAccountsResumesRetainedAdministratorWithoutReplacingKey(t *testing.T) {
	host := newNativeAccountsHost()
	runner := &nativeAccountsRunner{host: host}
	host.accounts["ada"] = ordinaryPrimary("ada", host.uidMin)
	host.passwords["ada"] = linuxhost.PasswordSet
	key := testAdministratorKey(t) + "\n"
	host.keys["ada"] = []byte(key)

	err := (NativeAccounts{Host: host, Runner: runner}).Prepare(context.Background(), AdministratorRequest{
		Username: "ada", Password: "secret", AuthorizedKey: key,
	})
	require.NoError(t, err)
	require.Empty(t, runner.calls)
	require.NotContains(t, host.calls, "install-key:ada")
}

func TestNativeAccountsResumesRetainedAdministratorByInstallingOnlyMissingKey(t *testing.T) {
	host := newNativeAccountsHost()
	runner := &nativeAccountsRunner{host: host}
	host.accounts["ada"] = ordinaryPrimary("ada", host.uidMin)
	host.passwords["ada"] = linuxhost.PasswordSet
	key := testAdministratorKey(t)

	err := (NativeAccounts{Host: host, Runner: runner}).Prepare(context.Background(), AdministratorRequest{
		Username: "ada", Password: "secret", AuthorizedKey: key,
	})
	require.NoError(t, err)
	require.Empty(t, runner.calls, "retry must not recreate the account or reset its password")
	require.Equal(t, key+"\n", string(host.keys["ada"]))
	require.Equal(t, []string{
		"uid-min", "lookup:ada", "password-status:ada",
		"uid-min", "lookup:ada", "read-key:ada", "install-key:ada",
	}, host.calls)
}

func TestNativeAccountsRefusesToReplaceRetainedAdministratorKey(t *testing.T) {
	host := newNativeAccountsHost()
	host.accounts["ada"] = ordinaryPrimary("ada", host.uidMin)
	host.passwords["ada"] = linuxhost.PasswordSet
	host.keys["ada"] = []byte(testAdministratorKey(t) + "\n")

	err := (NativeAccounts{Host: host}).Prepare(context.Background(), AdministratorRequest{
		Username: "ada", Password: "secret", AuthorizedKey: testAdministratorKey(t),
	})
	require.ErrorContains(t, err, "refusing to overwrite a different authorized_keys file")
	require.NotContains(t, host.calls, "install-key:ada")
}

func TestNativeAccountsListsOnlyOrdinaryWheelAdministrators(t *testing.T) {
	host := newNativeAccountsHost()
	host.wheel.Members = map[string]bool{"grace": true, "ada": true}
	host.accounts["ada"] = ordinaryPrimary("ada", host.uidMin)
	host.accounts["ada"].Groups[linuxhost.AdministratorGroup] = true
	host.passwords["ada"] = linuxhost.PasswordSet
	host.keys["ada"] = []byte(testAdministratorKey(t) + "\n")
	host.accounts["grace"] = ordinaryPrimary("grace", host.uidMin)
	host.accounts["grace"].Groups[workspaceGroup] = true

	administrators, err := (NativeAccounts{Host: host}).Administrators(context.Background())
	require.NoError(t, err)
	require.Equal(t, []Administrator{{Username: "ada", PasswordSet: true, SSHPublicKey: true}}, administrators)
}

func TestNativeAccountsUsesLinuxFactsForExistingWheelMember(t *testing.T) {
	host := newNativeAccountsHost()
	host.wheel.Members = map[string]bool{"alice_dev": true}
	host.accounts["alice_dev"] = ordinaryPrimary("alice_dev", host.uidMin)
	host.accounts["alice_dev"].Groups[linuxhost.AdministratorGroup] = true
	host.passwords["alice_dev"] = linuxhost.PasswordSet
	host.keys["alice_dev"] = []byte(testAdministratorKey(t) + "\n")

	administrators, err := (NativeAccounts{Host: host}).Administrators(context.Background())
	require.NoError(t, err)
	require.Equal(t, []Administrator{{Username: "alice_dev", PasswordSet: true, SSHPublicKey: true}}, administrators)
}
