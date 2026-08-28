package release

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/config"
	imagebuild "github.com/LevitateOS/soda-os/internal/image"
	"github.com/LevitateOS/soda-os/internal/installer"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/stretchr/testify/require"
)

const testRevision = "2b6b23e356ded84d4ef7fee52b242ae4855793ca"

var (
	testRegistryCA = []byte("test registry CA\n")
	testPublicKey  = []byte("test public key\n")
)

func TestPublishUsesExactDigestAndUpdatesCurrentLast(t *testing.T) {
	img := testImage(t, true)
	archive := writeOCIArchive(t, img)
	events := []string{}
	registry := &fakeRegistry{image: img, events: &events}
	signer := &fakeSigner{events: &events}
	publisher := &Publisher{Spec: testSpec(), Registry: registry, Signer: signer}
	output := t.TempDir()

	result, err := publisher.Publish(context.Background(), testOptions(t, archive, output))
	require.NoError(t, err)
	digest, err := img.Digest()
	require.NoError(t, err)
	exact := Repository + "@" + digest.String()
	require.Equal(t, exact, result.ImageReference)
	require.Equal(t, []string{
		"cosign-version",
		"push:" + Repository + ":0.2.0",
		"resolve:" + Repository + ":0.2.0",
		"sign-image:" + exact,
		"verify-image:" + exact,
		"sign-blob",
		"verify-blob",
		"push:" + Repository + ":current",
	}, events)

	contents, err := os.ReadFile(result.RecordPath)
	require.NoError(t, err)
	require.True(t, bytes.HasSuffix(contents, []byte("\n")))
	require.NotContains(t, string(contents), "  \"")
	var record Record
	require.NoError(t, json.Unmarshal(contents, &record))
	require.Equal(t, exact, record.SodaImageReference)
	require.Equal(t, testRevision, record.SourceRevision)
	require.Equal(t, uint32(2), record.StateSchema)
	require.Equal(t, sha256Hex([]byte("rpm inventory\n")), record.RPMInventorySHA256)
	require.Empty(t, record.ISOChecksum)
}

func TestPublishDeferredSignsExactImageWithoutRecordOrCurrent(t *testing.T) {
	img := testImage(t, true)
	archive := writeOCIArchive(t, img)
	events := []string{}
	registry := &fakeRegistry{image: img, events: &events}
	publisher := &Publisher{Spec: testSpec(), Registry: registry, Signer: &fakeSigner{events: &events}}
	output := filepath.Join(t.TempDir(), "deferred-release")
	options := testOptions(t, archive, output)
	options.DeferCurrent = true

	result, err := publisher.Publish(context.Background(), options)
	require.NoError(t, err)
	digest, err := img.Digest()
	require.NoError(t, err)
	exact := Repository + "@" + digest.String()
	require.Equal(t, exact, result.ImageReference)
	require.Empty(t, result.RecordPath)
	require.Empty(t, result.BundlePath)
	require.NoDirExists(t, output)
	require.Equal(t, []string{
		"cosign-version",
		"push:" + Repository + ":0.2.0",
		"resolve:" + Repository + ":0.2.0",
		"sign-image:" + exact,
		"verify-image:" + exact,
	}, events)
}

func TestPublishWithISOBindsExactDigestAndUpdatesCurrentLast(t *testing.T) {
	img := testImage(t, true)
	archive := writeOCIArchive(t, img)
	digest, err := img.Digest()
	require.NoError(t, err)
	exact := Repository + "@" + digest.String()
	iso := writeInstallerISO(t)
	events := []string{}
	registry := &fakeRegistry{image: img, events: &events}
	validator := &fakeISOValidator{}
	publisher := &Publisher{Spec: testSpec(), Registry: registry, Signer: &fakeSigner{events: &events}, ISOValidator: validator}
	options := testOptions(t, archive, t.TempDir())
	options.ISOPath = iso

	result, err := publisher.Publish(context.Background(), options)
	require.NoError(t, err)
	require.Equal(t, 1, validator.calls)
	require.Equal(t, []string{
		"cosign-version",
		"push:" + Repository + ":0.2.0",
		"resolve:" + Repository + ":0.2.0",
		"sign-image:" + exact,
		"verify-image:" + exact,
		"sign-blob",
		"verify-blob",
		"push:" + Repository + ":current",
	}, events)

	contents, err := os.ReadFile(result.RecordPath)
	require.NoError(t, err)
	var record Record
	require.NoError(t, json.Unmarshal(contents, &record))
	require.Equal(t, exact, record.SodaImageReference)
	require.Equal(t, sha256Hex([]byte("installer bytes")), record.ISOChecksum)
}

