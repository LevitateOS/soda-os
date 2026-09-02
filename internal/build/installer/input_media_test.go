package installer

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/LevitateOS/soda-os/internal/process"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

type installerInputRunner struct {
	run    func(process.Command) error
	output func(process.Command) (string, error)
}

func (r installerInputRunner) Run(_ context.Context, command process.Command) error {
	return r.run(command)
}

func (r installerInputRunner) Output(_ context.Context, command process.Command) (string, error) {
	if r.output == nil {
		return "256 SHA256:test administrator (ED25519)\n", nil
	}
	return r.output(command)
}

type installerInputFixture struct {
	options InstallerInputOptions
	spec    config.DistroSpec
}

func newInstallerInputFixture(t *testing.T) installerInputFixture {
	t.Helper()
	root := t.TempDir()
	iso := filepath.Join(root, "SodaOS.iso")
	isoContents := []byte("exact installer")
	require.NoError(t, os.WriteFile(iso, isoContents, 0o644))
	digest := sha256.Sum256(isoContents)
	record := filepath.Join(root, "release.json")
	recordContents, err := json.Marshal(map[string]any{
		"schema_version": 2,
		"platform":       "linux/amd64",
		"iso_sha256":     hex.EncodeToString(digest[:]),
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(record, recordContents, 0o644))

	_, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	public, err := ssh.NewPublicKey(private.Public())
	require.NoError(t, err)
	publicKey := filepath.Join(root, "administrator.pub")
	publicLine := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(public))) + " administrator comment\n"
	require.NoError(t, os.WriteFile(publicKey, []byte(publicLine), 0o644))

	password := filepath.Join(root, "password")
	require.NoError(t, os.WriteFile(password, []byte("correct horse battery staple\n"), 0o600))
	tailscale := filepath.Join(root, "tailscale")
	require.NoError(t, os.WriteFile(tailscale, []byte("tskey-auth-test-secret\n"), 0o600))

	return installerInputFixture{
		spec: config.DistroSpec{
			Identity: config.IdentitySpec{Architecture: "x86_64"},
			Base:     config.BaseSpec{Platform: "linux/amd64"},
			Platform: config.PlatformSpec{Architecture: config.PlatformArchitecture{
				Name: "x86_64", OCI: "amd64", Platform: "linux/amd64", Artifact: "x86_64", Installer: "x86_64",
			}},
		},
		options: InstallerInputOptions{
			ISOPath:              iso,
			ReleaseRecordPath:    record,
			Username:             "soda-test",
			SSHPublicKeyPath:     publicKey,
			TailscaleAuthKeyPath: tailscale,
			PasswordPath:         password,
			OutputPath:           filepath.Join(root, "answer.iso"),
		},
	}
}

