package installer

import (
	"context"
	"encoding/csv"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/LevitateOS/soda-os/internal/process"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/stretchr/testify/require"
)

const testExactImage = Repository + "@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

type recordingRunner struct {
	Commands []process.Command
	Outputs  map[string]string
	Err      error
}

func (r *recordingRunner) Run(_ context.Context, command process.Command) error {
	r.Commands = append(r.Commands, command)
	return r.Err
}

func (r *recordingRunner) Output(_ context.Context, command process.Command) (string, error) {
	r.Commands = append(r.Commands, command)
	return r.Outputs[command.String()], r.Err
}

func TestKickstartKeepsStockInteractiveFlowAndExactDigest(t *testing.T) {
	contents := kickstart(testExactImage, "soda")
	require.Contains(t, contents, "graphical\n")
	require.NotContains(t, contents, "text\n")
	require.Contains(t, contents, "network --bootproto=dhcp --device=link --activate --onboot=on --hostname=soda")
	require.Contains(t, contents, "rootpw --lock\n")
	require.Contains(t, contents, "firstboot --disable\n")
	require.Contains(t, contents, `--source-imgref="docker://`+testExactImage+`"`)
	require.Contains(t, contents, `--target-imgref="`+testExactImage+`"`)
	require.NotContains(t, contents, "%pre-install")
	require.NotContains(t, contents, "/mnt/sysimage/var/home")
	require.NotContains(t, contents, "--enforce-container-sigpolicy")
	require.NotContains(t, contents, "selinux=0")
	require.NotContains(t, contents, "clearpart")
	require.NotContains(t, contents, "user --name")
}

func TestInstallerStorageUsesOnePlainExt4Root(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	contents, err := os.ReadFile(filepath.Join(root, "packaging", "installer", "soda-storage.conf"))
	require.NoError(t, err)
	require.Equal(t, "# Soda OS automatic storage defaults for the trusted-LAN bootc appliance.\n[Storage]\nfile_system_type = ext4\ndefault_scheme = PLAIN\ndefault_partitioning =\n    / (min 1 GiB)\n", string(contents))
	require.NotContains(t, string(contents), "/home")
	require.NotContains(t, string(contents), "BTRFS")

	containerfile, err := os.ReadFile(filepath.Join(root, "packaging", "installer", "Containerfile"))
	require.NoError(t, err)
	require.Contains(t, string(containerfile), "COPY --chmod=0644 packaging/installer/soda-storage.conf /etc/anaconda/conf.d/90-soda-storage.conf")
}

func TestInstallerScratchMountRequiresExactUnitAndEnablement(t *testing.T) {
	root := t.TempDir()
	inspectDir := t.TempDir()
	expected := []byte("[Mount]\nWhat=tmpfs\nWhere=/var/tmp\n")
	expectedPath := filepath.Join(root, "packaging", "installer", "var-tmp.mount")
	actualPath := filepath.Join(inspectDir, "root", "usr", "lib", "systemd", "system", "var-tmp.mount")
	wantsPath := filepath.Join(inspectDir, "root", "etc", "systemd", "system", "anaconda.target.wants", "var-tmp.mount")
	require.NoError(t, os.MkdirAll(filepath.Dir(expectedPath), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Dir(actualPath), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Dir(wantsPath), 0o755))
	require.NoError(t, os.WriteFile(expectedPath, expected, 0o644))
	require.NoError(t, os.WriteFile(actualPath, expected, 0o644))
	require.NoError(t, os.Symlink("/usr/lib/systemd/system/var-tmp.mount", wantsPath))
	builder := NewBuilder(root, config.DistroSpec{}, &recordingRunner{})
	require.NoError(t, builder.validateExtractedInstallerScratch(inspectDir))

	require.NoError(t, os.Remove(wantsPath))
	require.NoError(t, os.Symlink("/wrong", wantsPath))
	require.EqualError(t, builder.validateExtractedInstallerScratch(inspectDir), "ISO installer scratch mount is not enabled for Anaconda")
}

