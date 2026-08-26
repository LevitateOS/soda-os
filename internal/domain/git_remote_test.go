package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateGitRemoteURLAcceptsCredentialSafeRemotes(t *testing.T) {
	t.Parallel()
	for _, remote := range []string{
		"https://example.com/team/project.git",
		"http://example.com/team/project.git",
		"ssh://git@example.com/team/project.git",
		"ssh://example.com/team/project.git",
		"git@example.com:team/project.git",
		"example.com:team/project.git",
		"git@[2001:db8::1]:team/project.git",
	} {
		remote := remote
		t.Run(remote, func(t *testing.T) {
			t.Parallel()
			require.NoError(t, ValidateGitRemoteURL(remote))
		})
	}
}

func TestValidateGitRemoteURLRejectsEmbeddedCredentials(t *testing.T) {
	t.Parallel()
	for _, remote := range []string{
		"https://user@example.com/team/project.git",
		"https://user:password@example.com/team/project.git",
		"http://token@example.com/team/project.git",
		"ssh://git:password@example.com/team/project.git",
		"user:password@example.com:team/project.git",
	} {
		remote := remote
		t.Run(remote, func(t *testing.T) {
			t.Parallel()
			require.Error(t, ValidateGitRemoteURL(remote))
		})
	}
}

func TestValidateGitRemoteURLRejectsUnsupportedOrMalformedRemotes(t *testing.T) {
	t.Parallel()
	for _, remote := range []string{
		"",
		"team/project.git",
		"file:///srv/git/project.git",
		"ftp://example.com/team/project.git",
		"ssh://@example.com/team/project.git",
		"git@example.com:",
		"git @example.com:team/project.git",
	} {
		remote := remote
		t.Run(remote, func(t *testing.T) {
			t.Parallel()
			require.Error(t, ValidateGitRemoteURL(remote))
		})
	}
}
