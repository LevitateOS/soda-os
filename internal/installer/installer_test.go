package installer

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/config"
	imagebuild "github.com/LevitateOS/soda-os/internal/image"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/stretchr/testify/require"
)

const testExactImage = Repository + "@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestKickstartKeepsStockInteractiveFlowAndExactDigest(t *testing.T) {
	contents := kickstart(testExactImage, "soda")
	require.Contains(t, contents, "text\n")
	require.NotContains(t, contents, "graphical\n")
	require.Contains(t, contents, "network --bootproto=dhcp --device=link --activate --onboot=on --hostname=soda")
	require.Contains(t, contents, `--source-imgref="containers-storage:`+testExactImage+`"`)
	require.Contains(t, contents, `--target-imgref="`+testExactImage+`"`)
	require.NotContains(t, contents, "--enforce-container-sigpolicy")
	require.NotContains(t, contents, "selinux=0")
	require.NotContains(t, contents, "clearpart")
	require.NotContains(t, contents, "user --name")
}

func TestInstallerEnvironmentPinsLegacyGRUBHybridBootModule(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", "packaging", "installer", "Containerfile"))
	require.NoError(t, err)
	require.Contains(t, string(contents), "grub2-pc-modules-1:2.12-64.fc44.noarch")
	require.Contains(t, string(contents), "rpm -q shim-aa64 grub2-efi-aa64 grub2-efi-aa64-cdboot grub2-pc-modules")
	require.Contains(t, string(contents), "test -f /usr/lib/grub/i386-pc/boot_hybrid.img")
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
	require.NoError(t, validateISOConfig(expected, expected))

	for name, malformed := range map[string]string{
		"stage2 label": strings.ReplaceAll(string(expected), "hd:LABEL=SodaOS-Installer", "hd:LABEL=Wrong"),
		"kernel path":  strings.ReplaceAll(string(expected), "/images/pxeboot/vmlinuz", "/wrong/vmlinuz"),
		"initrd path":  strings.ReplaceAll(string(expected), "/images/pxeboot/initrd.img", "/wrong/initrd.img"),
	} {
		t.Run(name, func(t *testing.T) {
			require.EqualError(t, validateISOConfig([]byte(malformed), expected), "ISO boot configuration differs from the Soda installer contract")
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
		Base:     config.BaseSpec{Platform: Platform},
	}, &imagebuild.RecordingRunner{})
	actual, err := builder.validate(options)
	require.NoError(t, err)
	require.Equal(t, "81.0.0", actual.Version)

	options.ImageReference = Repository + ":current"
	_, err = builder.validate(options)
	require.EqualError(t, err, "installer payload must be an exact registry.soda.local/soda/os@sha256 reference")
}

func TestVerifySignedImageUsesPinnedKeyAndExactDigest(t *testing.T) {
	runner := &imagebuild.RecordingRunner{Outputs: map[string]string{"cosign version": "GitVersion: v3.1.2\n"}}
	builder := NewBuilder("/workspace", config.DistroSpec{}, runner)
	options := Options{ImageReference: testExactImage, RegistryCA: "/keys/ca.crt", PublicKey: "/keys/cosign.pub", CosignPath: "cosign"}
	require.NoError(t, builder.verifySignedImage(context.Background(), options))
	require.Equal(t, []string{
		"cosign version",
		"cosign verify --key /keys/cosign.pub --registry-cacert /keys/ca.crt --insecure-ignore-tlog=true " + testExactImage,
	}, []string{runner.Commands[0].String(), runner.Commands[1].String()})
}

func TestValidateProvenanceBindsISOBytesAndPayload(t *testing.T) {
	iso := filepath.Join(t.TempDir(), "SodaOS.iso")
	require.NoError(t, os.WriteFile(iso, []byte("iso bytes"), 0o644))
	provenance := Provenance{
		SchemaVersion: 1, ISOPath: filepath.Base(iso), ISOSHA256: mustSHA256(t, iso),
		EmbeddedImageReference: testExactImage, Platform: Platform, Filesystem: "ext4",
		ImageBuilderVersion:   "81.0.0",
		ImageBuilderReference: "ghcr.io/osbuild/image-builder@sha256:704dc05d6033799248a33c415f7f7253ec20b40f0b2bff03b06d8687179e058a",
	}
	encoded, err := json.Marshal(provenance)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(iso+".payload.json", encoded, 0o644))
	actual, err := ValidateProvenance(iso, testExactImage)
	require.NoError(t, err)
	require.Equal(t, provenance, actual)

	require.NoError(t, os.WriteFile(iso, []byte("changed"), 0o644))
	_, err = ValidateProvenance(iso, testExactImage)
	require.EqualError(t, err, "installer payload provenance checksum does not match the ISO")
}

func TestVerifyArchiveDigestRequiresOneMatchingArm64Manifest(t *testing.T) {
	archive, digest := writeTestOCIArchive(t)
	exact := Repository + "@" + digest
	require.NoError(t, verifyArchiveDigest(archive, exact))

	err := verifyArchiveDigest(archive, Repository+"@sha256:"+strings.Repeat("f", 64))
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
	runner := &imagebuild.RecordingRunner{Outputs: map[string]string{options.CosignPath + " version": "GitVersion: v3.1.2\n"}}
	builder := NewBuilder(root, config.DistroSpec{Identity: config.IdentitySpec{Architecture: "aarch64", Hostname: "soda", Version: "0.2.0"}, Base: config.BaseSpec{Platform: Platform}}, runner)
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
	require.Contains(t, strings.Join(commands, "\n"), "--bootc-installer-payload-ref "+payloadTag)
	require.NotContains(t, strings.Join(commands, "\n"), "--bootc-installer-payload-ref "+options.ImageReference)
	require.NotContains(t, strings.Join(commands, "\n"), root+"/.artifacts/installer/containers-storage:/var/lib/containers/storage")
	require.Contains(t, strings.Join(commands, "\n"), volumeName+":/var/lib/containers/storage")
}

func mustSHA256(t *testing.T, path string) string {
	t.Helper()
	digest, err := fileSHA256(path)
	require.NoError(t, err)
	return digest
}

func writeTestOCIArchive(t *testing.T) (string, string) {
	t.Helper()
	return writeTestOCIArchiveAt(t, filepath.Join(t.TempDir(), "runtime.oci.tar"))
}

func writeTestOCIArchiveAt(t *testing.T, archive string) (string, string) {
	t.Helper()
	img, err := mutate.ConfigFile(empty.Image, &v1.ConfigFile{Architecture: "arm64", OS: "linux"})
	require.NoError(t, err)
	digest, err := img.Digest()
	require.NoError(t, err)
	directory := t.TempDir()
	path, err := layout.Write(directory, empty.Index)
	require.NoError(t, err)
	require.NoError(t, path.AppendImage(img, layout.WithPlatform(v1.Platform{OS: "linux", Architecture: "arm64"})))
	require.NoError(t, os.MkdirAll(filepath.Dir(archive), 0o755))
	output, err := os.Create(archive)
	require.NoError(t, err)
	writer := tar.NewWriter(output)
	require.NoError(t, filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == directory {
			return nil
		}
		relative, err := filepath.Rel(directory, path)
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relative)
		if err := writer.WriteHeader(header); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			input, err := os.Open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(writer, input)
			closeErr := input.Close()
			if copyErr != nil {
				return copyErr
			}
			return closeErr
		}
		return nil
	}))
	require.NoError(t, writer.Close())
	require.NoError(t, output.Close())
	return archive, digest.String()
}