func TestStorageConfigRequiresExactPlainExt4RootOnlyContract(t *testing.T) {
	expected := []byte("[Storage]\nfile_system_type = ext4\ndefault_scheme = PLAIN\ndefault_partitioning =\n    / (min 1 GiB)\n")
	root := t.TempDir()
	inspectDir := t.TempDir()
	expectedDir := filepath.Join(root, "packaging", "installer")
	extractedDir := filepath.Join(inspectDir, "root", "etc", "anaconda", "conf.d")
	bootConfigDir := filepath.Join(inspectDir, "root", "usr", "lib", "image-builder", "bootc")
	require.NoError(t, os.MkdirAll(expectedDir, 0o755))
	require.NoError(t, os.MkdirAll(extractedDir, 0o755))
	require.NoError(t, os.MkdirAll(bootConfigDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(expectedDir, "soda-storage.conf"), expected, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(expectedDir, "iso-aarch64.yaml"), []byte("valid ISO config\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bootConfigDir, "iso.yaml"), []byte("valid ISO config\n"), 0o644))
	actualPath := filepath.Join(extractedDir, "90-soda-storage.conf")
	require.NoError(t, os.WriteFile(actualPath, expected, 0o644))
	builder := NewBuilder(root, config.DistroSpec{Platform: config.PlatformSpec{Installer: config.PlatformInstaller{ISOConfig: "packaging/installer/iso-aarch64.yaml"}}}, &recordingRunner{})
	require.NoError(t, builder.validateExtractedConfiguration(inspectDir))

	for name, malformed := range map[string]string{
		"btrfs":         strings.ReplaceAll(string(expected), "ext4", "btrfs"),
		"btrfs scheme":  strings.ReplaceAll(string(expected), "PLAIN", "BTRFS"),
		"separate home": string(expected) + "    /home (min 500 MiB)\n",
	} {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, os.WriteFile(actualPath, []byte(malformed), 0o644))
			require.EqualError(t, builder.validateExtractedConfiguration(inspectDir), "ISO storage configuration differs from the Soda ext4 root-only contract")
		})
	}
}

func TestInstallerEnvironmentPinsBIOSHybridBootModule(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	contents, err := os.ReadFile(filepath.Join(root, "packaging", "installer", "Containerfile"))
	require.NoError(t, err)
	require.Contains(t, string(contents), "$(cat /usr/share/soda-installer/boot-packages.txt)")
	require.Contains(t, string(contents), "rpm -q $(cat /usr/share/soda-installer/boot-packages.txt)")
	require.Contains(t, string(contents), "test -f /usr/lib/grub/i386-pc/boot_hybrid.img")
	for _, architecture := range []string{"aarch64", "x86_64"} {
		lock, readErr := os.ReadFile(filepath.Join(root, "distro", "locks", "installer-packages-"+architecture+".toml"))
		require.NoError(t, readErr)
		require.Contains(t, string(lock), "grub2-pc-modules-1:2.12-64.fc44.noarch")
		require.Contains(t, string(lock), "shim-")
	}
}

func TestInstallerEnvironmentUsesDefaultTargetRequiredByAnacondaGenerator(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "installer", "Containerfile"))
	require.NoError(t, err)

	// Fedora 44's anaconda-generator compares readlink output literally with the /lib path.
	require.Contains(t, string(contents), "ln -sf /lib/systemd/system/anaconda.target /etc/systemd/system/default.target")
	require.NotContains(t, string(contents), "ln -sf /usr/lib/systemd/system/anaconda.target /etc/systemd/system/default.target")
}

func TestInstallerEnvironmentUsesPinnedFedoraInstallerBase(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "installer", "Containerfile"))
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(contents), "FROM installer-base\n"))
	require.Contains(t, string(contents), "COPY --chmod=0644 .artifacts/installer/context/interactive-defaults.ks /usr/share/anaconda/interactive-defaults.ks")
}

func TestInstallerInspectionRejectsDuplicatedBootcBase(t *testing.T) {
	require.NoError(t, validateNoDuplicatedBootcBase("drwxr-xr-x squashfs-root/usr"))
	require.EqualError(t, validateNoDuplicatedBootcBase("drwxr-xr-x squashfs-root/sysroot"), "installer squashfs contains a duplicated bootc base")
}

