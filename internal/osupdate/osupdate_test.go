package osupdate

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/process"
	"github.com/stretchr/testify/require"
)

const (
	testBootedDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	testUpdateDigest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

var testARM64Platform = platformContract{"arm64", "arm64", "aarch64", "linux/arm64"}

func TestSignedReleaseIndexSelectsExactlyOneSiblingPlatform(t *testing.T) {
	arm, err := platformFor("arm64")
	require.NoError(t, err)
	x86, err := platformFor("amd64")
	require.NoError(t, err)
	index := releaseIndex{SchemaVersion: 1, SodaVersion: "0.3.2", SourceRevision: strings.Repeat("a", 40), Releases: []indexRelease{
		{Architecture: arm.artifactArchitecture, ImageReference: Repository + "@" + testUpdateDigest, ISOAsset: "soda-os-0.3.2-aarch64.iso", ISOChecksum: strings.Repeat("b", 64), RecordAsset: "soda-os-0.3.2-aarch64.json", RecordChecksum: strings.Repeat("c", 64)},
		{Architecture: x86.artifactArchitecture, ImageReference: Repository + "@" + testBootedDigest, ISOAsset: "soda-os-0.3.2-x86_64.iso", ISOChecksum: strings.Repeat("d", 64), RecordAsset: "soda-os-0.3.2-x86_64.json", RecordChecksum: strings.Repeat("e", 64)},
	}}
	contents, err := json.Marshal(index)
	require.NoError(t, err)
	armReference, err := releaseForPlatform(contents, arm)
	require.NoError(t, err)
	require.Equal(t, Repository+"@"+testUpdateDigest, armReference)
	x86Reference, err := releaseForPlatform(contents, x86)
	require.NoError(t, err)
	require.Equal(t, Repository+"@"+testBootedDigest, x86Reference)
}

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

type fakeDiscovery struct {
	reference string
	err       error
}

func (d fakeDiscovery) ResolveCurrent(context.Context) (string, error) { return d.reference, d.err }

type fakeInspector struct {
	metadata imageMetadata
	err      error
	seen     *string
	calls    *int
}

func (i fakeInspector) Inspect(_ context.Context, reference string) (imageMetadata, error) {
	if i.seen != nil {
		*i.seen = reference
	}
	if i.calls != nil {
		*i.calls++
	}
	return i.metadata, i.err
}

type fakeVerifier struct {
	err  error
	seen *string
}

func (v fakeVerifier) Verify(_ context.Context, reference string) error {
	if v.seen != nil {
		*v.seen = reference
	}
	return v.err
}

func TestStatusComesOnlyFromBootcAndPreservesDownloadLock(t *testing.T) {
	runner := &recordingRunner{Outputs: map[string]string{
		"bootc status --format=json --format-version=1": bootcStatusJSON(testBootedDigest, testUpdateDigest, true),
	}}
	manager := &Manager{
		runner: runner, bootc: "bootc", verifier: fakeVerifier{},
		inspector: fakeInspector{metadata: validMetadata(testUpdateDigest)},
		platform:  testARM64Platform,
	}
	status, err := manager.Status(context.Background())
	require.NoError(t, err)
	require.Equal(t, Repository+"@"+testBootedDigest, status.Booted.ImageReference)
	require.Empty(t, status.Booted.Signature)
	require.Equal(t, Repository+"@"+testUpdateDigest, status.Staged.ImageReference)
	require.True(t, status.Staged.DownloadOnly)
	require.False(t, status.ReadOnly)
	require.Equal(t, "containerPolicy", status.Staged.Signature)
	require.Equal(t, "arm64", status.Staged.Architecture)
	require.Equal(t, []string{"bootc status --format=json --format-version=1"}, commandStrings(runner.Commands))
}

func TestCheckResolvesOnceVerifiesExactDigestAndRejectsWrongMetadata(t *testing.T) {
	exact := Repository + "@" + testUpdateDigest
	seen := ""
	runner := &recordingRunner{Outputs: map[string]string{
		"bootc status --format=json --format-version=1": bootcStatusJSON(testBootedDigest, "", false),
	}}
	manager := &Manager{
		runner: runner, bootc: "bootc", discovery: fakeDiscovery{reference: exact},
		verifier:  fakeVerifier{seen: &seen},
		inspector: fakeInspector{seen: &seen, metadata: validMetadata(testUpdateDigest)},
		platform:  testARM64Platform,
	}
	candidate, err := manager.Check(context.Background())
	require.NoError(t, err)
	require.Equal(t, exact, seen)
	require.Equal(t, exact, candidate.ImageReference)
	require.True(t, candidate.Available)
	require.Equal(t, StateSchema, candidate.StateSchema)

	metadata := validMetadata(testUpdateDigest)
	metadata.Architecture = "amd64"
	manager.inspector = fakeInspector{metadata: metadata}
	_, err = manager.Check(context.Background())
	require.ErrorIs(t, err, ErrRejected)

	inspectorCalls := 0
	manager.verifier = fakeVerifier{err: errors.New("signature rejected")}
	manager.inspector = fakeInspector{calls: &inspectorCalls, metadata: validMetadata(testUpdateDigest)}
	_, err = manager.Check(context.Background())
	require.ErrorIs(t, err, ErrRejected)
	require.Zero(t, inspectorCalls)
}

func TestCosignVerifierUsesEmbeddedKeyForExactDigest(t *testing.T) {
	exact := Repository + "@" + testUpdateDigest
	runner := &recordingRunner{}
	verifier := cosignVerifier{runner: runner, executable: "/usr/libexec/soda/cosign", publicKey: DefaultKey}
	require.NoError(t, verifier.Verify(context.Background(), exact))
	require.Equal(t, []string{
		"/usr/libexec/soda/cosign verify --key " + DefaultKey + " --insecure-ignore-tlog=true " + exact,
	}, commandStrings(runner.Commands))
}

func TestSkopeoInspectorReadsMetadataOnlyAfterVerification(t *testing.T) {
	exact := Repository + "@" + testUpdateDigest
	runner := &recordingRunner{Outputs: map[string]string{
		"skopeo --override-os linux --override-arch arm64 inspect --no-creds --no-tags --tls-verify=true docker://" + exact: `{"Digest":"` + testUpdateDigest + `","Architecture":"arm64","Os":"linux","Labels":{}}`,
	}}
	inspector := skopeoInspector{runner: runner, executable: "skopeo", architecture: "arm64"}
	metadata, err := inspector.Inspect(context.Background(), exact)
	require.NoError(t, err)
	require.Equal(t, testUpdateDigest, metadata.Digest)
	require.Equal(t, []string{"skopeo --override-os linux --override-arch arm64 inspect --no-creds --no-tags --tls-verify=true docker://" + exact}, commandStrings(runner.Commands))
}

func TestStageUsesOnlyExactDigestAndRequiresLockedMatchingStatus(t *testing.T) {
	exact := Repository + "@" + testUpdateDigest
	runner := &recordingRunner{Outputs: map[string]string{
		"bootc status --format=json --format-version=1": bootcStatusJSON(testBootedDigest, testUpdateDigest, true),
	}}
	manager := &Manager{
		runner: runner, bootc: "bootc", verifier: fakeVerifier{},
		inspector: fakeInspector{metadata: validMetadata(testUpdateDigest)},
		platform:  testARM64Platform,
	}
	status, err := manager.Stage(context.Background(), exact)
	require.NoError(t, err)
	require.Equal(t, exact, status.Staged.ImageReference)
	require.Equal(t, []string{
		"bootc status --format=json --format-version=1",
		"bootc switch --download-only --enforce-container-sigpolicy " + exact,
		"bootc status --format=json --format-version=1",
	}, commandStrings(runner.Commands))

	for _, invalid := range []string{Repository + ":current-aarch64", "quay.io/example/os@" + testUpdateDigest, Repository + "@sha256:short"} {
		runner.Commands = nil
		_, err = manager.Stage(context.Background(), invalid)
		require.ErrorIs(t, err, ErrInvalid)
		require.Empty(t, runner.Commands)
	}

	runner.Commands = nil
	runner.Outputs["bootc status --format=json --format-version=1"] = strings.Replace(bootcStatusJSON(testBootedDigest, testUpdateDigest, true), `"readOnly":false`, `"readOnly":true`, 1)
	_, err = manager.Stage(context.Background(), exact)
	require.ErrorIs(t, err, ErrPrecondition)
	require.Equal(t, []string{"bootc status --format=json --format-version=1"}, commandStrings(runner.Commands))

	runner.Commands = nil
	runner.Outputs["bootc status --format=json --format-version=1"] = bootcStatusJSON(testBootedDigest, testUpdateDigest, true)
	manager.verifier = fakeVerifier{err: errors.New("unsigned")}
	_, err = manager.Stage(context.Background(), exact)
	require.ErrorIs(t, err, ErrRejected)
	require.Equal(t, []string{"bootc status --format=json --format-version=1"}, commandStrings(runner.Commands))
	manager.verifier = fakeVerifier{}

	runner.Commands = nil
	runner.Outputs["bootc status --format=json --format-version=1"] = bootcStatusJSON(testBootedDigest, testUpdateDigest, false)
	_, err = manager.Stage(context.Background(), exact)
	require.ErrorIs(t, err, ErrPrecondition)
}

func TestActivateRequiresConfirmationAndDownloadedDeployment(t *testing.T) {
	runner := &recordingRunner{Outputs: map[string]string{
		"bootc status --format=json --format-version=1": bootcStatusJSON(testBootedDigest, testUpdateDigest, true),
	}}
	manager := &Manager{runner: runner, bootc: "bootc", platform: testARM64Platform}
	require.NoError(t, manager.Activate(context.Background()))
	require.Equal(t, []string{
		"bootc status --format=json --format-version=1",
		"bootc switch --from-downloaded --apply",
	}, commandStrings(runner.Commands))

	runner.Commands = nil
	runner.Outputs["bootc status --format=json --format-version=1"] = bootcStatusJSON(testBootedDigest, "", false)
	require.ErrorIs(t, manager.Activate(context.Background()), ErrPrecondition)
	require.Equal(t, []string{"bootc status --format=json --format-version=1"}, commandStrings(runner.Commands))

	runner.Commands = nil
	runner.Outputs["bootc status --format=json --format-version=1"] = strings.Replace(bootcStatusJSON(testBootedDigest, testUpdateDigest, true), `"readOnly":false`, `"readOnly":true`, 1)
	require.ErrorIs(t, manager.Activate(context.Background()), ErrPrecondition)
	require.Equal(t, []string{"bootc status --format=json --format-version=1"}, commandStrings(runner.Commands))
}

func validMetadata(digest string) imageMetadata {
	return imageMetadata{
		Digest: digest, Architecture: "arm64", OS: "linux",
		Labels: map[string]string{
			"org.sodaos.state-schema":            "3",
			"org.opencontainers.image.version":   "0.3.0",
			"org.opencontainers.image.revision":  strings.Repeat("c", 40),
			"org.opencontainers.image.base.name": "quay.io/fedora/fedora-bootc@sha256:" + strings.Repeat("d", 64),
		},
	}
}

func bootcStatusJSON(bootedDigest, stagedDigest string, downloadOnly bool) string {
	deployment := func(digest, version string, downloadOnly, signed bool) string {
		signature := ""
		if signed {
			signature = `,"signature":"containerPolicy"`
		}
		return `{"image":{"image":{"image":"` + Repository + `@` + digest + `","transport":"registry"` + signature + `},"version":"` + version + `","imageDigest":"` + digest + `","architecture":"arm64"},"incompatible":false,"downloadOnly":` + strconv.FormatBool(downloadOnly) + `}`
	}
	staged := "null"
	if stagedDigest != "" {
		staged = deployment(stagedDigest, "0.3.0", downloadOnly, true)
	}
	return `{"status":{"readOnly":false,"booted":` + deployment(bootedDigest, "0.2.0", false, false) + `,"staged":` + staged + `}}`
}

func commandStrings(commands []process.Command) []string {
	result := make([]string, len(commands))
	for index := range commands {
		result[index] = commands[index].String()
	}
	return result
}