func TestPublishRejectsCanonicalRegistryDigestMismatchBeforeSigning(t *testing.T) {
	img := testImage(t, true)
	events := []string{}
	registry := &fakeRegistry{image: img, digest: v1.Hash{Algorithm: "sha256", Hex: strings.Repeat("f", 64)}, events: &events}
	publisher := &Publisher{Spec: testSpec(), Registry: registry, Signer: &fakeSigner{events: &events}}

	_, err := publisher.Publish(context.Background(), testOptions(t, writeOCIArchive(t, img), t.TempDir()))
	require.ErrorContains(t, err, "canonical registry digest")
	require.Equal(t, []string{"cosign-version", "push:" + Repository + ":0.2.0", "resolve:" + Repository + ":0.2.0"}, events)
}

func TestPublishRejectsISOWithoutIndependentInspection(t *testing.T) {
	img := testImage(t, true)
	events := []string{}
	registry := &fakeRegistry{image: img, events: &events}
	validator := &fakeISOValidator{err: fmt.Errorf("embedded container storage mismatch")}
	publisher := &Publisher{Spec: testSpec(), Registry: registry, Signer: &fakeSigner{events: &events}, ISOValidator: validator}
	options := testOptions(t, writeOCIArchive(t, img), t.TempDir())
	options.ISOPath = filepath.Join(t.TempDir(), "forged.iso")
	require.NoError(t, os.WriteFile(options.ISOPath, []byte("arbitrary bytes"), 0o644))

	_, err := publisher.Publish(context.Background(), options)
	require.ErrorContains(t, err, "independently inspect installer ISO")
	require.Equal(t, 1, validator.calls)
	require.Equal(t, []string{
		"cosign-version",
		"push:" + Repository + ":0.2.0",
		"resolve:" + Repository + ":0.2.0",
	}, events)
}

func TestInspectRejectsRPMInventorySidecarMismatch(t *testing.T) {
	publisher := &Publisher{Spec: testSpec()}
	options := testOptions(t, "", t.TempDir())
	_, err := publisher.inspect(testImage(t, false), Repository+"@sha256:"+strings.Repeat("a", 64), options.RegistryCA, options.PublicKey)
	require.EqualError(t, err, "installed RPM inventory does not match its image sidecar")
}

func TestInspectRejectsTrustInputMismatch(t *testing.T) {
	for name, change := range map[string]struct {
		path     func(Options) string
		expected string
	}{
		"registry CA": {func(options Options) string { return options.RegistryCA }, "supplied registry CA differs from the file embedded in the release image"},
		"public key":  {func(options Options) string { return options.PublicKey }, "supplied signing public key differs from the file embedded in the release image"},
	} {
		t.Run(name, func(t *testing.T) {
			publisher := &Publisher{Spec: testSpec()}
			options := testOptions(t, "", t.TempDir())
			require.NoError(t, os.WriteFile(change.path(options), []byte("different trust input\n"), 0o644))
			_, err := publisher.inspect(testImage(t, true), Repository+"@sha256:"+strings.Repeat("a", 64), options.RegistryCA, options.PublicKey)
			require.EqualError(t, err, change.expected)
		})
	}
}

