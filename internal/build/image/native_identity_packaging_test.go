package image

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNativeIdentitySysusersContract(t *testing.T) {
	root := filepath.Join("..", "..", "..", "packaging", "rpm")
	_, err := os.Stat(filepath.Join(root, "runtime", "sources", "sysusers", "soda.conf"))
	require.ErrorIs(t, err, os.ErrNotExist)

	contents, err := os.ReadFile(filepath.Join(root, "projects", "sources", "sysusers", "soda-projects.conf"))
	require.NoError(t, err)
	require.Equal(t, []string{"g soda-workspaces -"}, nonCommentLines(string(contents)))
}

func TestForgejoPAMRejectsNonPrimaryAccounts(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", "..", "packaging", "rpm", "forgejo", "sources", "pam", "soda-forgejo"))
	require.NoError(t, err)

	lines := nonCommentLines(string(contents))
	require.Equal(t, []string{
		"auth include system-auth",
		"account requisite pam_usertype.so isregular",
		"account requisite pam_succeed_if.so quiet user notingroup soda-workspaces",
		"account include system-auth",
	}, lines)
}

func TestForgejoPAMHasOnlyTheDedicatedShadowReadBoundary(t *testing.T) {
	root := filepath.Join("..", "..", "..", "packaging", "rpm", "forgejo", "sources")

	sysusers, err := os.ReadFile(filepath.Join(root, "sysusers", "forgejo.conf"))
	require.NoError(t, err)
	require.Equal(t, []string{
		"g soda-forgejo-shadow -",
		"u git 975 \"Soda OS built-in Git\" /var/lib/forgejo /bin/sh",
	}, nonCommentLines(string(sysusers)))
	require.NotContains(t, string(sysusers), "m git")
	require.NotContains(t, string(sysusers), " shadow ")

	unit, err := os.ReadFile(filepath.Join(root, "systemd", "forgejo.service"))
	require.NoError(t, err)
	require.Contains(t, string(unit), "After=forgejo-init.service network.target systemd-tmpfiles-setup.service")
	require.Equal(t, 1, strings.Count(string(unit), "SupplementaryGroups=soda-forgejo-shadow"))

	tmpfiles, err := os.ReadFile(filepath.Join(root, "tmpfiles", "forgejo.conf"))
	require.NoError(t, err)
	require.Contains(t, nonCommentLines(string(tmpfiles)), "z /etc/shadow 0040 root soda-forgejo-shadow - -")
}

func nonCommentLines(contents string) []string {
	var lines []string
	for line := range strings.Lines(contents) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, strings.Join(strings.Fields(line), " "))
	}
	return lines
}
