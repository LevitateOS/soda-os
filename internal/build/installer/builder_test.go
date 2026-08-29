package installer

import (
	"context"
	"fmt"
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
	require.Contains(t, contents, "text\n")
	require.NotContains(t, contents, "graphical\n")
	require.Contains(t, contents, "network --bootproto=dhcp --device=link --activate --onboot=on --hostname=soda")
	require.Contains(t, contents, `--source-imgref="containers-storage:`+testExactImage+`"`)
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
	require.NoError(t, os.WriteFile(filepath.Join(expectedDir, "iso.yaml"), []byte("valid ISO config\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(bootConfigDir, "iso.yaml"), []byte("valid ISO config\n"), 0o644))
	actualPath := filepath.Join(extractedDir, "90-soda-storage.conf")
	require.NoError(t, os.WriteFile(actualPath, expected, 0o644))
	builder := NewBuilder(root, config.DistroSpec{Platform: config.PlatformSpec{Installer: config.PlatformInstaller{ISOConfig: "packaging/installer/iso.yaml"}}}, &recordingRunner{})
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

func TestInstallerEnvironmentPinsLegacyGRUBHybridBootModule(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	contents, err := os.ReadFile(filepath.Join(root, "packaging", "installer", "Containerfile"))
	require.NoError(t, err)
	require.Contains(t, string(contents), "rpm -q $(cat /usr/share/soda-installer/boot-packages.txt)")
	require.Contains(t, string(contents), "test -f /usr/lib/grub/i386-pc/boot_hybrid.img")
	for _, architecture := range []string{"aarch64", "x86_64"} {
		lock, readErr := os.ReadFile(filepath.Join(root, "distro", "locks", "installer-packages-"+architecture+".toml"))
		require.NoError(t, readErr)
		require.Contains(t, string(lock), "grub2-pc-modules-1:2.12-64.fc44.noarch")
		require.Contains(t, string(lock), "shim-")
	}
}

func TestInstallerEnvironmentUsesAnacondaGeneratorCompatibleDefaultTarget(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "installer", "Containerfile"))
	require.NoError(t, err)

	// Fedora 44's anaconda-generator compares readlink output literally with the /lib path.
	require.Contains(t, string(contents), "ln -sf /lib/systemd/system/anaconda.target /etc/systemd/system/default.target")
	require.NotContains(t, string(contents), "ln -sf /usr/lib/systemd/system/anaconda.target /etc/systemd/system/default.target")
}

func TestInstallerEnvironmentUsesVerifiedLocalFedoraBaseContext(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "installer", "Containerfile"))
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(contents), "FROM fedora-base\n"))
	require.Contains(t, string(contents), "COPY --chmod=0644 .artifacts/installer/context/interactive-defaults.ks /usr/share/anaconda/interactive-defaults.ks")
}

func TestPayloadStagingReferenceUsesFullExactDigestOnlyForImageBuilderStorage(t *testing.T) {
	expected := Repository + ":payload-" + strings.TrimPrefix(testExactImage, Repository+"@sha256:")
	require.Equal(t, expected, payloadStagingReference(testExactImage))
	require.NotEqual(t, testExactImage, payloadStagingReference(testExactImage))
}

func TestValidateEmbeddedPayloadRequiresStagingTagAndOriginalManifestDigest(t *testing.T) {
	payloadTag := payloadStagingReference(testExactImage)
	manifestDigest := strings.TrimPrefix(testExactImage, Repository+"@")
	metadata := []byte(`[{"names":["` + payloadTag + `"],"digest":"` + manifestDigest + `"}]`)
	require.NoError(t, validateEmbeddedPayload(metadata, payloadTag, testExactImage))

	malformed := []byte(`[{"names":`)
	require.ErrorContains(t, validateEmbeddedPayload(malformed, payloadTag, testExactImage), "decode embedded container storage metadata")

	missingTag := []byte(`[{"names":["` + testExactImage + `"],"digest":"` + manifestDigest + `"}]`)
	require.EqualError(t, validateEmbeddedPayload(missingTag, payloadTag, testExactImage), "ISO container storage does not contain the staged Soda payload and exact manifest digest")

	wrongDigest := []byte(`[{"names":["` + payloadTag + `"],"digest":"sha256:` + strings.Repeat("b", 64) + `"}]`)
	require.EqualError(t, validateEmbeddedPayload(wrongDigest, payloadTag, testExactImage), "ISO container storage does not contain the staged Soda payload and exact manifest digest")
}