func TestCosignCommandsPinVersionAndExactDigest(t *testing.T) {
	exact := Repository + "@sha256:" + strings.Repeat("a", 64)
	runner := &imagebuild.RecordingRunner{Outputs: map[string]string{"cosign version": "GitVersion:    v3.1.2\n"}}
	signer := &cosignSigner{runner: runner, executable: "cosign", ca: "/keys/registry-ca.crt", publicKey: "/keys/cosign.pub", privateKey: "/keys/cosign.key"}
	require.NoError(t, signer.CheckVersion(context.Background()))
	require.NoError(t, signer.SignImage(context.Background(), exact))
	require.NoError(t, signer.VerifyImage(context.Background(), exact))
	require.NoError(t, signer.SignBlob(context.Background(), "release.json", "release.sigstore.json"))
	require.NoError(t, signer.VerifyBlob(context.Background(), "release.json", "release.sigstore.json"))

	commands := make([]string, 0, len(runner.Commands))
	for _, command := range runner.Commands {
		commands = append(commands, command.String())
	}
	require.Equal(t, []string{
		"cosign version",
		"cosign sign --yes --use-signing-config=false --tlog-upload=false --registry-referrers-mode=legacy --new-bundle-format=false --key /keys/cosign.key --registry-cacert /keys/registry-ca.crt " + exact,
		"cosign verify --key /keys/cosign.pub --registry-cacert /keys/registry-ca.crt --insecure-ignore-tlog=true " + exact,
		"cosign sign-blob --yes --use-signing-config=false --tlog-upload=false --key /keys/cosign.key --bundle release.sigstore.json release.json",
		"cosign verify-blob --key /keys/cosign.pub --bundle release.sigstore.json --insecure-ignore-tlog=true release.json",
	}, commands)
}

func TestOCIArchiveRequiresExactlyOneArm64Manifest(t *testing.T) {
	img := testImage(t, true)
	index := mutate.AppendManifests(empty.Index,
		mutate.IndexAddendum{Add: img, Descriptor: v1.Descriptor{Platform: &v1.Platform{OS: "linux", Architecture: "arm64"}}},
		mutate.IndexAddendum{Add: img, Descriptor: v1.Descriptor{Platform: &v1.Platform{OS: "linux", Architecture: "amd64"}}},
	)
	archive := writeIndexArchive(t, index)
	_, cleanup, err := imageFromOCIArchive(archive)
	defer cleanup()
	require.EqualError(t, err, "OCI archive must contain exactly one manifest")
}

func TestCosignBinaryMustMatchAcquisitionLock(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "cosign")
	require.NoError(t, os.WriteFile(binary, []byte("pinned cosign"), 0o755))
	lock := filepath.Join(t.TempDir(), "tools.lock")
	contents := fmt.Sprintf("version = %q\n[[binary]]\nos = %q\narch = %q\nsha256 = %q\n", CosignVersion, runtime.GOOS, runtime.GOARCH, sha256Hex([]byte("pinned cosign")))
	require.NoError(t, os.WriteFile(lock, []byte(contents), 0o644))
	require.NoError(t, verifyCosignBinary(binary, lock))
	require.NoError(t, os.WriteFile(binary, []byte("different"), 0o755))
	require.ErrorContains(t, verifyCosignBinary(binary, lock), "differs from pinned")
}

