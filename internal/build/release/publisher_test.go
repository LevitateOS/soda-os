package release

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/process"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/require"
)

const testRevision = "2b6b23e356ded84d4ef7fee52b242ae4855793ca"

var (
	testRegistryCA = []byte("test registry CA\n")
	testPublicKey  = []byte("test public key\n")
)

func TestPublishUsesExactDigestAndUpdatesCurrentLast(t *testing.T) {
	img := matchingTestImage(t)
	archive := writeOCIArchive(t, img)
	events := []string{}
	registry := &fakeRegistry{image: img, events: &events}
	signer := &fakeSigner{events: &events}
	publisher := testPublisher(t, &Publisher{spec: testSpec(), registry: registry, signer: signer})
	output := t.TempDir()

	result, err := publisher.Publish(context.Background(), PublicationOptions{ArchivePath: archive, OutputDir: output})
	require.NoError(t, err)
	digest, err := img.Digest()
	require.NoError(t, err)
	exact := Repository + "@" + digest.String()
	require.Equal(t, exact, result.ImageReference)
	require.Equal(t, []string{
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
	img := matchingTestImage(t)
	archive := writeOCIArchive(t, img)
	events := []string{}
	registry := &fakeRegistry{image: img, events: &events}
	publisher := testPublisher(t, &Publisher{spec: testSpec(), registry: registry, signer: &fakeSigner{events: &events}})
	output := filepath.Join(t.TempDir(), "deferred-release")
	reference, err := publisher.Prepare(context.Background(), archive)
	require.NoError(t, err)
	digest, err := img.Digest()
	require.NoError(t, err)
	exact := Repository + "@" + digest.String()
	require.Equal(t, exact, reference)
	require.NoDirExists(t, output)
	require.Equal(t, []string{
		"push:" + Repository + ":0.2.0",
		"resolve:" + Repository + ":0.2.0",
		"sign-image:" + exact,
		"verify-image:" + exact,
	}, events)
}

func TestPublishWithISOBindsExactDigestAndUpdatesCurrentLast(t *testing.T) {
	img := matchingTestImage(t)
	archive := writeOCIArchive(t, img)
	digest, err := img.Digest()
	require.NoError(t, err)
	exact := Repository + "@" + digest.String()
	iso := filepath.Join(t.TempDir(), "SodaOS.iso")
	require.NoError(t, os.WriteFile(iso, []byte("installer bytes"), 0o644))
	events := []string{}
	registry := &fakeRegistry{image: img, events: &events}
	validator := &fakeISOValidator{}
	publisher := testPublisher(t, &Publisher{spec: testSpec(), registry: registry, signer: &fakeSigner{events: &events}, isoValidator: validator})
	options := PublicationOptions{ArchivePath: archive, OutputDir: t.TempDir()}
	options.ISOPath = iso

	result, err := publisher.Publish(context.Background(), options)
	require.NoError(t, err)
	require.Equal(t, 1, validator.calls)
	require.Equal(t, []string{
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
	img := matchingTestImage(t)
	events := []string{}
	registry := &fakeRegistry{image: img, digest: v1.Hash{Algorithm: "sha256", Hex: strings.Repeat("f", 64)}, events: &events}
	publisher := testPublisher(t, &Publisher{spec: testSpec(), registry: registry, signer: &fakeSigner{events: &events}})

	_, err := publisher.Publish(context.Background(), PublicationOptions{ArchivePath: writeOCIArchive(t, img), OutputDir: t.TempDir()})
	require.ErrorContains(t, err, "canonical registry digest")
	require.Equal(t, []string{"push:" + Repository + ":0.2.0", "resolve:" + Repository + ":0.2.0"}, events)
}

func TestPublishRejectsISOWithoutIndependentInspection(t *testing.T) {
	img := matchingTestImage(t)
	events := []string{}
	registry := &fakeRegistry{image: img, events: &events}
	validator := &fakeISOValidator{err: fmt.Errorf("embedded container storage mismatch")}
	publisher := testPublisher(t, &Publisher{spec: testSpec(), registry: registry, signer: &fakeSigner{events: &events}, isoValidator: validator})
	options := PublicationOptions{ArchivePath: writeOCIArchive(t, img), OutputDir: t.TempDir()}
	options.ISOPath = filepath.Join(t.TempDir(), "forged.iso")
	require.NoError(t, os.WriteFile(options.ISOPath, []byte("arbitrary bytes"), 0o644))

	_, err := publisher.Publish(context.Background(), options)
	require.ErrorContains(t, err, "independently inspect installer ISO")
	require.Equal(t, 1, validator.calls)
	require.Equal(t, []string{
		"push:" + Repository + ":0.2.0",
		"resolve:" + Repository + ":0.2.0",
	}, events)
}

func TestInspectRejectsRPMInventorySidecarMismatch(t *testing.T) {
	registryCA, publicKey := testTrust(t)
	publisher := &Publisher{spec: testSpec(), registryCA: registryCA, publicKey: publicKey}
	_, err := publisher.inspect(testImageWithSidecar(t, strings.Repeat("0", 64)), Repository+"@sha256:"+strings.Repeat("a", 64))
	require.EqualError(t, err, "installed RPM inventory does not match its image sidecar")
}

func TestInspectRejectsTrustInputMismatch(t *testing.T) {
	for name, change := range map[string]struct {
		path     func(SigningOptions) string
		expected string
	}{
		"registry CA": {func(options SigningOptions) string { return options.RegistryCA }, "supplied registry CA differs from the file embedded in the release image"},
		"public key":  {func(options SigningOptions) string { return options.PublicKey }, "supplied signing public key differs from the file embedded in the release image"},
	} {
		t.Run(name, func(t *testing.T) {
			registryCA, publicKey := testTrust(t)
			publisher := &Publisher{spec: testSpec(), registryCA: registryCA, publicKey: publicKey}
			options := SigningOptions{RegistryCA: registryCA, PublicKey: publicKey}
			require.NoError(t, os.WriteFile(change.path(options), []byte("different trust input\n"), 0o644))
			_, err := publisher.inspect(matchingTestImage(t), Repository+"@sha256:"+strings.Repeat("a", 64))
			require.EqualError(t, err, change.expected)
		})
	}
}

func TestCosignCommandsUseExactDigest(t *testing.T) {
	exact := Repository + "@sha256:" + strings.Repeat("a", 64)
	runner := &recordingRunner{}
	signer := &cosignSigner{runner: runner, executable: "cosign", ca: "/keys/registry-ca.crt", publicKey: "/keys/cosign.pub", privateKey: "/keys/cosign.key"}
	require.NoError(t, signer.SignImage(context.Background(), exact))
	require.NoError(t, signer.VerifyImage(context.Background(), exact))
	require.NoError(t, signer.SignBlob(context.Background(), "release.json", "release.sigstore.json"))
	require.NoError(t, signer.VerifyBlob(context.Background(), "release.json", "release.sigstore.json"))

	commands := make([]string, 0, len(runner.Commands))
	for _, command := range runner.Commands {
		commands = append(commands, command.String())
	}
	require.Equal(t, []string{
		"cosign sign --yes --use-signing-config=false --tlog-upload=false --registry-referrers-mode=legacy --new-bundle-format=false --key /keys/cosign.key --registry-cacert /keys/registry-ca.crt " + exact,
		"cosign verify --key /keys/cosign.pub --registry-cacert /keys/registry-ca.crt --insecure-ignore-tlog=true " + exact,
		"cosign sign-blob --yes --use-signing-config=false --tlog-upload=false --key /keys/cosign.key --bundle release.sigstore.json release.json",
		"cosign verify-blob --key /keys/cosign.pub --bundle release.sigstore.json --insecure-ignore-tlog=true release.json",
	}, commands)
}

func TestOCIArchiveRequiresExactlyOneArm64Manifest(t *testing.T) {
	img := matchingTestImage(t)
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
	img := matchingTestImage(t)
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
	img := matchingTestImage(t)
	require.NoError(t, client.Push(context.Background(), versionTag, img))
	digest, err := client.Resolve(context.Background(), versionTag)
	require.NoError(t, err)
	exact := strings.TrimSuffix(versionTag, ":0.2.0") + "@" + digest.String()

	keyDir := t.TempDir()
	const testPassphrase = "ephemeral-integration-passphrase"
	t.Setenv("COSIGN_PASSWORD", testPassphrase)
	var processOutput bytes.Buffer
	runner := process.OSRunner{Stdout: &processOutput, Stderr: &processOutput}
	require.NoError(t, runner.Run(context.Background(), process.Command{Name: cosign, Args: []string{"generate-key-pair", "--output-key-prefix", filepath.Join(keyDir, "release")}}))
	signer := &cosignSigner{runner: runner, executable: cosign, ca: caPath, publicKey: filepath.Join(keyDir, "release.pub"), privateKey: filepath.Join(keyDir, "release.key")}
	unsignedImage := matchingTestImage(t)
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

	require.NoError(t, runner.Run(context.Background(), process.Command{Name: cosign, Args: []string{"generate-key-pair", "--output-key-prefix", filepath.Join(keyDir, "wrong")}}))
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
	signer := &cosignSigner{runner: process.OSRunner{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}, executable: cosign, privateKey: key}
	require.NoError(t, signer.SignBlob(context.Background(), blob, bundle))
}
