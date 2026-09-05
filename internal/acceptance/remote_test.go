package acceptance

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// Execute the transmitted command through a shell, as sshd does. Tests of the
// original local argv alone miss the remote argument boundary entirely.
func installRemoteShell(t *testing.T) {
	t.Helper()
	installAcceptanceCommand(t, "ssh", `for command do :; done
exec /bin/sh -c "$command"
`)
	installAcceptanceCommand(t, "sudo", `test "$1" = -k && test "$2" = -S && test "$3" = -p && test "$4" = '' || exit 90
shift 4
IFS= read -r password
exec "$@"
`)
}

func TestRemoteArgumentsSurviveTheRemoteShell(t *testing.T) {
	installRemoteShell(t)
	args := []string{"", "two words", "a'b", `{"name":"kept","auto_init":false}`, "$HOME", "$(exit 91)", "line\nbreak", "; exit 92", "\\", "*"}
	command := append([]string{"printf", "%s\\000"}, args...)
	remote := testPerson(t, "owner").Remote
	output, err := remote.Output(context.Background(), nil, command...)
	require.NoError(t, err)
	expected := []byte{}
	for _, arg := range args {
		expected = append(expected, []byte(arg)...)
		expected = append(expected, 0)
	}
	require.Equal(t, expected, output)
	output, err = remote.CaptureOutput(context.Background(), "argv", nil, command...)
	require.NoError(t, err)
	require.Equal(t, expected, output)
}

func TestSudoPreservesEmptyPromptAndScriptStdin(t *testing.T) {
	installRemoteShell(t)
	remote := testPerson(t, "owner").Remote
	output, err := remote.SudoOutput(context.Background(), []byte("password with spaces\n"), "printf '%s' \"a'b\"\n", "sudo")
	require.NoError(t, err)
	require.Equal(t, "a'b", string(output))
}

func TestBashProgramAndKeyStdinSurviveSSH(t *testing.T) {
	installRemoteShell(t)
	directory := t.TempDir()
	keys := filepath.Join(directory, "keys")
	require.NoError(t, os.WriteFile(keys, []byte("original\n"), 0o600))
	remote := testPerson(t, "owner").Remote
	program := laterAuthorizedKeyAbsent
	for _, key := range []string{"later\n", "original\n"} {
		_, err := remote.Output(context.Background(), []byte(key), "/bin/bash", "-c", program, "key-check", keys)
		require.Equal(t, bytes.Equal([]byte(key), []byte("original\n")), err != nil)
	}
	require.NoError(t, os.Remove(keys))
	_, err := remote.Output(context.Background(), []byte("later\n"), "/bin/bash", "-c", program, "key-check", keys)
	require.Error(t, err, "missing key file is not evidence that the later key is absent")
}
