package updates

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReleaseFetchRejectsErrorsAndOversizedDocuments(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusTooManyRequests, http.StatusInternalServerError} {
		f := newReleaseFixture(t, "x86_64")
		f.releases.client.Transport = transportFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("untrusted error body"))}, nil
		})
		_, err := f.releases.Latest(context.Background(), "x86_64")
		require.ErrorContains(t, err, "HTTP")
		require.NotContains(t, err.Error(), "untrusted error body")
		require.Empty(t, f.runner.commands)
	}
	f := newReleaseFixture(t, "x86_64")
	f.documents[releaseAPI] = strings.Repeat(" ", maximumResponse+1)
	_, err := f.releases.Latest(context.Background(), "x86_64")
	require.ErrorContains(t, err, "exceeds 1 MiB")
	require.Empty(t, f.runner.commands)
}

func TestReleaseFetchPropagatesCancellation(t *testing.T) {
	f := newReleaseFixture(t, "x86_64")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f.releases.client.Transport = transportFunc(func(request *http.Request) (*http.Response, error) {
		return nil, request.Context().Err()
	})
	_, err := f.releases.Latest(ctx, "x86_64")
	require.ErrorIs(t, err, context.Canceled)
	require.Empty(t, f.runner.commands)
}

func TestSignedRecordRejectsAmbiguousJSON(t *testing.T) {
	for _, contents := range []string{
		`{"schema_version":3,"schema_version":2}`,
		`{"schema_version":3} {}`,
		`[]`,
	} {
		path := filepath.Join(t.TempDir(), "record.json")
		require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
		_, err := readIdentity(path, testVersion, "linux/amd64")
		require.Error(t, err)
	}
}

func TestReleaseChannelMustMatchHost(t *testing.T) {
	f := newReleaseFixture(t, "x86_64")
	for url, contents := range f.documents {
		if strings.HasSuffix(url, ".release.json") {
			f.documents[url] = strings.Replace(contents, `"channel":"x86_64"`, `"channel":"aarch64"`, 1)
		}
	}
	_, err := f.releases.Latest(context.Background(), "x86_64")
	require.ErrorContains(t, err, "channel")
	require.Len(t, f.runner.commands, 1)
}
