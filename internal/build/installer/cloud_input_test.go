package installer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/process"
	"github.com/stretchr/testify/require"
)

func TestCloudInputBuildsTheTwoSupportedProtectedMediaLayouts(t *testing.T) {
	for _, source := range []CloudDataSource{CloudNoCloud, CloudConfigDrive} {
		t.Run(string(source), func(t *testing.T) {
			fixture := newInstallerInputFixture(t)
			options := CloudInputOptions{
				DataSource:           source,
				Username:             fixture.options.Username,
				SSHPublicKeyPath:     fixture.options.SSHPublicKeyPath,
				TailscaleAuthKeyPath: fixture.options.TailscaleAuthKeyPath,
				PasswordPath:         fixture.options.PasswordPath,
				OutputPath:           filepath.Join(filepath.Dir(fixture.options.OutputPath), string(source)+".iso"),
			}
			staged := map[string][]byte{}
			runner := cloudInputRunner(t, source, staged)
			builder := NewCloudInputBuilder(fixture.spec, runner, nil)
			builder.input.hostArchitecture = "amd64"

			output, err := builder.Build(context.Background(), options)
			require.NoError(t, err)
			require.Equal(t, options.OutputPath, output)
			info, err := os.Stat(output)
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
			userData := string(staged[cloudUserDataPath(source)])
			require.Contains(t, userData, "#cloud-config")
			require.Contains(t, userData, "  - [ /usr/bin/install, -d, -o, root, -g, root, -m, '0700', /var/lib/soda-install/cloud ]")
			require.Contains(t, userData, "soda-cloud-finalize")
			require.Contains(t, userData, "correct horse battery staple")
			require.Contains(t, userData, "tskey-auth-test-secret")
			require.NotContains(t, strings.Join(commandStrings(runner.commands), "\n"), "correct horse battery staple")
			require.NotContains(t, strings.Join(commandStrings(runner.commands), "\n"), "tskey-auth-test-secret")
		})
	}
}

func TestCloudInputRejectsAnyOtherDatasourceBeforeReadingCredentials(t *testing.T) {
	fixture := newInstallerInputFixture(t)
	options := CloudInputOptions{DataSource: "provider", OutputPath: fixture.options.OutputPath}
	called := false
	builder := NewCloudInputBuilder(fixture.spec, installerInputRunner{
		run:    func(process.Command) error { called = true; return nil },
		output: func(process.Command) (string, error) { called = true; return "", nil },
	}, nil)
	builder.input.hostArchitecture = "amd64"

	_, err := builder.Build(context.Background(), options)
	require.EqualError(t, err, "cloud input datasource must be nocloud or configdrive")
	require.False(t, called)
}

type cloudRecordingRunner struct {
	commands []process.Command
	source   CloudDataSource
	staged   map[string][]byte
}

func cloudInputRunner(t *testing.T, source CloudDataSource, staged map[string][]byte) *cloudRecordingRunner {
	t.Helper()
	return &cloudRecordingRunner{source: source, staged: staged}
}

func (r *cloudRecordingRunner) Output(_ context.Context, command process.Command) (string, error) {
	r.commands = append(r.commands, command)
	if command.Name != "ssh-keygen" {
		return "", os.ErrInvalid
	}
	return "256 SHA256:test administrator (ED25519)\n", nil
}

func (r *cloudRecordingRunner) Run(_ context.Context, command process.Command) error {
	r.commands = append(r.commands, command)
	if command.Name != "xorriso" {
		return os.ErrInvalid
	}
	if command.Args[0] == "-osirrox" {
		return r.extract(command)
	}
	return r.create(command)
}

func (r *cloudRecordingRunner) create(command process.Command) error {
	label := "CIDATA"
	if r.source == CloudConfigDrive {
		label = "CONFIG-2"
	}
	if len(command.Args) < 8 || strings.Join(command.Args[:7], "\x00") != strings.Join([]string{"-as", "mkisofs", "-quiet", "-V", label, "-graft-points", "-o"}, "\x00") {
		return os.ErrInvalid
	}
	for _, name := range cloudInputPaths(r.source) {
		contents, err := os.ReadFile(filepath.Join(command.Dir, name))
		if err != nil {
			return err
		}
		r.staged[name] = contents
	}
	return os.WriteFile(command.Args[7], []byte("cloud seed"), 0o600)
}

func (r *cloudRecordingRunner) extract(command process.Command) error {
	for _, name := range cloudInputPaths(r.source) {
		path := filepath.Join(command.Args[6], name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(path, r.staged[name], 0o600); err != nil {
			return err
		}
	}
	return nil
}

func cloudInputPaths(source CloudDataSource) []string {
	if source == CloudConfigDrive {
		return []string{"openstack/latest/meta_data.json", "openstack/latest/user_data"}
	}
	return []string{"meta-data", "user-data"}
}

func cloudUserDataPath(source CloudDataSource) string {
	if source == CloudConfigDrive {
		return "openstack/latest/user_data"
	}
	return "user-data"
}

func commandStrings(commands []process.Command) []string {
	result := make([]string, 0, len(commands))
	for _, command := range commands {
		result = append(result, command.String())
	}
	return result
}
