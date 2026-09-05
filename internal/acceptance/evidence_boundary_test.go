package acceptance

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEvidenceNeverOverwritesEarlierObservation(t *testing.T) {
	evidence := testPerson(t, "owner").Remote.Evidence
	require.NoError(t, evidence.Write("observation", []byte("first")))
	require.Error(t, evidence.Write("observation", []byte("second")))
	contents, err := os.ReadFile(filepath.Join(evidence.Root, "observation"))
	require.NoError(t, err)
	require.Equal(t, "first", string(contents))
}

func TestReturnedDiagnosticsRedactSecretsAndRetainErrorIdentity(t *testing.T) {
	cause := errors.New("native failure: private-password")
	err := redactError(fmt.Errorf("check failed: %w", cause), []Secret{{Label: "password", Value: []byte("private-password\n")}})
	require.ErrorIs(t, err, cause)
	require.NotContains(t, fmt.Sprint(err), "private-password")
	require.Contains(t, err.Error(), "[REDACTED]")
}

func TestRegistryCopyCannotIgnoreEvidenceFailure(t *testing.T) {
	for _, command := range []string{"skopeo", "docker"} {
		t.Run(command, func(t *testing.T) {
			installAcceptanceCommand(t, command, "printf copied\n")
			evidence := testPerson(t, "owner").Remote.Evidence
			require.NoError(t, os.MkdirAll(filepath.Join(evidence.Root, "registry", "candidate-copy.txt"), 0o700))
			registry := Registry{Evidence: evidence}
			var err error
			if command == "skopeo" {
				_, err = registry.publishNative(context.Background(), "fixture.tar", "candidate")
			} else {
				_, err = registry.publishContainer(context.Background(), "fixture.tar", "candidate", "fixture-image")
			}
			require.ErrorContains(t, err, "write evidence")
		})
	}
}
