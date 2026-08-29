package release

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/process"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/stretchr/testify/require"
)

const testRevision = "2b6b23e356ded84d4ef7fee52b242ae4855793ca"

var testPublicKey = []byte("test public key\n")

func TestPublishUsesExactDigestAndWritesSignedRecord(t *testing.T) {
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
		"push:" + Repository + ":0.2.0-aarch64",
		"resolve:" + Repository + ":0.2.0-aarch64",
		"sign-image:" + exact,
		"verify-image:" + exact,
		"sign-blob",
		"verify-blob",
	}, events)

	contents, err := os.ReadFile(result.RecordPath)
	require.NoError(t, err)
	require.True(t, bytes.HasSuffix(contents, []byte("\n")))
	require.NotContains(t, string(contents), "  \"")
	var record Record
	require.NoError(t, json.Unmarshal(contents, &record))
	require.Equal(t, exact, record.SodaImageReference)
	require.Equal(t, testRevision, record.SourceRevision)
	require.Equal(t, uint32(3), record.StateSchema)
	require.Equal(t, sha256Hex([]byte("rpm inventory\n")), record.RPMInventorySHA256)
	require.Empty(t, record.ISOChecksum)
}

func TestVersionTagsRemainArchitectureSpecific(t *testing.T) {
	for architecture, channel := range map[string]string{"aarch64": "aarch64", "x86_64": "x86_64"} {
		t.Run(architecture, func(t *testing.T) {
			spec := testSpec()
			spec.Platform.Architecture.Name = architecture
			spec.Platform.Architecture.Artifact = channel
			spec.Platform.Release.Channel = channel
			publisher := &Publisher{spec: spec}
			require.Equal(t, Repository+":0.2.0-"+channel, publisher.versionTag())
		})
	}
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
		"push:" + Repository + ":0.2.0-aarch64",
		"resolve:" + Repository + ":0.2.0-aarch64",
		"sign-image:" + exact,
		"verify-image:" + exact,
	}, events)
}

func TestPublishWithISOBindsExactDigest(t *testing.T) {
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
		"push:" + Repository + ":0.2.0-aarch64",
		"resolve:" + Repository + ":0.2.0-aarch64",
		"sign-image:" + exact,
		"verify-image:" + exact,
		"sign-blob",
		"verify-blob",
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
	require.Equal(t, []string{"push:" + Repository + ":0.2.0-aarch64", "resolve:" + Repository + ":0.2.0-aarch64"}, events)
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
		"push:" + Repository + ":0.2.0-aarch64",
		"resolve:" + Repository + ":0.2.0-aarch64",
	}, events)
}

func TestInspectRejectsRPMInventorySidecarMismatch(t *testing.T) {
	publicKey := testTrust(t)
	publisher := &Publisher{spec: testSpec(), publicKey: publicKey}
	_, err := publisher.inspect(testImageWithSidecar(t, strings.Repeat("0", 64)), Repository+"@sha256:"+strings.Repeat("a", 64))
	require.EqualError(t, err, "installed RPM inventory does not match its image sidecar")
}

func TestInspectRejectsTrustInputMismatch(t *testing.T) {
	for name, change := range map[string]struct {
		path     func(SigningOptions) string
		expected string
	}{
		"public key": {func(options SigningOptions) string { return options.PublicKey }, "supplied signing public key differs from the file embedded in the release image"},
	} {
		t.Run(name, func(t *testing.T) {
			publicKey := testTrust(t)
			publisher := &Publisher{spec: testSpec(), publicKey: publicKey}
			options := SigningOptions{PublicKey: publicKey}
			require.NoError(t, os.WriteFile(change.path(options), []byte("different trust input\n"), 0o644))
			_, err := publisher.inspect(matchingTestImage(t), Repository+"@sha256:"+strings.Repeat("a", 64))
			require.EqualError(t, err, change.expected)
		})
	}
}

func TestCosignCommandsUseExactDigest(t *testing.T) {
	exact := Repository + "@sha256:" + strings.Repeat("a", 64)
	runner := &recordingRunner{}
	signer := &cosignSigner{runner: runner, executable: "cosign", publicKey: "/keys/cosign.pub", privateKey: "/keys/cosign.key"}
	require.NoError(t, signer.SignImage(context.Background(), exact))
	require.NoError(t, signer.VerifyImage(context.Background(), exact))
	require.NoError(t, signer.SignBlob(context.Background(), "release.json", "release.sigstore.json"))
	require.NoError(t, signer.VerifyBlob(context.Background(), "release.json", "release.sigstore.json"))

	commands := make([]string, 0, len(runner.Commands))
	for _, command := range runner.Commands {
		commands = append(commands, command.String())
	}
	require.Equal(t, []string{
		"cosign sign --yes --use-signing-config=false --tlog-upload=false --registry-referrers-mode=legacy --new-bundle-format=false --key /keys/cosign.key " + exact,
		"cosign verify --key /keys/cosign.pub --insecure-ignore-tlog=true " + exact,
		"cosign sign-blob --yes --use-signing-config=false --tlog-upload=false --key /keys/cosign.key --bundle release.sigstore.json release.json",
		"cosign verify-blob --key /keys/cosign.pub --bundle release.sigstore.json --insecure-ignore-tlog=true release.json",
	}, commands)
}

