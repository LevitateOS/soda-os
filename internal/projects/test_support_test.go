package projects

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/LevitateOS/soda-os/internal/projects/catalog"
	"github.com/LevitateOS/soda-os/internal/projects/workspace"
	"github.com/stretchr/testify/require"
)

type rootTestHost struct {
	uidMin       int
	accounts     map[string]linuxhost.Account
	candidates   []linuxhost.Account
	preflightErr map[string]error
	deleteErr    map[string]error
	events       []string
}

func newRootTestHost() *rootTestHost {
	return &rootTestHost{
		uidMin: 1000, accounts: map[string]linuxhost.Account{},
		preflightErr: map[string]error{}, deleteErr: map[string]error{},
	}
}

func (host *rootTestHost) UIDMin() (int, error) { return host.uidMin, nil }

func (host *rootTestHost) LookupAccount(_ context.Context, username string) (linuxhost.Account, error) {
	account, found := host.accounts[username]
	if !found {
		return linuxhost.Account{}, linuxhost.ErrAccountNotFound
	}
	return account, nil
}

func (host *rootTestHost) CandidateAccounts(context.Context, string, string) ([]linuxhost.Account, error) {
	return append([]linuxhost.Account(nil), host.candidates...), nil
}

func (host *rootTestHost) ReadAuthorizedKeys(linuxhost.Account) ([]byte, error) {
	return []byte("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIG9uZS10ZXN0LWtleQ test\n"), nil
}

func (host *rootTestHost) InstallAuthorizedKeys(linuxhost.Account, []byte) error { return nil }

func (host *rootTestHost) PasswordStatus(_ context.Context, account linuxhost.Account) (linuxhost.PasswordStatus, error) {
	host.events = append(host.events, "password:"+account.Username)
	return linuxhost.PasswordLocked, nil
}

func (host *rootTestHost) PreflightDeleteAccount(_ context.Context, account linuxhost.Account) error {
	host.events = append(host.events, "preflight:"+account.Username)
	return host.preflightErr[account.Username]
}

func (host *rootTestHost) DeleteAccount(_ context.Context, account linuxhost.Account) error {
	host.events = append(host.events, "linux:"+account.Username)
	if err := host.deleteErr[account.Username]; err != nil {
		return err
	}
	delete(host.accounts, account.Username)
	return nil
}

func (host *rootTestHost) Run(context.Context, linuxhost.Command) (linuxhost.CommandResult, error) {
	return linuxhost.CommandResult{}, errors.New("unexpected native command")
}

type rootTestForgejo struct {
	events *[]string
	err    error
}

func (forgejo rootTestForgejo) DeleteUser(_ context.Context, username string) error {
	*forgejo.events = append(*forgejo.events, "forgejo:"+username)
	return forgejo.err
}

func rootPrimary(username string) linuxhost.Account {
	groups := map[string]bool{username: true}
	return linuxhost.Account{
		Username: username, UID: os.Getuid(), GID: os.Getgid(), PrimaryGroup: username,
		Home: "/home/" + username, Shell: "/bin/bash", Groups: groups,
	}
}

func rootAdministrator(username string) linuxhost.Account {
	account := rootPrimary(username)
	account.Groups[linuxhost.AdministratorGroup] = true
	return account
}

func rootWorkspace(t *testing.T, primary, projectID string, uid int) linuxhost.Account {
	t.Helper()
	username, err := workspace.DerivedUsername(primary, projectID)
	require.NoError(t, err)
	marker, err := workspace.Marker(primary, projectID)
	require.NoError(t, err)
	return linuxhost.Account{
		Username: username, UID: uid, GID: uid, PrimaryGroup: username, GECOS: marker,
		Home: "/home/" + username, Shell: workspace.Shell,
		Groups: map[string]bool{username: true, workspace.Group: true},
	}
}

func rootTestStore(t *testing.T) *catalog.Store {
	t.Helper()
	root := t.TempDir()
	store, err := catalog.NewStore(filepath.Join(root, "catalog", "projects.json"), filepath.Join(root, "run", "projects.lock"))
	require.NoError(t, err)
	return store
}

func rootTestOperationLocker(t *testing.T) OperationLocker {
	t.Helper()
	path := filepath.Join(t.TempDir(), "workspace-operations.lock")
	require.NoError(t, os.WriteFile(path, nil, 0o600))
	require.NoError(t, os.Chmod(path, 0o444))
	locker, err := NewOperationLocker(path, os.Getuid())
	require.NoError(t, err)
	return locker
}

