package release

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const releaseIndexAsset = "soda-os-release-index.json"

type PairedPublicationOptions struct {
	AArch64     ReleaseArtifact
	X8664       ReleaseArtifact
	GitHubToken string
	OutputDir   string
}

type ReleaseArtifact struct {
	ISOPath    string
	RecordPath string
	BundlePath string
}

type PairedResult struct {
	Tag        string
	IndexPath  string
	BundlePath string
}

type releaseIndex struct {
	SchemaVersion  uint32         `json:"schema_version"`
	SodaVersion    string         `json:"soda_version"`
	SourceRevision string         `json:"source_revision"`
	Releases       []indexRelease `json:"releases"`
}

type indexRelease struct {
	Architecture   string `json:"architecture"`
	ImageReference string `json:"image_reference"`
	ISOAsset       string `json:"iso_asset"`
	ISOChecksum    string `json:"iso_sha256"`
	RecordAsset    string `json:"record_asset"`
	RecordChecksum string `json:"record_sha256"`
}

type githubReleaseClient interface {
	CreateDraft(context.Context, string, string, string) (githubDraft, error)
	Upload(context.Context, githubDraft, string) error
	VerifyAssets(context.Context, githubDraft, []string) error
	Publish(context.Context, githubDraft) error
}

type githubDraft struct {
	ID        int64
	UploadURL string
}

func (p *Publisher) PublishPaired(ctx context.Context, options PairedPublicationOptions) (PairedResult, error) {
	if options.GitHubToken == "" {
		return PairedResult{}, errors.New("GitHub token is required to publish a Soda release")
	}
	if p.spec.Distribution.GitHubRepository == "" {
		return PairedResult{}, errors.New("Soda distribution has no GitHub release repository")
	}
	artifacts := map[string]ReleaseArtifact{"aarch64": options.AArch64, "x86_64": options.X8664}
	index, paths, err := p.releaseIndex(ctx, artifacts)
	if err != nil {
		return PairedResult{}, err
	}
	if options.OutputDir == "" {
		options.OutputDir = ".artifacts/releases"
	}
	if err := os.MkdirAll(options.OutputDir, 0o755); err != nil {
		return PairedResult{}, fmt.Errorf("create paired release output: %w", err)
	}
	indexPath := filepath.Join(options.OutputDir, releaseIndexAsset)
	encoded, err := json.Marshal(index)
	if err != nil {
		return PairedResult{}, err
	}
	if err := os.WriteFile(indexPath, append(encoded, '\n'), 0o644); err != nil {
		return PairedResult{}, fmt.Errorf("write signed release index: %w", err)
	}
	bundlePath := indexPath + ".sigstore.json"
	if err := p.signer.SignBlob(ctx, indexPath, bundlePath); err != nil {
		return PairedResult{}, fmt.Errorf("sign paired release index: %w", err)
	}
	if err := p.signer.VerifyBlob(ctx, indexPath, bundlePath); err != nil {
		return PairedResult{}, fmt.Errorf("verify paired release index: %w", err)
	}
	paths = append(paths, indexPath, bundlePath)
	tag := "v" + index.SodaVersion
	client := githubAPI{token: options.GitHubToken, client: http.DefaultClient}
	return publishPaired(ctx, client, pairedUpload{repository: p.spec.Distribution.GitHubRepository, tag: tag, indexPath: indexPath, bundlePath: bundlePath, paths: paths})
}

func (p *Publisher) releaseIndex(ctx context.Context, artifacts map[string]ReleaseArtifact) (releaseIndex, []string, error) {
	index := releaseIndex{SchemaVersion: 1, Releases: make([]indexRelease, 0, 2)}
	paths := make([]string, 0, 6)
	for _, architecture := range []string{"aarch64", "x86_64"} {
		indexed, err := p.releaseIndexEntry(ctx, architecture, artifacts[architecture])
		if err != nil {
			return releaseIndex{}, nil, err
		}
		if index.SodaVersion == "" {
			index.SodaVersion, index.SourceRevision = indexed.record.SodaVersion, indexed.record.SourceRevision
		} else if indexed.record.SourceRevision != index.SourceRevision {
			return releaseIndex{}, nil, errors.New("paired release records have different source revisions")
		}
		index.Releases = append(index.Releases, indexed.release)
		paths = append(paths, indexed.paths...)
	}
	return index, paths, nil
}

type indexedArtifact struct {
	release indexRelease
	record  Record
	paths   []string
}

func (p *Publisher) releaseIndexEntry(ctx context.Context, architecture string, artifact ReleaseArtifact) (indexedArtifact, error) {
	if !regularFile(artifact.ISOPath) || !regularFile(artifact.RecordPath) || !regularFile(artifact.BundlePath) {
		return indexedArtifact{}, fmt.Errorf("%s paired release artifacts must be regular files", architecture)
	}
	record, contents, err := p.readReleaseRecord(ctx, architecture, artifact)
	if err != nil {
		return indexedArtifact{}, err
	}
	isoDigest, err := fileSHA256(artifact.ISOPath)
	if err != nil {
		return indexedArtifact{}, err
	}
	if record.ISOChecksum != isoDigest {
		return indexedArtifact{}, fmt.Errorf("%s installer ISO checksum differs from its signed release record", architecture)
	}
	digest := sha256.Sum256(contents)
	entry := indexRelease{Architecture: architecture, ImageReference: record.SodaImageReference, ISOAsset: filepath.Base(artifact.ISOPath), ISOChecksum: isoDigest, RecordAsset: filepath.Base(artifact.RecordPath), RecordChecksum: hex.EncodeToString(digest[:])}
	return indexedArtifact{release: entry, record: record, paths: []string{artifact.ISOPath, artifact.RecordPath, artifact.BundlePath}}, nil
}