func TestInstallerEnvironmentUsesCurrentSodaAnacondaBranding(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	profile, err := os.ReadFile(filepath.Join(root, "packaging", "installer", "branding", "sodaos.conf"))
	require.NoError(t, err)
	require.Equal(t, "# Soda OS Anaconda profile layered on Fedora's installer defaults.\n\n[Profile]\nprofile_id = sodaos\nbase_profile = fedora\n\n[Anaconda]\noptional_modules =\n    org.fedoraproject.Anaconda.Modules.Subscription\n    org.fedoraproject.Anaconda.Addons.Kdump\n\n[Profile Detection]\nos_id = sodaos\n\n[Installation Target]\ncan_copy_input_kickstart = False\ncan_save_output_kickstart = False\n\n[User Interface]\ncustom_stylesheet = /usr/share/anaconda/pixmaps/soda.css\nhidden_spokes = UserSpoke PasswordSpoke\n", string(profile))
	release, err := os.ReadFile(filepath.Join(root, "packaging", "installer", "branding", "os-release"))
	require.NoError(t, err)
	require.Contains(t, string(release), "VERSION=\"0.5\"")
	require.Contains(t, string(release), "VERSION_ID=\"0.5\"")
	require.Contains(t, string(release), "PRETTY_NAME=\"Soda OS 0.5.0\"")
	buildstamp, err := os.ReadFile(filepath.Join(root, "packaging", "installer", "branding", "buildstamp"))
	require.NoError(t, err)
	require.Contains(t, string(buildstamp), "Product=Soda OS\nVersion=0.5.0\n")

	containerfile, err := os.ReadFile(filepath.Join(root, "packaging", "installer", "Containerfile"))
	require.NoError(t, err)
	for _, copyInstruction := range []string{
		"COPY --chmod=0644 packaging/installer/branding/sodaos.conf /etc/anaconda/profile.d/sodaos.conf",
		"COPY --chmod=0644 packaging/installer/branding/os-release /usr/lib/os-release",
		"COPY --chmod=0644 packaging/installer/branding/buildstamp /.buildstamp",
		"COPY --chmod=0644 packaging/installer/branding/soda.css /usr/share/anaconda/pixmaps/soda.css",
		"COPY --chmod=0644 assets/branding/installer/soda-logo-horizontal-dark.png /usr/share/anaconda/pixmaps/soda-sidebar-logo.png",
		"COPY --chmod=0644 assets/branding/installer/soda-symbol.png /usr/share/anaconda/pixmaps/soda-symbol.png",
	} {
		require.Contains(t, string(containerfile), copyInstruction)
	}
	require.NotContains(t, string(containerfile), "soda-sidebar-bg.png")
	require.NotContains(t, string(containerfile), "soda-topbar-bg.png")

	stylesheet, err := os.ReadFile(filepath.Join(root, "packaging", "installer", "branding", "soda.css"))
	require.NoError(t, err)
	require.NotContains(t, string(stylesheet), "soda-sidebar-bg.png")
	require.NotContains(t, string(stylesheet), "soda-topbar-bg.png")
}

