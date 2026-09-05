package acceptance

import (
	"bytes"
	"context"
	"os/exec"
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

func TestWorkspaceRemovalRejectsRetainedAccountsAndLookupFailures(t *testing.T) {
	for _, status := range []string{"0", "1", "2", "3"} {
		t.Run(status, func(t *testing.T) {
			installRemoteShell(t)
			t.Setenv("ACCOUNT_STATUS", status)
			installAcceptanceCommand(t, "getent", `case "$2" in deleted) exit "$ACCOUNT_STATUS" ;; *) printf 'survivor\n' ;; esac`)
			// Only the Projects operation is simulated; its verification program
			// executes through the remote shell and sudo stdin boundary unchanged.
			installAcceptanceCommand(t, "ssh", `for command do :; done
case "$command" in
 *soda-projects*"'remove'") cat >/dev/null; printf 'administrator status is required' >&2; exit 1 ;;
 *soda-projects*) cat >/dev/null; printf '{"ok":true}' ;;
 *) exec /bin/sh -c "$command" ;;
esac
`)
			person := testPerson(t, "owner")
			project := projectFixture{
				Admin: workspaceFixture{Person: person, Remote: person.Remote.As("admin-workspace", "key")},
				Alice: workspaceFixture{Person: person, Remote: person.Remote.As("deleted", "key"), ProjectID: "kept"},
				Bob:   workspaceFixture{Person: person, Remote: person.Remote.As("bob-workspace", "key")},
			}
			err := verifyWorkspaceRemoval(context.Background(), project)
			require.Equal(t, status != "2", err != nil, "%v", err)
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
