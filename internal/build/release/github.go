package release

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/LevitateOS/soda-os/internal/process"
)

const repositoryStateQuery = `query($owner:String!,$name:String!,$revision:String!,$ref:String!,$tag:String!){repository(owner:$owner,name:$name){viewerPermission object(expression:$revision){oid} ref(qualifiedName:$ref){target{oid}} release(tagName:$tag){tagName isDraft}}}`

const githubOIDCIssuer = "https://token.actions.githubusercontent.com"

var ghEnvironment = []string{"GH_PROMPT_DISABLED=1", "GH_NO_UPDATE_NOTIFIER=1", "NO_COLOR=1"}

type DraftOptions struct {
	NotesPath         string
	AArch64RecordPath string
	X86RecordPath     string
}

type UploadOptions struct {
	Architecture     string
	ISOPath          string
	QCOW2ZSTPath     string
	RecordPath       string
	RecordBundlePath string
}

type PublishOptions struct {
	AArch64RecordPath string
	X86RecordPath     string
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
	workflowRunURL   string
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
		workflowRunURL:   workflowRunURLFromEnvironment(aarch64.Distribution.GitHubRepository),
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

type releaseRecordPath struct {
	architecture string
	path         string
	record       Record
}

func (p *Publication) validateReleaseRecords(revision, aarch64Path, x86Path string) ([]releaseRecordPath, error) {
	records := []struct {
		architecture string
		path         string
	}{
		{architecture: "aarch64", path: aarch64Path},
		{architecture: "x86_64", path: x86Path},
	}
	validated := make([]releaseRecordPath, 0, len(records))
	for _, item := range records {
		architecture, path := item.architecture, item.path
		spec := p.specs[architecture]
		record, err := readStrictRecord(path)
		if err != nil {
			return nil, fmt.Errorf("read %s release record: %w", architecture, err)
		}
		if err := validatePublicationRecord(record, spec, revision); err != nil {
			return nil, fmt.Errorf("validate %s release record: %w", architecture, err)
		}
		validated = append(validated, releaseRecordPath{architecture: architecture, path: path, record: record})
	}
	return validated, nil
}

func requireReleaseNotesDigests(path string, records []releaseRecordPath) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read release notes: %w", err)
	}
	for _, item := range records {
		if !strings.Contains(string(contents), item.record.SodaImageReference) {
			return fmt.Errorf("release notes omit the %s exact GHCR digest", item.architecture)
		}
	}
	return nil
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

func (p *Publication) cosign(args ...string) process.Command {
	return process.Command{Dir: p.root, Env: []string{"NO_COLOR=1"}, Name: "cosign", Args: args}
}

func (p *Publication) releaseWorkflowIdentity() string {
	return "https://github.com/" + p.repository + "/.github/workflows/release.yml@refs/heads/release/" + p.version
}

func workflowRunURLFromEnvironment(repository string) string {
	runID := os.Getenv("GITHUB_RUN_ID")
	if runID == "" {
		return ""
	}
	for _, character := range runID {
		if character < '0' || character > '9' {
			return ""
		}
	}
	return "https://github.com/" + repository + "/actions/runs/" + runID
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
