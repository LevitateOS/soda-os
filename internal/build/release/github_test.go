package release

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/LevitateOS/soda-os/internal/process"
	"github.com/stretchr/testify/require"
)

type publicationRunner struct {
	commands       []process.Command
	states         []string
	views          []string
	imageTags      []string
	imageDigests   []string
	remoteBranches []string
	revision       string
	status         string
	failRunPrefix  string
}

func (r *publicationRunner) Run(_ context.Context, command process.Command) error {
	r.commands = append(r.commands, command)
	if r.failRunPrefix != "" && strings.HasPrefix(command.String(), r.failRunPrefix) {
		return errors.New("injected command failure")
	}
	return nil
}

func (r *publicationRunner) Output(_ context.Context, command process.Command) (string, error) {
	r.commands = append(r.commands, command)
	switch {
	case command.String() == "git status --porcelain=v1 --untracked-files=no":
		return r.status, nil
	case command.String() == "git rev-parse HEAD":
		return r.revision + "\n", nil
	case strings.HasPrefix(command.String(), "gh api graphql "):
		return popOutput(&r.states)
	case strings.HasPrefix(command.String(), "gh release view "):
		return popOutput(&r.views)
	case strings.HasPrefix(command.String(), "skopeo list-tags "):
		return popOutput(&r.imageTags)
	case strings.HasPrefix(command.String(), "skopeo inspect "):
		return popOutput(&r.imageDigests)
	case command.String() == "git ls-remote --exit-code origin refs/heads/production":
		return popOutput(&r.remoteBranches)
	default:
		return "", errors.New("unexpected output command: " + command.String())
	}
}

func popOutput(outputs *[]string) (string, error) {
	if len(*outputs) == 0 {
		return "", errors.New("scripted output is exhausted")
	}
	result := (*outputs)[0]
	*outputs = (*outputs)[1:]
	return result, nil
}

func TestDraftUsesFixedAppendOnlyGitHubCLISequence(t *testing.T) {
	notes := filepath.Join(t.TempDir(), "notes.md")
	require.NoError(t, os.WriteFile(notes, []byte("release notes\n"), 0o644))
	runner := &publicationRunner{
		revision: testRevision,
		states: []string{
			repositoryStateJSON(testRevision, "absent"),
			repositoryStateJSON(testRevision, draftRelease),
		},
		views: []string{releaseViewJSON(draftRelease, nil)},
	}
	publication := testPublication(t, runner, "arm64")

	result, err := publication.Draft(context.Background(), writeDraftOptions(t, notes, testRevision))
	require.NoError(t, err)
	require.Equal(t, "v0.2.0", result.Tag)
	require.Equal(t, testRevision, result.Revision)

	commands := commandStrings(runner.commands)
	require.Equal(t, "gh auth status --active --hostname github.com", commands[2])
	require.Contains(t, commands[3], "gh api graphql")
	require.Contains(t, commands[3], "--raw-field ref=refs/tags/v0.2.0")
	require.Contains(t, commands[3], "--raw-field tag=v0.2.0")
	require.Equal(t, []string{"api", "repos/LevitateOS/soda-os/git/refs", "--method", "POST", "--raw-field", "ref=refs/tags/v0.2.0", "--raw-field", "sha=" + testRevision, "--silent"}, runner.commands[4].Args)
	require.Equal(t, []string{"release", "create", "v0.2.0", "--repo", "LevitateOS/soda-os", "--verify-tag", "--draft", "--title", "Soda OS 0.2.0", "--notes-file", notes}, runner.commands[5].Args)
	require.Equal(t, "gh release view v0.2.0 --repo LevitateOS/soda-os --json tagName,isDraft,assets", commands[7])
	requireGHEnvironment(t, runner.commands)
	require.NotContains(t, strings.Join(commands, "\n"), "token")
	require.NotContains(t, strings.Join(commands, "\n"), "clobber")
}

func TestDraftDoesNotCompensateAfterPartialFailure(t *testing.T) {
	notes := filepath.Join(t.TempDir(), "notes.md")
	require.NoError(t, os.WriteFile(notes, []byte("release notes\n"), 0o644))
	runner := &publicationRunner{
		revision:      testRevision,
		states:        []string{repositoryStateJSON(testRevision, "absent")},
		failRunPrefix: "gh release create",
	}

	_, err := testPublication(t, runner, "arm64").Draft(context.Background(), writeDraftOptions(t, notes, testRevision))
	require.ErrorContains(t, err, "create GitHub draft release")
	commands := strings.Join(commandStrings(runner.commands), "\n")
	require.Equal(t, 1, strings.Count(commands, "gh release create"))
	require.NotContains(t, commands, "delete")
	require.NotContains(t, commands, "retry")
}

