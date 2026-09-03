package release

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/LevitateOS/soda-os/internal/process"
)

const repositoryStateQuery = `query($owner:String!,$name:String!,$revision:String!,$ref:String!,$tag:String!){repository(owner:$owner,name:$name){viewerPermission object(expression:$revision){oid} ref(qualifiedName:$ref){target{oid}} release(tagName:$tag){tagName isDraft}}}`

var ghEnvironment = []string{"GH_PROMPT_DISABLED=1", "GH_NO_UPDATE_NOTIFIER=1", "NO_COLOR=1"}

type DraftOptions struct{ NotesPath string }

type UploadOptions struct {
	Architecture string
	ISOPath      string
	RecordPath   string
}

type ImageStageOptions struct {
	Architecture string
	ArchivePath  string
}

type ImagePromoteOptions struct {
	Architecture string
	RecordPath   string
}

type ImageResult struct {
	Architecture string
	Reference    string
	Revision     string
}

type PublicationResult struct {
	Tag      string
	Revision string
	Assets   []string
}

type releasePhase string

const (
	draftRelease     releasePhase = "draft"
	publishedRelease releasePhase = "published"
)

// Publication is the stateless operator boundary around Git, gh, and GitHub
// Releases. GitHub remains authoritative for tags, drafts, assets, and releases.
type Publication struct {
	root             string
	specs            map[string]config.DistroSpec
	runner           process.Runner
	hostArchitecture string
	repository       string
	version          string
}

func NewPublication(root string, aarch64, x86 config.DistroSpec, runner process.Runner) (*Publication, error) {
	if runner == nil {
		runner = process.OSRunner{}
	}
	if err := validatePublicationSpecs(aarch64, x86); err != nil {
		return nil, err
	}
	return &Publication{
		root:             root,
		specs:            map[string]config.DistroSpec{"aarch64": aarch64, "x86_64": x86},
		runner:           runner,
		hostArchitecture: runtime.GOARCH,
		repository:       aarch64.Distribution.GitHubRepository,
		version:          aarch64.Identity.Version,
	}, nil
}

func validatePublicationSpecs(aarch64, x86 config.DistroSpec) error {
	for architecture, spec := range map[string]config.DistroSpec{"aarch64": aarch64, "x86_64": x86} {
		if spec.Platform.Architecture.Name != architecture {
			return fmt.Errorf("%s publication specification has architecture %q", architecture, spec.Platform.Architecture.Name)
		}
		if spec.Image.Registry != Repository {
			return fmt.Errorf("%s release repository must be %s", architecture, Repository)
		}
		if spec.Distribution.GitHubRepository == "" {
			return fmt.Errorf("%s Soda distribution has no GitHub release repository", architecture)
		}
	}
	if aarch64.Identity.Version != x86.Identity.Version || aarch64.Distribution.GitHubRepository != x86.Distribution.GitHubRepository {
		return errors.New("architecture specifications disagree on Soda release identity")
	}
	return nil
}

func (p *Publication) Draft(ctx context.Context, options DraftOptions) (PublicationResult, error) {
	if err := requireRegularFile("release notes", options.NotesPath); err != nil {
		return PublicationResult{}, err
	}
	revision, err := p.cleanRevision(ctx)
	if err != nil {
		return PublicationResult{}, err
	}
	if err := p.authenticate(ctx); err != nil {
		return PublicationResult{}, err
	}
	state, err := p.repositoryState(ctx, revision)
	if err != nil {
		return PublicationResult{}, err
	}
	if err := requireAbsentReleaseState(state, revision); err != nil {
		return PublicationResult{}, err
	}
	if err := p.createTag(ctx, revision); err != nil {
		return PublicationResult{}, fmt.Errorf("create GitHub release tag: %w", err)
	}
	if err := p.createDraft(ctx, options.NotesPath); err != nil {
		return PublicationResult{}, fmt.Errorf("create GitHub draft release: %w", err)
	}
	view, err := p.requireRelease(ctx, revision, draftRelease)
	if err != nil {
		return PublicationResult{}, fmt.Errorf("verify GitHub draft release: %w", err)
	}
	if len(view.Assets) != 0 {
		return PublicationResult{}, errors.New("new GitHub draft release is not empty")
	}
	return PublicationResult{Tag: p.tag(), Revision: revision}, nil
}

