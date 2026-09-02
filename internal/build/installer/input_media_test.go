package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/process"
	"github.com/stretchr/testify/require"
)

func TestInstallerInputBuildsExactProtectedAnswerMedia(t *testing.T) {
	fixture := newInstallerInputFixture(t)
	staged := map[string][]byte{}
	runner := exactInstallerInputRunner(t, fixture, staged)
	builder := NewInstallerInputBuilder(fixture.spec, runner, func(string) ([]byte, error) {
		return nil, errors.New("password prompt must not run")
	})
	builder.hostArchitecture = "amd64"

	output, err := builder.Build(context.Background(), fixture.options)
	require.NoError(t, err)
	require.Equal(t, fixture.options.OutputPath, output)
	contents, err := os.ReadFile(output)
	require.NoError(t, err)
	require.Equal(t, []byte("OEMDRV image"), contents)
	info, err := os.Stat(output)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	require.Equal(t, []byte(fixture.options.kickstart()), staged["ks.cfg"])
	require.Equal(t, []byte("soda-test"), staged["soda/administrator-username"])
	require.Equal(t, []byte("correct horse battery staple"), staged["soda/administrator-password"])
	require.Equal(t, []byte("tskey-auth-test-secret"), staged["soda/tailscale-auth-key"])
	keyFields := strings.Fields(string(staged["soda/administrator-authorized-key"]))
	require.Len(t, keyFields, 2)
	require.Equal(t, "ssh-ed25519", keyFields[0])
}

func TestInstallerInputUnattendedModeAddsOnlyFixedNonSecretAutomation(t *testing.T) {
	fixture := newInstallerInputFixture(t)
	fixture.options.Unattended = true
	staged := map[string][]byte{}
	runner := installerInputRunner{run: func(command process.Command) error {
		if len(command.Args) > 0 && command.Args[0] == "-osirrox" {
			materializeInstallerInputExtraction(t, command, staged)
			return nil
		}
		staged = readStagedInstallerInput(t, command.Dir)
		return os.WriteFile(command.Args[7], []byte("OEMDRV image"), 0o600)
	}}
	builder := NewInstallerInputBuilder(fixture.spec, runner, nil)
	builder.hostArchitecture = "amd64"

	_, err := builder.Build(context.Background(), fixture.options)
	require.NoError(t, err)
	kickstart := string(staged["ks.cfg"])
	require.Equal(t, fixture.options.kickstart(), kickstart)
	for _, expected := range []string{
		"cmdline\n",
		"clearpart --all --initlabel\n",
		"autopart --type=plain --fstype=ext4\n",
		"eula --agreed\n",
		"reboot\n",
	} {
		require.Contains(t, kickstart, expected)
	}
	require.NotContains(t, kickstart, "soda-acceptance")
	require.NotContains(t, kickstart, "network --bootproto=dhcp")
	require.NotContains(t, kickstart, "correct horse battery staple")
	require.NotContains(t, kickstart, "tskey-auth-test-secret")
}

func TestInstallerInputPromptsTwiceWithoutPasswordFile(t *testing.T) {
	fixture := newInstallerInputFixture(t)
	fixture.options.PasswordPath = ""
	prompts := []string{}
	answers := [][]byte{[]byte("prompted secret"), []byte("prompted secret")}
	staged := map[string][]byte{}
	runner := installerInputRunner{run: func(command process.Command) error {
		if len(command.Args) > 0 && command.Args[0] == "-osirrox" {
			materializeInstallerInputExtraction(t, command, staged)
			return nil
		}
		password, err := os.ReadFile(filepath.Join(command.Dir, "soda", "administrator-password"))
		require.NoError(t, err)
		require.Equal(t, []byte("prompted secret"), password)
		staged = readStagedInstallerInput(t, command.Dir)
		return os.WriteFile(command.Args[7], []byte("OEMDRV image"), 0o600)
	}}
	builder := NewInstallerInputBuilder(fixture.spec, runner, func(prompt string) ([]byte, error) {
		prompts = append(prompts, prompt)
		answer := answers[0]
		answers = answers[1:]
		return answer, nil
	})
	builder.hostArchitecture = "amd64"

	_, err := builder.Build(context.Background(), fixture.options)
	require.NoError(t, err)
	require.Equal(t, []string{"Administrator password: ", "Confirm administrator password: "}, prompts)
}

func TestInstallerInputRejectsAKeyRejectedByOpenSSH(t *testing.T) {
	fixture := newInstallerInputFixture(t)
	runner := installerInputRunner{
		output: func(command process.Command) (string, error) {
			require.Equal(t, "ssh-keygen", command.Name)
			return "", errors.New("not a public key")
		},
		run: func(process.Command) error {
			t.Fatal("xorriso must not run for an invalid public key")
			return nil
		},
	}
	builder := NewInstallerInputBuilder(fixture.spec, runner, nil)
	builder.hostArchitecture = "amd64"

	_, err := builder.Build(context.Background(), fixture.options)
	require.EqualError(t, err, "administrator SSH public key is invalid")
	require.NoFileExists(t, fixture.options.OutputPath)
}