func TestDraftCollisionFailsBeforeMutation(t *testing.T) {
	notes := filepath.Join(t.TempDir(), "notes.md")
	require.NoError(t, os.WriteFile(notes, []byte("release notes\n"), 0o644))
	for name, state := range map[string]string{
		"tag": repositoryResponseJSON(testRevision, repositoryResponseFixture{permission: "ADMIN", ref: true}),
		"release": repositoryResponseJSON(testRevision, repositoryResponseFixture{
			permission: "ADMIN", release: true, draft: true,
		}),
	} {
		t.Run(name, func(t *testing.T) {
			runner := &publicationRunner{revision: testRevision, states: []string{state}}
			_, err := testPublication(t, runner, "arm64").Draft(context.Background(), writeDraftOptions(t, notes, testRevision))
			require.Error(t, err)
			for _, command := range runner.commands {
				require.NotEqual(t, "POST", argumentAfter(command.Args, "--method"))
				require.False(t, strings.HasPrefix(command.String(), "gh release create "))
			}
		})
	}
}

func TestDraftRequiresBothExactImageDigestsInNotesBeforeGitHubMutation(t *testing.T) {
	notes := filepath.Join(t.TempDir(), "notes.md")
	require.NoError(t, os.WriteFile(notes, []byte("release notes\n"), 0o644))
	runner := &publicationRunner{revision: testRevision}
	options := writeDraftOptions(t, notes, testRevision)
	require.NoError(t, os.WriteFile(notes, []byte("release notes\n"), 0o644))
	_, err := testPublication(t, runner, "arm64").Draft(context.Background(), options)
	require.ErrorContains(t, err, "release notes omit")
	require.NotContains(t, strings.Join(commandStrings(runner.commands), "\n"), "gh auth")
}

func TestUploadValidatesAndVerifiesOnlyNativeArchitectureAssets(t *testing.T) {
	options, local := writeUploadArtifacts(t, testArmPublicationSpec(), testRevision)
	pre := releaseViewJSON(draftRelease, nil)
	post := releaseViewJSON(draftRelease, localRemoteAssets(t, local))
	runner := &publicationRunner{
		revision: testRevision,
		states: []string{
			repositoryStateJSON(testRevision, draftRelease),
			repositoryStateJSON(testRevision, draftRelease),
		},
		views: []string{pre, post},
	}
	publication := testPublication(t, runner, "arm64")

	result, err := publication.Upload(context.Background(), options)
	require.NoError(t, err)
	require.Equal(t, []string{filepath.Base(options.ISOPath), filepath.Base(options.ISOPath) + ".sha256", filepath.Base(options.QCOW2ZSTPath), filepath.Base(options.QCOW2ZSTPath) + ".sha256", filepath.Base(options.RecordPath), filepath.Base(options.RecordBundlePath)}, result.Assets)

	var upload process.Command
	for _, command := range runner.commands {
		if strings.HasPrefix(command.String(), "gh release upload ") {
			upload = command
		}
	}
	require.Equal(t, "gh", upload.Name)
	require.Equal(t, []string{"release", "upload", "v0.2.0", options.ISOPath, options.ISOPath + ".sha256", options.QCOW2ZSTPath, options.QCOW2ZSTPath + ".sha256", options.RecordPath, options.RecordBundlePath, "--repo", "LevitateOS/soda-os"}, upload.Args)
	require.NotContains(t, upload.Args, "--clobber")
	requireGHEnvironment(t, runner.commands)
}

func TestUploadRejectsSiblingHostBeforeFilesystemOrNetwork(t *testing.T) {
	runner := &publicationRunner{revision: testRevision}
	publication := testPublication(t, runner, "amd64")
	_, err := publication.Upload(context.Background(), UploadOptions{Architecture: "aarch64"})
	require.EqualError(t, err, "Soda aarch64 artifact operations require a native arm64 host; running on amd64")
	require.Empty(t, runner.commands)
}

func TestUploadRejectsExistingAssetWithoutUploading(t *testing.T) {
	options, local := writeUploadArtifacts(t, testArmPublicationSpec(), testRevision)
	runner := &publicationRunner{
		revision: testRevision,
		states:   []string{repositoryStateJSON(testRevision, draftRelease)},
		views:    []string{releaseViewJSON(draftRelease, localRemoteAssets(t, local[:1]))},
	}
	publication := testPublication(t, runner, "arm64")
	_, err := publication.Upload(context.Background(), options)
	require.ErrorContains(t, err, "already exists")
	require.NotContains(t, strings.Join(commandStrings(runner.commands), "\n"), "gh release upload")
}

func testPublication(t *testing.T, runner process.Runner, host string) *Publication {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "runtime.lock"), []byte("runtime lock\n"), 0o644))
	publication, err := NewPublication(root, testArmPublicationSpec(), testX86PublicationSpec(), runner)
	require.NoError(t, err)
	publication.hostArchitecture = host
	publication.workflowRunURL = "https://github.com/LevitateOS/soda-os/actions/runs/1"
	return publication
}

