package projects

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type teaRecordingRunner struct {
	t       *testing.T
	request []Command
}

type humanAccountRunner struct {
	exists   bool
	requests []Command
	password string
}

func (runner *humanAccountRunner) Run(_ context.Context, request Command) (CommandResult, error) {
	runner.requests = append(runner.requests, request)
	switch request.Name {
	case "/usr/bin/getent":
		if !runner.exists {
			return CommandResult{ExitCode: 2}, nil
		}
		return CommandResult{Stdout: "bob:x:1001:1001::/home/bob:/bin/bash\n"}, nil
	case "/usr/bin/id":
		if request.Args[1] == "--groups" {
			return CommandResult{Stdout: "bob\n"}, nil
		}
		return CommandResult{Stdout: "bob\n"}, nil
	case "/usr/sbin/useradd":
		runner.exists = true
		return CommandResult{}, nil
	case "/usr/bin/passwd":
		if request.Args[0] == "--stdin" {
			secret, _ := io.ReadAll(request.Input)
			runner.password = string(secret)
			return CommandResult{}, nil
		}
		return CommandResult{Stdout: "bob P 2026-09-03 0 99999 7 -1\n"}, nil
	default:
		return CommandResult{ExitCode: 127}, nil
	}
}

func (runner *teaRecordingRunner) Run(_ context.Context, request Command) (CommandResult, error) {
	runner.request = append(runner.request, request)
	if len(request.Args) >= 2 && request.Args[0] == "logins" {
		secret, err := io.ReadAll(request.Input)
		require.NoError(runner.t, err)
		require.Equal(runner.t, "initial secret", string(secret))
		configRoot := environmentValue(request.Environment, "XDG_CONFIG_HOME")
		require.NoError(runner.t, os.MkdirAll(filepath.Join(configRoot, "tea"), 0o700))
		require.NoError(runner.t, os.WriteFile(filepath.Join(configRoot, "tea", "config.yml"), []byte("opaque-token"), 0o600))
		return CommandResult{}, nil
	}
	return CommandResult{Stdout: `{"login":"bob"}`}, nil
}

func environmentValue(environment []string, name string) string {
	prefix := name + "="
	for _, value := range environment {
		if strings.HasPrefix(value, prefix) {
			return strings.TrimPrefix(value, prefix)
		}
	}
	return ""
}

func TestNativeTeaUsesStdinAndPrivateStaging(t *testing.T) {
	runtimeRoot := t.TempDir()
	actor := primaryAccount("admin", primaryRoleAdministrator)
	actor.UID, actor.GID = os.Getuid(), os.Getgid()
	require.NoError(t, os.Mkdir(filepath.Join(runtimeRoot, strconv.Itoa(actor.UID)), 0o700))
	runner := &teaRecordingRunner{t: t}
	platform := &NativePlatform{RuntimeRoot: runtimeRoot, Runner: runner}
	tea := NativeTea{Platform: platform, Runner: runner, Binary: "/usr/bin/tea"}

	require.NoError(t, tea.StageLogin(context.Background(), actor, "bob", "http://forgejo.test:30000", "initial secret"))
	require.NoError(t, tea.VerifyLogin(context.Background(), actor, "bob"))
	require.Len(t, runner.request, 2)
	login := runner.request[0]
	require.Equal(t, []string{"logins", "add", "--name", "soda", "--url", "http://forgejo.test:30000", "--user", "bob", "--password-stdin", "--token-name", "soda-os-tea", "--scopes", teaScopes}, login.Args)
	require.NotContains(t, strings.Join(login.Args, " "), "initial secret")
	require.NotContains(t, strings.Join(login.Environment, " "), "initial secret")
	require.NotEqual(t, actor.Home, environmentValue(login.Environment, "HOME"))
	require.NoError(t, tea.CleanupStaging(actor, "bob"))
}

func TestNativeTeaPreflightRejectsUnavailableBinaryAndStagingConflict(t *testing.T) {
	runtimeRoot := t.TempDir()
	actor := primaryAccount("admin", primaryRoleAdministrator)
	actor.UID, actor.GID = os.Getuid(), os.Getgid()
	require.NoError(t, os.Mkdir(filepath.Join(runtimeRoot, strconv.Itoa(actor.UID)), 0o700))
	platform := &NativePlatform{RuntimeRoot: runtimeRoot}
	tea := NativeTea{Platform: platform, Binary: filepath.Join(t.TempDir(), "missing-tea")}
	require.ErrorContains(t, tea.Preflight(actor, "bob"), "unavailable")

	require.NoError(t, os.WriteFile(tea.Binary, []byte("#!/bin/sh\n"), 0o700))
	require.NoError(t, tea.Preflight(actor, "bob"))
	conflict := filepath.Join(runtimeRoot, strconv.Itoa(actor.UID), "soda-projects", "people", "bob")
	require.NoError(t, os.MkdirAll(conflict, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(conflict, "unexpected"), []byte("state"), 0o600))
	require.Error(t, tea.Preflight(actor, "bob"))
}