func (p *Publication) Upload(ctx context.Context, options UploadOptions) (PublicationResult, error) {
	spec, err := p.nativeSpec(options.Architecture)
	if err != nil {
		return PublicationResult{}, err
	}
	revision, err := p.cleanRevision(ctx)
	if err != nil {
		return PublicationResult{}, err
	}
	artifacts, err := validateUploadArtifacts(spec, revision, options)
	if err != nil {
		return PublicationResult{}, err
	}
	if err := p.authenticate(ctx); err != nil {
		return PublicationResult{}, err
	}
	view, err := p.requireRelease(ctx, revision, draftRelease)
	if err != nil {
		return PublicationResult{}, err
	}
	if err := requireUploadNamesAbsent(view.Assets, artifacts); err != nil {
		return PublicationResult{}, err
	}
	if err := p.runner.Run(ctx, p.gh("release", "upload", p.tag(), artifacts[0].Path, artifacts[1].Path, artifacts[2].Path, "--repo", p.repository)); err != nil {
		return PublicationResult{}, fmt.Errorf("upload GitHub release assets: %w", err)
	}
	view, err = p.requireRelease(ctx, revision, draftRelease)
	if err != nil {
		return PublicationResult{}, fmt.Errorf("verify uploaded GitHub release assets: %w", err)
	}
	if err := verifyUploadedAssets(view.Assets, artifacts); err != nil {
		return PublicationResult{}, err
	}
	return PublicationResult{Tag: p.tag(), Revision: revision, Assets: artifactNames(artifacts)}, nil
}

// ImageStage publishes one matching-native local OCI archive under an
// immutable source-revision candidate tag, then verifies the registry digest.
func (p *Publication) ImageStage(ctx context.Context, options ImageStageOptions) (ImageResult, error) {
	spec, err := p.nativeSpec(options.Architecture)
	if err != nil {
		return ImageResult{}, err
	}
	revision, err := p.cleanRevision(ctx)
	if err != nil {
		return ImageResult{}, err
	}
	reference, err := stageArchiveReference(options.ArchivePath, spec, revision)
	if err != nil {
		return ImageResult{}, err
	}
	candidate := p.candidateImageTag(revision, spec)
	if err := p.requireImageTagAbsent(ctx, candidate); err != nil {
		return ImageResult{}, err
	}
	if err := p.runner.Run(ctx, p.skopeo("copy", "--dest-tls-verify=true", "oci-archive:"+options.ArchivePath, "docker://"+candidate)); err != nil {
		return ImageResult{}, fmt.Errorf("publish immutable GHCR candidate image: %w", err)
	}
	if err := p.requireImageDigest(ctx, candidate, reference); err != nil {
		return ImageResult{}, err
	}
	return ImageResult{Architecture: options.Architecture, Reference: reference, Revision: revision}, nil
}

