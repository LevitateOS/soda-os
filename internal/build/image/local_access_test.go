package image

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLocalAccessSelectsOnlyTheNamedConnection(t *testing.T) {
	source := filepath.Join("..", "..", "..", "packaging", "rpm", "runtime", "sources", "soda-local-access")
	tools := t.TempDir()
	log := filepath.Join(t.TempDir(), "nmcli.log")
	writeLocalAccessCommand(t, tools, "id", "printf '%s\\n' \"${SODA_TEST_UID:-0}\"\n")
	writeLocalAccessCommand(t, tools, "nmcli", `
if [ "$1" = -g ]; then
  printf '%s\n' "$SODA_TEST_ZONE"
  exit 0
fi
for argument in "$@"; do
  printf '%s\n' "$argument" >> "$SODA_TEST_NMCLI_LOG"
done
printf '%s\n' -- >> "$SODA_TEST_NMCLI_LOG"
`)

	for _, test := range []struct {
		name     string
		args     []string
		zone     string
		wantOut  string
		wantLog  string
		wantFail bool
		uid      string
	}{
		{name: "reports explicitly trusted connection", args: []string{"office"}, zone: "trusted", wantOut: "on\n"},
		{name: "reports untrusted connection off", args: []string{"office"}, zone: "drop", wantOut: "off\n"},
		{name: "enables named connection", args: []string{"office", "on"}, wantLog: "connection\nmodify\noffice\nconnection.zone\ntrusted\n--\nconnection\nup\noffice\n--\n"},
		{name: "disables named connection", args: []string{"office", "off"}, wantLog: "connection\nmodify\noffice\nconnection.zone\ndrop\n--\nconnection\nup\noffice\n--\n"},
		{name: "requires root", args: []string{"office", "on"}, wantFail: true, uid: "1000"},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.NoError(t, os.WriteFile(log, nil, 0o600))
			command := exec.Command("sh", append([]string{source}, test.args...)...)
			command.Env = append(os.Environ(), "PATH="+tools+":"+os.Getenv("PATH"), "SODA_TEST_NMCLI_LOG="+log, "SODA_TEST_ZONE="+test.zone, "SODA_TEST_UID="+test.uid)
			output, err := command.CombinedOutput()
			if test.wantFail {
				require.Error(t, err)
				require.Contains(t, string(output), "must run as root")
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.wantOut, string(output))
			contents, readErr := os.ReadFile(log)
			require.NoError(t, readErr)
			require.Equal(t, test.wantLog, string(contents))
		})
	}
}

func writeLocalAccessCommand(t *testing.T, directory, name, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(directory, name), []byte("#!/bin/sh\n"+body), 0o755))
}
