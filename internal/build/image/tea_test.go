package image

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/stretchr/testify/require"
)

func TestStageTeaSourceUsesLockedNativeAsset(t *testing.T) {
	for _, test := range []struct {
		architecture string
		archive      string
	}{
		{architecture: "x86_64", archive: "tea-0.15.1-linux-amd64.xz"},
		{architecture: "aarch64", archive: "tea-0.15.1-linux-arm64.xz"},
	} {
		t.Run(test.architecture, func(t *testing.T) {
			root := t.TempDir()
			for _, directory := range []string{
				filepath.Join(root, "distro", "locks"),
				filepath.Join(root, "packaging", "rpm", "tea", "sources"),
				filepath.Join(root, ".artifacts", "tools"),
			} {
				require.NoError(t, os.MkdirAll(directory, 0o755))
			}
			license := []byte("pinned Tea license\n")
			licenseDigest := sha256.Sum256(license)
			require.NoError(t, os.WriteFile(filepath.Join(root, "packaging", "rpm", "tea", "sources", "LICENSE"), license, 0o644))
			archive := []byte("native Tea xz fixture")
			archiveDigest := sha256.Sum256(archive)
			require.NoError(t, os.WriteFile(filepath.Join(root, ".artifacts", "tools", test.archive), archive, 0o644))
			lock := testTeaSourceLock(test.architecture, test.archive, licenseDigest, archiveDigest, "")
			require.NoError(t, os.WriteFile(filepath.Join(root, "distro", "locks", "tea-source.toml"), []byte(lock), 0o644))

			sources := t.TempDir()
			builder := &Builder{Root: root, Spec: config.DistroSpec{Platform: config.PlatformSpec{Architecture: config.PlatformArchitecture{Name: test.architecture}}}}
			require.NoError(t, builder.stageTeaSource(sources))
			contents, err := os.ReadFile(filepath.Join(sources, "tea.xz"))
			require.NoError(t, err)
			require.Equal(t, archive, contents)
			contents, err = os.ReadFile(filepath.Join(sources, "tea-LICENSE"))
			require.NoError(t, err)
			require.Equal(t, license, contents)

			require.NoError(t, os.WriteFile(filepath.Join(root, ".artifacts", "tools", test.archive), []byte("corrupted"), 0o644))
			require.ErrorContains(t, builder.stageTeaSource(sources), "SHA-256 checksum mismatch")
		})
	}
}

func TestStageTeaSourceRejectsUnknownFieldsAndLicenseChanges(t *testing.T) {
	root := t.TempDir()
	for _, directory := range []string{
		filepath.Join(root, "distro", "locks"),
		filepath.Join(root, "packaging", "rpm", "tea", "sources"),
		filepath.Join(root, ".artifacts", "tools"),
	} {
		require.NoError(t, os.MkdirAll(directory, 0o755))
	}
	license := []byte("pinned Tea license\n")
	licenseDigest := sha256.Sum256(license)
	archive := []byte("native Tea xz fixture")
	archiveDigest := sha256.Sum256(archive)
	require.NoError(t, os.WriteFile(filepath.Join(root, "packaging", "rpm", "tea", "sources", "LICENSE"), license, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".artifacts", "tools", "tea-0.15.1-linux-amd64.xz"), archive, 0o644))
	lock := testTeaSourceLock("x86_64", "tea-0.15.1-linux-amd64.xz", licenseDigest, archiveDigest, "unexpected = true\n")
	require.NoError(t, os.WriteFile(filepath.Join(root, "distro", "locks", "tea-source.toml"), []byte(lock), 0o644))
	builder := &Builder{Root: root, Spec: config.DistroSpec{Platform: config.PlatformSpec{Architecture: config.PlatformArchitecture{Name: "x86_64"}}}}
	require.ErrorContains(t, builder.stageTeaSource(t.TempDir()), "unknown fields")

	lock = testTeaSourceLock("x86_64", "tea-0.15.1-linux-amd64.xz", licenseDigest, archiveDigest, "")
	require.NoError(t, os.WriteFile(filepath.Join(root, "distro", "locks", "tea-source.toml"), []byte(lock), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "packaging", "rpm", "tea", "sources", "LICENSE"), []byte("changed"), 0o644))
	require.ErrorContains(t, builder.stageTeaSource(t.TempDir()), "verify Tea license")
}

func TestTeaSourceAssetsRequireBothNativeArchitectures(t *testing.T) {
	assets := map[string]teaSourceAsset{
		"aarch64": {Archive: "tea-linux-arm64.xz", URL: "https://example.invalid/arm64", SHA256: fmt.Sprintf("%064x", 1)},
		"x86_64":  {Archive: "tea-linux-amd64.xz", URL: "https://example.invalid/amd64", SHA256: fmt.Sprintf("%064x", 2)},
	}
	require.True(t, validTeaAssets(assets))

	delete(assets, "aarch64")
	assets["unsupported"] = teaSourceAsset{Archive: "tea.xz", URL: "https://example.invalid/tea", SHA256: fmt.Sprintf("%064x", 3)}
	require.False(t, validTeaAssets(assets))

	delete(assets, "unsupported")
	assets["aarch64"] = teaSourceAsset{Archive: "../tea.xz", URL: "https://example.invalid/tea", SHA256: fmt.Sprintf("%064x", 3)}
	require.False(t, validTeaAssets(assets))
}

func testTeaSourceLock(architecture, archive string, licenseDigest, archiveDigest [sha256.Size]byte, extra string) string {
	siblingArchitecture := "aarch64"
	siblingArchive := "tea-0.15.1-linux-arm64.xz"
	if architecture == "aarch64" {
		siblingArchitecture = "x86_64"
		siblingArchive = "tea-0.15.1-linux-amd64.xz"
	}
	return fmt.Sprintf(`version = "0.15.1"
license_url = "https://example.invalid/LICENSE"
license_sha256 = "%x"
checksum_manifest_url = "https://example.invalid/checksums.txt"
checksum_manifest_sha256 = "%064x"
%s
[asset.%s]
archive = "%s"
url = "https://example.invalid/%s"
sha256 = "%x"

[asset.%s]
archive = "%s"
url = "https://example.invalid/%s"
sha256 = "%064x"
`, licenseDigest, 1, extra, architecture, archive, archive, archiveDigest, siblingArchitecture, siblingArchive, siblingArchive, 2)
}