// ImagePromote creates the immutable version tag only after the locally
// validated record and existing source-revision candidate agree on one digest.
func (p *Publication) ImagePromote(ctx context.Context, options ImagePromoteOptions) (ImageResult, error) {
	spec, err := p.nativeSpec(options.Architecture)
	if err != nil {
		return ImageResult{}, err
	}
	revision, err := p.cleanRevision(ctx)
	if err != nil {
		return ImageResult{}, err
	}
	record, err := readStrictRecord(options.RecordPath)
	if err != nil {
		return ImageResult{}, err
	}
	if err := validatePublicationRecord(record, spec, revision); err != nil {
		return ImageResult{}, err
	}
	candidate := p.candidateImageTag(revision, spec)
	if err := p.requireImageDigest(ctx, candidate, record.SodaImageReference); err != nil {
		return ImageResult{}, err
	}
	versionTag := p.versionImageTag(spec)
	if err := p.requireImageTagAbsent(ctx, versionTag); err != nil {
		return ImageResult{}, err
	}
	if err := p.runner.Run(ctx, p.skopeo("copy", "--src-tls-verify=true", "--dest-tls-verify=true", "docker://"+candidate, "docker://"+versionTag)); err != nil {
		return ImageResult{}, fmt.Errorf("promote immutable GHCR image: %w", err)
	}
	if err := p.requireImageDigest(ctx, versionTag, record.SodaImageReference); err != nil {
		return ImageResult{}, err
	}
	return ImageResult{Architecture: options.Architecture, Reference: record.SodaImageReference, Revision: revision}, nil
}

func (p *Publication) nativeSpec(architecture string) (config.DistroSpec, error) {
	spec, ok := p.specs[architecture]
	if !ok {
		return config.DistroSpec{}, fmt.Errorf("unsupported Soda architecture %q", architecture)
	}
	if err := config.RequireNativeHostArchitecture(architecture, p.hostArchitecture); err != nil {
		return config.DistroSpec{}, err
	}
	return spec, nil
}

func (p *Publication) Publish(ctx context.Context) (PublicationResult, error) {
	revision, err := p.cleanRevision(ctx)
	if err != nil {
		return PublicationResult{}, err
	}
	if err := p.authenticate(ctx); err != nil {
		return PublicationResult{}, err
	}
	view, err := p.requireRelease(ctx, revision, draftRelease)
	if err != nil {
		return PublicationResult{}, err
	}
	if err := requirePublishableAssets(view.Assets, p.version); err != nil {
		return PublicationResult{}, err
	}
	before := assetIdentities(view.Assets)
	if err := p.runner.Run(ctx, p.gh("release", "edit", p.tag(), "--repo", p.repository, "--verify-tag", "--draft=false")); err != nil {
		return PublicationResult{}, fmt.Errorf("publish GitHub release: %w", err)
	}
	view, err = p.requireRelease(ctx, revision, publishedRelease)
	if err != nil {
		return PublicationResult{}, fmt.Errorf("verify published GitHub release: %w", err)
	}
	if after := assetIdentities(view.Assets); !equalStrings(before, after) {
		return PublicationResult{}, errors.New("GitHub release assets changed while publishing")
	}
	return PublicationResult{Tag: p.tag(), Revision: revision, Assets: before}, nil
}

func (p *Publication) authenticate(ctx context.Context) error {
	if err := p.runner.Run(ctx, p.gh("auth", "status", "--active", "--hostname", "github.com")); err != nil {
		return fmt.Errorf("verify GitHub CLI authentication: %w", err)
	}
	return nil
}

func (p *Publication) cleanRevision(ctx context.Context) (string, error) {
	status, err := p.runner.Output(ctx, process.Command{Dir: p.root, Name: "git", Args: []string{"status", "--porcelain=v1", "--untracked-files=no"}})
	if err != nil {
		return "", fmt.Errorf("inspect release worktree: %w", err)
	}
	if strings.TrimSpace(status) != "" {
		return "", errors.New("release publication requires clean tracked and staged Git state")
	}
	revision, err := p.runner.Output(ctx, process.Command{Dir: p.root, Name: "git", Args: []string{"rev-parse", "HEAD"}})
	if err != nil {
		return "", fmt.Errorf("resolve release source revision: %w", err)
	}
	revision = strings.TrimSpace(revision)
	if !validHexadecimal(revision, 40) {
		return "", fmt.Errorf("source revision %q is not a full Git commit ID", revision)
	}
	return revision, nil
}