func TestISOConfigRequiresExactStage2KernelAndInitrdContract(t *testing.T) {
	expected := []byte("label: \"SodaOS-Installer\"\ngrub2:\n  default: 0\n  timeout: 10\n  entries:\n    - name: \"Install Soda OS\"\n      linux: \"/images/pxeboot/vmlinuz inst.stage2=hd:LABEL=SodaOS-Installer console=tty0 enforcing=0\"\n      initrd: \"/images/pxeboot/initrd.img\"\n")
	root := t.TempDir()
	inspectDir := t.TempDir()
	expectedDir := filepath.Join(root, "packaging", "installer")
	extractedStorageDir := filepath.Join(inspectDir, "root", "etc", "anaconda", "conf.d")
	extractedConfigDir := filepath.Join(inspectDir, "root", "usr", "lib", "image-builder", "bootc")
	require.NoError(t, os.MkdirAll(expectedDir, 0o755))
	require.NoError(t, os.MkdirAll(extractedStorageDir, 0o755))
	require.NoError(t, os.MkdirAll(extractedConfigDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(expectedDir, "iso.yaml"), expected, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(expectedDir, "soda-storage.conf"), []byte("valid storage config\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(extractedStorageDir, "90-soda-storage.conf"), []byte("valid storage config\n"), 0o644))
	actualPath := filepath.Join(extractedConfigDir, "iso.yaml")
	require.NoError(t, os.WriteFile(actualPath, expected, 0o644))
	builder := NewBuilder(root, config.DistroSpec{Platform: config.PlatformSpec{Installer: config.PlatformInstaller{ISOConfig: "packaging/installer/iso.yaml"}}}, &recordingRunner{})
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
	options := Options{ImageReference: testExactImage, ToolLock: lock}
	for _, target := range []*string{&options.ArchivePath, &options.RegistryCA, &options.PublicKey, &options.CosignPath} {
		*target = filepath.Join(root, strings.ReplaceAll(strings.TrimPrefix(*target, root), "/", "")+"input")
		require.NoError(t, os.WriteFile(*target, []byte("input"), 0o644))
	}
	builder := NewBuilder(root, config.DistroSpec{
		Identity: config.IdentitySpec{Architecture: "aarch64", Hostname: "soda"},
		Base:     config.BaseSpec{Platform: "linux/arm64"},
		Platform: config.PlatformSpec{Architecture: config.PlatformArchitecture{Name: "aarch64", OCI: "arm64", Platform: "linux/arm64"}},
	}, &recordingRunner{})
	actual, err := builder.validate(options)
	require.NoError(t, err)
	require.Equal(t, "81.0.0", actual.Version)

	options.ImageReference = Repository + ":current"
	_, err = builder.validate(options)
	require.EqualError(t, err, "installer payload must be an exact registry.soda.local/soda/os@sha256 reference")
}

func TestVerifySignedImageUsesPinnedKeyAndExactDigest(t *testing.T) {
	runner := &recordingRunner{Outputs: map[string]string{"cosign version": "GitVersion: v3.1.2\n"}}
	builder := NewBuilder("/workspace", config.DistroSpec{}, runner)
	options := Options{ImageReference: testExactImage, RegistryCA: "/keys/ca.crt", PublicKey: "/keys/cosign.pub", CosignPath: "cosign"}
	require.NoError(t, builder.verifySignedImage(context.Background(), options))
	require.Equal(t, []string{
		"cosign version",
		"cosign verify --key /keys/cosign.pub --registry-cacert /keys/ca.crt --insecure-ignore-tlog=true " + testExactImage,
	}, []string{runner.Commands[0].String(), runner.Commands[1].String()})
}

func TestVerifyArchiveDigestRequiresOneMatchingArm64Manifest(t *testing.T) {
	archive, digest := writeTestOCIArchiveAt(t, filepath.Join(t.TempDir(), "runtime.oci.tar"))
	exact := Repository + "@" + digest
	require.NoError(t, verifyArchiveDigest(archive, exact, "arm64"))

	err := verifyArchiveDigest(archive, Repository+"@sha256:"+strings.Repeat("f", 64), "arm64")
	require.ErrorContains(t, err, "differs from exact payload")
}

func TestBuildDoesNotDeleteRuntimeArchiveFromOutputDirectory(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, ".artifacts", "images")
	require.NoError(t, os.MkdirAll(output, 0o755))
	archive, digest := writeTestOCIArchiveAt(t, filepath.Join(output, "runtime.oci.tar"))
	lock := filepath.Join(root, "image-builder.lock")
	require.NoError(t, os.WriteFile(lock, []byte(`version = "81.0.0"
commit = "3130fb87ee1f684b6e9d1909f354861c43d7a092"
reference = "ghcr.io/osbuild/image-builder@sha256:704dc05d6033799248a33c415f7f7253ec20b40f0b2bff03b06d8687179e058a"
platform = "linux/arm64"
`), 0o644))
	options := Options{ImageReference: Repository + "@" + digest, ArchivePath: archive, ToolLock: lock, OutputDir: output, CosignPath: filepath.Join(root, "cosign"), RegistryCA: filepath.Join(root, "ca"), PublicKey: filepath.Join(root, "pub")}
	for _, path := range []string{options.CosignPath, options.RegistryCA, options.PublicKey} {
		require.NoError(t, os.WriteFile(path, []byte("input"), 0o644))
	}
	runner := &recordingRunner{Outputs: map[string]string{options.CosignPath + " version": "GitVersion: v3.1.2\n"}}
	packageLock := filepath.Join(root, "installer-packages.toml")
	require.NoError(t, os.WriteFile(packageLock, []byte("schema_version = 1\nplatform = \"linux/arm64\"\npackages = [\"anaconda\"]\nboot_packages = [\"shim-aa64\"]\nefi_vendor = \"fedora\"\n"), 0o644))
	isoConfig := filepath.Join(root, "iso.yaml")
	require.NoError(t, os.WriteFile(isoConfig, []byte("test ISO config\n"), 0o644))
	platform := config.PlatformSpec{
		Architecture: config.PlatformArchitecture{Name: "aarch64", OCI: "arm64", Platform: "linux/arm64", Artifact: "aarch64", Installer: "aarch64"},
		Base:         config.PlatformBase{Reference: "quay.io/fedora/fedora-bootc@sha256:85677d47c03b2e1f8f9a3a19d838023ea154229817d579d4b4da5b87a21c9c1a", Archive: "unused.oci.tar", ArchiveSHA256: strings.Repeat("a", 64)},
		Installer:    config.PlatformInstaller{PackageLock: packageLock, ISOConfig: isoConfig},
	}
	builder := NewBuilder(root, config.DistroSpec{Identity: config.IdentitySpec{Architecture: "aarch64", Hostname: "soda", Version: "0.2.0"}, Base: config.BaseSpec{Reference: platform.Base.Reference, Platform: platform.Architecture.Platform}, Platform: platform}, runner)
	_, err := builder.Build(context.Background(), options)
	require.ErrorContains(t, err, "image-builder did not create")
	require.FileExists(t, archive)
	volumeName := fmt.Sprintf("soda-installer-%s-%d", strings.TrimPrefix(options.ImageReference, Repository+"@sha256:")[:12], os.Getpid())
	commands := make([]string, 0, len(runner.Commands))
	for _, command := range runner.Commands {
		commands = append(commands, command.String())
	}
	require.Contains(t, commands, "docker volume create "+volumeName)
	require.Contains(t, commands, "docker volume rm --force "+volumeName)
	payloadTag := payloadStagingReference(options.ImageReference)
	require.Contains(t, strings.Join(commands, "\n"), "containers-storage:"+payloadTag)
	require.Contains(t, strings.Join(commands, "\n"), "--build-context fedora-base=docker-image://soda-fedora-bootc:sha256-85677d47c03b2e1f8f9a3a19d838023ea154229817d579d4b4da5b87a21c9c1a")
	require.Contains(t, strings.Join(commands, "\n"), "--bootc-installer-payload-ref "+payloadTag)
	require.NotContains(t, strings.Join(commands, "\n"), "--bootc-installer-payload-ref "+options.ImageReference)
	require.NotContains(t, strings.Join(commands, "\n"), root+"/.artifacts/installer/containers-storage:/var/lib/containers/storage")
	require.Contains(t, strings.Join(commands, "\n"), volumeName+":/var/lib/containers/storage")
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