func TestInstallerInputRejectsPasswordConfirmationMismatch(t *testing.T) {
	fixture := newInstallerInputFixture(t)
	fixture.options.PasswordPath = ""
	runnerCalled := false
	runner := installerInputRunner{run: func(process.Command) error {
		runnerCalled = true
		return nil
	}}
	answers := [][]byte{[]byte("first secret"), []byte("second secret")}
	builder := NewInstallerInputBuilder(fixture.spec, runner, func(string) ([]byte, error) {
		answer := answers[0]
		answers = answers[1:]
		return answer, nil
	})
	builder.hostArchitecture = "amd64"

	_, err := builder.Build(context.Background(), fixture.options)
	require.EqualError(t, err, "administrator password confirmation does not match")
	require.False(t, runnerCalled)
	require.NotContains(t, err.Error(), "first secret")
	require.NotContains(t, err.Error(), "second secret")
}

func TestInstallerInputNeverOverwritesAnOutputCreatedDuringBuild(t *testing.T) {
	fixture := newInstallerInputFixture(t)
	staged := map[string][]byte{}
	runner := installerInputRunner{run: func(command process.Command) error {
		if len(command.Args) > 0 && command.Args[0] == "-osirrox" {
			materializeInstallerInputExtraction(t, command, staged)
			return nil
		}
		staged = readStagedInstallerInput(t, command.Dir)
		require.NoError(t, os.WriteFile(command.Args[7], []byte("generated"), 0o600))
		return os.WriteFile(fixture.options.OutputPath, []byte("existing"), 0o600)
	}}
	builder := NewInstallerInputBuilder(fixture.spec, runner, nil)
	builder.hostArchitecture = "amd64"

	_, err := builder.Build(context.Background(), fixture.options)
	require.EqualError(t, err, "installer input output already exists")
	contents, readErr := os.ReadFile(fixture.options.OutputPath)
	require.NoError(t, readErr)
	require.Equal(t, []byte("existing"), contents)
}

func TestInstallerInputCleansPrivateWorkspaceAfterGenerationFailure(t *testing.T) {
	fixture := newInstallerInputFixture(t)
	runner := installerInputRunner{run: func(command process.Command) error {
		require.Equal(t, "xorriso", command.Name)
		return errors.New("synthetic xorriso failure")
	}}
	builder := NewInstallerInputBuilder(fixture.spec, runner, nil)
	builder.hostArchitecture = "amd64"

	_, err := builder.Build(context.Background(), fixture.options)
	require.EqualError(t, err, "create installer input ISO: synthetic xorriso failure")
	require.NotContains(t, err.Error(), "correct horse battery staple")
	require.NotContains(t, err.Error(), "tskey-auth-test-secret")
	require.NoFileExists(t, fixture.options.OutputPath)
	workspaces, globErr := filepath.Glob(filepath.Join(filepath.Dir(fixture.options.OutputPath), ".soda-installer-input-*"))
	require.NoError(t, globErr)
	require.Empty(t, workspaces)
}

func TestInstallerInputRejectsAnExistingOutputBeforeReadingCredentials(t *testing.T) {
	fixture := newInstallerInputFixture(t)
	require.NoError(t, os.WriteFile(fixture.options.OutputPath, []byte("existing"), 0o600))
	runnerCalled := false
	builder := NewInstallerInputBuilder(fixture.spec, installerInputRunner{
		run: func(process.Command) error {
			runnerCalled = true
			return nil
		},
		output: func(process.Command) (string, error) {
			runnerCalled = true
			return "", nil
		},
	}, nil)
	builder.hostArchitecture = "amd64"

	_, err := builder.Build(context.Background(), fixture.options)
	require.EqualError(t, err, "installer input output already exists")
	require.False(t, runnerCalled)
	contents, readErr := os.ReadFile(fixture.options.OutputPath)
	require.NoError(t, readErr)
	require.Equal(t, []byte("existing"), contents)
}

func TestInstallerInputRejectsUnexpectedGeneratedMediaContents(t *testing.T) {
	fixture := newInstallerInputFixture(t)
	staged := map[string][]byte{}
	runner := installerInputRunner{run: func(command process.Command) error {
		if len(command.Args) > 0 && command.Args[0] == "-osirrox" {
			materializeInstallerInputExtraction(t, command, staged)
			return os.WriteFile(filepath.Join(command.Args[6], "unexpected"), []byte("not allowed"), 0o600)
		}
		staged = readStagedInstallerInput(t, command.Dir)
		return os.WriteFile(command.Args[7], []byte("generated"), 0o600)
	}}
	builder := NewInstallerInputBuilder(fixture.spec, runner, nil)
	builder.hostArchitecture = "amd64"

	_, err := builder.Build(context.Background(), fixture.options)
	require.EqualError(t, err, "installer input ISO does not contain exactly the required files")
	require.NoFileExists(t, fixture.options.OutputPath)
}