func (p *Publication) repositoryState(ctx context.Context, revision string) (repositoryState, error) {
	owner, name, _ := strings.Cut(p.repository, "/")
	output, err := p.runner.Output(ctx, p.gh(
		"api", "graphql",
		"--raw-field", "query="+repositoryStateQuery,
		"--raw-field", "owner="+owner,
		"--raw-field", "name="+name,
		"--raw-field", "revision="+revision,
		"--raw-field", "ref=refs/tags/"+p.tag(),
		"--raw-field", "tag="+p.tag(),
	))
	if err != nil {
		return repositoryState{}, fmt.Errorf("inspect GitHub release state: %w", err)
	}
	var response repositoryStateResponse
	if err := json.Unmarshal([]byte(output), &response); err != nil {
		return repositoryState{}, fmt.Errorf("decode GitHub release state: %w", err)
	}
	if len(response.Errors) > 0 || response.Data.Repository == nil {
		return repositoryState{}, errors.New("GitHub did not return the configured release repository")
	}
	return *response.Data.Repository, nil
}

func (p *Publication) requireRelease(ctx context.Context, revision string, phase releasePhase) (releaseView, error) {
	state, err := p.repositoryState(ctx, revision)
	if err != nil {
		return releaseView{}, err
	}
	if err := validateExistingReleaseState(state, revision, p.tag(), phase); err != nil {
		return releaseView{}, err
	}
	view, err := p.releaseView(ctx)
	if err != nil {
		return releaseView{}, err
	}
	if view.TagName != p.tag() || view.IsDraft != phase.isDraft() {
		return releaseView{}, errors.New("GitHub release view differs from the intended tag or draft state")
	}
	return view, nil
}

func (p *Publication) releaseView(ctx context.Context) (releaseView, error) {
	output, err := p.runner.Output(ctx, p.gh("release", "view", p.tag(), "--repo", p.repository, "--json", "tagName,isDraft,assets"))
	if err != nil {
		return releaseView{}, fmt.Errorf("inspect GitHub release: %w", err)
	}
	var view releaseView
	if err := json.Unmarshal([]byte(output), &view); err != nil {
		return releaseView{}, fmt.Errorf("decode GitHub release: %w", err)
	}
	return view, nil
}

func (p *Publication) createTag(ctx context.Context, revision string) error {
	return p.runner.Run(ctx, p.gh(
		"api", "repos/"+p.repository+"/git/refs",
		"--method", "POST",
		"--raw-field", "ref=refs/tags/"+p.tag(),
		"--raw-field", "sha="+revision,
		"--silent",
	))
}

func (p *Publication) createDraft(ctx context.Context, notesPath string) error {
	return p.runner.Run(ctx, p.gh(
		"release", "create", p.tag(),
		"--repo", p.repository,
		"--verify-tag",
		"--draft",
		"--title", "Soda OS "+p.version,
		"--notes-file", notesPath,
	))
}

func (p *Publication) gh(args ...string) process.Command {
	return process.Command{Dir: p.root, Env: append([]string(nil), ghEnvironment...), Name: "gh", Args: args}
}

func (p *Publication) skopeo(args ...string) process.Command {
	return process.Command{Dir: p.root, Env: []string{"NO_COLOR=1"}, Name: "skopeo", Args: args}
}

func (p *Publication) candidateImageTag(revision string, spec config.DistroSpec) string {
	return Repository + ":sha-" + revision + "-" + spec.Platform.Architecture.Artifact
}

func (p *Publication) versionImageTag(spec config.DistroSpec) string {
	return Repository + ":" + p.version + "-" + spec.Platform.Architecture.Artifact
}

func (p *Publication) requireImageTagAbsent(ctx context.Context, tag string) error {
	output, err := p.runner.Output(ctx, p.skopeo("list-tags", "docker://"+Repository))
	if err != nil {
		return fmt.Errorf("inspect GHCR image tags: %w", err)
	}
	var response struct {
		Tags []string `json:"Tags"`
	}
	if err := json.Unmarshal([]byte(output), &response); err != nil {
		return fmt.Errorf("decode GHCR image tags: %w", err)
	}
	_, name, found := strings.Cut(tag, ":")
	if !found || name == "" {
		return errors.New("invalid immutable GHCR image tag")
	}
	for _, existing := range response.Tags {
		if existing == name {
			return fmt.Errorf("immutable GHCR image tag %q already exists", name)
		}
	}
	return nil
}