func TestImageProvenanceBindsTheRecordedRuntimeLock(t *testing.T) {
	publication := testPublication(t, &publicationRunner{}, "arm64")
	options, _ := writeUploadArtifacts(t, testArmPublicationSpec(), testRevision)
	record, err := readStrictRecord(options.RecordPath)
	require.NoError(t, err)

	path, cleanup, err := publication.writeImageProvenance(record, testArmPublicationSpec())
	require.NoError(t, err)
	defer cleanup()
	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	var statement imageProvenance
	require.NoError(t, json.Unmarshal(contents, &statement))
	inputs := statement.Predicate.BuildDefinition.ExternalParameters
	require.Equal(t, record.FedoraBaseReference, inputs.FedoraBaseReference)
	require.Equal(t, record.RuntimePackageLock, inputs.RuntimePackageLock)
	require.Equal(t, record.RuntimeLockSHA256, inputs.RuntimeLockSHA256)

	record.RuntimeLockSHA256 = strings.Repeat("0", 64)
	_, cleanup, err = publication.writeImageProvenance(record, testArmPublicationSpec())
	if cleanup != nil {
		cleanup()
	}
	require.ErrorContains(t, err, "runtime package lock checksum differs")
}

func TestImageProvenanceAcceptsConfiguredAbsoluteRuntimeLock(t *testing.T) {
	publication := testPublication(t, &publicationRunner{}, "arm64")
	options, _ := writeUploadArtifacts(t, testArmPublicationSpec(), testRevision)
	record, err := readStrictRecord(options.RecordPath)
	require.NoError(t, err)
	spec := testArmPublicationSpec()
	lockPath := filepath.Join(publication.root, "runtime.lock")
	spec.Platform.Base.RuntimePackageLock = lockPath
	record.RuntimePackageLock = lockPath

	path, cleanup, err := publication.writeImageProvenance(record, spec)
	require.NoError(t, err)
	defer cleanup()
	require.FileExists(t, path)
}

func TestImageSigningChecksRecordedRuntimeLockBeforeCosign(t *testing.T) {
	runner := &publicationRunner{}
	publication := testPublication(t, runner, "arm64")
	options, _ := writeUploadArtifacts(t, testArmPublicationSpec(), testRevision)
	record, err := readStrictRecord(options.RecordPath)
	require.NoError(t, err)
	record.RuntimeLockSHA256 = strings.Repeat("0", 64)

	err = publication.signAndAttestImage(context.Background(), record, testArmPublicationSpec())
	require.ErrorContains(t, err, "runtime package lock checksum differs")
	require.Empty(t, runner.commands)
}

func testArmPublicationSpec() config.DistroSpec {
	spec := testSpec()
	spec.Distribution.GitHubRepository = "LevitateOS/soda-os"
	return spec
}

func testX86PublicationSpec() config.DistroSpec {
	spec := testArmPublicationSpec()
	spec.Base.Platform = "linux/amd64"
	spec.Platform.Architecture = config.PlatformArchitecture{Name: "x86_64", OCI: "amd64", Platform: "linux/amd64", Artifact: "x86_64"}
	spec.Platform.Release.Channel = "x86_64"
	return spec
}

func writeUploadArtifacts(t *testing.T, spec config.DistroSpec, revision string) (UploadOptions, []localAsset) {
	t.Helper()
	directory := t.TempDir()
	iso := filepath.Join(directory, "SodaOS-"+spec.Identity.Version+"-"+spec.Platform.Architecture.Artifact+".iso")
	require.NoError(t, os.WriteFile(iso, []byte("installer bytes"), 0o644))
	digest, err := fileSHA256(iso)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(iso+".sha256", []byte(digest+"  "+filepath.Base(iso)+"\n"), 0o644))
	qcow2 := filepath.Join(directory, "SodaOS-"+spec.Identity.Version+"-"+spec.Platform.Architecture.Artifact+".qcow2.zst")
	require.NoError(t, os.WriteFile(qcow2, []byte("compressed QCOW2"), 0o644))
	qcow2Digest, err := fileSHA256(qcow2)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(qcow2+".sha256", []byte(qcow2Digest+"  "+filepath.Base(qcow2)+"\n"), 0o644))
	record := Record{
		SchemaVersion:       4,
		SodaVersion:         spec.Identity.Version,
		SourceRevision:      revision,
		Platform:            spec.Base.Platform,
		Channel:             spec.Platform.Release.Channel,
		FedoraBaseReference: spec.Base.Reference,
		RuntimePackageLock:  spec.Platform.Base.RuntimePackageLock,
		RuntimeLockSHA256:   sha256Hex([]byte("runtime lock\n")),
		SodaImageReference:  Repository + "@sha256:" + strings.Repeat("a", 64),
		ArtifactChecksums: ArtifactChecksums{
			RPMInventorySHA256: strings.Repeat("b", 64),
			ISOChecksum:        digest,
			QCOW2Checksum:      strings.Repeat("d", 64),
			QCOW2ZSTChecksum:   qcow2Digest,
		},
	}
	recordPath := filepath.Join(directory, "soda-os-"+spec.Identity.Version+"-"+spec.Platform.Release.Channel+".release.json")
	encoded, err := json.Marshal(record)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(recordPath, append(encoded, '\n'), 0o644))
	bundle := recordPath + ".sigstore.json"
	require.NoError(t, os.WriteFile(bundle, []byte("{\"bundle\":true}\n"), 0o644))
	options := UploadOptions{Architecture: spec.Platform.Architecture.Name, ISOPath: iso, QCOW2ZSTPath: qcow2, RecordPath: recordPath, RecordBundlePath: bundle}
	local, err := validateUploadArtifacts(spec, revision, options)
	require.NoError(t, err)
	return options, local
}

