package projects

import (
	"context"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/stretchr/testify/require"
)

func testHelper(t *testing.T) (Helper, *fakePlatform) {
	t.Helper()
	catalog := testCatalog(t)
	platform := newFakePlatform()
	platform.accounts["alice"] = primaryAccount("alice", primaryRoleUser)
	return Helper{Lifecycle: Lifecycle{Catalog: catalog, Host: platform, Platform: platform}}, platform
}

func TestHelperRejectsUnsupportedCommandsAndParameters(t *testing.T) {
	helper, _ := testHelper(t)
	alice := linuxhost.PKExecIdentity{Username: "alice", UID: 1000}
	_, err := helper.Execute(context.Background(), alice, "run", strings.NewReader(`{}`))
	require.ErrorContains(t, err, "unsupported")

	_, err = helper.Execute(context.Background(), alice, "workspace-publish", strings.NewReader(`{"id":"site","canonical_url":"git@git.example.test:site.git","path":"/etc"}`))
	require.ErrorContains(t, err, "unknown field")

}

func TestHelperPublishesArbitraryCatalogMetadata(t *testing.T) {
	helper, platform := testHelper(t)
	identity := linuxhost.PKExecIdentity{Username: "alice", UID: platform.accounts["alice"].UID}
	response, err := helper.Execute(context.Background(), identity, "catalog-add", strings.NewReader(
		`{"id":"site","display_name":"Site","canonical_url":"git@git.example.test:site.git","team":"web","labels":["public"]}`,
	))
	require.NoError(t, err)
	require.True(t, response.OK)
	entry, err := helper.Lifecycle.Catalog.Get("site")
	require.NoError(t, err)
	require.JSONEq(t, `"web"`, string(entry.Additional["team"]))
	require.JSONEq(t, `["public"]`, string(entry.Additional["labels"]))

	response, err = helper.Execute(context.Background(), identity, "catalog-edit", strings.NewReader(
		`{"id":"site","display_name":"Renamed","owner":"new-owner","labels":["internal"]}`,
	))
	require.NoError(t, err)
	require.True(t, response.OK)
	entry, err = helper.Lifecycle.Catalog.Get("site")
	require.NoError(t, err)
	require.Equal(t, "Renamed", entry.DisplayName)
	require.Equal(t, "git@git.example.test:site.git", entry.CanonicalURL)
	require.JSONEq(t, `"new-owner"`, string(entry.Additional["owner"]))
	require.JSONEq(t, `["internal"]`, string(entry.Additional["labels"]))
}

func TestHelperEditRejectsEveryCanonicalURLBeforeCatalogMutation(t *testing.T) {
	for name, canonicalURL := range map[string]string{
		"unchanged": "git@git.example.test:site.git",
		"changed":   "git@git.example.test:other.git",
	} {
		t.Run(name, func(t *testing.T) {
			helper, platform := testHelper(t)
			identity := linuxhost.PKExecIdentity{Username: "alice", UID: platform.accounts["alice"].UID}
			require.NoError(t, helper.Lifecycle.Catalog.Add(CatalogEntry{
				ID: "site", DisplayName: "Site", CanonicalURL: "git@git.example.test:site.git",
			}))
			input := `{"id":"site","display_name":"Renamed","canonical_url":"` + canonicalURL + `"}`

			_, err := helper.Execute(context.Background(), identity, "catalog-edit", strings.NewReader(input))

			require.ErrorContains(t, err, `must not include "canonical_url"`)
			entry, getErr := helper.Lifecycle.Catalog.Get("site")
			require.NoError(t, getErr)
			require.Equal(t, "Site", entry.DisplayName)
			require.Equal(t, "git@git.example.test:site.git", entry.CanonicalURL)
		})
	}
}

func TestHelperRejectsWorkspaceAndSystemCallers(t *testing.T) {
	helper, platform := testHelper(t)
	workspace, err := platform.CreateWorkspace(context.Background(), platform.accounts["alice"], "site")
	require.NoError(t, err)
	platform.accounts["sshd"] = linuxhost.Account{Username: "sshd", UID: 74, Shell: "/usr/sbin/nologin", Groups: map[string]bool{}}

	for _, caller := range []string{workspace.Username, "sshd"} {
		account := platform.accounts[caller]
		identity := linuxhost.PKExecIdentity{Username: caller, UID: account.UID}
		_, err = helper.Execute(context.Background(), identity, "catalog-add", strings.NewReader(`{"id":"site","display_name":"Site","canonical_url":"git@git.example.test:site.git"}`))
		require.ErrorContains(t, err, "not a supported primary")
	}
}

func TestHelperRejectsPKExecUIDAccountMismatch(t *testing.T) {
	helper, _ := testHelper(t)
	identity := linuxhost.PKExecIdentity{Username: "alice", UID: 2000}
	_, err := helper.Execute(context.Background(), identity, "catalog-add", strings.NewReader(`{"id":"site","display_name":"Site","canonical_url":"git@git.example.test:site.git"}`))
	require.ErrorContains(t, err, "no longer matches")
}
