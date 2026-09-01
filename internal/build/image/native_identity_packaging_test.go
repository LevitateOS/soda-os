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
	contents, err := os.ReadFile(filepath.Join(root, "runtime", "sources", "sysusers", "soda.conf"))
	require.NoError(t, err)

	lines := nonCommentLines(string(contents))
	require.Equal(t, []string{"g soda-api 976"}, lines)
	require.NotContains(t, lines, "g soda-people -")
	require.NotContains(t, string(contents), "soda-cockpit")

	contents, err = os.ReadFile(filepath.Join(root, "projects", "sources", "sysusers", "soda-projects.conf"))
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
