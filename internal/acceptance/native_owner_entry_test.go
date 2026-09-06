package acceptance

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOwnerEntryRequiresTheBlockedAuthenticationOutcomes(t *testing.T) {
	for _, tc := range []struct{ name, api, web string }{
		{"valid", "401", "303 " + forgejoLoopbackEndpoint + "/user/sign_up"},
		{"api-created-account", "200", "303 " + forgejoLoopbackEndpoint + "/user/sign_up"},
		{"api-error", "500", "303 " + forgejoLoopbackEndpoint + "/user/sign_up"},
		{"api-transport", "transport", "303 " + forgejoLoopbackEndpoint + "/user/sign_up"},
		{"web-login-accepted", "401", "303 " + forgejoLoopbackEndpoint + "/"},
		{"web-error", "401", "500"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("API_STATUS", tc.api)
			t.Setenv("WEB_STATUS", tc.web)
			installAcceptanceCommand(t, "ssh", `config=$(cat)
case "$config" in
 */api/v1/user*) test "$API_STATUS" != transport || exit 7; printf '%s' "$API_STATUS" ;;
 */user/login*) printf '%s' "$WEB_STATUS" ;;
 *) printf 'Create your Forgejo administrator account' ;;
esac
`)
			err := verifyOwnerEntry(context.Background(), testPerson(t, "owner"), "owner-entry")
			require.Equal(t, tc.name != "valid", err != nil, "%v", err)
		})
	}
}

func TestOwnerEntryRequiresVisibleGuidance(t *testing.T) {
	installAcceptanceCommand(t, "ssh", "printf 'Soda OS'\n")
	require.ErrorContains(t, verifyOwnerEntry(context.Background(), testPerson(t, "owner"), "owner-entry"), "guidance is missing")
}
