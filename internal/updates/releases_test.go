package updates

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/process"
	"github.com/stretchr/testify/require"
)

const testVersion = "0.6.4"

var testDigest = "sha256:" + strings.Repeat("a", 64)
var testRevision = strings.Repeat("b", 40)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

type releaseRunner struct {
	commands   []process.Command
	failAt     string
	metadata   string
	recordPath string
}

func (r *releaseRunner) Run(_ context.Context, command process.Command) error {
	r.commands = append(r.commands, command)
	if command.Args[0] == "verify-blob" {
		r.recordPath = command.Args[len(command.Args)-1]
	}
	if command.Args[0] == r.failAt {
		return errors.New("test verification failed")
	}
	return nil
}
func (r *releaseRunner) Output(ctx context.Context, command process.Command) (string, error) {
	return r.metadata, r.Run(ctx, command)
}

type releaseFixture struct {
	releases  *Releases
	runner    *releaseRunner
	documents map[string]string
	published githubRelease
	record    map[string]any
	requests  []string
}

func newReleaseFixture(t *testing.T, architecture string) *releaseFixture {
	t.Helper()
	platform, err := platformFor(architecture)
	require.NoError(t, err)
	recordName := "soda-os-" + testVersion + "-" + architecture + ".release.json"
	f := &releaseFixture{runner: &releaseRunner{}, documents: map[string]string{}}
	f.published.Tag = "v" + testVersion
	for _, name := range []string{recordName, recordName + ".sigstore.json"} {
		url := releaseSite + "/download/v" + testVersion + "/" + name
		f.published.Assets = append(f.published.Assets, struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}{name, url})
		f.documents[url] = "test sigstore bundle"
	}
	f.record = map[string]any{"schema_version": 3, "soda_version": testVersion, "source_revision": testRevision, "platform": platform, "channel": architecture, "soda_image_reference": repository + "@" + testDigest, "iso_sha256": strings.Repeat("c", 64)}
	f.runner.metadata = `{"Digest":"` + testDigest + `","Os":"linux","Architecture":"` + strings.TrimPrefix(platform, "linux/") + `","Labels":{"org.opencontainers.image.version":"` + testVersion + `","org.opencontainers.image.revision":"` + testRevision + `"}}`
	f.releases = NewReleases(f.runner)
	f.releases.client.Transport = transportFunc(func(request *http.Request) (*http.Response, error) {
		f.requests = append(f.requests, request.URL.String())
		require.Empty(t, request.Header.Get("Authorization"))
		contents, ok := f.documents[request.URL.String()]
		status := http.StatusOK
		if !ok {
			status = http.StatusNotFound
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(contents)), Header: make(http.Header)}, nil
	})
	f.encode(t)
	return f
}

func (f *releaseFixture) encode(t *testing.T) {
	t.Helper()
	metadata, err := json.Marshal(f.published)
	require.NoError(t, err)
	f.documents[releaseAPI] = string(metadata)
	contents, err := json.Marshal(f.record)
	require.NoError(t, err)
	architecture := f.record["channel"].(string)
	f.documents[releaseSite+"/download/v"+testVersion+"/soda-os-"+testVersion+"-"+architecture+".release.json"] = string(contents)
}

func TestLatestVerifiesMatchingReleaseWithoutDeploying(t *testing.T) {
	for _, architecture := range []string{"x86_64", "aarch64"} {
		t.Run(architecture, func(t *testing.T) {
			f := newReleaseFixture(t, architecture)
			selected, err := f.releases.Latest(context.Background(), architecture)
			require.NoError(t, err)
			require.Equal(t, Release{Version: testVersion, Revision: testRevision, Architecture: architecture, Reference: repository + "@" + testDigest, NotesURL: releaseSite + "/tag/v" + testVersion}, selected)
			require.Len(t, f.requests, 3)
			require.Len(t, f.runner.commands, 4)
			var operations []string
			for _, command := range f.runner.commands {
				operations = append(operations, command.Args[0])
			}
			require.Equal(t, []string{"verify-blob", "verify", "verify-attestation", "inspect"}, operations)
			require.Contains(t, f.runner.commands[0].Args, signer)
			require.Contains(t, f.runner.commands[0].Args, issuer)
			require.Contains(t, f.runner.commands[2].Args, "slsaprovenance")
			require.Equal(t, []string{"inspect", "--no-creds", "docker://" + selected.Reference}, f.runner.commands[3].Args)
			_, err = os.Stat(f.runner.recordPath)
			require.True(t, os.IsNotExist(err), "temporary verification files must be removed")
		})
	}
}

func TestLatestFailsClosedAtEveryVerificationStep(t *testing.T) {
	for _, operation := range []string{"verify-blob", "verify", "verify-attestation", "inspect"} {
		t.Run(operation, func(t *testing.T) {
			f := newReleaseFixture(t, "x86_64")
			f.runner.failAt = operation
			_, err := f.releases.Latest(context.Background(), "x86_64")
			require.ErrorContains(t, err, "test verification failed")
			require.Equal(t, operation, f.runner.commands[len(f.runner.commands)-1].Args[0])
			_, err = os.Stat(f.runner.recordPath)
			require.True(t, os.IsNotExist(err))
		})
	}
}

