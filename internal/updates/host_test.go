package updates

import (
	"context"
	"testing"

	"github.com/LevitateOS/soda-os/internal/process"
	"github.com/stretchr/testify/require"
)

type statusRunner struct {
	t      *testing.T
	output string
}

func (runner statusRunner) Run(context.Context, process.Command) error {
	runner.t.Fatal("status must not run a mutation")
	return nil
}

func (runner statusRunner) Output(_ context.Context, command process.Command) (string, error) {
	require.Equal(runner.t, process.Command{Name: "/usr/bin/bootc", Args: []string{"status", "--json"}}, command)
	return runner.output, nil
}

func TestReadNativeStatusFacts(t *testing.T) {
	// bootc v1 places cachedUpdate on each deployment, not on status or spec.
	output := `{
	  "apiVersion":"org.containers.bootc/v1", "kind":"BootcHost",
	  "metadata":{"name":"host"},
	  "spec":{"image":{"image":"example.test/os:next","transport":"registry"}},
	  "status":{
	    "booted":{"image":{"image":{"image":"example.test/os:old","transport":"registry"},
	      "imageDigest":"sha256:booted","version":null,"architecture":"amd64"}, "cachedUpdate":null},
	    "staged":{"image":{"image":{"image":"example.test/os:next","transport":"registry"},
	      "imageDigest":"sha256:staged","version":"0.6.3","architecture":"amd64"},
	      "cachedUpdate":{"image":{"image":"example.test/os:next","transport":"registry"},
	        "imageDigest":"sha256:cached","version":"0.6.3","architecture":"amd64"},
	      "downloadOnly":true,"incompatible":false},
	    "rollbackQueued":false,"usrOverlay":null,"readOnly":false
	  }
	}`
	host, err := ReadHost(t.Context(), statusRunner{t: t, output: output})
	require.NoError(t, err)
	require.Equal(t, "example.test/os:next", host.Spec.Image.Image)
	require.Equal(t, "sha256:booted", host.Status.Booted.Image.ImageDigest)
	require.Nil(t, host.Status.Booted.Image.Version)
	require.Nil(t, host.Status.Booted.CachedUpdate)
	require.True(t, host.Status.Staged.DownloadOnly)
	require.Equal(t, "sha256:cached", host.Status.Staged.CachedUpdate.ImageDigest)
	require.NoError(t, host.mutable())
}

func TestReadHostRejectsMalformedOrNonBootcStatus(t *testing.T) {
	for _, output := range []string{
		`not JSON`,
		`{} {}`,
		`{"kind":"Other","apiVersion":"org.containers.bootc/v1","status":{"booted":{}}}`,
		`{"kind":"BootcHost","apiVersion":"unknown","status":{"booted":{}}}`,
		`{"kind":"BootcHost","apiVersion":"org.containers.bootc/v1","status":{"booted":null}}`,
	} {
		_, err := ReadHost(t.Context(), statusRunner{t: t, output: output})
		require.Error(t, err)
	}
}