func (p *Publication) requireImageDigest(ctx context.Context, tag, reference string) error {
	output, err := p.runner.Output(ctx, p.skopeo("inspect", "--format", "{{.Digest}}", "docker://"+tag))
	if err != nil {
		return fmt.Errorf("inspect GHCR image digest: %w", err)
	}
	digest := strings.TrimSpace(output)
	expected := strings.TrimPrefix(reference, Repository+"@")
	if digest != expected {
		return errors.New("GHCR image digest differs from the exact Soda release record")
	}
	return nil
}

func stageArchiveReference(path string, spec config.DistroSpec, revision string) (string, error) {
	image, cleanup, err := imageFromOCIArchive(path, spec.Platform.Architecture.OCI)
	if err != nil {
		return "", err
	}
	defer cleanup()
	digest, err := image.Digest()
	if err != nil {
		return "", fmt.Errorf("compute local image digest: %w", err)
	}
	record, err := (&Publisher{spec: spec}).inspect(image, Repository+"@"+digest.String())
	if err != nil {
		return "", err
	}
	if record.SourceRevision != revision {
		return "", errors.New("local OCI archive source revision differs from clean Soda source revision")
	}
	return record.SodaImageReference, nil
}

func (p *Publication) tag() string { return "v" + p.version }

type repositoryStateResponse struct {
	Data struct {
		Repository *repositoryState `json:"repository"`
	} `json:"data"`
	Errors []json.RawMessage `json:"errors"`
}

type repositoryState struct {
	ViewerPermission string `json:"viewerPermission"`
	Object           *struct {
		OID string `json:"oid"`
	} `json:"object"`
	Ref *struct {
		Target struct {
			OID string `json:"oid"`
		} `json:"target"`
	} `json:"ref"`
	Release *struct {
		TagName string `json:"tagName"`
		IsDraft bool   `json:"isDraft"`
	} `json:"release"`
}

type releaseView struct {
	TagName string        `json:"tagName"`
	IsDraft bool          `json:"isDraft"`
	Assets  []remoteAsset `json:"assets"`
}

type remoteAsset struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	State  string `json:"state"`
	Digest string `json:"digest"`
}

func requireAbsentReleaseState(state repositoryState, revision string) error {
	if err := validateRepositoryAccess(state, revision); err != nil {
		return err
	}
	if state.Ref != nil {
		return errors.New("GitHub release tag already exists")
	}
	if state.Release != nil {
		return errors.New("GitHub release or draft already exists")
	}
	return nil
}

func validateExistingReleaseState(state repositoryState, revision, tag string, phase releasePhase) error {
	if err := validateRepositoryAccess(state, revision); err != nil {
		return err
	}
	if state.Ref == nil || state.Ref.Target.OID != revision {
		return errors.New("GitHub release tag does not target the clean source revision")
	}
	if state.Release == nil || state.Release.TagName != tag || state.Release.IsDraft != phase.isDraft() {
		return errors.New("GitHub release differs from the intended tag or draft state")
	}
	return nil
}

func (p releasePhase) isDraft() bool { return p == draftRelease }

func validateRepositoryAccess(state repositoryState, revision string) error {
	switch state.ViewerPermission {
	case "WRITE", "MAINTAIN", "ADMIN":
	default:
		return fmt.Errorf("GitHub release publication requires write permission; repository returned %q", state.ViewerPermission)
	}
	if state.Object == nil || state.Object.OID != revision {
		return errors.New("clean source revision is not present in the GitHub repository")
	}
	return nil
}