func TestInstallerBrandingManifestCoversEverySVGMaster(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	manifest, err := os.Open(filepath.Join(root, "assets", "branding", "installer", "manifest.tsv"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, manifest.Close()) })
	reader := csv.NewReader(manifest)
	reader.Comma = '\t'
	reader.Comment = '#'
	records, err := reader.ReadAll()
	require.NoError(t, err)

	masters, err := filepath.Glob(filepath.Join(root, "assets", "branding", "source", "*.svg"))
	require.NoError(t, err)
	manifestSources := make([]string, 0, len(records))
	outputs := make(map[string]struct{}, len(records))
	roles := make(map[string]string, len(records))
	roleCounts := make(map[string]int, len(records))
	for _, record := range records {
		require.Len(t, record, 4)
		kind, source, output, role := record[0], record[1], record[2], record[3]
		manifestSources = append(manifestSources, filepath.Join(root, source))
		_, duplicate := outputs[output]
		require.False(t, duplicate, "duplicate generated installer branding output %s", output)
		outputs[output] = struct{}{}
		roles[role] = source
		roleCounts[role]++

		asset, openErr := os.Open(filepath.Join(root, output))
		require.NoError(t, openErr)
		config, decodeErr := png.DecodeConfig(asset)
		require.NoError(t, asset.Close())
		require.NoError(t, decodeErr)
		switch kind {
		case "horizontal":
			require.Equal(t, 114, config.Width)
			require.Equal(t, 36, config.Height)
		case "symbol":
			require.Equal(t, 256, config.Width)
			require.Equal(t, 256, config.Height)
		default:
			t.Fatalf("unknown installer branding asset kind %q", kind)
		}
	}
	require.ElementsMatch(t, masters, manifestSources)
	require.Equal(t, "assets/branding/source/soda-logo-horizontal-dark.svg", roles["sidebar-logo"])
	require.Equal(t, "assets/branding/source/soda-symbol.svg", roles["product-logo"])
	require.Equal(t, 7, roleCounts["managed-variant"])
	require.Equal(t, 1, roleCounts["sidebar-logo"])
	require.Equal(t, 1, roleCounts["product-logo"])
	require.Len(t, records, 9)

	check := exec.Command("scripts/render-installer-branding.sh", "--check")
	check.Dir = root
	output, err := check.CombinedOutput()
	require.NoErrorf(t, err, "installer branding is stale:\n%s", output)
}

func TestISOInspectionRequiresExactSodaAnacondaBranding(t *testing.T) {
	root := t.TempDir()
	inspectDir := t.TempDir()
	files := []struct{ actual, expected string }{
		{".buildstamp", "packaging/installer/branding/buildstamp"},
		{"usr/lib/os-release", "packaging/installer/branding/os-release"},
		{"etc/anaconda/profile.d/sodaos.conf", "packaging/installer/branding/sodaos.conf"},
		{"usr/share/anaconda/pixmaps/soda.css", "packaging/installer/branding/soda.css"},
		{"usr/share/anaconda/pixmaps/soda-sidebar-logo.png", "assets/branding/installer/soda-logo-horizontal-dark.png"},
		{"usr/share/anaconda/pixmaps/soda-symbol.png", "assets/branding/installer/soda-symbol.png"},
	}
	for _, file := range files {
		contents := []byte(file.expected)
		expectedPath := filepath.Join(root, file.expected)
		actualPath := filepath.Join(inspectDir, "root", file.actual)
		require.NoError(t, os.MkdirAll(filepath.Dir(expectedPath), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Dir(actualPath), 0o755))
		require.NoError(t, os.WriteFile(expectedPath, contents, 0o644))
		require.NoError(t, os.WriteFile(actualPath, contents, 0o644))
	}
	builder := NewBuilder(root, config.DistroSpec{}, &recordingRunner{})
	require.NoError(t, builder.validateExtractedBranding(inspectDir))
	require.NoError(t, os.WriteFile(filepath.Join(inspectDir, "root", "usr", "share", "anaconda", "pixmaps", "soda.css"), []byte("not Soda"), 0o644))
	require.EqualError(t, builder.validateExtractedBranding(inspectDir), "ISO Anaconda branding differs from the Soda installer contract")
}

func TestInspectionTreeBecomesReadableAndRemovableByOwner(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "root", "usr", "share", "anaconda")
	require.NoError(t, os.MkdirAll(directory, 0o755))
	file := filepath.Join(directory, "interactive-defaults.ks")
	require.NoError(t, os.WriteFile(file, []byte("kickstart"), 0o600))
	require.NoError(t, os.Chmod(file, 0o400))
	require.NoError(t, os.Chmod(directory, 0o500))

	require.NoError(t, makeInspectionOwnerWritable(root))
	contents, err := os.ReadFile(file)
	require.NoError(t, err)
	require.Equal(t, "kickstart", string(contents))
	require.NoError(t, os.RemoveAll(filepath.Join(root, "root")))
}

