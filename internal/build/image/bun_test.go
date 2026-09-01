package image

import (
	"archive/zip"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/stretchr/testify/require"
)

func TestStageBunSourceUsesLockedNativeAsset(t *testing.T) {
	for _, test := range []struct {
		architecture string
		member       string
	}{
		{architecture: "x86_64", member: "bun-linux-x64-baseline/bun"},
		{architecture: "aarch64", member: "bun-linux-aarch64/bun"},
	} {
		t.Run(test.architecture, func(t *testing.T) {
			root := t.TempDir()
			for _, directory := range []string{
				filepath.Join(root, "distro", "locks"),
				filepath.Join(root, "packaging", "rpm", "bun", "sources"),
				filepath.Join(root, ".artifacts", "tools"),
			} {
				require.NoError(t, os.MkdirAll(directory, 0o755))
			}
			license := []byte("pinned Bun license\n")
			licenseDigest := sha256.Sum256(license)
			require.NoError(t, os.WriteFile(filepath.Join(root, "packaging", "rpm", "bun", "sources", "LICENSE.md"), license, 0o644))

			archive := filepath.Join(root, ".artifacts", "tools", "bun-test.zip")
			file, err := os.Create(archive)
			require.NoError(t, err)
			writer := zip.NewWriter(file)
			entry, err := writer.Create(test.member)
			require.NoError(t, err)
			_, err = entry.Write([]byte("native bun"))
			require.NoError(t, err)
			require.NoError(t, writer.Close())
			require.NoError(t, file.Close())
			archiveBytes, err := os.ReadFile(archive)
			require.NoError(t, err)
			archiveDigest := sha256.Sum256(archiveBytes)

			lock := fmt.Sprintf(`version = "1.4.0"
license_sha256 = "%x"

[asset.%s]
archive = "bun-test.zip"
member = "%s"
url = "https://example.invalid/bun.zip"
sha256 = "%x"
`, licenseDigest, test.architecture, test.member, archiveDigest)
			require.NoError(t, os.WriteFile(filepath.Join(root, "distro", "locks", "bun-source.toml"), []byte(lock), 0o644))

			sources := t.TempDir()
			builder := &Builder{Root: root, Spec: config.DistroSpec{Platform: config.PlatformSpec{Architecture: config.PlatformArchitecture{Name: test.architecture}}}}
			require.NoError(t, builder.stageBunSource(sources))
			contents, err := os.ReadFile(filepath.Join(sources, "bun"))
			require.NoError(t, err)
			require.Equal(t, []byte("native bun"), contents)
			contents, err = os.ReadFile(filepath.Join(sources, "LICENSE.md"))
			require.NoError(t, err)
			require.Equal(t, license, contents)

			require.NoError(t, os.WriteFile(archive, []byte("corrupted"), 0o644))
			require.ErrorContains(t, builder.stageBunSource(sources), "SHA-256 checksum mismatch")
		})
	}
}