func TestValidateSupportedNewPrimaryRejectsAdministratorAndWorkspace(t *testing.T) {
	account := primaryAccount("bob", primaryRoleUser)
	require.NoError(t, validateSupportedNewPrimary(account, "bob", 1000))
	account.Groups["wheel"] = true
	require.Error(t, validateSupportedNewPrimary(account, "bob", 1000))
	delete(account.Groups, "wheel")
	account.Groups[WorkspaceGroup] = true
	require.Error(t, validateSupportedNewPrimary(account, "bob", 1000))
}

func TestCreatePrimaryUsesNativePasswordStdinAndNoAdministratorGroup(t *testing.T) {
	loginDefs := filepath.Join(t.TempDir(), "login.defs")
	require.NoError(t, os.WriteFile(loginDefs, []byte("UID_MIN 1000\n"), 0o600))
	runner := &humanAccountRunner{}
	platform := &NativePlatform{Runner: runner, LoginDefsPath: loginDefs}
	account, err := platform.CreatePrimary(context.Background(), "bob", "initial secret")
	require.NoError(t, err)
	require.Equal(t, "bob", account.Username)
	require.Equal(t, "initial secret\n", runner.password)
	for _, request := range runner.requests {
		arguments := strings.Join(request.Args, " ")
		require.NotContains(t, arguments, "initial secret")
		require.NotContains(t, strings.Join(request.Environment, " "), "initial secret")
		if request.Name == "/usr/sbin/useradd" {
			require.NotContains(t, arguments, "wheel")
			require.NotContains(t, arguments, WorkspaceGroup)
		}
	}
}

func TestHumanTeaPublicationAndWorkspaceCopyAreOpaqueAndOneTime(t *testing.T) {
	root := t.TempDir()
	homeRoot := filepath.Join(root, "home")
	runtimeRoot := filepath.Join(root, "run")
	require.NoError(t, os.MkdirAll(homeRoot, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(runtimeRoot, strconv.Itoa(os.Getuid())), 0o700))
	actor := Account{Username: "admin", UID: os.Getuid(), GID: os.Getgid(), Home: filepath.Join(homeRoot, "admin")}
	target := Account{Username: "bob", UID: os.Getuid(), GID: os.Getgid(), Home: filepath.Join(homeRoot, "bob")}
	workspace := Account{Username: "soda-w-example", UID: os.Getuid(), GID: os.Getgid(), Home: filepath.Join(homeRoot, "soda-w-example")}
	for _, account := range []Account{actor, target, workspace} {
		require.NoError(t, os.Mkdir(account.Home, 0o700))
	}
	runner := &teaRecordingRunner{t: t}
	platform := &NativePlatform{HomeRoot: homeRoot, RuntimeRoot: runtimeRoot, Runner: runner}
	tea := NativeTea{Platform: platform, Runner: runner}
	require.NoError(t, tea.StageLogin(context.Background(), actor, "bob", "http://forgejo.test:30000", "initial secret"))
	contents, err := platform.readStagedHumanTea(actor, "bob")
	require.NoError(t, err)
	require.Equal(t, []byte("opaque-token"), contents)

	require.NoError(t, platform.installTeaConfig(context.Background(), target, contents))
	targetConfig := filepath.Join(target.Home, ".config", "tea", "config.yml")
	installed, err := os.ReadFile(targetConfig)
	require.NoError(t, err)
	require.Equal(t, contents, installed)
	require.Equal(t, os.FileMode(0o600), fileMode(t, targetConfig))
	require.NoError(t, platform.installTeaConfig(context.Background(), target, contents))
	require.NoError(t, os.WriteFile(targetConfig, []byte("conflict"), 0o600))
	require.ErrorContains(t, platform.installTeaConfig(context.Background(), target, contents), "refusing to overwrite")
	require.NoError(t, os.WriteFile(targetConfig, contents, 0o600))

	require.NoError(t, platform.InstallWorkspaceTea(target, workspace))
	workspaceConfig := filepath.Join(workspace.Home, ".config", "tea", "config.yml")
	copied, err := os.ReadFile(workspaceConfig)
	require.NoError(t, err)
	require.Equal(t, contents, copied)
	require.Equal(t, os.FileMode(0o600), fileMode(t, workspaceConfig))
}

func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	return info.Mode().Perm()
}