func TestValidateNoEmbeddedPayloadRejectsTheExactRuntimeDigest(t *testing.T) {
	manifestDigest := strings.TrimPrefix(testExactImage, Repository+"@")
	metadata := []byte(`[{"names":["localhost/soda-installer"],"digest":"` + manifestDigest + `"}]`)
	require.EqualError(t, validateNoEmbeddedPayload(metadata, testExactImage), "ISO embeds the Soda runtime payload instead of using the exact remote image reference")

	malformed := []byte(`[{"names":`)
	require.ErrorContains(t, validateNoEmbeddedPayload(malformed, testExactImage), "decode ISO container storage metadata")

	remoteReference := []byte(`[{"names":["` + testExactImage + `"],"digest":"sha256:` + strings.Repeat("b", 64) + `"}]`)
	require.EqualError(t, validateNoEmbeddedPayload(remoteReference, testExactImage), "ISO embeds the Soda runtime payload instead of using the exact remote image reference")

	otherImage := []byte(`[{"names":["localhost/soda-installer"],"digest":"sha256:` + strings.Repeat("b", 64) + `"}]`)
	require.NoError(t, validateNoEmbeddedPayload(otherImage, testExactImage))
}

func TestValidateExtractedPayloadAcceptsNoContainerStorage(t *testing.T) {
	inspectDir := t.TempDir()
	require.NoError(t, validateExtractedPayload(inspectDir, testExactImage))

	metadataPath := filepath.Join(inspectDir, "root", "var", "lib", "containers", "storage", "overlay-images", "images.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(metadataPath), 0o755))
	require.NoError(t, os.WriteFile(metadataPath, []byte(`[{"names":["`+testExactImage+`"],"digest":"sha256:`+strings.Repeat("b", 64)+`"}]`), 0o644))
	require.EqualError(t, validateExtractedPayload(inspectDir, testExactImage), "ISO embeds the Soda runtime payload instead of using the exact remote image reference")
}

func TestISOConfigRequiresExactStage2KernelAndInitrdContract(t *testing.T) {
	expected := []byte("label: \"SodaOS-Installer\"\ngrub2:\n  default: 0\n  timeout: 10\n  entries:\n    - name: \"Install Soda OS\"\n      linux: \"/images/pxeboot/vmlinuz inst.stage2=hd:LABEL=SodaOS-Installer inst.ks=hd:LABEL=OEMDRV:/ks.cfg inst.nosave=all_ks console=tty0 inst.graphical enforcing=0\"\n      initrd: \"/images/pxeboot/initrd.img\"\n")
	root := t.TempDir()
	inspectDir := t.TempDir()
	expectedDir := filepath.Join(root, "packaging", "installer")
	extractedStorageDir := filepath.Join(inspectDir, "root", "etc", "anaconda", "conf.d")
	extractedConfigDir := filepath.Join(inspectDir, "root", "usr", "lib", "image-builder", "bootc")
	require.NoError(t, os.MkdirAll(expectedDir, 0o755))
	require.NoError(t, os.MkdirAll(extractedStorageDir, 0o755))
	require.NoError(t, os.MkdirAll(extractedConfigDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(expectedDir, "iso-aarch64.yaml"), expected, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(expectedDir, "soda-storage.conf"), []byte("valid storage config\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(extractedStorageDir, "90-soda-storage.conf"), []byte("valid storage config\n"), 0o644))
	actualPath := filepath.Join(extractedConfigDir, "iso.yaml")
	require.NoError(t, os.WriteFile(actualPath, expected, 0o644))
	builder := NewBuilder(root, config.DistroSpec{Platform: config.PlatformSpec{Installer: config.PlatformInstaller{ISOConfig: "packaging/installer/iso-aarch64.yaml"}}}, &recordingRunner{})
	require.NoError(t, builder.validateExtractedConfiguration(inspectDir))

	for name, malformed := range map[string]string{
		"stage2 label": strings.ReplaceAll(string(expected), "hd:LABEL=SodaOS-Installer", "hd:LABEL=Wrong"),
		"kernel path":  strings.ReplaceAll(string(expected), "/images/pxeboot/vmlinuz", "/wrong/vmlinuz"),
		"initrd path":  strings.ReplaceAll(string(expected), "/images/pxeboot/initrd.img", "/wrong/initrd.img"),
	} {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, os.WriteFile(actualPath, []byte(malformed), 0o644))
			require.EqualError(t, builder.validateExtractedConfiguration(inspectDir), "ISO boot configuration differs from the Soda installer contract")
		})
	}
}

func TestValidatePinsArm64ToolAndExactRepository(t *testing.T) {
	root := t.TempDir()
	lock := filepath.Join(root, "image-builder.lock")
	require.NoError(t, os.WriteFile(lock, []byte(`version = "81.0.0"
commit = "3130fb87ee1f684b6e9d1909f354861c43d7a092"
reference = "ghcr.io/osbuild/image-builder@sha256:704dc05d6033799248a33c415f7f7253ec20b40f0b2bff03b06d8687179e058a"
platform = "linux/arm64"
`), 0o644))
	options := Options{ArchivePath: filepath.Join(root, "runtime.oci.tar"), ToolLock: lock}
	require.NoError(t, os.WriteFile(options.ArchivePath, []byte("input"), 0o644))
	builder := NewBuilder(root, config.DistroSpec{
		Identity: config.IdentitySpec{Architecture: "aarch64", Hostname: "soda"},
		Base:     config.BaseSpec{Platform: "linux/arm64"},
		Platform: config.PlatformSpec{Architecture: config.PlatformArchitecture{Name: "aarch64", OCI: "arm64", Platform: "linux/arm64"}},
	}, &recordingRunner{})
	actual, err := builder.validate(options)
	require.NoError(t, err)
	require.Equal(t, "81.0.0", actual.Version)
}

func TestSelectedToolLockAcceptsTheConfiguredRelativePath(t *testing.T) {
	root := t.TempDir()
	builder := NewBuilder(root, config.DistroSpec{Platform: config.PlatformSpec{Installer: config.PlatformInstaller{ToolLock: "distro/locks/image-builder.toml"}}}, &recordingRunner{})
	require.NoError(t, builder.validateSelectedToolLock("distro/locks/image-builder.toml", "installer"))
	require.ErrorContains(t, builder.validateSelectedToolLock("distro/locks/other.toml", "installer"), "must use the selected platform image-builder lock")
}

func TestArchiveReferenceDerivesExactDigestFromOneMatchingArm64Manifest(t *testing.T) {
	archive, digest := writeTestOCIArchiveAt(t, filepath.Join(t.TempDir(), "runtime.oci.tar"))
	reference, err := archiveReference(archive, "arm64")
	require.NoError(t, err)
	require.Equal(t, Repository+"@"+digest, reference)
	_, err = archiveReference(archive, "amd64")
	require.ErrorContains(t, err, "must be linux/amd64")
}

func TestBuildRejectsMismatchedHostBeforeValidatingInputs(t *testing.T) {
	runner := &recordingRunner{}
	builder := &Builder{
		Spec:             config.DistroSpec{Platform: config.PlatformSpec{Architecture: config.PlatformArchitecture{Name: "aarch64"}}},
		hostArchitecture: "amd64",
		runner:           runner,
	}
	_, err := builder.Build(context.Background(), Options{})
	require.EqualError(t, err, "Soda aarch64 artifact operations require a native arm64 host; running on amd64")
	require.Empty(t, runner.Commands)
}

func writeTestOCIArchiveAt(t *testing.T, archive string) (string, string) {
	t.Helper()
	image, err := mutate.ConfigFile(empty.Image, &v1.ConfigFile{Architecture: "arm64", OS: "linux"})
	require.NoError(t, err)
	digest, err := image.Digest()
	require.NoError(t, err)
	directory := t.TempDir()
	path, err := layout.Write(directory, empty.Index)
	require.NoError(t, err)
	require.NoError(t, path.AppendImage(image, layout.WithPlatform(v1.Platform{OS: "linux", Architecture: "arm64"})))
	require.NoError(t, os.MkdirAll(filepath.Dir(archive), 0o755))
	require.NoError(t, exec.Command("tar", "-cf", archive, "-C", directory, ".").Run())
	return archive, digest.String()
}