func (p *Publisher) readReleaseRecord(ctx context.Context, architecture string, artifact ReleaseArtifact) (Record, []byte, error) {
	if err := p.signer.VerifyBlob(ctx, artifact.RecordPath, artifact.BundlePath); err != nil {
		return Record{}, nil, fmt.Errorf("verify %s signed release record: %w", architecture, err)
	}
	contents, err := os.ReadFile(artifact.RecordPath)
	if err != nil {
		return Record{}, nil, err
	}
	var record Record
	if err := json.Unmarshal(contents, &record); err != nil {
		return Record{}, nil, fmt.Errorf("decode %s release record: %w", architecture, err)
	}
	if record.SodaVersion != p.spec.Identity.Version || record.SourceRevision == "" || !isSodaDigestReference(record.SodaImageReference) {
		return Record{}, nil, fmt.Errorf("%s release record does not match the Soda release contract", architecture)
	}
	return record, contents, nil
}

type pairedUpload struct {
	repository, tag, indexPath, bundlePath string
	paths                                  []string
}

func publishPaired(ctx context.Context, client githubReleaseClient, upload pairedUpload) (PairedResult, error) {
	draft, err := client.CreateDraft(ctx, upload.repository, upload.tag, "Soda OS "+strings.TrimPrefix(upload.tag, "v"))
	if err != nil {
		return PairedResult{}, fmt.Errorf("create GitHub draft release: %w", err)
	}
	for _, path := range upload.paths {
		if err := client.Upload(ctx, draft, path); err != nil {
			return PairedResult{}, fmt.Errorf("upload GitHub release asset %s: %w", filepath.Base(path), err)
		}
	}
	if err := client.VerifyAssets(ctx, draft, upload.paths); err != nil {
		return PairedResult{}, fmt.Errorf("verify uploaded GitHub release assets: %w", err)
	}
	if err := client.Publish(ctx, draft); err != nil {
		return PairedResult{}, fmt.Errorf("publish GitHub release: %w", err)
	}
	return PairedResult{Tag: upload.tag, IndexPath: upload.indexPath, BundlePath: upload.bundlePath}, nil
}

func fileSHA256(path string) (string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(contents)
	return hex.EncodeToString(digest[:]), nil
}

func isSodaDigestReference(reference string) bool {
	prefix := Repository + "@sha256:"
	if !strings.HasPrefix(reference, prefix) || len(reference) != len(prefix)+64 {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(reference, prefix))
	return err == nil
}

type githubAPI struct {
	token  string
	client *http.Client
}

func (g githubAPI) CreateDraft(ctx context.Context, repository, tag, title string) (githubDraft, error) {
	var response struct {
		ID        int64  `json:"id"`
		UploadURL string `json:"upload_url"`
	}
	if err := g.requestJSON(ctx, http.MethodPost, "https://api.github.com/repos/"+repository+"/releases", map[string]any{"tag_name": tag, "name": title, "draft": true}, &response); err != nil {
		return githubDraft{}, err
	}
	return githubDraft{ID: response.ID, UploadURL: strings.Split(response.UploadURL, "{")[0]}, nil
}

func (g githubAPI) Upload(ctx context.Context, draft githubDraft, path string) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	u, err := url.Parse(draft.UploadURL)
	if err != nil {
		return err
	}
	query := u.Query()
	query.Set("name", filepath.Base(path))
	u.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(contents))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+g.token)
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Content-Type", "application/octet-stream")
	response, err := g.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return githubError(response)
	}
	return nil
}

func (g githubAPI) VerifyAssets(ctx context.Context, draft githubDraft, paths []string) error {
	var assets []struct {
		Name       string `json:"name"`
		BrowserURL string `json:"browser_download_url"`
	}
	if err := g.requestJSON(ctx, http.MethodGet, fmt.Sprintf("https://api.github.com/releases/%d/assets", draft.ID), nil, &assets); err != nil {
		return err
	}
	byName := make(map[string]string, len(assets))
	for _, asset := range assets {
		byName[asset.Name] = asset.BrowserURL
	}
	for _, path := range paths {
		if err := g.verifyAsset(ctx, byName[filepath.Base(path)], path); err != nil {
			return err
		}
	}
	return nil
}

func (g githubAPI) verifyAsset(ctx context.Context, remoteURL, path string) error {
	if remoteURL == "" {
		return fmt.Errorf("missing asset %s", filepath.Base(path))
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, remoteURL, nil)
	if err != nil {
		return err
	}
	response, err := g.client.Do(request)
	if err != nil {
		return err
	}
	contents, readErr := io.ReadAll(response.Body)
	response.Body.Close()
	if readErr != nil {
		return readErr
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", filepath.Base(path), response.Status)
	}
	local, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(local, contents) {
		return fmt.Errorf("uploaded bytes differ for %s", filepath.Base(path))
	}
	return nil
}

func (g githubAPI) Publish(ctx context.Context, draft githubDraft) error {
	return g.requestJSON(ctx, http.MethodPatch, fmt.Sprintf("https://api.github.com/releases/%d", draft.ID), map[string]any{"draft": false}, nil)
}

func (g githubAPI) requestJSON(ctx context.Context, method, endpoint string, body any, result any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+g.token)
	request.Header.Set("Accept", "application/vnd.github+json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := g.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return githubError(response)
	}
	if result == nil {
		return nil
	}
	return json.NewDecoder(response.Body).Decode(result)
}

func githubError(response *http.Response) error {
	contents, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	return fmt.Errorf("GitHub API %s: %s", response.Status, strings.TrimSpace(string(contents)))
}