func TestInstallerInputBuildsExactProtectedAnswerMedia(t *testing.T) {
	fixture := newInstallerInputFixture(t)
	staged := map[string][]byte{}
	runner := installerInputRunner{
		output: func(command process.Command) (string, error) {
			require.Equal(t, process.Command{
				Name: "ssh-keygen",
				Args: []string{"-l", "-f", fixture.options.SSHPublicKeyPath},
			}, command)
			return "256 SHA256:test administrator (ED25519)\n", nil
		},
		run: func(command process.Command) error {
			if len(command.Args) > 0 && command.Args[0] == "-osirrox" {
				materializeInstallerInputExtraction(t, command, staged)
				return nil
			}
			require.Equal(t, "xorriso", command.Name)
			require.Equal(t, []string{
				"-as", "mkisofs", "-quiet", "-V", "OEMDRV", "-graft-points", "-o", command.Args[7],
				"ks.cfg=ks.cfg",
				"soda/administrator-username=soda/administrator-username",
				"soda/administrator-password=soda/administrator-password",
				"soda/administrator-authorized-key=soda/administrator-authorized-key",
				"soda/tailscale-auth-key=soda/tailscale-auth-key",
			}, command.Args)
			entries := []string{}
			require.NoError(t, filepath.WalkDir(command.Dir, func(path string, entry os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				relative, err := filepath.Rel(command.Dir, path)
				if err != nil || relative == "." {
					return err
				}
				entries = append(entries, filepath.ToSlash(relative))
				info, err := entry.Info()
				if err != nil {
					return err
				}
				if entry.IsDir() {
					require.Equal(t, os.FileMode(0o700), info.Mode().Perm())
					return nil
				}
				require.True(t, info.Mode().IsRegular())
				require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
				contents, err := os.ReadFile(path)
				if err == nil {
					staged[filepath.ToSlash(relative)] = contents
				}
				return err
			}))
			sort.Strings(entries)
			require.Equal(t, []string{
				"ks.cfg",
				"soda",
				"soda/administrator-authorized-key",
				"soda/administrator-password",
				"soda/administrator-username",
				"soda/tailscale-auth-key",
			}, entries)
			for _, secret := range []string{"correct horse battery staple", "tskey-auth-test-secret"} {
				require.NotContains(t, command.String(), secret)
			}
			return os.WriteFile(command.Args[7], []byte("OEMDRV image"), 0o644)
		},
	}
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

	require.Equal(t, []byte(installerInputKickstart(false)), staged["ks.cfg"])
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
	require.Equal(t, installerInputKickstart(true), kickstart)
	for _, expected := range []string{
		"cmdline\n",
		"clearpart --all --initlabel\n",
		"autopart --type=plain --fstype=ext4\n",
		"network --bootproto=dhcp --device=link --activate --onboot=on --hostname=soda-acceptance\n",
		"eula --agreed\n",
		"reboot\n",
	} {
		require.Contains(t, kickstart, expected)
	}
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

func TestInstallerInputValidatesArchitectureArtifactAndProtectedInputs(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*testing.T, *installerInputFixture, *InstallerInputBuilder)
		wantError string
	}{
		{
			name: "native architecture",
			mutate: func(_ *testing.T, _ *installerInputFixture, builder *InstallerInputBuilder) {
				builder.hostArchitecture = "arm64"
			},
			wantError: "Soda x86_64 artifact operations require a native amd64 host; running on arm64",
		},
		{
			name: "release platform",
			mutate: func(t *testing.T, fixture *installerInputFixture, _ *InstallerInputBuilder) {
				writeInstallerInputRecord(t, fixture.options.ReleaseRecordPath, "linux/arm64", fixture.options.ISOPath)
			},
			wantError: "installer release record platform differs from the selected architecture",
		},
		{
			name: "ISO checksum",
			mutate: func(t *testing.T, fixture *installerInputFixture, _ *InstallerInputBuilder) {
				require.NoError(t, os.WriteFile(fixture.options.ISOPath, []byte("different installer"), 0o644))
			},
			wantError: "installer ISO checksum differs from its release record",
		},
		{
			name: "username",
			mutate: func(_ *testing.T, fixture *installerInputFixture, _ *InstallerInputBuilder) {
				fixture.options.Username = "Invalid_User"
			},
			wantError: "administrator username does not match the Soda account contract",
		},
		{
			name: "password permissions",
			mutate: func(t *testing.T, fixture *installerInputFixture, _ *InstallerInputBuilder) {
				require.NoError(t, os.Chmod(fixture.options.PasswordPath, 0o644))
			},
			wantError: "administrator password file must not be accessible by group or other users",
		},
		{
			name: "Tailscale symlink",
			mutate: func(t *testing.T, fixture *installerInputFixture, _ *InstallerInputBuilder) {
				target := fixture.options.TailscaleAuthKeyPath
				link := target + "-link"
				require.NoError(t, os.Symlink(target, link))
				fixture.options.TailscaleAuthKeyPath = link
			},
			wantError: "Tailscale auth key input must be a regular file, not a symlink",
		},
		{
			name: "embedded secret newline",
			mutate: func(t *testing.T, fixture *installerInputFixture, _ *InstallerInputBuilder) {
				require.NoError(t, os.WriteFile(fixture.options.TailscaleAuthKeyPath, []byte("private-value\nsecond-value"), 0o600))
			},
			wantError: "Tailscale auth key must contain exactly one value",
		},
		{
			name: "SSH authorized key options",
			mutate: func(t *testing.T, fixture *installerInputFixture, _ *InstallerInputBuilder) {
				key, err := os.ReadFile(fixture.options.SSHPublicKeyPath)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(fixture.options.SSHPublicKeyPath, append([]byte("no-pty "), key...), 0o644))
			},
			wantError: "administrator SSH public key is invalid",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newInstallerInputFixture(t)
			runnerCalled := false
			builder := NewInstallerInputBuilder(fixture.spec, installerInputRunner{run: func(process.Command) error {
				runnerCalled = true
				return nil
			}}, nil)
			builder.hostArchitecture = "amd64"
			test.mutate(t, &fixture, builder)
			_, err := builder.Build(context.Background(), fixture.options)
			require.EqualError(t, err, test.wantError)
			require.False(t, runnerCalled)
			require.NotContains(t, err.Error(), "private-value")
		})
	}
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

func readStagedInstallerInput(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := make(map[string][]byte, len(installerInputPaths()))
	for _, name := range installerInputPaths() {
		contents, err := os.ReadFile(filepath.Join(root, name))
		require.NoError(t, err)
		files[name] = contents
	}
	return files
}

func materializeInstallerInputExtraction(t *testing.T, command process.Command, files map[string][]byte) {
	t.Helper()
	require.Equal(t, "xorriso", command.Name)
	require.Equal(t, []string{"-osirrox", "on", "-indev", command.Args[3], "-extract", "/", command.Args[6]}, command.Args)
	for _, name := range installerInputPaths() {
		path := filepath.Join(command.Args[6], name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		require.NoError(t, os.WriteFile(path, files[name], 0o600))
	}
}

func writeInstallerInputRecord(t *testing.T, path, platform, iso string) {
	t.Helper()
	contents, err := os.ReadFile(iso)
	require.NoError(t, err)
	digest := sha256.Sum256(contents)
	record, err := json.Marshal(map[string]any{
		"schema_version": 2,
		"platform":       platform,
		"iso_sha256":     hex.EncodeToString(digest[:]),
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, record, 0o644))
}
