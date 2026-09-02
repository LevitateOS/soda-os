package installer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/LevitateOS/soda-os/internal/process"
	"github.com/stretchr/testify/require"
)

type installerInputValidationCase struct {
	name      string
	mutate    func(*testing.T, *installerInputFixture, *InstallerInputBuilder)
	wantError string
}

func TestInstallerInputValidatesArchitectureArtifactAndProtectedInputs(t *testing.T) {
	for _, test := range installerInputValidationCases() {
		t.Run(test.name, func(t *testing.T) {
			runInstallerInputValidationCase(t, test)
		})
	}
}

func runInstallerInputValidationCase(t *testing.T, test installerInputValidationCase) {
	t.Helper()
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
}

func installerInputValidationCases() []installerInputValidationCase {
	return []installerInputValidationCase{
		{
			name:      "native architecture",
			mutate:    useWrongInstallerInputHostArchitecture,
			wantError: "Soda x86_64 artifact operations require a native amd64 host; running on arm64",
		},
		{
			name:      "release platform",
			mutate:    useWrongInstallerInputReleasePlatform,
			wantError: "installer release record platform differs from the selected architecture",
		},
		{
			name:      "ISO checksum",
			mutate:    changeInstallerInputISO,
			wantError: "installer ISO checksum differs from its release record",
		},
		{
			name:      "username",
			mutate:    useInvalidInstallerInputUsername,
			wantError: "administrator username does not match the Soda account contract",
		},
		{
			name:      "password permissions",
			mutate:    exposeInstallerInputPassword,
			wantError: "administrator password file must not be accessible by group or other users",
		},
		{
			name:      "Tailscale symlink",
			mutate:    symlinkInstallerInputTailscaleKey,
			wantError: "Tailscale auth key input must be a regular file, not a symlink",
		},
		{
			name:      "embedded secret newline",
			mutate:    addInstallerInputSecretNewline,
			wantError: "Tailscale auth key must contain exactly one value",
		},
		{
			name:      "SSH authorized key options",
			mutate:    addInstallerInputAuthorizedKeyOption,
			wantError: "administrator SSH public key is invalid",
		},
	}
}

func useWrongInstallerInputHostArchitecture(
	_ *testing.T,
	_ *installerInputFixture,
	builder *InstallerInputBuilder,
) {
	builder.hostArchitecture = "arm64"
}

func useWrongInstallerInputReleasePlatform(t *testing.T, fixture *installerInputFixture, _ *InstallerInputBuilder) {
	t.Helper()
	writeInstallerInputRecord(t, fixture.options.ReleaseRecordPath, "linux/arm64", fixture.options.ISOPath)
}

func changeInstallerInputISO(t *testing.T, fixture *installerInputFixture, _ *InstallerInputBuilder) {
	t.Helper()
	require.NoError(t, os.WriteFile(fixture.options.ISOPath, []byte("different installer"), 0o644))
}

func useInvalidInstallerInputUsername(_ *testing.T, fixture *installerInputFixture, _ *InstallerInputBuilder) {
	fixture.options.Username = "Invalid_User"
}

func exposeInstallerInputPassword(t *testing.T, fixture *installerInputFixture, _ *InstallerInputBuilder) {
	t.Helper()
	require.NoError(t, os.Chmod(fixture.options.PasswordPath, 0o644))
}

func symlinkInstallerInputTailscaleKey(t *testing.T, fixture *installerInputFixture, _ *InstallerInputBuilder) {
	t.Helper()
	target := fixture.options.TailscaleAuthKeyPath
	link := target + "-link"
	require.NoError(t, os.Symlink(target, link))
	fixture.options.TailscaleAuthKeyPath = link
}

func addInstallerInputSecretNewline(t *testing.T, fixture *installerInputFixture, _ *InstallerInputBuilder) {
	t.Helper()
	secret := []byte("private-value\nsecond-value")
	require.NoError(t, os.WriteFile(fixture.options.TailscaleAuthKeyPath, secret, 0o600))
}

func addInstallerInputAuthorizedKeyOption(t *testing.T, fixture *installerInputFixture, _ *InstallerInputBuilder) {
	t.Helper()
	key, err := os.ReadFile(fixture.options.SSHPublicKeyPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(fixture.options.SSHPublicKeyPath, append([]byte("no-pty "), key...), 0o644))
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