func TestReleaseIndexRequiresTwoMatchingSignedSiblingArtifacts(t *testing.T) {
	root := t.TempDir()
	events := []string{}
	publisher := &Publisher{spec: testSpec(), signer: &fakeSigner{events: &events}}
	artifacts := map[string]ReleaseArtifact{}
	for architecture, digest := range map[string]string{"aarch64": strings.Repeat("a", 64), "x86_64": strings.Repeat("b", 64)} {
		isoPath := filepath.Join(root, architecture+".iso")
		require.NoError(t, os.WriteFile(isoPath, []byte(architecture+" installer"), 0o644))
		checksum, err := fileSHA256(isoPath)
		require.NoError(t, err)
		record := Record{SodaVersion: "0.2.0", SourceRevision: testRevision, SodaImageReference: Repository + "@sha256:" + digest, ISOChecksum: checksum}
		recordPath := filepath.Join(root, architecture+".release.json")
		encoded, err := json.Marshal(record)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(recordPath, encoded, 0o644))
		bundlePath := recordPath + ".sigstore.json"
		require.NoError(t, os.WriteFile(bundlePath, []byte("bundle"), 0o644))
		artifacts[architecture] = ReleaseArtifact{ISOPath: isoPath, RecordPath: recordPath, BundlePath: bundlePath}
	}
	index, paths, err := publisher.releaseIndex(context.Background(), artifacts)
	require.NoError(t, err)
	require.Equal(t, uint32(1), index.SchemaVersion)
	require.Equal(t, []string{"aarch64", "x86_64"}, []string{index.Releases[0].Architecture, index.Releases[1].Architecture})
	require.Len(t, paths, 6)

	contents, err := os.ReadFile(artifacts["x86_64"].RecordPath)
	require.NoError(t, err)
	var mismatched Record
	require.NoError(t, json.Unmarshal(contents, &mismatched))
	mismatched.SourceRevision = strings.Repeat("c", 40)
	encoded, err := json.Marshal(mismatched)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(artifacts["x86_64"].RecordPath, encoded, 0o644))
	_, _, err = publisher.releaseIndex(context.Background(), artifacts)
	require.EqualError(t, err, "paired release records have different source revisions")
}

type fakeGitHubReleaseClient struct{ events []string }

func (f *fakeGitHubReleaseClient) CreateDraft(_ context.Context, repository, tag, title string) (githubDraft, error) {
	f.events = append(f.events, "draft:"+repository+":"+tag+":"+title)
	return githubDraft{ID: 7}, nil
}
func (f *fakeGitHubReleaseClient) Upload(_ context.Context, _ githubDraft, path string) error {
	f.events = append(f.events, "upload:"+filepath.Base(path))
	return nil
}
func (f *fakeGitHubReleaseClient) VerifyAssets(_ context.Context, _ githubDraft, paths []string) error {
	f.events = append(f.events, fmt.Sprintf("verify:%d", len(paths)))
	return nil
}
func (f *fakeGitHubReleaseClient) Publish(_ context.Context, _ githubDraft) error {
	f.events = append(f.events, "publish")
	return nil
}

func TestPairedGitHubReleasePublishesOnlyAfterUploadVerification(t *testing.T) {
	client := &fakeGitHubReleaseClient{}
	_, err := publishPaired(context.Background(), client, pairedUpload{repository: "LevitateOS/soda-os", tag: "v0.2.0", indexPath: "/tmp/index.json", bundlePath: "/tmp/index.sigstore.json", paths: []string{"/tmp/a.iso", "/tmp/x.iso"}})
	require.NoError(t, err)
	require.Equal(t, []string{
		"draft:LevitateOS/soda-os:v0.2.0:Soda OS 0.2.0", "upload:a.iso", "upload:x.iso", "verify:2", "publish",
	}, client.events)
}

func TestOCIArchiveRequiresExactlyOneArm64Manifest(t *testing.T) {
	img := matchingTestImage(t)
	index := mutate.AppendManifests(empty.Index,
		mutate.IndexAddendum{Add: img, Descriptor: v1.Descriptor{Platform: &v1.Platform{OS: "linux", Architecture: "arm64"}}},
		mutate.IndexAddendum{Add: img, Descriptor: v1.Descriptor{Platform: &v1.Platform{OS: "linux", Architecture: "amd64"}}},
	)
	archive := writeIndexArchive(t, index)
	_, cleanup, err := imageFromOCIArchive(archive, "arm64")
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
