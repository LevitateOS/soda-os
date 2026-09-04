package runners

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type recordingCommandRunner struct {
	commands          []Command
	secretHadDeadline bool
}

func (runner *recordingCommandRunner) Run(_ context.Context, command Command) (CommandResult, error) {
	runner.commands = append(runner.commands, command)
	return CommandResult{}, nil
}

func (runner *recordingCommandRunner) RunSecret(ctx context.Context, _ Command, _ string) error {
	_, runner.secretHadDeadline = ctx.Deadline()
	return nil
}

func TestForgejoConfigurationUsesNativeTokenFileAndOneHostSlot(t *testing.T) {
	state := t.TempDir()
	native := NewNative()
	request := CreateRequest{
		ID: "forgejo-one", Provider: ProviderForgejo,
		RegistrationURL: "http://soda.example.test:30000",
		RegistrationID:  "33834eef-e758-48c4-a676-1745426747aa",
		Labels:          "soda-arm64:host", RegistrationToken: "provider-input",
	}
	require.NoError(t, native.configureForgejo(state, identity{uint32(os.Getuid()), uint32(os.Getgid())}, request))

	contents, err := os.ReadFile(filepath.Join(state, "forgejo-runner.yml"))
	require.NoError(t, err)
	require.NotContains(t, string(contents), request.RegistrationToken)
	var configuration map[string]any
	require.NoError(t, json.Unmarshal(contents, &configuration))
	runner := configuration["runner"].(map[string]any)
	require.Equal(t, float64(1), runner["capacity"])
	require.Equal(t, []any{"soda-arm64:host"}, runner["labels"])
	require.Equal(t, map[string]any{"enabled": false}, configuration["cache"])

	token, err := os.ReadFile(filepath.Join(state, "forgejo-token"))
	require.NoError(t, err)
	require.Equal(t, request.RegistrationToken+"\n", string(token))
	info, err := os.Stat(filepath.Join(state, "forgejo-token"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestGitHubRegistrationUsesBoundedPromptWithoutReplacingProviderRecords(t *testing.T) {
	request := CreateRequest{
		ID: "github-one", Provider: ProviderGitHub, RegistrationURL: "https://github.com/example/repository",
		Labels: "soda", RegistrationToken: "provider-input",
	}
	command := githubRegistrationCommand("/runner", "/state", identity{UID: 1200, GID: 1300}, request)
	require.Equal(t, []string{
		"--url", request.RegistrationURL, "--name", request.ID, "--runnergroup", "default",
		"--work", "_work", "--disableupdate", "--labels", request.Labels,
	}, command.Args)
	require.NotContains(t, command.Args, "--unattended")
	require.NotContains(t, command.Args, "--replace")
	require.NotContains(t, strings.Join(command.Args, " "), request.RegistrationToken)
	require.NotContains(t, strings.Join(command.Environment, " "), request.RegistrationToken)

	runner := &recordingCommandRunner{}
	native := &Native{Runner: runner}
	require.NoError(t, native.runGitHubRegistration(context.Background(), command, request.RegistrationToken))
	require.True(t, runner.secretHadDeadline)
}

func TestLifecycleActionsPersistListenerStateAcrossBoot(t *testing.T) {
	root := t.TempDir()
	runner := &recordingCommandRunner{}
	native := &Native{RootPath: root, Runner: runner}
	id := "one"
	require.NoError(t, os.MkdirAll(filepath.Join(root, id), 0o755))
	require.NoError(t, native.writeDescriptor(Descriptor{ID: id, Provider: ProviderGitHub, Account: "soda-runner-one"}))

	require.NoError(t, native.Start(context.Background(), id))
	require.NoError(t, native.Stop(context.Background(), id))
	require.NoError(t, native.Restart(context.Background(), id))
	require.Equal(t, []Command{
		{Name: "systemctl", Args: []string{"enable", "--now", "soda-runner@one.service"}},
		{Name: "systemctl", Args: []string{"disable", "--now", "soda-runner@one.service"}},
		{Name: "systemctl", Args: []string{"enable", "soda-runner@one.service"}},
		{Name: "systemctl", Args: []string{"restart", "soda-runner@one.service"}},
	}, runner.commands)
}

func TestPrepareAccountRemovesLinuxAccountWhenIdentityLookupFails(t *testing.T) {
	root := t.TempDir()
	runner := &recordingCommandRunner{}
	native := &Native{RootPath: root, Runner: runner}

	_, err := native.prepareAccount(context.Background(), "uncreated")
	require.ErrorContains(t, err, "look up new runner account")
	require.Len(t, runner.commands, 2)
	require.Equal(t, "useradd", runner.commands[0].Name)
	require.Equal(t, Command{Name: "userdel", Args: []string{"soda-runner-uncreated"}}, runner.commands[1])
	require.NoDirExists(t, filepath.Join(root, "uncreated"))
}