func TestLatestRejectsUnapprovedMetadata(t *testing.T) {
	for name, change := range map[string]func(*releaseFixture){
		"draft":           func(f *releaseFixture) { f.published.Draft = true },
		"prerelease":      func(f *releaseFixture) { f.published.Prerelease = true },
		"invalid tag":     func(f *releaseFixture) { f.published.Tag = "v01.2.3" },
		"missing asset":   func(f *releaseFixture) { f.published.Assets = nil },
		"duplicate asset": func(f *releaseFixture) { f.published.Assets = append(f.published.Assets, f.published.Assets[0]) },
		"foreign URL":     func(f *releaseFixture) { f.published.Assets[0].URL = "https://example.invalid/record" },
	} {
		t.Run(name, func(t *testing.T) {
			f := newReleaseFixture(t, "x86_64")
			change(f)
			f.encode(t)
			_, err := f.releases.Latest(context.Background(), "x86_64")
			require.Error(t, err)
			require.Empty(t, f.runner.commands)
		})
	}
}

func TestLatestRejectsSignedIdentityMismatch(t *testing.T) {
	for field, value := range map[string]any{"schema_version": 2, "soda_version": "0.6.5", "platform": "linux/arm64", "source_revision": "short", "soda_image_reference": "ghcr.io/levitateos/soda-os:latest"} {
		t.Run(field, func(t *testing.T) {
			f := newReleaseFixture(t, "x86_64")
			f.record[field] = value
			f.encode(t)
			_, err := f.releases.Latest(context.Background(), "x86_64")
			require.Error(t, err)
			require.Len(t, f.runner.commands, 1)
		})
	}
}

func TestLatestDistinguishesMissingReleaseAndTransportFailure(t *testing.T) {
	f := newReleaseFixture(t, "x86_64")
	delete(f.documents, releaseAPI)
	_, err := f.releases.Latest(context.Background(), "x86_64")
	require.ErrorIs(t, err, ErrNoRelease)
	f.releases.client.Transport = transportFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") })
	_, err = f.releases.Latest(context.Background(), "x86_64")
	require.ErrorContains(t, err, "offline")
	require.NotErrorIs(t, err, ErrNoRelease)
	require.Empty(t, f.runner.commands)
}

func TestLatestRejectsUnknownArchitectureBeforeNetwork(t *testing.T) {
	f := newReleaseFixture(t, "x86_64")
	_, err := f.releases.Latest(context.Background(), "riscv64")
	require.Error(t, err)
	require.Empty(t, f.requests)
}

func TestLatestRejectsOCIIdentityMismatch(t *testing.T) {
	for _, replacement := range []struct{ old, new string }{
		{testDigest, "sha256:" + strings.Repeat("d", 64)}, {"amd64", "arm64"}, {testVersion, "0.1.0"}, {testRevision, strings.Repeat("e", 40)},
	} {
		f := newReleaseFixture(t, "x86_64")
		f.runner.metadata = strings.ReplaceAll(f.runner.metadata, replacement.old, replacement.new)
		_, err := f.releases.Latest(context.Background(), "x86_64")
		require.ErrorContains(t, err, "differs from the signed Soda release")
	}
}

func TestPublishedReverifiesSelectedVersionInsteadOfLatest(t *testing.T) {
	f := newReleaseFixture(t, "x86_64")
	url := "https://api.github.com/repos/LevitateOS/soda-os/releases/tags/v" + testVersion
	f.documents[url] = f.documents[releaseAPI]
	delete(f.documents, releaseAPI)
	selected, err := f.releases.Published(context.Background(), "x86_64", testVersion)
	require.NoError(t, err)
	require.Equal(t, testVersion, selected.Version)
	require.Equal(t, url, f.requests[0])
	delete(f.documents, url)
	_, err = f.releases.Published(context.Background(), "x86_64", testVersion)
	require.ErrorContains(t, err, "HTTP 404")
}

func TestCompareStableVersions(t *testing.T) {
	for _, item := range []struct {
		installed, available string
		want                 int
	}{
		{"0.6.3", "0.6.4", -1}, {"0.6.4", "0.6.4", 0}, {"0.10.0", "0.9.9", 1}, {"100000000000000000000.0.0", "2.0.0", 1},
	} {
		result, err := CompareStableVersions(item.installed, item.available)
		require.NoError(t, err)
		require.Equal(t, item.want, result)
	}
	for _, version := range []string{"0.7.0-dev", "01.2.3", "unknown", "v0.6.3", "0.6.3+local"} {
		_, err := CompareStableVersions(version, testVersion)
		require.Error(t, err)
	}
}
