// Package updates discovers approved Soda releases without retaining update state.
package updates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/LevitateOS/soda-os/internal/process"
	"github.com/LevitateOS/soda-os/internal/strictjson"
)

const (
	repository      = "ghcr.io/levitateos/soda-os"
	releaseAPI      = "https://api.github.com/repos/LevitateOS/soda-os/releases/latest"
	releaseSite     = "https://github.com/LevitateOS/soda-os/releases"
	signer          = "https://github.com/LevitateOS/soda-os/.github/workflows/release.yml@refs/heads/production"
	issuer          = "https://token.actions.githubusercontent.com"
	maximumResponse = 1 << 20
)

var (
	stableVersion  = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	exactImage     = regexp.MustCompile(`^ghcr\.io/levitateos/soda-os@sha256:[0-9a-f]{64}$`)
	sourceRevision = regexp.MustCompile(`^[0-9a-f]{40}$`)
	ErrNoRelease   = errors.New("no published stable Soda release is available")
)

// Release is a verified release identity, not a stored deployment or approval cache.
type Release struct {
	Version      string `json:"version"`
	Revision     string `json:"revision"`
	Architecture string `json:"architecture"`
	Reference    string `json:"reference"`
	NotesURL     string `json:"notes_url"`
}

// Releases delegates GitHub transport to net/http and signatures to native Cosign.
// It performs no bootc operations. Callers must reverify before downloading.
type Releases struct {
	client *http.Client
	runner process.Runner
}

func NewReleases(runner process.Runner) *Releases {
	return &Releases{client: &http.Client{Timeout: 30 * time.Second}, runner: runner}
}

type githubRelease struct {
	Tag        string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// Latest returns only the host architecture's verified identity. A 404 is not
// "up to date"; neither are network, rate-limit, or verification errors.
func (r *Releases) Latest(ctx context.Context, architecture string) (Release, error) {
	platform, err := platformFor(architecture)
	if err != nil {
		return Release{}, err
	}
	return r.fromURL(ctx, architecture, platform, releaseAPI)
}

// Published reverifies a selected version even if a newer release appeared
// after download. Withdrawn or prerelease records are never accepted.
func (r *Releases) Published(ctx context.Context, architecture, version string) (Release, error) {
	platform, err := platformFor(architecture)
	if err != nil {
		return Release{}, err
	}
	if !stableVersion.MatchString(version) {
		return Release{}, errors.New("invalid stable Soda version")
	}
	selected, err := r.fromURL(ctx, architecture, platform, "https://api.github.com/repos/LevitateOS/soda-os/releases/tags/v"+version)
	if err != nil {
		return Release{}, err
	}
	if selected.Version != version {
		return Release{}, errors.New("published release version changed")
	}
	return selected, nil
}

func (r *Releases) fromURL(ctx context.Context, architecture, platform, url string) (Release, error) {
	contents, err := r.fetch(ctx, url)
	if err != nil {
		return Release{}, err
	}
	var published githubRelease
	if err := json.Unmarshal(contents, &published); err != nil {
		return Release{}, fmt.Errorf("decode GitHub release: %w", err)
	}
	version, err := published.version()
	if err != nil {
		return Release{}, err
	}
	directory, err := os.MkdirTemp("", "soda-release-check-")
	if err != nil {
		return Release{}, err
	}
	defer os.RemoveAll(directory)
	record := "soda-os-" + version + "-" + architecture + ".release.json"
	if err := r.fetchAssets(ctx, published, directory, record); err != nil {
		return Release{}, err
	}
	path := filepath.Join(directory, record)
	if err := r.verifyBlob(ctx, path); err != nil {
		return Release{}, err
	}
	identity, err := readIdentity(path, version, platform)
	if err != nil {
		return Release{}, err
	}
	selected := Release{Version: version, Revision: identity.Revision, Architecture: architecture, Reference: identity.Reference, NotesURL: releaseSite + "/tag/" + published.Tag}
	if err := r.verifyImage(ctx, selected, platform); err != nil {
		return Release{}, err
	}
	return selected, nil
}

func platformFor(architecture string) (string, error) {
	switch architecture {
	case "x86_64":
		return "linux/amd64", nil
	case "aarch64":
		return "linux/arm64", nil
	default:
		return "", errors.New("Soda releases require an x86_64 or aarch64 host")
	}
}

func (published githubRelease) version() (string, error) {
	if published.Draft || published.Prerelease {
		return "", errors.New("GitHub latest release is not a published stable release")
	}
	if len(published.Tag) < 2 || published.Tag[0] != 'v' || !stableVersion.MatchString(published.Tag[1:]) {
		return "", errors.New("GitHub release tag must be vMAJOR.MINOR.PATCH")
	}
	return published.Tag[1:], nil
}

func (r *Releases) fetch(ctx context.Context, url string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "Soda-Updates")
	response, err := r.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch Soda release: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound && url == releaseAPI {
		return nil, ErrNoRelease
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch Soda release: HTTP %d", response.StatusCode)
	}
	contents, err := io.ReadAll(io.LimitReader(response.Body, maximumResponse+1))
	if err != nil {
		return nil, err
	}
	if len(contents) > maximumResponse {
		return nil, errors.New("Soda release response exceeds 1 MiB")
	}
	return contents, nil
}

func (r *Releases) fetchAssets(ctx context.Context, published githubRelease, directory, record string) error {
	for _, name := range []string{record, record + ".sigstore.json"} {
		expected := releaseSite + "/download/" + published.Tag + "/" + name
		if err := published.requireAsset(name, expected); err != nil {
			return err
		}
		contents, err := r.fetch(ctx, expected)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(directory, name), contents, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func (published githubRelease) requireAsset(name, url string) error {
	count := 0
	for _, asset := range published.Assets {
		if asset.Name != name {
			continue
		}
		if asset.URL != url {
			return fmt.Errorf("release asset %s has an unexpected download URL", name)
		}
		count++
	}
	if count != 1 {
		return fmt.Errorf("release must contain exactly one %s asset", name)
	}
	return nil
}

// releaseIdentity reads the signed schema's identity fields; artifact checksums
// and other metadata remain publication-owned, not a second runtime record model.
type releaseIdentity struct {
	Schema    int    `json:"schema_version"`
	Version   string `json:"soda_version"`
	Revision  string `json:"source_revision"`
	Platform  string `json:"platform"`
	Channel   string `json:"channel"`
	Reference string `json:"soda_image_reference"`
}

func readIdentity(path, version, platform string) (releaseIdentity, error) {
	file, err := os.Open(path)
	if err != nil {
		return releaseIdentity{}, err
	}
	defer file.Close()
	var fields map[string]json.RawMessage
	if err := strictjson.Decode(file, &fields); err != nil {
		return releaseIdentity{}, err
	}
	contents, err := json.Marshal(fields)
	if err != nil {
		return releaseIdentity{}, err
	}
	var identity releaseIdentity
	if err := json.Unmarshal(contents, &identity); err != nil {
		return identity, err
	}
	return identity, identity.validate(version, platform)
}

func (identity releaseIdentity) validate(version, platform string) error {
	if identity.Schema != 3 || identity.Version != version || identity.Platform != platform {
		return errors.New("signed release schema, version, or platform does not match the selected release")
	}
	expectedPlatform, err := platformFor(identity.Channel)
	if err != nil || expectedPlatform != platform {
		return errors.New("signed release channel does not match the host architecture")
	}
	if !sourceRevision.MatchString(identity.Revision) || !exactImage.MatchString(identity.Reference) {
		return errors.New("signed release lacks an exact Soda digest and source revision")
	}
	return nil
}