func TestRemoteRegistryPushAndCanonicalResolveOverTLS(t *testing.T) {
	server := httptest.NewTLSServer(registry.New())
	defer server.Close()
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	caPath := filepath.Join(t.TempDir(), "registry-ca.crt")
	require.NoError(t, os.WriteFile(caPath, certificate, 0o644))
	transport, err := registryTransport(caPath)
	require.NoError(t, err)
	client := &remoteRegistry{options: []remote.Option{remote.WithTransport(transport)}}
	reference := strings.TrimPrefix(server.URL, "https://") + "/soda/os:0.2.0"
	img := testImage(t, true)
	require.NoError(t, client.Push(context.Background(), reference, img))
	got, err := client.Resolve(context.Background(), reference)
	require.NoError(t, err)
	want, err := img.Digest()
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestCosignExactImageDigestIntegration(t *testing.T) {
	cosign := os.Getenv("SODA_COSIGN_INTEGRATION")
	if cosign == "" {
		t.Skip("set SODA_COSIGN_INTEGRATION to the pinned Cosign binary")
	}
	server := httptest.NewTLSServer(registry.New())
	defer server.Close()
	caPath := filepath.Join(t.TempDir(), "registry-ca.crt")
	require.NoError(t, os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0o644))
	transport, err := registryTransport(caPath)
	require.NoError(t, err)
	client := &remoteRegistry{options: []remote.Option{remote.WithTransport(transport)}}
	versionTag := strings.TrimPrefix(server.URL, "https://") + "/soda/os:0.2.0"
	img := testImage(t, true)
	require.NoError(t, client.Push(context.Background(), versionTag, img))
	digest, err := client.Resolve(context.Background(), versionTag)
	require.NoError(t, err)
	exact := strings.TrimSuffix(versionTag, ":0.2.0") + "@" + digest.String()

	keyDir := t.TempDir()
	const testPassphrase = "ephemeral-integration-passphrase"
	t.Setenv("COSIGN_PASSWORD", testPassphrase)
	var processOutput bytes.Buffer
	runner := imagebuild.OSRunner{Stdout: &processOutput, Stderr: &processOutput}
	require.NoError(t, runner.Run(context.Background(), imagebuild.Command{Name: cosign, Args: []string{"generate-key-pair", "--output-key-prefix", filepath.Join(keyDir, "release")}}))
	signer := &cosignSigner{runner: runner, executable: cosign, ca: caPath, publicKey: filepath.Join(keyDir, "release.pub"), privateKey: filepath.Join(keyDir, "release.key")}
	require.NoError(t, signer.CheckVersion(context.Background()))
	unsignedImage := testImage(t, true)
	unsignedConfig, err := unsignedImage.ConfigFile()
	require.NoError(t, err)
	unsignedConfig.Config.Labels["org.sodaos.integration-unsigned"] = "true"
	unsignedImage, err = mutate.ConfigFile(unsignedImage, unsignedConfig)
	require.NoError(t, err)
	unsignedTag := strings.TrimSuffix(versionTag, ":0.2.0") + ":unsigned"
	require.NoError(t, client.Push(context.Background(), unsignedTag, unsignedImage))
	unsignedDigest, err := client.Resolve(context.Background(), unsignedTag)
	require.NoError(t, err)
	unsignedExact := strings.TrimSuffix(versionTag, ":0.2.0") + "@" + unsignedDigest.String()
	require.Error(t, signer.VerifyImage(context.Background(), unsignedExact))
	require.NoError(t, signer.SignImage(context.Background(), exact))
	require.NoError(t, signer.VerifyImage(context.Background(), exact), processOutput.String())

	require.NoError(t, runner.Run(context.Background(), imagebuild.Command{Name: cosign, Args: []string{"generate-key-pair", "--output-key-prefix", filepath.Join(keyDir, "wrong")}}))
	wrongVerifier := &cosignSigner{runner: runner, executable: cosign, ca: caPath, publicKey: filepath.Join(keyDir, "wrong.pub")}
	require.Error(t, wrongVerifier.VerifyImage(context.Background(), exact))

	require.NotContains(t, processOutput.String(), testPassphrase)
}

func TestCosignInteractivePassphraseIntegration(t *testing.T) {
	cosign := os.Getenv("SODA_COSIGN_INTERACTIVE")
	key := os.Getenv("SODA_COSIGN_INTERACTIVE_KEY")
	blob := os.Getenv("SODA_COSIGN_INTERACTIVE_BLOB")
	if cosign == "" || key == "" || blob == "" {
		t.Skip("set SODA_COSIGN_INTERACTIVE, SODA_COSIGN_INTERACTIVE_KEY, and SODA_COSIGN_INTERACTIVE_BLOB")
	}
	require.NoError(t, os.Unsetenv("COSIGN_PASSWORD"))
	bundle := blob + ".sigstore.json"
	signer := &cosignSigner{runner: imagebuild.OSRunner{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}, executable: cosign, privateKey: key}
	require.NoError(t, signer.SignBlob(context.Background(), blob, bundle))
}

type fakeRegistry struct {
	image  v1.Image
	digest v1.Hash
	events *[]string
}

func (r *fakeRegistry) Push(_ context.Context, reference string, _ v1.Image) error {
	*r.events = append(*r.events, "push:"+reference)
	return nil
}

func (r *fakeRegistry) Resolve(_ context.Context, reference string) (v1.Hash, error) {
	*r.events = append(*r.events, "resolve:"+reference)
	if r.digest.Hex != "" {
		return r.digest, nil
	}
	return r.image.Digest()
}

type fakeSigner struct{ events *[]string }

type fakeISOValidator struct {
	calls int
	err   error
}

func (v *fakeISOValidator) ValidateISO(_ context.Context, isoPath, reference, _ string, _ string) (installer.Provenance, error) {
	v.calls++
	if v.err != nil {
		return installer.Provenance{}, v.err
	}
	contents, err := os.ReadFile(isoPath)
	if err != nil {
		return installer.Provenance{}, err
	}
	return installer.Provenance{ISOPath: filepath.Base(isoPath), ISOSHA256: sha256Hex(contents), EmbeddedImageReference: reference}, nil
}

