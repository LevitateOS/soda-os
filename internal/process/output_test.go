package process

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestTraceFailurePreventsStartingTheCommand(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "must-not-exist")
	runner := OSRunner{Stdout: failingWriter{}}
	command := Command{Name: "sh", Args: []string{"-c", `printf started > "$1"`, "sh", marker}}
	require.ErrorIs(t, runner.Run(t.Context(), command), io.ErrClosedPipe)
	output, err := runner.Output(t.Context(), command)
	require.ErrorIs(t, err, io.ErrClosedPipe)
	require.Empty(t, output)
	_, err = os.Stat(marker)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestRunnerSeparatesTracesCapturedOutputAndDiagnostics(t *testing.T) {
	var stdout, stderr bytes.Buffer
	runner := OSRunner{Stdout: &stdout, Stderr: &stderr}
	command := Command{Name: "sh", Args: []string{"-c", "printf result; printf diagnostic >&2"}}
	require.NoError(t, runner.Run(t.Context(), command))
	require.Equal(t, "+ "+command.String()+"\nresult", stdout.String())
	require.Equal(t, "diagnostic", stderr.String())
	stdout.Reset()
	stderr.Reset()
	output, err := runner.Output(t.Context(), command)
	require.NoError(t, err)
	require.Equal(t, "result", output)
	require.Equal(t, "+ "+command.String()+"\n", stdout.String())
	require.Empty(t, stderr.String(), "Output captures stderr for commandError instead of streaming it")
}
