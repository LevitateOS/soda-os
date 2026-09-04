package projects

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCatalogEntryValidation(t *testing.T) {
	t.Parallel()
	for _, remote := range []string{
		"ssh://git@git.example.test/team/site.git",
		"git@git.example.test:alice/site.git",
		"ssh://git@git.example.test/team/site.git",
		"ssh://git.example.test/team/site.git",
		"git@git.example.test:team/site.git",
		"git.example.test:team/site.git",
		"git@[2001:db8::1]:team/site.git",
		"[2001:db8::1]:team/site.git",
	} {
		remote := remote
		t.Run(remote, func(t *testing.T) {
			t.Parallel()
			require.NoError(t, (CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: remote}).Validate())
		})
	}
}

func TestCatalogEntryRejectsCredentialsAndNonRemotePaths(t *testing.T) {
	t.Parallel()
	for _, remote := range []string{
		"", "team/site", "/srv/site", "file:///srv/site", "file:/srv/site", "FILE:relative", "C:/site", `D:\\site`, "ftp://git.example.test/site",
		"https://alice@git.example.test/site", "https://alice:secret@git.example.test/site",
		"https://git.example.test/site?token=secret", "https://git.example.test/site?",
		"https://git.example.test/site#", "ssh://git:secret@git.example.test/site",
		"git@git.example.test:team/site.git\x07",
	} {
		remote := remote
		t.Run(remote, func(t *testing.T) {
			t.Parallel()
			require.Error(t, (CatalogEntry{ID: "site", DisplayName: "Site", CanonicalURL: remote}).Validate())
		})
	}
}

func TestRemoteSSHHostIdentifiesBundledForgejoWithoutOwningExternalHosts(t *testing.T) {
	t.Parallel()
	for remote, expected := range map[string]bool{
		"git@SODA.example.test:alice/site.git":       true,
		"ssh://git@soda.example.test/alice/site.git": true,
		"git@soda.example.test.:alice/site.git":      true,
		"git@localhost:alice/site.git":               true,
		"git@external.example.test:alice/site.git":   false,
	} {
		matched, err := remoteUsesBundledForgejo(remote, "soda.example.test")
		require.NoError(t, err)
		require.Equal(t, expected, matched, remote)
	}
}

func TestDerivedAccountConvention(t *testing.T) {
	t.Parallel()
	username, err := DerivedUsername("alice", "website")
	require.NoError(t, err)
	require.Equal(t, "soda-w-9bf62dc7a4af46c08d5730ed", username)
	marker, err := WorkspaceMarker("alice", "website")
	require.NoError(t, err)
	require.Equal(t, "soda-workspace=alice/website", marker)
	primary, project, err := ParseWorkspaceMarker(marker)
	require.NoError(t, err)
	require.Equal(t, "alice", primary)
	require.Equal(t, "website", project)
}

func TestAccountClassificationAndWorkspaceValidation(t *testing.T) {
	t.Parallel()
	primary := Account{Username: "alice", UID: 1000, Home: "/home/alice", Shell: "/bin/bash", Groups: map[string]bool{"wheel": true}}
	require.True(t, primary.IsPrimary(1000))
	require.True(t, primary.IsAdministrator(1000))
	primary.GECOS = "soda-workspace engineer"
	require.True(t, primary.IsPrimary(1000), "supplementary group membership is the derived-account discriminator")
	username, _ := DerivedUsername("alice", "website")
	marker, _ := WorkspaceMarker("alice", "website")
	workspace := Account{Username: username, UID: 1001, PrimaryGroup: username, Home: "/home/" + username, Shell: WorkspaceShell, GECOS: marker, Groups: map[string]bool{WorkspaceGroup: true}}
	require.NoError(t, workspace.ValidateWorkspace("alice", "website", 1000))
	workspace.Groups["wheel"] = true
	require.ErrorContains(t, workspace.ValidateWorkspace("alice", "website", 1000), "must not be an administrator")
}
