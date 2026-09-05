package acceptance

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExchangeSeparatesExecutionAndEvidenceErrors(t *testing.T) {
	for _, exit := range []string{"0", "7"} {
		t.Run(exit, func(t *testing.T) {
			installAcceptanceCommand(t, "ssh", "printf output; printf diagnostic >&2; exit "+exit+"\n")
			evidence, err := CreateEvidence(filepath.Join(t.TempDir(), "evidence"))
			require.NoError(t, err)
			remote := Remote{Evidence: evidence}
			result, err := remote.Exchange(context.Background(), "command", nil, "fixture")
			require.NoError(t, err)
			require.Equal(t, "output", string(result.Stdout))
			require.Equal(t, "diagnostic", string(result.Stderr))
			require.Equal(t, exit != "0", result.Err != nil)
			require.NoError(t, os.Mkdir(filepath.Join(evidence.Root, "blocked.stdout"), 0o700))
			result, err = remote.Exchange(context.Background(), "blocked", nil, "fixture")
			require.Error(t, err)
			require.Equal(t, exit != "0", result.Err != nil)
			require.Equal(t, "diagnostic", string(result.Stderr))
			require.Error(t, remote.Capture(context.Background(), "blocked", nil, "fixture"))
		})
	}
}

func TestExpectedSetupFailureCannotConsumeEvidenceFailure(t *testing.T) {
	installAcceptanceCommand(t, "ssh", "printf 'retained retry "+testPublicKey+"' >&2; exit 1\n")
	evidence, err := CreateEvidence(filepath.Join(t.TempDir(), "evidence"))
	require.NoError(t, err)
	require.NoError(t, os.Mkdir(filepath.Join(evidence.Root, "setup-key-required.stdout"), 0o700))
	_, err = requireRetainedWorkspace(context.Background(), Remote{Evidence: evidence}, "kept", "setup")
	require.ErrorContains(t, err, "write evidence")
}

func TestRetainedSetupUsesReturnedDiagnostic(t *testing.T) {
	installAcceptanceCommand(t, "ssh", `for command do :; done
eval "set -- $command"
for last do :; done
case "$last" in
 setup) printf 'retained retry `+testPublicKey+`' >&2; exit 1 ;;
 list) printf '%s' '{"projects":[{"id":"kept","workspace_username":"workspace","workspace_exists":true}]}' ;;
esac
`)
	evidence, err := CreateEvidence(filepath.Join(t.TempDir(), "evidence"))
	require.NoError(t, err)
	retained, err := requireRetainedWorkspace(context.Background(), Remote{Evidence: evidence}, "kept", "setup")
	require.NoError(t, err)
	require.Equal(t, "workspace", retained.Username)
	require.Equal(t, testPublicKey, string(retained.PublicKey))
	require.Contains(t, string(retained.Diagnostic), "retained retry")
}

func TestManifestComparisonUsesCapturedBytes(t *testing.T) {
	installAcceptanceCommand(t, "ssh", "cat >/dev/null; printf 'snapshot\\n'\n")
	evidence, err := CreateEvidence(filepath.Join(t.TempDir(), "evidence"))
	require.NoError(t, err)
	remote := Remote{Evidence: evidence}
	snapshot, err := remote.SudoOutput(context.Background(), []byte("password"), stableManifestScript, "snapshot")
	require.NoError(t, err)
	require.NoError(t, os.Remove(filepath.Join(evidence.Root, "snapshot.stdout")))
	require.NoError(t, compareManifests(snapshot, []byte("snapshot\n"), "restored"))
	require.ErrorContains(t, compareManifests(snapshot, []byte("snapshot"), "restored"), "restored")
}

func TestCancelledExchangeDoesNotClaimExecutionSuccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	evidence, err := CreateEvidence(filepath.Join(t.TempDir(), "evidence"))
	require.NoError(t, err)
	result, err := (Remote{Evidence: evidence}).Exchange(ctx, "cancelled", nil, "unused")
	require.NoError(t, err)
	require.Error(t, result.Err)
}
