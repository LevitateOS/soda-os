package image

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/stretchr/testify/require"
)

func TestMiseIsOneCheckedExternalRPMPerArchitecture(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err)
	for _, architecture := range []string{"aarch64", "x86_64"} {
		t.Run(architecture, func(t *testing.T) {
			builder, buildErr := NewBuilder(root, "distro/soda.toml", architecture, nil)
			require.NoError(t, buildErr)
			lock, lockErr := builder.packageLock()
			require.NoError(t, lockErr)
			require.Contains(t, lock.Package, lockedPackage{
				Name: "mise", NEVRA: "mise-0:2026.9.1-1.fc44." + architecture,
				Source: "external-rpm", File: "mise-2026.9.1-1.fc44." + architecture + ".rpm",
			})
		})
	}

	lock, err := readMiseSourceLock(filepath.Join(root, "distro", "locks", "mise-source.toml"))
	require.NoError(t, err)
	require.Equal(t, "2026.9.1", lock.Version)
	require.Equal(t, "mise-0:2026.9.1-1.fc44.aarch64", lock.Asset["aarch64"].NEVRA)
	require.Equal(t, "e6b22b909ad98fc125d3b840bc2424066db223dcbea5b29049a2fd756ccc7974", lock.Asset["aarch64"].SHA256)
	require.Equal(t, "b582905a0d2673127b3771d22304ce7da8148336b96fb8e10b7aca20dbaacadf", lock.Asset["x86_64"].SHA256)
}

func TestStageMiseRPMUsesSelectedChecksummedAsset(t *testing.T) {
	root := t.TempDir()
	for _, directory := range []string{filepath.Join(root, "distro", "locks"), filepath.Join(root, ".artifacts", "tools")} {
		require.NoError(t, os.MkdirAll(directory, 0o755))
	}
	aarch64 := []byte("aarch64 mise RPM")
	x86_64 := []byte("x86_64 mise RPM")
	require.NoError(t, os.WriteFile(filepath.Join(root, ".artifacts", "tools", "mise-2026.9.1-1.fc44.aarch64.rpm"), aarch64, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".artifacts", "tools", "mise-2026.9.1-1.fc44.x86_64.rpm"), x86_64, 0o644))
	aarch64Digest := sha256.Sum256(aarch64)
	x86_64Digest := sha256.Sum256(x86_64)
	lock := fmt.Sprintf(`version = "2026.9.1"

[asset.aarch64]
nevra = "mise-0:2026.9.1-1.fc44.aarch64"
file = "mise-2026.9.1-1.fc44.aarch64.rpm"
url = "https://example.invalid/fedora-44-aarch64/mise-2026.9.1-1.fc44.aarch64.rpm"
sha256 = "%x"

[asset.x86_64]
nevra = "mise-0:2026.9.1-1.fc44.x86_64"
file = "mise-2026.9.1-1.fc44.x86_64.rpm"
url = "https://example.invalid/fedora-44-x86_64/mise-2026.9.1-1.fc44.x86_64.rpm"
sha256 = "%x"
`, aarch64Digest, x86_64Digest)
	require.NoError(t, os.WriteFile(filepath.Join(root, "distro", "locks", "mise-source.toml"), []byte(lock), 0o644))

	destination := t.TempDir()
	builder := &Builder{Root: root, Spec: config.DistroSpec{Platform: config.PlatformSpec{Architecture: config.PlatformArchitecture{Name: "aarch64"}}}}
	require.NoError(t, builder.stageMiseRPM(destination))
	contents, err := os.ReadFile(filepath.Join(destination, "mise-2026.9.1-1.fc44.aarch64.rpm"))
	require.NoError(t, err)
	require.Equal(t, aarch64, contents)

	require.NoError(t, os.WriteFile(filepath.Join(root, ".artifacts", "tools", "mise-2026.9.1-1.fc44.aarch64.rpm"), []byte("corrupted"), 0o644))
	require.ErrorContains(t, builder.stageMiseRPM(destination), "SHA-256 checksum mismatch")
}

func TestMiseSourceLockRejectsUnknownFieldsAndInvalidAssets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mise-source.toml")
	contents := `version = "2026.9.1"
unexpected = true

[asset.aarch64]
nevra = "mise-0:2026.9.1-1.fc44.aarch64"
file = "mise-2026.9.1-1.fc44.aarch64.rpm"
url = "https://example.invalid/fedora-44-aarch64/mise-2026.9.1-1.fc44.aarch64.rpm"
sha256 = "e6b22b909ad98fc125d3b840bc2424066db223dcbea5b29049a2fd756ccc7974"

[asset.x86_64]
nevra = "mise-0:2026.9.1-1.fc44.x86_64"
file = "mise-2026.9.1-1.fc44.x86_64.rpm"
url = "https://example.invalid/fedora-44-x86_64/mise-2026.9.1-1.fc44.x86_64.rpm"
sha256 = "b582905a0d2673127b3771d22304ce7da8148336b96fb8e10b7aca20dbaacadf"
`
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
	_, err := readMiseSourceLock(path)
	require.ErrorContains(t, err, "invalid")

	mirrored := strings.ReplaceAll(contents, "unexpected = true\n", "")
	mirrored = strings.ReplaceAll(mirrored, "mise-2026.9.1-1.fc44.aarch64.rpm", "mise-reviewed-aarch64.rpm")
	mirrored = strings.ReplaceAll(mirrored, "mise-2026.9.1-1.fc44.x86_64.rpm", "mise-reviewed-x86_64.rpm")
	mirrored = strings.ReplaceAll(mirrored, "https://example.invalid/fedora-44-aarch64/mise-reviewed-aarch64.rpm", "https://mirror.example/mise-reviewed-aarch64.rpm")
	mirrored = strings.ReplaceAll(mirrored, "https://example.invalid/fedora-44-x86_64/mise-reviewed-x86_64.rpm", "https://mirror.example/mise-reviewed-x86_64.rpm")
	require.NoError(t, os.WriteFile(path, []byte(mirrored), 0o644))
	lock, err := readMiseSourceLock(path)
	require.NoError(t, err)
	require.Equal(t, "mise-reviewed-x86_64.rpm", lock.Asset["x86_64"].File)
}