func (s *fakeSigner) CheckVersion(context.Context) error {
	*s.events = append(*s.events, "cosign-version")
	return nil
}
func (s *fakeSigner) SignImage(_ context.Context, reference string) error {
	*s.events = append(*s.events, "sign-image:"+reference)
	return nil
}
func (s *fakeSigner) VerifyImage(_ context.Context, reference string) error {
	*s.events = append(*s.events, "verify-image:"+reference)
	return nil
}
func (s *fakeSigner) SignBlob(context.Context, string, string) error {
	*s.events = append(*s.events, "sign-blob")
	return nil
}
func (s *fakeSigner) VerifyBlob(context.Context, string, string) error {
	*s.events = append(*s.events, "verify-blob")
	return nil
}

func testSpec() config.DistroSpec {
	return config.DistroSpec{
		Identity: config.IdentitySpec{Version: "0.2.0"},
		Base:     config.BaseSpec{Reference: "quay.io/fedora/fedora-bootc@sha256:" + strings.Repeat("b", 64)},
		Image:    config.ImageSpec{Registry: Repository, StateSchema: 2},
	}
}

func testImage(t *testing.T, matchingSidecar bool) v1.Image {
	t.Helper()
	inventory := []byte("rpm inventory\n")
	digest := sha256Hex(inventory)
	if !matchingSidecar {
		digest = strings.Repeat("0", 64)
	}
	var layer bytes.Buffer
	writer := tar.NewWriter(&layer)
	for name, contents := range map[string][]byte{
		"usr/share/soda/rpm-inventory.txt":                           inventory,
		"usr/share/soda/rpm-inventory.sha256":                        []byte(digest + "  rpm-inventory.txt\n"),
		"usr/share/pki/ca-trust-source/anchors/soda-registry-ca.crt": testRegistryCA,
		"usr/share/soda/release/cosign.pub":                          testPublicKey,
	} {
		require.NoError(t, writer.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(contents))}))
		_, err := writer.Write(contents)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	img, err := mutate.AppendLayers(empty.Image, static.NewLayer(layer.Bytes(), types.OCILayer))
	require.NoError(t, err)
	configFile, err := img.ConfigFile()
	require.NoError(t, err)
	configFile.OS = "linux"
	configFile.Architecture = "arm64"
	configFile.Config.Labels = map[string]string{
		"org.opencontainers.image.version":   "0.2.0",
		"org.opencontainers.image.revision":  testRevision,
		"org.opencontainers.image.base.name": testSpec().Base.Reference,
		"org.sodaos.state-schema":            "2",
	}
	img, err = mutate.ConfigFile(img, configFile)
	require.NoError(t, err)
	return img
}

func testOptions(t *testing.T, archive, output string) Options {
	t.Helper()
	directory := t.TempDir()
	ca := filepath.Join(directory, "registry-ca.crt")
	publicKey := filepath.Join(directory, "cosign.pub")
	require.NoError(t, os.WriteFile(ca, testRegistryCA, 0o644))
	require.NoError(t, os.WriteFile(publicKey, testPublicKey, 0o644))
	return Options{ArchivePath: archive, OutputDir: output, RegistryCA: ca, PublicKey: publicKey}
}

func writeInstallerISO(t *testing.T) string {
	t.Helper()
	iso := filepath.Join(t.TempDir(), "SodaOS.iso")
	require.NoError(t, os.WriteFile(iso, []byte("installer bytes"), 0o644))
	return iso
}

func writeOCIArchive(t *testing.T, img v1.Image) string {
	t.Helper()
	index := mutate.AppendManifests(empty.Index, mutate.IndexAddendum{Add: img, Descriptor: v1.Descriptor{Platform: &v1.Platform{OS: "linux", Architecture: "arm64"}}})
	return writeIndexArchive(t, index)
}

func writeIndexArchive(t *testing.T, index v1.ImageIndex) string {
	t.Helper()
	directory := filepath.Join(t.TempDir(), "layout")
	_, err := layout.Write(directory, index)
	require.NoError(t, err)
	archive := filepath.Join(t.TempDir(), "image.oci.tar")
	file, err := os.Create(archive)
	require.NoError(t, err)
	writer := tar.NewWriter(file)
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
		if info.IsDir() {
			return nil
		}
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
	}))
	require.NoError(t, writer.Close())
	require.NoError(t, file.Close())
	return archive
}
