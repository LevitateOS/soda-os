package projects

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func testHelper(t *testing.T) (Helper, *fakePlatform) {
	t.Helper()
	catalog := testCatalog(t)
	platform := newFakePlatform()
	platform.accounts["alice"] = primaryAccount("alice", primaryRoleUser)
	return Helper{Lifecycle: Lifecycle{Catalog: catalog, Platform: platform}}, platform
}

func TestHelperRejectsUnsupportedCommandsAndParameters(t *testing.T) {
	helper, _ := testHelper(t)
	alice := PKExecIdentity{Username: "alice", UID: 1000}
	_, err := helper.Execute(context.Background(), alice, "run", strings.NewReader(`{}`))
	require.ErrorContains(t, err, "unsupported")

	_, err = helper.Execute(context.Background(), alice, "workspace-publish", strings.NewReader(`{"id":"site","canonical_url":"https://git.example.test/site.git","path":"/etc"}`))
	require.ErrorContains(t, err, "unknown field")

	_, err = helper.Execute(context.Background(), alice, "catalog-add", strings.NewReader(`{"id":"site","display_name":"Site","canonical_url":"https://git.example.test/site.git","password":"secret"}`))
	require.ErrorContains(t, err, "unknown field")
}

func TestHelperRejectsWorkspaceAndSystemCallers(t *testing.T) {
	helper, platform := testHelper(t)
	workspace, err := platform.CreateWorkspace(context.Background(), platform.accounts["alice"], "site")
	require.NoError(t, err)
	platform.accounts["sshd"] = Account{Username: "sshd", UID: 74, Shell: "/usr/sbin/nologin", Groups: map[string]bool{}}

	for _, caller := range []string{workspace.Username, "sshd"} {
		account := platform.accounts[caller]
		identity := PKExecIdentity{Username: caller, UID: account.UID}
		_, err = helper.Execute(context.Background(), identity, "catalog-add", strings.NewReader(`{"id":"site","display_name":"Site","canonical_url":"https://git.example.test/site.git"}`))
		require.ErrorContains(t, err, "not a supported primary")
	}
}

func TestHelperRejectsPKExecUIDAccountMismatch(t *testing.T) {
	helper, _ := testHelper(t)
	identity := PKExecIdentity{Username: "alice", UID: 2000}
	_, err := helper.Execute(context.Background(), identity, "catalog-add", strings.NewReader(`{"id":"site","display_name":"Site","canonical_url":"https://git.example.test/site.git"}`))
	require.ErrorContains(t, err, "no longer matches")
}

func TestPKExecCallerRequiresThePrivilegedBoundary(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("this negative boundary test requires a non-root test process")
	}
	t.Setenv("PKEXEC_UID", "1000")
	_, err := PKExecCaller()
	require.ErrorContains(t, err, "effective UID 0")
}
