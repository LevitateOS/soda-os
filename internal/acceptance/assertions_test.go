package acceptance

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestForbiddenServicesRequireSuccessfulEmptyInventory(t *testing.T) {
	for _, outcome := range []string{"absent", "present", "unreachable"} {
		t.Run(outcome, func(t *testing.T) {
			t.Setenv("SERVICE_OUTCOME", outcome)
			installAcceptanceCommand(t, "systemctl", `case "$SERVICE_OUTCOME" in
 absent) exit 0 ;;
 present) printf 'sodad.service disabled\n' ;;
 unreachable) exit 1 ;;
esac
`)
			command := exec.Command("/bin/bash", "-eu", "-o", "pipefail", "-s")
			command.Stdin = bytes.NewBufferString(forbiddenServiceChecks)
			output, err := command.CombinedOutput()
			require.Equal(t, outcome != "absent", err != nil, "%s", output)
		})
	}
}

func TestRemovalRequiresAbsentAccountHomeAndProcesses(t *testing.T) {
	for _, state := range []struct{ name, account, process string }{
		{"removed", "2", "1"}, {"account", "0", "1"}, {"lookup-error", "3", "1"},
		{"process", "2", "0"}, {"process-error", "2", "2"}, {"home", "2", "1"}, {"symlink", "2", "1"},
	} {
		t.Run(state.name, func(t *testing.T) {
			installRemoteShell(t)
			installAcceptanceCommand(t, "getent", "exit "+state.account)
			installAcceptanceCommand(t, "pgrep", "exit "+state.process)
			home := filepath.Join(t.TempDir(), "deleted")
			if state.name == "home" {
				require.NoError(t, os.Mkdir(home, 0o700))
			}
			if state.name == "symlink" {
				require.NoError(t, os.Symlink(home+"-missing", home))
			}
			target := deletionTarget{Username: "deleted", UID: 1001, Home: home}
			err := verifyAccountRemoved(context.Background(), testPerson(t, "owner"), target, "removed")
			require.Equal(t, state.name != "removed", err != nil, "%v", err)
		})
	}
}

func TestExpectedProjectRejectionRequiresItsExitAndDiagnostic(t *testing.T) {
	for _, status := range []string{"0", "1", "2", "127", "255"} {
		t.Run(status, func(t *testing.T) {
			result := CommandResult{Err: exec.Command("/bin/sh", "-c", "exit "+status).Run(), Stderr: []byte("administrator status is required")}
			err := requireProjectRejection(result, "administrator status is required")
			require.Equal(t, status != "1", err != nil)
			result.Stderr = []byte("unrelated failure")
			require.Error(t, requireProjectRejection(result, "administrator status is required"))
		})
	}
}

func TestOwnerPasswordRejectionRequiresHTTP401NotTransportFailure(t *testing.T) {
	for _, status := range []string{"200", "401", "403", "500", "transport"} {
		t.Run(status, func(t *testing.T) {
			t.Setenv("HTTP_STATUS", status)
			installAcceptanceCommand(t, "ssh", `config=$(cat)
case "$config" in
 *forgejo-password*) printf '{"login":"owner","is_admin":true}' ;;
 *) test "$HTTP_STATUS" != transport || exit 7; printf '%s' "$HTTP_STATUS" ;;
esac
`)
			err := verifyOwnerCredentials(context.Background(), testPerson(t, "owner"), "owner")
			require.Equal(t, status != "401", err != nil)
		})
	}
}