func rootTestSetupLocker(t *testing.T, account linuxhost.Account) workspace.SetupLocker {
	t.Helper()
	runtimeRoot := t.TempDir()
	userRuntime := filepath.Join(runtimeRoot, fmt.Sprint(account.UID))
	require.NoError(t, os.Mkdir(userRuntime, 0o700))
	return workspace.NewSetupLocker(runtimeRoot)
}

type testPKExec struct {
	invoker      PKExecInvoker
	actionsPath  string
	requestsPath string
}

type blockingTestPKExec struct {
	testPKExec
	startedPath string
	releasePath string
}

func newTestPKExec(t *testing.T, failAction string) testPKExec {
	t.Helper()
	root := t.TempDir()
	scriptPath := filepath.Join(root, "pkexec")
	actionsPath := filepath.Join(root, "actions")
	requestsPath := filepath.Join(root, "requests")
	script := fmt.Sprintf(`#!/bin/sh
action="$3"
printf '%%s\n' "$action" >> '%s'
request=$(cat)
printf '%%s\n' "$request" >> '%s'
if [ "$action" = '%s' ]; then
  printf 'native SSH authentication failed\n' >&2
  exit 1
fi
case "$action" in
  catalog-add)
    printf '{"ok":true,"project":{"id":"site","display_name":"Site","canonical_url":"git@git.example.test:site.git","catalog_metadata":{"team":"web","labels":["public"]},"workspace_username":"soda-w-example","workspace_exists":false}}\n'
    ;;
  catalog-edit)
    printf '{"ok":true,"project":{"id":"site","display_name":"Renamed","canonical_url":"git@git.example.test:site.git","catalog_metadata":{"owner":"new-owner","labels":["public"]},"workspace_username":"soda-w-example","workspace_exists":false}}\n'
    ;;
  workspace-prepare)
    printf '{"ok":true,"workspace_username":"soda-w-example","workspace_public_key":"ssh-ed25519 test"}\n'
    ;;
  workspace-publish)
    printf '{"ok":true,"workspace_username":"soda-w-example"}\n'
    ;;
  *)
    printf '{"ok":true}\n'
    ;;
esac
`, actionsPath, requestsPath, failAction)
	require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0o700))
	invoker, err := NewPKExecInvoker(scriptPath, filepath.Join(root, "helper"))
	require.NoError(t, err)
	return testPKExec{invoker: invoker, actionsPath: actionsPath, requestsPath: requestsPath}
}

func newBlockingTestPKExec(t *testing.T) blockingTestPKExec {
	t.Helper()
	root := t.TempDir()
	scriptPath := filepath.Join(root, "pkexec")
	actionsPath := filepath.Join(root, "actions")
	requestsPath := filepath.Join(root, "requests")
	startedPath := filepath.Join(root, "publish-started")
	releasePath := filepath.Join(root, "publish-release")
	t.Cleanup(func() { _ = os.WriteFile(releasePath, nil, 0o600) })
	script := fmt.Sprintf(`#!/bin/sh
action="$3"
printf '%%s\n' "$action" >> '%s'
request=$(cat)
printf '%%s\n' "$request" >> '%s'
case "$action" in
  workspace-prepare)
    printf '{"ok":true,"workspace_username":"soda-w-example","workspace_public_key":"ssh-ed25519 test"}\n'
    ;;
  workspace-publish)
    : > '%s'
    while [ ! -f '%s' ]; do sleep 0.01; done
    printf '{"ok":true,"workspace_username":"soda-w-example"}\n'
    ;;
esac
`, actionsPath, requestsPath, startedPath, releasePath)
	require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0o700))
	invoker, err := NewPKExecInvoker(scriptPath, filepath.Join(root, "helper"))
	require.NoError(t, err)
	return blockingTestPKExec{
		testPKExec:  testPKExec{invoker: invoker, actionsPath: actionsPath, requestsPath: requestsPath},
		startedPath: startedPath, releasePath: releasePath,
	}
}

func (pkexec testPKExec) actions(t *testing.T) []string {
	t.Helper()
	contents, err := os.ReadFile(pkexec.actionsPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	require.NoError(t, err)
	return strings.Fields(string(contents))
}

func (pkexec testPKExec) requests(t *testing.T) []string {
	t.Helper()
	contents, err := os.ReadFile(pkexec.requestsPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(contents)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}
