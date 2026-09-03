package installer

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
		"schema_version": 3,
		"platform":       "linux/amd64",
		"iso_sha256":     hex.EncodeToString(digest[:]),
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(record, recordContents, 0o644))

	writeInstallerInputPublicKey(t, root)
	require.NoError(t, os.WriteFile(filepath.Join(root, "password"), []byte("correct horse battery staple\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "tailscale"), []byte("tskey-auth-test-secret\n"), 0o600))
	return installerInputFixture{spec: installerInputTestSpec(), options: installerInputTestOptions(root)}
}

func writeInstallerInputPublicKey(t *testing.T, root string) string {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	public, err := ssh.NewPublicKey(private.Public())
	require.NoError(t, err)
	path := filepath.Join(root, "administrator.pub")
	line := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(public))) + " administrator comment\n"
	require.NoError(t, os.WriteFile(path, []byte(line), 0o644))
	return path
}

func installerInputTestSpec() config.DistroSpec {
	return config.DistroSpec{
		Identity: config.IdentitySpec{Architecture: "x86_64"},
		Base:     config.BaseSpec{Platform: "linux/amd64"},
		Platform: config.PlatformSpec{Architecture: config.PlatformArchitecture{
			Name: "x86_64", OCI: "amd64", Platform: "linux/amd64", Artifact: "x86_64", Installer: "x86_64",
		}},
	}
}

func installerInputTestOptions(root string) InstallerInputOptions {
	return InstallerInputOptions{
		ISOPath:              filepath.Join(root, "SodaOS.iso"),
		ReleaseRecordPath:    filepath.Join(root, "release.json"),
		Username:             "soda-test",
		SSHPublicKeyPath:     filepath.Join(root, "administrator.pub"),
		TailscaleAuthKeyPath: filepath.Join(root, "tailscale"),
		PasswordPath:         filepath.Join(root, "password"),
		OutputPath:           filepath.Join(root, "answer.iso"),
	}
}

func exactInstallerInputRunner(
	t *testing.T,
	fixture installerInputFixture,
	staged map[string][]byte,
) installerInputRunner {
	t.Helper()
	return installerInputRunner{
		output: func(command process.Command) (string, error) {
			require.Equal(t, process.Command{
				Name: "ssh-keygen",
				Args: []string{"-l", "-f", fixture.options.SSHPublicKeyPath},
			}, command)
			return "256 SHA256:test administrator (ED25519)\n", nil
		},
		run: func(command process.Command) error {
			return runExactInstallerInputCommand(t, command, staged)
		},
	}
}

func runExactInstallerInputCommand(t *testing.T, command process.Command, staged map[string][]byte) error {
	t.Helper()
	if len(command.Args) > 0 && command.Args[0] == "-osirrox" {
		materializeInstallerInputExtraction(t, command, staged)
		return nil
	}
	requireInstallerInputCreationCommand(t, command)
	recordProtectedInstallerInputStaging(t, command.Dir, staged)
	for _, secret := range []string{"correct horse battery staple", "tskey-auth-test-secret"} {
		require.NotContains(t, command.String(), secret)
	}
	return os.WriteFile(command.Args[7], []byte("OEMDRV image"), 0o644)
}

func requireInstallerInputCreationCommand(t *testing.T, command process.Command) {
	t.Helper()
	require.Equal(t, "xorriso", command.Name)
	require.Equal(t, []string{
		"-as", "mkisofs", "-quiet", "-V", "OEMDRV", "-graft-points", "-o", command.Args[7],
		"ks.cfg=ks.cfg",
		"soda/administrator-username=soda/administrator-username",
		"soda/administrator-password=soda/administrator-password",
		"soda/administrator-authorized-key=soda/administrator-authorized-key",
		"soda/tailscale-auth-key=soda/tailscale-auth-key",
	}, command.Args)
}

func recordProtectedInstallerInputStaging(t *testing.T, root string, staged map[string][]byte) {
	t.Helper()
	entries := installerInputStagingEntries(t, root)
	require.Equal(t, []string{
		"ks.cfg",
		"soda",
		"soda/administrator-authorized-key",
		"soda/administrator-password",
		"soda/administrator-username",
		"soda/tailscale-auth-key",
	}, entries)
	requireInstallerInputPathMode(t, filepath.Join(root, "soda"), 0o700, true)
	for _, name := range installerInputPaths() {
		path := filepath.Join(root, name)
		requireInstallerInputPathMode(t, path, 0o600, false)
		contents, err := os.ReadFile(path)
		require.NoError(t, err)
		staged[name] = contents
	}
}

func installerInputStagingEntries(t *testing.T, root string) []string {
	t.Helper()
	entries := []string{}
	require.NoError(t, filepath.WalkDir(root, func(path string, _ os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err == nil && relative != "." {
			entries = append(entries, filepath.ToSlash(relative))
		}
		return err
	}))
	sort.Strings(entries)
	return entries
}

func requireInstallerInputPathMode(t *testing.T, path string, mode os.FileMode, directory bool) {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, directory, info.IsDir())
	require.Equal(t, mode, info.Mode().Perm())
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