func writePublishOptions(t *testing.T, revision string) PublishOptions {
	t.Helper()
	aarch64, _ := writeUploadArtifacts(t, testArmPublicationSpec(), revision)
	x86, _ := writeUploadArtifacts(t, testX86PublicationSpec(), revision)
	return PublishOptions{AArch64RecordPath: aarch64.RecordPath, X86RecordPath: x86.RecordPath}
}

func writeDraftOptions(t *testing.T, notes string, revision string) DraftOptions {
	t.Helper()
	publish := writePublishOptions(t, revision)
	for _, path := range []string{publish.AArch64RecordPath, publish.X86RecordPath} {
		record, err := readStrictRecord(path)
		require.NoError(t, err)
		contents, err := os.ReadFile(notes)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(notes, append(contents, []byte(record.SodaImageReference+"\n")...), 0o644))
	}
	return DraftOptions{NotesPath: notes, AArch64RecordPath: publish.AArch64RecordPath, X86RecordPath: publish.X86RecordPath}
}

func repositoryStateJSON(revision string, phase releasePhase) string {
	return repositoryResponseJSON(revision, repositoryResponseFixture{
		permission: "ADMIN",
		ref:        phase != "absent",
		release:    phase == draftRelease || phase == publishedRelease,
		draft:      phase.isDraft(),
	})
}

type repositoryResponseFixture struct {
	permission string
	ref        bool
	release    bool
	draft      bool
}

func repositoryResponseJSON(revision string, fixture repositoryResponseFixture) string {
	repository := map[string]any{
		"viewerPermission": fixture.permission,
		"object":           map[string]string{"oid": revision},
		"ref":              nil,
		"release":          nil,
	}
	if fixture.ref {
		repository["ref"] = map[string]any{"target": map[string]string{"oid": revision}}
	}
	if fixture.release {
		repository["release"] = map[string]any{"tagName": "v0.2.0", "isDraft": fixture.draft}
	}
	encoded, _ := json.Marshal(map[string]any{"data": map[string]any{"repository": repository}})
	return string(encoded)
}

func releaseViewJSON(phase releasePhase, assets []remoteAsset) string {
	encoded, _ := json.Marshal(releaseView{TagName: "v0.2.0", IsDraft: phase.isDraft(), Assets: assets})
	return string(encoded)
}

func localRemoteAssets(t *testing.T, assets []localAsset) []remoteAsset {
	t.Helper()
	result := make([]remoteAsset, 0, len(assets))
	for _, asset := range assets {
		result = append(result, remoteAsset{Name: asset.Name, Size: asset.Size, State: "uploaded", Digest: asset.Digest})
	}
	return result
}

func requiredRemoteAssets(version string) []remoteAsset {
	assets := make([]remoteAsset, 0, 6)
	for _, name := range requiredAssetNames(version) {
		assets = append(assets, remoteAsset{Name: name, Size: 1, State: "uploaded", Digest: "sha256:" + strings.Repeat("a", 64)})
	}
	return assets
}

func commandStrings(commands []process.Command) []string {
	result := make([]string, 0, len(commands))
	for _, command := range commands {
		result = append(result, command.String())
	}
	return result
}

func requireGHEnvironment(t *testing.T, commands []process.Command) {
	t.Helper()
	for _, command := range commands {
		if command.Name == "gh" {
			require.Equal(t, ghEnvironment, command.Env)
		}
	}
}

func argumentAfter(arguments []string, target string) string {
	for index, argument := range arguments {
		if argument == target && index+1 < len(arguments) {
			return arguments[index+1]
		}
	}
	return ""
}
